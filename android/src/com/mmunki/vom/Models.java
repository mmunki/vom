package com.mmunki.vom;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;

final class ServerStatus {
    String version = "";
    String os = "";
    String arch = "";
    String host = "";
    String recommendedAbi = "x86_64";
    String acceleration = "";
    String catalogSource = "";
    boolean emulator;
    boolean platformTools;
    boolean accelOk;
    long freeDisk;
    int images;
    int installedImages;
    int avds;

    static ServerStatus from(JSONObject o) {
        ServerStatus s = new ServerStatus();
        s.version = o.optString("version", "");
        s.os = o.optString("os", "");
        s.arch = o.optString("arch", "");
        s.host = o.optString("host", "");
        s.recommendedAbi = o.optString("recommendedAbi", "x86_64");
        s.acceleration = o.optString("acceleration", "");
        s.catalogSource = o.optString("catalogSource", "");
        s.emulator = o.optBoolean("emulator", false);
        s.platformTools = o.optBoolean("platformTools", false);
        s.accelOk = o.optBoolean("accelOk", false);
        s.freeDisk = o.optLong("freeDisk", 0);
        s.images = o.optInt("images", 0);
        s.installedImages = o.optInt("installedImages", 0);
        s.avds = o.optInt("avds", 0);
        return s;
    }
}

final class ImageInfo {
    String id = "";
    int api;
    String apiLevel = "";
    String version = "";
    String codename = "";
    String tag = "";
    String tagName = "";
    String abi = "";
    String url = "";
    String sha1 = "";
    String revision = "";
    String display = "";
    String source = "";
    long size;
    boolean installed;
    boolean recommended;

    static ImageInfo from(JSONObject o) {
        ImageInfo i = new ImageInfo();
        i.id = o.optString("id", "");
        i.api = o.optInt("api", 0);
        i.apiLevel = o.optString("apiLevel", "");
        i.version = o.optString("version", "");
        i.codename = o.optString("codename", "");
        i.tag = o.optString("tag", "");
        i.tagName = o.optString("tagName", i.tag);
        i.abi = o.optString("abi", "");
        i.url = o.optString("url", "");
        i.sha1 = o.optString("sha1", "");
        i.revision = o.optString("revision", "");
        i.display = o.optString("display", "");
        i.source = o.optString("source", "");
        i.size = o.optLong("size", 0);
        i.installed = o.optBoolean("installed", false);
        i.recommended = o.optBoolean("recommended", false);
        return i;
    }
}

final class ReleaseInfo {
    int api;
    String version = "";
    String codename = "";
    final List<ImageInfo> images = new ArrayList<>();

    static ReleaseInfo from(JSONObject o) {
        ReleaseInfo r = new ReleaseInfo();
        r.api = o.optInt("api", 0);
        r.version = o.optString("version", "");
        r.codename = o.optString("codename", "");
        JSONArray a = o.optJSONArray("images");
        if (a != null) {
            for (int i = 0; i < a.length(); i++) {
                JSONObject item = a.optJSONObject(i);
                if (item != null) {
                    r.images.add(ImageInfo.from(item));
                }
            }
        }
        return r;
    }
}

final class AvdInfo {
    String name = "";
    String imageId = "";
    int api;
    String apiLevel = "";
    String version = "";
    String tag = "";
    String abi = "";
    String dir = "";
    String serial = "";
    String log = "";
    int ramMb;
    int cores;
    int width;
    int height;
    int dpi;
    int pid;
    boolean running;
    boolean headless;

    static AvdInfo from(JSONObject o) {
        AvdInfo a = new AvdInfo();
        a.name = o.optString("name", "");
        a.imageId = o.optString("imageId", "");
        a.api = o.optInt("api", 0);
        a.apiLevel = o.optString("apiLevel", "");
        a.version = o.optString("version", "");
        a.tag = o.optString("tag", "");
        a.abi = o.optString("abi", "");
        a.dir = o.optString("dir", "");
        a.serial = o.optString("serial", "");
        a.log = o.optString("log", "");
        a.ramMb = o.optInt("ramMb", 0);
        a.cores = o.optInt("cores", 0);
        a.width = o.optInt("width", 1080);
        a.height = o.optInt("height", 1920);
        a.dpi = o.optInt("dpi", 420);
        a.pid = o.optInt("pid", 0);
        a.running = o.optBoolean("running", false);
        a.headless = o.optBoolean("headless", false);
        return a;
    }
}

final class JobInfo {
    String id = "";
    String kind = "";
    String label = "";
    String stage = "";
    String state = "";
    String error = "";
    long done;
    long total;

    static JobInfo from(JSONObject o) {
        JobInfo j = new JobInfo();
        j.id = o.optString("id", "");
        j.kind = o.optString("kind", "");
        j.label = o.optString("label", "");
        j.stage = o.optString("stage", "");
        j.state = o.optString("state", "");
        j.error = o.optString("error", "");
        j.done = o.optLong("done", 0);
        j.total = o.optLong("total", 0);
        return j;
    }

    boolean running() {
        return "running".equals(state);
    }

    int percent() {
        if (total <= 0) return -1;
        long p = done * 100 / total;
        if (p < 0) p = 0;
        if (p > 100) p = 100;
        return (int) p;
    }
}
