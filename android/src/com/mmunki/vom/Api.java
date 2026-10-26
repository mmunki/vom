package com.mmunki.vom;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

final class Api {
    static final class ApiException extends IOException {
        final int code;

        ApiException(int code, String message) {
            super(message);
            this.code = code;
        }
    }

    static final class Bytes {
        final byte[] data;
        final int screenWidth;
        final int screenHeight;

        Bytes(byte[] data, int screenWidth, int screenHeight) {
            this.data = data;
            this.screenWidth = screenWidth;
            this.screenHeight = screenHeight;
        }
    }

    private final String base;

    Api(String base) {
        this.base = normalize(base);
    }

    static String normalize(String server) {
        String s = server == null ? "" : server.trim();
        if (s.isEmpty()) return "";
        if (!s.startsWith("http://") && !s.startsWith("https://")) {
            s = "http://" + s;
        }
        while (s.endsWith("/")) {
            s = s.substring(0, s.length() - 1);
        }
        return s;
    }

    String base() {
        return base;
    }

    ServerStatus status() throws IOException {
        return ServerStatus.from(object("GET", "/api/status", null));
    }

    List<ReleaseInfo> catalog() throws IOException {
        JSONObject o = object("GET", "/api/catalog", null);
        List<ReleaseInfo> out = new ArrayList<>();
        JSONArray a = o.optJSONArray("releases");
        if (a != null) {
            for (int i = 0; i < a.length(); i++) {
                JSONObject item = a.optJSONObject(i);
                if (item != null) out.add(ReleaseInfo.from(item));
            }
        }
        return out;
    }

    List<AvdInfo> avds() throws IOException {
        JSONObject o = object("GET", "/api/library", null);
        List<AvdInfo> out = new ArrayList<>();
        JSONArray a = o.optJSONArray("avds");
        if (a != null) {
            for (int i = 0; i < a.length(); i++) {
                JSONObject item = a.optJSONObject(i);
                if (item != null) out.add(AvdInfo.from(item));
            }
        }
        return out;
    }

    List<JobInfo> jobs() throws IOException {
        JSONArray a = array("GET", "/api/jobs", null);
        List<JobInfo> out = new ArrayList<>();
        for (int i = 0; i < a.length(); i++) {
            JSONObject item = a.optJSONObject(i);
            if (item != null) out.add(JobInfo.from(item));
        }
        return out;
    }

    JobInfo setup(boolean force) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("force", force);
        } catch (Exception ignored) {
        }
        JSONObject o = object("POST", "/api/setup", body);
        if (o.has("id")) return JobInfo.from(o);
        return null;
    }

    JobInfo install(String imageId) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("id", imageId);
        } catch (Exception ignored) {
        }
        return JobInfo.from(object("POST", "/api/install", body));
    }

    JobInfo refreshCatalog() throws IOException {
        JSONObject o = object("POST", "/api/catalog/refresh", null);
        if (o.has("id")) return JobInfo.from(o);
        return null;
    }

    AvdInfo createAvd(String imageId, String name, int ramMb) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("imageId", imageId);
            if (name != null && !name.trim().isEmpty()) body.put("name", name.trim());
            if (ramMb > 0) body.put("ramMb", ramMb);
        } catch (Exception ignored) {
        }
        return AvdInfo.from(object("POST", "/api/avd", body));
    }

    AvdInfo start(String name, boolean headless, boolean wipe) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("headless", headless);
            body.put("wipe", wipe);
        } catch (Exception ignored) {
        }
        return AvdInfo.from(object("POST", "/api/avd/start", body));
    }

    void stop(String name) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
        } catch (Exception ignored) {
        }
        object("POST", "/api/avd/stop", body);
    }

    void delete(String name) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
        } catch (Exception ignored) {
        }
        object("POST", "/api/avd/delete", body);
    }

    void cancelJob(String id) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("id", id);
        } catch (Exception ignored) {
        }
        object("POST", "/api/jobs/cancel", body);
    }

    String log(String name) throws IOException {
        return text("GET", "/api/avd/log?name=" + enc(name));
    }

    Bytes screenshot(String name, int width) throws IOException {
        HttpURLConnection c = open("GET", "/api/avd/screen?name=" + enc(name) + "&w=" + width, null);
        int code = c.getResponseCode();
        if (code < 200 || code >= 300) {
            String message = readError(c);
            c.disconnect();
            throw new ApiException(code, message);
        }
        int w = parseInt(c.getHeaderField("X-Screen-Width"), 1080);
        int h = parseInt(c.getHeaderField("X-Screen-Height"), 1920);
        byte[] data = readAll(c.getInputStream());
        c.disconnect();
        return new Bytes(data, w, h);
    }

    void input(JSONObject body) throws IOException {
        object("POST", "/api/avd/input", body);
    }

    void tap(String name, int x, int y) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("action", "tap");
            body.put("x", x);
            body.put("y", y);
        } catch (Exception ignored) {
        }
        input(body);
    }

    void swipe(String name, int x1, int y1, int x2, int y2, int duration) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("action", "swipe");
            body.put("x", x1);
            body.put("y", y1);
            body.put("x2", x2);
            body.put("y2", y2);
            body.put("duration", duration);
        } catch (Exception ignored) {
        }
        input(body);
    }

    void key(String name, int keyCode) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("action", "key");
            body.put("keyCode", keyCode);
        } catch (Exception ignored) {
        }
        input(body);
    }

    void typeText(String name, String text) throws IOException {
        JSONObject body = new JSONObject();
        try {
            body.put("name", name);
            body.put("action", "text");
            body.put("text", text);
        } catch (Exception ignored) {
        }
        input(body);
    }

    private JSONObject object(String method, String path, JSONObject body) throws IOException {
        String raw = text(method, path, body);
        try {
            return new JSONObject(raw);
        } catch (Exception e) {
            throw new IOException("the server returned invalid JSON", e);
        }
    }

    private JSONArray array(String method, String path, JSONObject body) throws IOException {
        String raw = text(method, path, body);
        try {
            return new JSONArray(raw);
        } catch (Exception e) {
            throw new IOException("the server returned invalid JSON", e);
        }
    }

    private String text(String method, String path) throws IOException {
        return text(method, path, null);
    }

    private String text(String method, String path, JSONObject body) throws IOException {
        HttpURLConnection c = open(method, path, body);
        int code = c.getResponseCode();
        if (code < 200 || code >= 300) {
            String message = readError(c);
            c.disconnect();
            throw new ApiException(code, message);
        }
        String raw = new String(readAll(c.getInputStream()), StandardCharsets.UTF_8);
        c.disconnect();
        return raw;
    }

    private HttpURLConnection open(String method, String path, JSONObject body) throws IOException {
        if (base.isEmpty()) throw new IOException("no VOM host is configured");
        HttpURLConnection c = (HttpURLConnection) new URL(base + path).openConnection();
        c.setRequestMethod(method);
        c.setConnectTimeout(10000);
        c.setReadTimeout(45000);
        c.setRequestProperty("Accept", "application/json, image/png, text/plain");
        c.setRequestProperty("User-Agent", "VOM-Mobile/0.2");
        if (body != null) {
            byte[] data = body.toString().getBytes(StandardCharsets.UTF_8);
            c.setDoOutput(true);
            c.setRequestProperty("Content-Type", "application/json; charset=utf-8");
            c.setFixedLengthStreamingMode(data.length);
            try (OutputStream out = c.getOutputStream()) {
                out.write(data);
            }
        }
        return c;
    }

    private static String readError(HttpURLConnection c) {
        String raw = "";
        try {
            InputStream in = c.getErrorStream();
            if (in != null) raw = new String(readAll(in), StandardCharsets.UTF_8);
        } catch (Exception ignored) {
        }
        try {
            JSONObject o = new JSONObject(raw);
            String message = o.optString("error", "");
            if (!message.isEmpty()) return message;
        } catch (Exception ignored) {
        }
        if (raw.isEmpty()) {
            try {
                raw = c.getResponseMessage();
            } catch (Exception ignored) {
            }
        }
        if (raw != null && !raw.isEmpty()) return raw;
        try {
            return "HTTP " + c.getResponseCode();
        } catch (Exception e) {
            return "the request failed";
        }
    }

    private static byte[] readAll(InputStream in) throws IOException {
        try (InputStream input = in; ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            byte[] buf = new byte[16384];
            int n;
            while ((n = input.read(buf)) >= 0) {
                out.write(buf, 0, n);
            }
            return out.toByteArray();
        }
    }

    static String enc(String value) {
        try {
            return URLEncoder.encode(value, "UTF-8");
        } catch (Exception e) {
            return "";
        }
    }

    private static int parseInt(String value, int fallback) {
        try {
            return Integer.parseInt(value);
        } catch (Exception e) {
            return fallback;
        }
    }

    static String humanBytes(long n) {
        if (n < 1024) return n + " B";
        String[] units = {"KiB", "MiB", "GiB", "TiB"};
        double v = n;
        int u = -1;
        do {
            v /= 1024;
            u++;
        } while (v >= 1024 && u < units.length - 1);
        return String.format(java.util.Locale.US, "%.1f %s", v, units[u]);
    }
}
