package com.mmunki.vom;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.DialogInterface;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Color;
import android.graphics.Matrix;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class ScreenActivity extends Activity {
    private final ExecutorService io = Executors.newFixedThreadPool(2);
    private final Handler main = new Handler(Looper.getMainLooper());

    private Api api;
    private String name = "";
    private int deviceWidth = 1080;
    private int deviceHeight = 1920;

    private ImageView screen;
    private TextView status;
    private Bitmap bitmap;
    private int bitmapWidth;
    private volatile boolean polling;
    private volatile boolean inFlight;

    private float downX;
    private float downY;
    private long downTime;

    private final Runnable capture = new Runnable() {
        @Override
        public void run() {
            if (!polling) return;
            if (!inFlight) {
                inFlight = true;
                final Api current = api;
                final String avd = name;
                io.execute(new Runnable() {
                    @Override
                    public void run() {
                        try {
                            final Api.Bytes frame = current.screenshot(avd, 540);
                            main.post(new Runnable() {
                                @Override
                                public void run() {
                                    inFlight = false;
                                    draw(frame);
                                }
                            });
                        } catch (final Exception e) {
                            main.post(new Runnable() {
                                @Override
                                public void run() {
                                    inFlight = false;
                                    status.setText("Waiting for Android to boot…");
                                }
                            });
                        }
                    }
                });
            }
            main.postDelayed(this, 550);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        name = getIntent().getStringExtra("name");
        String base = getIntent().getStringExtra("baseUrl");
        deviceWidth = getIntent().getIntExtra("width", 1080);
        deviceHeight = getIntent().getIntExtra("height", 1920);
        if (name == null || base == null) {
            finish();
            return;
        }
        api = new Api(base);

        Window window = getWindow();
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        window.setStatusBarColor(Color.BLACK);
        window.setNavigationBarColor(Color.BLACK);
        hideSystemBars();

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);

        screen = new ImageView(this);
        screen.setScaleType(ImageView.ScaleType.FIT_CENTER);
        screen.setBackgroundColor(Color.BLACK);
        FrameLayout.LayoutParams screenLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
        screenLp.setMargins(0, Ui.dp(this, 54), 0, Ui.dp(this, 76));
        root.addView(screen, screenLp);
        screen.setOnTouchListener(new View.OnTouchListener() {
            @Override
            public boolean onTouch(View v, MotionEvent e) {
                return onScreenTouch(e);
            }
        });

        LinearLayout top = Ui.row(this);
        top.setPadding(Ui.dp(this, 8), Ui.dp(this, 4), Ui.dp(this, 8), Ui.dp(this, 4));
        top.setBackgroundColor(0xCC000000);
        FrameLayout.LayoutParams topLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, Ui.dp(this, 54), Gravity.TOP);
        root.addView(top, topLp);

        ImageView close = Ui.icon(this, R.drawable.ic_close, Color.WHITE, 22);
        FrameLayout closeWrap = new FrameLayout(this);
        closeWrap.setClickable(true);
        FrameLayout.LayoutParams closeIcon = new FrameLayout.LayoutParams(Ui.dp(this, 22), Ui.dp(this, 22));
        closeIcon.gravity = Gravity.CENTER;
        closeWrap.addView(close, closeIcon);
        closeWrap.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                finish();
            }
        });
        top.addView(closeWrap, new LinearLayout.LayoutParams(Ui.dp(this, 44), Ui.dp(this, 44)));

        LinearLayout titleBox = Ui.column(this);
        TextView title = Ui.text(this, name, 14, Color.WHITE, true);
        status = Ui.text(this, "Connecting…", 10, 0xFFCAC4D0);
        titleBox.addView(title);
        titleBox.addView(status);
        Ui.margins(titleBox, 8, 0, 0, 0);
        top.addView(titleBox, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f));

        TextView keyboard = Ui.textButton(this, "Type");
        keyboard.setTextColor(Color.WHITE);
        keyboard.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                showTextDialog();
            }
        });
        top.addView(keyboard);

        LinearLayout controls = Ui.row(this);
        controls.setGravity(Gravity.CENTER);
        controls.setPadding(Ui.dp(this, 8), Ui.dp(this, 6), Ui.dp(this, 8), Ui.dp(this, 6));
        controls.setBackgroundColor(0xCC000000);
        FrameLayout.LayoutParams controlsLp = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, Ui.dp(this, 76), Gravity.BOTTOM);
        root.addView(controls, controlsLp);

        controls.addView(control(R.drawable.ic_back, "Back", 4), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        controls.addView(control(R.drawable.ic_home, "Home", 3), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        controls.addView(control(R.drawable.ic_recents, "Recents", 187), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        controls.addView(control(R.drawable.ic_volume, "Volume up", 24), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));
        controls.addView(control(R.drawable.ic_power, "Power", 26), new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.MATCH_PARENT, 1f));

        setContentView(root);
    }

    private View control(int iconRes, String label, final int keyCode) {
        LinearLayout item = Ui.column(this);
        item.setGravity(Gravity.CENTER);
        item.setContentDescription(label);
        item.setClickable(true);
        item.setBackground(new android.graphics.drawable.RippleDrawable(
                android.content.res.ColorStateList.valueOf(0x44FFFFFF), null, null));
        ImageView icon = Ui.icon(this, iconRes, Color.WHITE, 23);
        item.addView(icon);
        TextView text = Ui.text(this, label, 9, 0xFFCAC4D0);
        item.addView(text);
        item.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                sendKey(keyCode);
            }
        });
        return item;
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) hideSystemBars();
    }

    private void draw(Api.Bytes frame) {
        if (frame.screenWidth > 0) deviceWidth = frame.screenWidth;
        if (frame.screenHeight > 0) deviceHeight = frame.screenHeight;
        Bitmap b = BitmapFactory.decodeByteArray(frame.data, 0, frame.data.length);
        if (b == null) {
            status.setText("Decoding the screen failed");
            return;
        }
        if (bitmap != null) bitmap.recycle();
        bitmap = b;
        bitmapWidth = b.getWidth();
        screen.setImageBitmap(b);
        status.setText("Live · " + deviceWidth + "×" + deviceHeight);
    }

    private boolean onScreenTouch(MotionEvent e) {
        switch (e.getActionMasked()) {
            case MotionEvent.ACTION_DOWN:
                downX = e.getX();
                downY = e.getY();
                downTime = System.currentTimeMillis();
                return true;
            case MotionEvent.ACTION_UP:
                float dx = e.getX() - downX;
                float dy = e.getY() - downY;
                double distance = Math.hypot(dx, dy);
                int[] from = map(downX, downY);
                int[] to = map(e.getX(), e.getY());
                if (from == null || to == null) return true;
                if (distance < Ui.dp(this, 12)) {
                    sendTap(from[0], from[1]);
                } else {
                    int duration = (int) Math.max(80, Math.min(1500, System.currentTimeMillis() - downTime));
                    sendSwipe(from[0], from[1], to[0], to[1], duration);
                }
                return true;
            default:
                return true;
        }
    }

    private int[] map(float viewX, float viewY) {
        if (bitmap == null || bitmapWidth <= 0) return null;
        Matrix inverse = new Matrix();
        if (!screen.getImageMatrix().invert(inverse)) return null;
        float[] point = {viewX, viewY};
        inverse.mapPoints(point);
        int bx = Math.round(point[0] / bitmapWidth * deviceWidth);
        int by = Math.round(point[1] / bitmap.getHeight() * deviceHeight);
        if (bx < 0) bx = 0;
        if (by < 0) by = 0;
        if (bx >= deviceWidth) bx = deviceWidth - 1;
        if (by >= deviceHeight) by = deviceHeight - 1;
        return new int[]{bx, by};
    }

    private void sendTap(final int x, final int y) {
        screen.performHapticFeedback(android.view.HapticFeedbackConstants.VIRTUAL_KEY);
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.tap(name, x, y);
                } catch (final Exception e) {
                    showError(e);
                }
            }
        });
    }

    private void sendSwipe(final int x1, final int y1, final int x2, final int y2, final int duration) {
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.swipe(name, x1, y1, x2, y2, duration);
                } catch (final Exception e) {
                    showError(e);
                }
            }
        });
    }

    private void sendKey(final int keyCode) {
        screen.performHapticFeedback(android.view.HapticFeedbackConstants.VIRTUAL_KEY);
        final Api current = api;
        io.execute(new Runnable() {
            @Override
            public void run() {
                try {
                    current.key(name, keyCode);
                } catch (final Exception e) {
                    showError(e);
                }
            }
        });
    }

    private void showTextDialog() {
        final EditText input = new EditText(this);
        input.setHint("Text to type in Android");
        input.setSingleLine(true);
        input.setInputType(InputType.TYPE_CLASS_TEXT);
        int pad = Ui.dp(this, 18);
        input.setPadding(pad, pad, pad, pad);
        final AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("Type text")
                .setView(input)
                .setNegativeButton("Cancel", null)
                .setPositiveButton("Send", new DialogInterface.OnClickListener() {
                    @Override
                    public void onClick(DialogInterface d, int which) {
                        final String text = input.getText().toString();
                        if (text.isEmpty()) return;
                        final Api current = api;
                        io.execute(new Runnable() {
                            @Override
                            public void run() {
                                try {
                                    current.typeText(name, text);
                                } catch (final Exception e) {
                                    showError(e);
                                }
                            }
                        });
                    }
                })
                .create();
        dialog.getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_STATE_ALWAYS_VISIBLE);
        dialog.setOnShowListener(new DialogInterface.OnShowListener() {
            @Override
            public void onShow(DialogInterface d) {
                input.requestFocus();
                android.view.inputmethod.InputMethodManager imm =
                        (android.view.inputmethod.InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
                if (imm != null) imm.showSoftInput(input, android.view.inputmethod.InputMethodManager.SHOW_IMPLICIT);
            }
        });
        dialog.setOnDismissListener(new DialogInterface.OnDismissListener() {
            @Override
            public void onDismiss(DialogInterface d) {
                main.postDelayed(new Runnable() {
                    @Override
                    public void run() {
                        hideSystemBars();
                    }
                }, 300);
            }
        });
        dialog.show();
    }

    private void showError(final Exception e) {
        main.post(new Runnable() {
            @Override
            public void run() {
                Toast.makeText(ScreenActivity.this, e.getMessage() == null ? e.toString() : e.getMessage(), Toast.LENGTH_SHORT).show();
            }
        });
    }

    private void hideSystemBars() {
        Window window = getWindow();
        if (Build.VERSION.SDK_INT >= 30) {
            window.setDecorFitsSystemWindows(false);
            WindowInsetsController controller = window.getInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
        } else {
            window.getDecorView().setSystemUiVisibility(
                    View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                            | View.SYSTEM_UI_FLAG_FULLSCREEN
                            | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                            | View.SYSTEM_UI_FLAG_LAYOUT_STABLE
                            | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                            | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION);
        }
    }

    @Override
    protected void onResume() {
        super.onResume();
        polling = true;
        hideSystemBars();
        main.removeCallbacks(capture);
        main.post(capture);
    }

    @Override
    protected void onPause() {
        polling = false;
        main.removeCallbacks(capture);
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        polling = false;
        io.shutdownNow();
        if (bitmap != null) {
            bitmap.recycle();
            bitmap = null;
        }
        super.onDestroy();
    }
}
