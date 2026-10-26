package com.mmunki.vom;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.content.SharedPreferences;
import android.graphics.Color;
import android.graphics.Typeface;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowInsets;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.ScrollView;
import android.widget.Switch;
import android.widget.TextView;
import android.widget.Toast;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class MainActivity extends Activity {
    private static final String PREFS = "vom";
    private static final String KEY_URL = "server";
    private static final String KEY_HEADLESS = "headless";
    private static final int TAB_DEVICES = 0;
    private static final int TAB_CATALOG = 1;
    private static final int TAB_SETTINGS = 2;

    private final ExecutorService io = Executors.newFixedThreadPool(3);
    private final Handler main = new Handler(Looper.getMainLooper());
    private final int[] scrollPositions = new int[3];

    private SharedPreferences prefs;
    private Api api;
    private String baseUrl = "";

    private FrameLayout content;
    private LinearLayout navigation;
    private TextView subtitle;
    private int tab = TAB_DEVICES;
    private boolean paused;
    private volatile boolean refreshing;

    private ServerStatus status;
    private List<ReleaseInfo> releases = new ArrayList<>();
    private List<AvdInfo> avds = new ArrayList<>();
    private List<JobInfo> jobs = new ArrayList<>();
    private String lastError = "";

    private final Runnable poll = new Runnable() {
        @Override
        public void run() {
            if (paused) return;
            if (tab != TAB_SETTINGS) refresh(true);
            main.postDelayed(this, 5000);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        prefs = getSharedPreferences(PREFS, MODE_PRIVATE);
        String launchUrl = getIntent() == null ? null : getIntent().getStringExtra("baseUrl");
        if (launchUrl != null && !launchUrl.trim().isEmpty()) {
            baseUrl = Api.normalize(launchUrl);
            prefs.edit().putString(KEY_URL, baseUrl).apply();
        } else {
            baseUrl = prefs.getString(KEY_URL, "");
        }
        if (!baseUrl.isEmpty()) api = new Api(baseUrl);

        Window window = getWindow();
        window.setStatusBarColor(Ui.color(this, R.color.background));
        window.setNavigationBarColor(Ui.color(this, R.color.background));
        Ui.setLightStatusBar(window, !Ui.isNight(this));

        LinearLayout root = Ui.column(this);
        root.setBackgroundColor(Ui.color(this, R.color.background));
        root.setFitsSystemWindows(false);
        if (Build.VERSION.SDK_INT >= 30) {
            window.setDecorFitsSystemWindows(false);
        }
        root.setOnApplyWindowInsetsListener(new View.OnApplyWindowInsetsListener() {
            @Override
            public WindowInsets onApplyWindowInsets(View v, WindowInsets insets) {
                int top;
                int bottom;
                if (Build.VERSION.SDK_INT >= 30) {
                    android.graphics.Insets i = insets.getInsets(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                    top = i.top;
                    bottom = i.bottom;
                } else {
                    top = insets.getSystemWindowInsetTop();
                    bottom = insets.getSystemWindowInsetBottom();
                }
                v.setPadding(0, top, 0, bottom);
                return insets;
            }
        });

        root.addView(buildTopBar());
        content = new FrameLayout(this);
        root.addView(content, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, 0, 1f));
        navigation = Ui.row(this);
        navigation.setBackgroundColor(Ui.color(this, R.color.surface));
        navigation.setElevation(Ui.dp(this, 3));
        root.addView(navigation, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, Ui.dp(this, 72)));

        setContentView(root);
        buildNavigation();
        if (api == null) {
            showConnect("");
        } else {
            showLoading();
            refresh(false);
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        paused = false;
        main.removeCallbacks(poll);
        main.postDelayed(poll, 5000);
    }

    @Override
    protected void onPause() {
        paused = true;
        main.removeCallbacks(poll);
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        io.shutdownNow();
        super.onDestroy();
    }

    private View buildTopBar() {
        LinearLayout bar = Ui.row(this);
        bar.setPadding(Ui.dp(this, 20), Ui.dp(this, 14), Ui.dp(this, 12), Ui.dp(this, 10));
        LinearLayout titles = Ui.column(this);
        TextView title = Ui.text(this, "VOM", 27, Ui.color(this, R.color.primary), true);
        subtitle = Ui.text(this, "Virtual Android", 12, Ui.color(this, R.color.onSurfaceVariant));
        titles.addView(title);
        titles.addView(subtitle);
        bar.addView(titles, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        ImageView refresh = Ui.icon(this, R.drawable.ic_refresh, Ui.color(this, R.color.onSurfaceVariant), 24);
        FrameLayout wrap = new FrameLayout(this);
        wrap.setPadding(Ui.dp(this, 12), Ui.dp(this, 12), Ui.dp(this, 12), Ui.dp(this, 12));
        wrap.addView(refresh);
        wrap.setClickable(true);
        wrap.setBackground(new android.graphics.drawable.RippleDrawable(
                android.content.res.ColorStateList.valueOf(Ui.withAlpha(Ui.color(this, R.color.primary), 40)), null, null));
        wrap.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                refresh(false);
            }
        });
        bar.addView(wrap, new LinearLayout.LayoutParams(Ui.dp(this, 52), Ui.dp(this, 52)));
        return bar;
    }

    private void buildNavigation() {
        navigation.removeAllViews();
        navigation.addView(navItem(TAB_DEVICES, R.drawable.ic_devices, getString(R.string.devices)),
                new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        navigation.addView(navItem(TAB_CATALOG, R.drawable.ic_catalog, getString(R.string.catalog)),
                new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        navigation.addView(navItem(TAB_SETTINGS, R.drawable.ic_settings, getString(R.string.settings)),
                new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
    }

    private View navItem(final int which, int iconRes, String label) {
        boolean selected = tab == which;
        int tint = Ui.color(this, selected ? R.color.primary : R.color.onSurfaceVariant);
        LinearLayout item = Ui.column(this);
        item.setGravity(Gravity.CENTER);
        item.setClickable(true);
        item.setBackground(new android.graphics.drawable.RippleDrawable(
                android.content.res.ColorStateList.valueOf(Ui.withAlpha(tint, 36)), null, null));
        ImageView icon = Ui.icon(this, iconRes, tint, 23);
        item.addView(icon);
        TextView text = Ui.text(this, label, 11, tint, selected);
        item.addView(text);
        item.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (tab != which) {
                    tab = which;
                    buildNavigation();
                    render();
                }
            }
        });
        return item;
    }

    private void selectTab(int which) {
        tab = which;
        buildNavigation();
        render();
    }

    private void showLoading() {
        LinearLayout box = Ui.column(this);
        box.setGravity(Gravity.CENTER);
        ProgressBar p = new ProgressBar(this);
        box.addView(p);
        TextView t = Ui.text(this, "Connecting to the VOM host...", 14, Ui.color(this, R.color.onSurfaceVariant));
        Ui.margins(t, 0, 16, 0, 0);
        box.addView(t);
        content.removeAllViews();
        content.addView(box, Ui.match());
    }

    private void showConnect(String error) {
        if (subtitle != null) subtitle.setText("Virtual Android");
        ScrollView scroll = Ui.scroll(this);
        LinearLayout body = Ui.column(this);
        body.setPadding(Ui.dp(this, 24), Ui.dp(this, 8), Ui.dp(this, 24), Ui.dp(this, 32));

        ImageView logo = new ImageView(this);
        logo.setImageResource(R.mipmap.ic_launcher);
        LinearLayout.LayoutParams logoLp = new LinearLayout.LayoutParams(Ui.dp(this, 88), Ui.dp(this, 88));
        logoLp.gravity = Gravity.CENTER_HORIZONTAL;
        body.addView(logo, logoLp);

        TextView title = Ui.text(this, getString(R.string.connect_title), 26, Ui.color(this, R.color.onBackground), true);
        title.setGravity(Gravity.CENTER_HORIZONTAL);
        Ui.margins(title, 0, 20, 0, 8);
        body.addView(title);

        TextView intro = Ui.text(this,
                "Enter the address of the VOM host on your network. The Android system images and the emulator run there; this app is the phone display and remote control.",
                14, Ui.color(this, R.color.onSurfaceVariant));
        intro.setGravity(Gravity.CENTER_HORIZONTAL);
        body.addView(intro);

        if (!error.isEmpty()) {
            TextView err = Ui.text(this, error, 13, Ui.color(this, R.color.error), true);
            err.setGravity(Gravity.CENTER_HORIZONTAL);
            Ui.margins(err, 0, 14, 0, 0);
            body.addView(err);
        }

        LinearLayout card = Ui.card(this);
        Ui.margins(card, 0, 22, 0, 0);
        TextView label = Ui.text(this, "HOST ADDRESS", 11, Ui.color(this, R.color.onSurfaceVariant), true);
        label.setLetterSpacing(0.08f);
        card.addView(label);
        final EditText input = new EditText(this);
        input.setHint(getString(R.string.connect_hint));
        input.setText(baseUrl);
        input.setSingleLine(true);
        input.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        input.setTextSize(16);
        input.setTextColor(Ui.color(this, R.color.onSurface));
        input.setHintTextColor(Ui.color(this, R.color.outline));
        input.setBackgroundTintList(android.content.res.ColorStateList.valueOf(Ui.color(this, R.color.primary)));
        Ui.margins(input, 0, 8, 0, 8);
        card.addView(input);

        TextView connect = Ui.button(this, getString(R.string.connect), true);
        connect.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                String server = Api.normalize(input.getText().toString());
                if (server.isEmpty()) {
                    input.setError("Enter the host address");
                    return;
                }
                baseUrl = server;
                prefs.edit().putString(KEY_URL, baseUrl).apply();
                api = new Api(baseUrl);
                showLoading();
                refresh(false);
            }
        });
        card.addView(connect, Ui.matchWrap());
        body.addView(card);

        LinearLayout help = Ui.card(this);
        Ui.margins(help, 0, 14, 0, 0);
        TextView helpTitle = Ui.text(this, "On the host", 15, Ui.color(this, R.color.onSurface), true);
        help.addView(helpTitle);
        TextView code = Ui.text(this, "vom serve --listen 0.0.0.0:8765", 13, Ui.color(this, R.color.primary));
        code.setTypeface(Typeface.create("monospace", Typeface.NORMAL));
        code.setTextIsSelectable(true);
        Ui.margins(code, 0, 8, 0, 8);
        help.addView(code);
        help.addView(Ui.text(this,
                "The host needs KVM on Linux, Hypervisor.framework on macOS or Windows Hypervisor Platform on Windows. A phone browser can also open the same page.",
                13, Ui.color(this, R.color.onSurfaceVariant)));
        body.addView(help);

        TextView why = Ui.text(this,
                "Why not a phone-only hypervisor? Android does not give a normal APK access to KVM or AVF, and Google's Android 7-17 system images need the emulator's virtual hardware. VOM uses Google's own emulator on a computer and streams the screen here.",
                12, Ui.color(this, R.color.onSurfaceVariant));
        Ui.margins(why, 4, 18, 4, 0);
        body.addView(why);

        scroll.addView(body);
        content.removeAllViews();
        content.addView(scroll, Ui.match());
    }

    private void refresh(final boolean silent) {
        if (api == null || refreshing) return;
        refreshing = true;
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    final ServerStatus s = current.status();
                    final List<ReleaseInfo> c = current.catalog();
                    final List<AvdInfo> a = current.avds();
                    final List<JobInfo> j = current.jobs();
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refreshing = false;
                            status = s;
                            releases = c;
                            avds = a;
                            jobs = j;
                            lastError = "";
                            if (subtitle != null) {
                                subtitle.setText("Android 7-17 · " + (s.host.isEmpty() ? "VOM host" : s.host));
                            }
                            render();
                        }
                    });
                } catch (final Exception e) {
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refreshing = false;
                            lastError = e.getMessage() == null ? e.toString() : e.getMessage();
                            if (silent) {
                                if (status == null) showConnect(lastError);
                                else toast(lastError);
                            } else {
                                showConnect(lastError);
                            }
                        }
                    });
                }
            }
        });
    }

    private void render() {
        if (api == null || status == null) {
            if (!lastError.isEmpty()) showConnect(lastError);
            return;
        }
        if (tab == TAB_DEVICES) renderDevices();
        else if (tab == TAB_CATALOG) renderCatalog();
        else renderSettings();
    }

    private void replace(ScrollView scroll, LinearLayout body) {
        scroll.addView(body);
        content.removeAllViews();
        content.addView(scroll, Ui.match());
        scroll.post(new Runnable() {
            @Override
            public void run() {
                scroll.scrollTo(0, scrollPositions[tab]);
            }
        });
        scroll.setOnScrollChangeListener(new View.OnScrollChangeListener() {
            @Override
            public void onScrollChange(View v, int x, int y, int oldX, int oldY) {
                scrollPositions[tab] = y;
            }
        });
    }

    private ScrollView page() {
        ScrollView scroll = Ui.scroll(this);
        LinearLayout body = Ui.column(this);
        body.setPadding(Ui.dp(this, 20), Ui.dp(this, 8), Ui.dp(this, 20), Ui.dp(this, 28));
        replace(scroll, body);
        return scroll;
    }

    private LinearLayout pageBody(ScrollView scroll) {
        return (LinearLayout) scroll.getChildAt(0);
    }

    private void renderDevices() {
        ScrollView scroll = Ui.scroll(this);
        LinearLayout body = Ui.column(this);
        body.setPadding(Ui.dp(this, 20), Ui.dp(this, 8), Ui.dp(this, 20), Ui.dp(this, 28));
        replace(scroll, body);

        body.addView(sectionTitle("Your devices", "Android 7.0 through 17 on the VOM host"));
        body.addView(statusCard());

        if (!status.emulator || !status.platformTools) {
            LinearLayout setup = Ui.card(this);
            Ui.margins(setup, 0, 12, 0, 0);
            setup.addView(Ui.text(this, "Runtime is not ready", 16, Ui.color(this, R.color.onSurface), true));
            setup.addView(Ui.text(this,
                    "Install Google's emulator and platform-tools on the host. This is a one-time download of a few hundred megabytes.",
                    13, Ui.color(this, R.color.onSurfaceVariant)));
            TextView button = Ui.button(this, "Install runtime", true);
            Ui.margins(button, 0, 12, 0, 0);
            button.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    doSetup(false);
                }
            });
            setup.addView(button, Ui.matchWrap());
            body.addView(setup);
        }

        if (!jobs.isEmpty()) {
            boolean anyRunning = false;
            for (JobInfo j : jobs) if (j.running()) anyRunning = true;
            if (anyRunning) body.addView(jobsCard());
        }

        TextView heading = Ui.text(this, "Virtual devices", 17, Ui.color(this, R.color.onBackground), true);
        Ui.margins(heading, 2, 22, 0, 10);
        body.addView(heading);

        if (avds.isEmpty()) {
            LinearLayout empty = Ui.card(this);
            empty.addView(Ui.text(this, "No devices yet", 16, Ui.color(this, R.color.onSurface), true));
            empty.addView(Ui.text(this,
                    "Pick an Android version from the catalog, download the official image and create a device. Everything stays on your VOM host.",
                    13, Ui.color(this, R.color.onSurfaceVariant)));
            TextView browse = Ui.button(this, "Browse Android 7-17", true);
            Ui.margins(browse, 0, 12, 0, 0);
            browse.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    selectTab(TAB_CATALOG);
                }
            });
            empty.addView(browse, Ui.matchWrap());
            body.addView(empty);
        } else {
            for (final AvdInfo avd : avds) {
                body.addView(avdCard(avd), Ui.matchWrap());
            }
        }
    }

    private View statusCard() {
        LinearLayout card = Ui.card(this);
        LinearLayout top = Ui.row(this);
        ImageView icon = Ui.icon(this, R.drawable.ic_server, Ui.color(this, R.color.primary), 26);
        top.addView(icon);
        LinearLayout host = Ui.column(this);
        Ui.margins(host, 12, 0, 0, 0);
        host.addView(Ui.text(this, status.host.isEmpty() ? "VOM host" : status.host, 16, Ui.color(this, R.color.onSurface), true));
        host.addView(Ui.text(this, status.os + "/" + status.arch + " · v" + status.version, 12, Ui.color(this, R.color.onSurfaceVariant)));
        top.addView(host, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        top.addView(Ui.pill(this, "ONLINE", Ui.color(this, R.color.successContainer), Ui.color(this, R.color.success)));
        card.addView(top);

        card.addView(Ui.divider(this));
        LinearLayout stats = Ui.row(this);
        stats.addView(stat("ANDROID IMAGES", String.valueOf(status.images)), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        stats.addView(stat("INSTALLED", String.valueOf(status.installedImages)), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        stats.addView(stat("DEVICES", String.valueOf(status.avds)), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Ui.margins(stats, 0, 12, 0, 4);
        card.addView(stats);

        String runtime = status.emulator && status.platformTools ? "Emulator ready" : "Runtime setup needed";
        card.addView(infoRow("Runtime", runtime,
                status.emulator && status.platformTools ? Ui.color(this, R.color.success) : Ui.color(this, R.color.warning)));
        String accel = status.acceleration.isEmpty() ? "unknown" : status.acceleration;
        card.addView(infoRow("Acceleration", accel + (status.accelOk ? " · usable" : " · not available"),
                status.accelOk ? Ui.color(this, R.color.success) : Ui.color(this, R.color.warning)));
        card.addView(infoRow("Catalog", status.catalogSource + " · " + status.recommendedAbi,
                Ui.color(this, R.color.onSurfaceVariant)));

        LinearLayout actions = Ui.row(this);
        TextView refresh = Ui.button(this, "Refresh", false);
        refresh.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                refresh(false);
            }
        });
        actions.addView(refresh, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        TextView setup = Ui.button(this, status.emulator && status.platformTools ? "Check runtime" : "Install runtime", true);
        Ui.margins(setup, 10, 0, 0, 0);
        setup.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                doSetup(!status.emulator || !status.platformTools);
            }
        });
        actions.addView(setup, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Ui.margins(actions, 0, 14, 0, 0);
        card.addView(actions);
        return card;
    }

    private View stat(String label, String value) {
        LinearLayout c = Ui.column(this);
        c.addView(Ui.text(this, value, 21, Ui.color(this, R.color.onSurface), true));
        TextView l = Ui.text(this, label, 9, Ui.color(this, R.color.onSurfaceVariant), true);
        l.setLetterSpacing(0.06f);
        c.addView(l);
        return c;
    }

    private View infoRow(String label, String value, int valueColor) {
        LinearLayout row = Ui.row(this);
        row.setPadding(0, Ui.dp(this, 6), 0, Ui.dp(this, 2));
        row.addView(Ui.text(this, label, 13, Ui.color(this, R.color.onSurfaceVariant)), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        TextView v = Ui.text(this, value, 13, valueColor, true);
        v.setGravity(Gravity.END);
        row.addView(v);
        return row;
    }

    private View avdCard(final AvdInfo avd) {
        LinearLayout card = Ui.card(this);
        Ui.margins(card, 0, 0, 0, 12);
        LinearLayout top = Ui.row(this);
        LinearLayout names = Ui.column(this);
        names.addView(Ui.text(this, avd.name, 16, Ui.color(this, R.color.onSurface), true));
        String detail = "Android " + avd.version + " · " + tagName(avd.tag) + " · " + avd.abi;
        names.addView(Ui.text(this, detail, 12, Ui.color(this, R.color.onSurfaceVariant)));
        if (!avd.serial.isEmpty()) {
            names.addView(Ui.text(this, avd.serial, 11, Ui.color(this, R.color.outline)));
        }
        top.addView(names, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        top.addView(Ui.pill(this, avd.running ? "RUNNING" : "STOPPED",
                Ui.color(this, avd.running ? R.color.successContainer : R.color.surfaceVariant),
                Ui.color(this, avd.running ? R.color.success : R.color.onSurfaceVariant)));
        card.addView(top);

        LinearLayout actions = Ui.row(this);
        Ui.margins(actions, 0, 14, 0, 0);
        TextView power = Ui.button(this, avd.running ? "Stop" : "Start", !avd.running);
        power.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (avd.running) stopAvd(avd); else startAvd(avd);
            }
        });
        actions.addView(power, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        TextView screen = Ui.button(this, "Screen", false);
        Ui.margins(screen, 8, 0, 0, 0);
        screen.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                if (!avd.running) {
                    toast("Start the device first");
                    return;
                }
                openScreen(avd);
            }
        });
        actions.addView(screen, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        ImageView log = Ui.icon(this, R.drawable.ic_log, Ui.color(this, R.color.onSurfaceVariant), 22);
        FrameLayout logWrap = iconButton(log, "Log");
        logWrap.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                showLog(avd);
            }
        });
        Ui.margins(logWrap, 8, 0, 0, 0);
        actions.addView(logWrap, new LinearLayout.LayoutParams(Ui.dp(this, 42), Ui.dp(this, 42)));

        ImageView trash = Ui.icon(this, R.drawable.ic_trash, Ui.color(this, R.color.error), 21);
        FrameLayout delWrap = iconButton(trash, "Delete");
        delWrap.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                confirmDelete(avd);
            }
        });
        Ui.margins(delWrap, 6, 0, 0, 0);
        actions.addView(delWrap, new LinearLayout.LayoutParams(Ui.dp(this, 42), Ui.dp(this, 42)));
        card.addView(actions);
        return card;
    }

    private FrameLayout iconButton(ImageView icon, String description) {
        FrameLayout wrap = new FrameLayout(this);
        wrap.setContentDescription(description);
        wrap.setClickable(true);
        wrap.setBackground(new android.graphics.drawable.RippleDrawable(
                android.content.res.ColorStateList.valueOf(Ui.withAlpha(Ui.color(this, R.color.primary), 36)), null, null));
        FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(Ui.dp(this, 22), Ui.dp(this, 22));
        lp.gravity = Gravity.CENTER;
        wrap.addView(icon, lp);
        return wrap;
    }

    private View jobsCard() {
        LinearLayout card = Ui.card(this);
        Ui.margins(card, 0, 12, 0, 0);
        card.addView(Ui.text(this, "Working", 15, Ui.color(this, R.color.onSurface), true));
        for (final JobInfo job : jobs) {
            if (!job.running()) continue;
            LinearLayout box = Ui.column(this);
            Ui.margins(box, 0, 10, 0, 0);
            LinearLayout line = Ui.row(this);
            line.addView(Ui.text(this, job.label, 13, Ui.color(this, R.color.onSurface), true), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
            TextView cancel = Ui.textButton(this, "Cancel");
            cancel.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    cancelJob(job);
                }
            });
            line.addView(cancel);
            box.addView(line);
            String stage = job.stage;
            int pct = job.percent();
            if (pct >= 0) stage += " · " + pct + "%";
            box.addView(Ui.text(this, stage, 11, Ui.color(this, R.color.onSurfaceVariant)));
            ProgressBar bar = Ui.progress(this);
            if (pct >= 0) {
                bar.setIndeterminate(false);
                bar.setProgress(pct * 10);
            } else {
                bar.setIndeterminate(true);
            }
            Ui.margins(bar, 0, 6, 0, 0);
            box.addView(bar);
            card.addView(box);
        }
        return card;
    }

    private void renderCatalog() {
        ScrollView scroll = Ui.scroll(this);
        LinearLayout body = Ui.column(this);
        body.setPadding(Ui.dp(this, 20), Ui.dp(this, 8), Ui.dp(this, 20), Ui.dp(this, 28));
        replace(scroll, body);

        LinearLayout header = Ui.row(this);
        LinearLayout texts = Ui.column(this);
        texts.addView(Ui.text(this, "Android 7.0 – 17", 22, Ui.color(this, R.color.onBackground), true));
        texts.addView(Ui.text(this, releases.size() + " releases · official Google images · " + status.recommendedAbi + " recommended",
                12, Ui.color(this, R.color.onSurfaceVariant)));
        header.addView(texts, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        TextView refresh = Ui.textButton(this, "Refresh");
        refresh.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                refreshCatalog();
            }
        });
        header.addView(refresh);
        body.addView(header);

        if (releases.isEmpty()) {
            LinearLayout empty = Ui.card(this);
            Ui.margins(empty, 0, 16, 0, 0);
            empty.addView(Ui.text(this, "No catalog available", 16, Ui.color(this, R.color.onSurface), true));
            empty.addView(Ui.text(this, "Refresh the catalog from Google on the host.", 13, Ui.color(this, R.color.onSurfaceVariant)));
            body.addView(empty);
            return;
        }

        for (ReleaseInfo release : releases) {
            LinearLayout title = Ui.column(this);
            Ui.margins(title, 2, 20, 0, 8);
            LinearLayout line = Ui.row(this);
            line.addView(Ui.text(this, "Android " + release.version, 18, Ui.color(this, R.color.onBackground), true));
            if (!release.codename.isEmpty()) {
                TextView code = Ui.pill(this, release.codename.toUpperCase(), Ui.color(this, R.color.secondaryContainer), Ui.color(this, R.color.onSecondaryContainer));
                Ui.margins(code, 8, 0, 0, 0);
                line.addView(code);
            }
            title.addView(line);
            title.addView(Ui.text(this, "API " + release.api, 11, Ui.color(this, R.color.onSurfaceVariant)));
            body.addView(title);

            LinearLayout card = Ui.card(this);
            Ui.margins(card, 0, 0, 0, 10);
            for (int i = 0; i < release.images.size(); i++) {
                ImageInfo image = release.images.get(i);
                if (i > 0) {
                    View divider = Ui.divider(this);
                    Ui.margins(divider, 0, 10, 0, 10);
                    card.addView(divider);
                }
                card.addView(imageRow(image));
            }
            body.addView(card);
        }
    }

    private View imageRow(final ImageInfo image) {
        LinearLayout row = Ui.row(this);
        LinearLayout text = Ui.column(this);
        LinearLayout name = Ui.row(this);
        name.addView(Ui.text(this, tagName(image.tag), 14, Ui.color(this, R.color.onSurface), true));
        if (image.recommended) {
            TextView rec = Ui.pill(this, "RECOMMENDED", Ui.color(this, R.color.primaryContainer), Ui.color(this, R.color.onPrimaryContainer));
            Ui.margins(rec, 8, 0, 0, 0);
            name.addView(rec);
        }
        text.addView(name);
        text.addView(Ui.text(this, image.abi + " · " + Api.humanBytes(image.size) + " · r" + image.revision,
                12, Ui.color(this, R.color.onSurfaceVariant)));
        row.addView(text, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        JobInfo runningJob = jobFor(image.id);
        if (image.installed) {
            TextView create = Ui.button(this, "Create", true);
            create.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    createAndStart(image);
                }
            });
            row.addView(create);
        } else if (runningJob != null) {
            int pct = runningJob.percent();
            TextView busy = Ui.pill(this, pct >= 0 ? "DOWNLOADING " + pct + "%" : "DOWNLOADING",
                    Ui.color(this, R.color.primaryContainer), Ui.color(this, R.color.onPrimaryContainer));
            row.addView(busy);
        } else {
            TextView install = Ui.button(this, "Install", false);
            install.setOnClickListener(new View.OnClickListener() {
                @Override
                public void onClick(View v) {
                    confirmInstall(image);
                }
            });
            row.addView(install);
        }
        return row;
    }

    private JobInfo jobFor(String imageId) {
        for (JobInfo job : jobs) {
            if (!job.running()) continue;
            if (job.label.contains("·") && job.label.contains(imageId)) return job;
            for (ReleaseInfo r : releases) {
                for (ImageInfo i : r.images) {
                    if (!i.id.equals(imageId)) continue;
                    if (job.label.contains("Android " + i.version) && job.label.contains(tagName(i.tag)) && job.label.contains(i.abi)) {
                        return job;
                    }
                }
            }
        }
        return null;
    }

    private void renderSettings() {
        ScrollView scroll = Ui.scroll(this);
        LinearLayout body = Ui.column(this);
        body.setPadding(Ui.dp(this, 20), Ui.dp(this, 8), Ui.dp(this, 20), Ui.dp(this, 28));
        replace(scroll, body);

        body.addView(sectionTitle("Settings", "Connection, runtime and about"));

        LinearLayout connection = Ui.card(this);
        connection.addView(Ui.text(this, "Connection", 16, Ui.color(this, R.color.onSurface), true));
        final EditText input = new EditText(this);
        input.setText(baseUrl);
        input.setSingleLine(true);
        input.setHint(getString(R.string.connect_hint));
        input.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        input.setTextSize(15);
        input.setTextColor(Ui.color(this, R.color.onSurface));
        input.setHintTextColor(Ui.color(this, R.color.outline));
        input.setBackgroundTintList(android.content.res.ColorStateList.valueOf(Ui.color(this, R.color.primary)));
        Ui.margins(input, 0, 10, 0, 6);
        connection.addView(input);
        LinearLayout buttons = Ui.row(this);
        TextView apply = Ui.button(this, "Reconnect", true);
        apply.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                String server = Api.normalize(input.getText().toString());
                if (server.isEmpty()) {
                    input.setError("Enter the host address");
                    return;
                }
                baseUrl = server;
                prefs.edit().putString(KEY_URL, baseUrl).apply();
                api = new Api(baseUrl);
                tab = TAB_DEVICES;
                buildNavigation();
                showLoading();
                refresh(false);
            }
        });
        buttons.addView(apply, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        TextView disconnect = Ui.button(this, "Disconnect", false);
        Ui.margins(disconnect, 8, 0, 0, 0);
        disconnect.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                prefs.edit().remove(KEY_URL).apply();
                baseUrl = "";
                api = null;
                status = null;
                releases = new ArrayList<>();
                avds = new ArrayList<>();
                jobs = new ArrayList<>();
                showConnect("");
            }
        });
        buttons.addView(disconnect, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        Ui.margins(buttons, 0, 8, 0, 0);
        connection.addView(buttons);
        body.addView(connection);

        LinearLayout runtime = Ui.card(this);
        Ui.margins(runtime, 0, 12, 0, 0);
        runtime.addView(Ui.text(this, "Runtime", 16, Ui.color(this, R.color.onSurface), true));
        LinearLayout headlessRow = Ui.row(this);
        LinearLayout headlessText = Ui.column(this);
        headlessText.addView(Ui.text(this, "Start devices headless", 14, Ui.color(this, R.color.onSurface)));
        headlessText.addView(Ui.text(this, "Recommended: the phone is the display. Turn off to show the emulator window on the host too.",
                11, Ui.color(this, R.color.onSurfaceVariant)));
        headlessRow.addView(headlessText, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));
        final Switch headless = new Switch(this);
        headless.setChecked(prefs.getBoolean(KEY_HEADLESS, true));
        headless.setOnCheckedChangeListener(new android.widget.CompoundButton.OnCheckedChangeListener() {
            @Override
            public void onCheckedChanged(android.widget.CompoundButton buttonView, boolean isChecked) {
                prefs.edit().putBoolean(KEY_HEADLESS, isChecked).apply();
            }
        });
        headlessRow.addView(headless);
        Ui.margins(headlessRow, 0, 10, 0, 0);
        runtime.addView(headlessRow);
        TextView refresh = Ui.button(this, "Refresh catalog from Google", false);
        Ui.margins(refresh, 0, 12, 0, 0);
        refresh.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                refreshCatalog();
            }
        });
        runtime.addView(refresh, Ui.matchWrap());
        body.addView(runtime);

        LinearLayout how = Ui.card(this);
        Ui.margins(how, 0, 12, 0, 0);
        how.addView(Ui.text(this, "How it works", 16, Ui.color(this, R.color.onSurface), true));
        how.addView(Ui.text(this,
                "VOM Mobile is the phone client. The VOM host downloads Google's official system image, creates a standard AVD and runs Google's emulator. The app receives screenshots over your network and sends taps, swipes and keys back through adb.",
                13, Ui.color(this, R.color.onSurfaceVariant)));
        how.addView(Ui.text(this,
                "A normal APK cannot boot a full Android release inside itself: Android does not expose KVM or AVF to third-party apps, and the official images target the emulator's virtual hardware. Anyone claiming a phone-only APK runs Android 7-17 is either using root with a vendor hypervisor, or showing a remote screen.",
                12, Ui.color(this, R.color.onSurfaceVariant)));
        body.addView(how);

        LinearLayout about = Ui.card(this);
        Ui.margins(about, 0, 12, 0, 0);
        about.addView(Ui.text(this, "About", 16, Ui.color(this, R.color.onSurface), true));
        about.addView(infoRow("Version", getString(R.string.app_version), Ui.color(this, R.color.onSurfaceVariant)));
        about.addView(infoRow("Project", "github.com/mmunki/vom", Ui.color(this, R.color.primary)));
        TextView open = Ui.button(this, "Open project page", false);
        Ui.margins(open, 0, 12, 0, 0);
        open.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                try {
                    startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse("https://github.com/mmunki/vom")));
                } catch (Exception e) {
                    toast(e.getMessage());
                }
            }
        });
        about.addView(open, Ui.matchWrap());
        body.addView(about);
    }

    private LinearLayout sectionTitle(String title, String sub) {
        LinearLayout box = Ui.column(this);
        box.addView(Ui.text(this, title, 22, Ui.color(this, R.color.onBackground), true));
        box.addView(Ui.text(this, sub, 12, Ui.color(this, R.color.onSurfaceVariant)));
        Ui.margins(box, 2, 0, 0, 12);
        return box;
    }

    private String tagName(String tag) {
        if ("default".equals(tag)) return "AOSP";
        if ("google_apis".equals(tag)) return "Google APIs";
        if ("google_apis_playstore".equals(tag)) return "Google Play";
        return tag;
    }

    private void doSetup(boolean force) {
        final Api current = api;
        toast("Starting runtime setup on the host");
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.setup(force);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void refreshCatalog() {
        final Api current = api;
        toast("Refreshing the catalog from Google");
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.refreshCatalog();
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void confirmInstall(final ImageInfo image) {
        new AlertDialog.Builder(this)
                .setTitle("Download Android " + image.version + "?")
                .setMessage(tagName(image.tag) + " · " + image.abi + "\n" + Api.humanBytes(image.size)
                        + "\n\nThe download runs on the VOM host and is checked against Google's SHA-1.")
                .setNegativeButton("Cancel", null)
                .setPositiveButton("Download", new android.content.DialogInterface.OnClickListener() {
                    @Override
                    public void onClick(android.content.DialogInterface dialog, int which) {
                        installImage(image);
                    }
                })
                .show();
    }

    private void installImage(final ImageInfo image) {
        final Api current = api;
        toast("Downloading Android " + image.version + " on the host");
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.install(image.id);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void createAndStart(final ImageInfo image) {
        final Api current = api;
        toast("Creating Android " + image.version + " device");
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    AvdInfo created;
                    try {
                        created = current.createAvd(image.id, null, 0);
                    } catch (Api.ApiException e) {
                        if (e.code != 400 || e.getMessage() == null || !e.getMessage().contains("already exists")) throw e;
                        created = null;
                        for (AvdInfo a : current.avds()) {
                            if (image.id.equals(a.imageId)) {
                                created = a;
                                break;
                            }
                        }
                        if (created == null) throw e;
                    }
                    final AvdInfo started = current.start(created.name, prefs.getBoolean(KEY_HEADLESS, true), false);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                            openScreen(started);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void startAvd(final AvdInfo avd) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    final AvdInfo started = current.start(avd.name, prefs.getBoolean(KEY_HEADLESS, true), false);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                            openScreen(started);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void stopAvd(final AvdInfo avd) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.stop(avd.name);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            toast(avd.name + " stopped");
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void confirmDelete(final AvdInfo avd) {
        new AlertDialog.Builder(this)
                .setTitle("Delete " + avd.name + "?")
                .setMessage("This removes the AVD and its user data. The downloaded system image stays installed.")
                .setNegativeButton("Cancel", null)
                .setPositiveButton("Delete", new android.content.DialogInterface.OnClickListener() {
                    @Override
                    public void onClick(android.content.DialogInterface dialog, int which) {
                        deleteAvd(avd);
                    }
                })
                .show();
    }

    private void deleteAvd(final AvdInfo avd) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.delete(avd.name);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void cancelJob(final JobInfo job) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.cancelJob(job.id);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            refresh(false);
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void showLog(final AvdInfo avd) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    final String log = current.log(avd.name);
                    main.post(new Runnable() {
                        @Override
                        public void run() {
                            TextView text = new TextView(MainActivity.this);
                            text.setText(log.isEmpty() ? "(the log is empty)" : log);
                            text.setTypeface(Typeface.create("monospace", Typeface.NORMAL));
                            text.setTextSize(10);
                            text.setTextColor(Ui.color(MainActivity.this, R.color.onSurface));
                            text.setTextIsSelectable(true);
                            ScrollView scroll = Ui.scroll(MainActivity.this);
                            scroll.addView(text);
                            int pad = Ui.dp(MainActivity.this, 18);
                            scroll.setPadding(pad, pad, pad, pad);
                            new AlertDialog.Builder(MainActivity.this)
                                    .setTitle(avd.name)
                                    .setView(scroll)
                                    .setPositiveButton("Close", null)
                                    .show();
                        }
                    });
                } catch (final Exception e) {
                    fail(e);
                }
            }
        });
    }

    private void openScreen(AvdInfo avd) {
        if (avd == null) return;
        if (avd.serial == null || avd.serial.isEmpty()) {
            toast("Waiting for the device; tap Screen again in a moment");
            return;
        }
        Intent intent = new Intent(this, ScreenActivity.class);
        intent.putExtra("baseUrl", baseUrl);
        intent.putExtra("name", avd.name);
        intent.putExtra("width", avd.width);
        intent.putExtra("height", avd.height);
        startActivity(intent);
    }

    private void fail(final Exception e) {
        main.post(new Runnable() {
            @Override
            public void run() {
                toast(e.getMessage() == null ? e.toString() : e.getMessage());
            }
        });
    }

    private void toast(String message) {
        Toast.makeText(this, message, Toast.LENGTH_LONG).show();
    }
}
