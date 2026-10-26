package com.mmunki.vom;

import android.content.Context;
import android.content.res.ColorStateList;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.graphics.drawable.RippleDrawable;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.ScrollView;
import android.widget.TextView;

final class Ui {
    private Ui() {
    }

    static int dp(Context c, float value) {
        return Math.round(value * c.getResources().getDisplayMetrics().density);
    }

    static int color(Context c, int res) {
        return c.getColor(res);
    }

    static TextView text(Context c, String value, float sp, int color, boolean medium) {
        TextView v = new TextView(c);
        v.setText(value);
        v.setTextSize(TypedValue.COMPLEX_UNIT_SP, sp);
        v.setTextColor(color);
        v.setTypeface(Typeface.create("sans-serif", medium ? Typeface.BOLD : Typeface.NORMAL));
        v.setLineSpacing(0, 1.15f);
        return v;
    }

    static TextView text(Context c, String value, float sp, int color) {
        return text(c, value, sp, color, false);
    }

    static LinearLayout column(Context c) {
        LinearLayout v = new LinearLayout(c);
        v.setOrientation(LinearLayout.VERTICAL);
        return v;
    }

    static LinearLayout row(Context c) {
        LinearLayout v = new LinearLayout(c);
        v.setOrientation(LinearLayout.HORIZONTAL);
        v.setGravity(Gravity.CENTER_VERTICAL);
        return v;
    }

    static LinearLayout card(Context c) {
        LinearLayout v = column(c);
        v.setPadding(dp(c, 18), dp(c, 16), dp(c, 18), dp(c, 16));
        v.setBackground(rounded(c, color(c, R.color.surface), 24));
        v.setElevation(dp(c, 1));
        return v;
    }

    static GradientDrawable rounded(Context c, int fill, float radiusDp) {
        GradientDrawable d = new GradientDrawable();
        d.setColor(fill);
        d.setCornerRadius(dp(c, radiusDp));
        return d;
    }

    static GradientDrawable roundedStroke(Context c, int fill, int stroke, float radiusDp) {
        GradientDrawable d = rounded(c, fill, radiusDp);
        d.setStroke(dp(c, 1), stroke);
        return d;
    }

    static TextView button(Context c, String label, boolean filled) {
        TextView v = text(c, label, 14, filled ? color(c, R.color.onPrimary) : color(c, R.color.primary), true);
        v.setGravity(Gravity.CENTER);
        v.setMinHeight(dp(c, 40));
        v.setPadding(dp(c, 16), dp(c, 10), dp(c, 16), dp(c, 10));
        v.setClickable(true);
        v.setFocusable(true);
        int base = filled ? color(c, R.color.primary) : Color.TRANSPARENT;
        GradientDrawable bg = roundedStroke(c, base, filled ? base : color(c, R.color.outline), 20);
        v.setBackground(new RippleDrawable(ColorStateList.valueOf(withAlpha(filled ? color(c, R.color.onPrimary) : color(c, R.color.primary), 48)), bg, null));
        return v;
    }

    static TextView textButton(Context c, String label) {
        TextView v = text(c, label, 14, color(c, R.color.primary), true);
        v.setGravity(Gravity.CENTER);
        v.setPadding(dp(c, 12), dp(c, 8), dp(c, 12), dp(c, 8));
        v.setClickable(true);
        v.setFocusable(true);
        v.setBackground(new RippleDrawable(ColorStateList.valueOf(withAlpha(color(c, R.color.primary), 40)), null, null));
        return v;
    }

    static ImageView icon(Context c, int drawable, int tint, int sizeDp) {
        ImageView v = new ImageView(c);
        v.setImageResource(drawable);
        v.setColorFilter(tint);
        v.setScaleType(ImageView.ScaleType.FIT_CENTER);
        v.setLayoutParams(new LinearLayout.LayoutParams(dp(c, sizeDp), dp(c, sizeDp)));
        return v;
    }

    static TextView pill(Context c, String label, int background, int foreground) {
        TextView v = text(c, label, 11, foreground, true);
        v.setPadding(dp(c, 9), dp(c, 4), dp(c, 9), dp(c, 4));
        v.setBackground(rounded(c, background, 20));
        return v;
    }

    static View spacer(Context c, int heightDp) {
        View v = new View(c);
        v.setLayoutParams(new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(c, heightDp)));
        return v;
    }

    static View divider(Context c) {
        View v = new View(c);
        v.setBackgroundColor(color(c, R.color.outlineVariant));
        v.setLayoutParams(new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, Math.max(1, dp(c, 0.6f))));
        return v;
    }

    static void margins(View v, int left, int top, int right, int bottom) {
        ViewGroup.LayoutParams lp = v.getLayoutParams();
        if (lp == null) {
            lp = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        } else if (!(lp instanceof ViewGroup.MarginLayoutParams)) {
            lp = new LinearLayout.LayoutParams(lp.width, lp.height);
        }
        ViewGroup.MarginLayoutParams m = (ViewGroup.MarginLayoutParams) lp;
        Context c = v.getContext();
        m.setMargins(dp(c, left), dp(c, top), dp(c, right), dp(c, bottom));
        v.setLayoutParams(m);
    }

    static void padding(View v, int left, int top, int right, int bottom) {
        Context c = v.getContext();
        v.setPadding(dp(c, left), dp(c, top), dp(c, right), dp(c, bottom));
    }

    static ScrollView scroll(Context c) {
        ScrollView s = new ScrollView(c);
        s.setFillViewport(true);
        s.setClipToPadding(false);
        s.setOverScrollMode(View.OVER_SCROLL_NEVER);
        return s;
    }

    static FrameLayout.LayoutParams match() {
        return new FrameLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
    }

    static LinearLayout.LayoutParams matchWrap() {
        return new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
    }

    static ProgressBar progress(Context c) {
        ProgressBar p = new ProgressBar(c, null, android.R.attr.progressBarStyleHorizontal);
        p.setMax(1000);
        p.setProgressTintList(ColorStateList.valueOf(color(c, R.color.primary)));
        p.setProgressBackgroundTintList(ColorStateList.valueOf(color(c, R.color.surfaceVariant)));
        p.setLayoutParams(new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, dp(c, 6)));
        return p;
    }

    static int withAlpha(int color, int alpha) {
        return Color.argb(alpha, Color.red(color), Color.green(color), Color.blue(color));
    }

    static void setLightStatusBar(android.view.Window window, boolean light) {
        if (android.os.Build.VERSION.SDK_INT >= 23) {
            int flags = window.getDecorView().getSystemUiVisibility();
            if (light) {
                flags |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
            } else {
                flags &= ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
            }
            if (android.os.Build.VERSION.SDK_INT >= 26) {
                if (light) {
                    flags |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                } else {
                    flags &= ~View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                }
            }
            window.getDecorView().setSystemUiVisibility(flags);
        }
    }

    static boolean isNight(Context c) {
        int mode = c.getResources().getConfiguration().uiMode & android.content.res.Configuration.UI_MODE_NIGHT_MASK;
        return mode == android.content.res.Configuration.UI_MODE_NIGHT_YES;
    }
}
