"use strict";

const ICONS = {
  grid: '<rect x="3.5" y="3.5" width="7" height="7" rx="2"/><rect x="13.5" y="3.5" width="7" height="7" rx="2"/><rect x="3.5" y="13.5" width="7" height="7" rx="2"/><rect x="13.5" y="13.5" width="7" height="7" rx="2"/>',
  devices: '<rect x="3" y="4" width="13" height="16" rx="2.5"/><rect x="16" y="8" width="5" height="12" rx="1.5"/><path d="M6.5 8.5h6"/>',
  tune: '<path d="M4 7h9M17 7h3M4 17h3M11 17h9"/><circle cx="15" cy="7" r="2.2"/><circle cx="9" cy="17" r="2.2"/>',
  refresh: '<path d="M20 11a8 8 0 1 0-2.3 5.7"/><path d="M20 5v6h-6"/>',
  download: '<path d="M12 3v12"/><path d="m7.5 10.5 4.5 4.5 4.5-4.5"/><path d="M4 20h16"/>',
  play: '<path d="M8 5.5v13l10-6.5z"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
  add: '<path d="M12 5v14M5 12h14"/>',
  close: '<path d="m6 6 12 12M18 6 6 18"/>',
  search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4 4"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/>',
  memory: '<rect x="6" y="6" width="12" height="12" rx="2"/><path d="M9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4"/>',
  bolt: '<path d="M13 2 4.5 13H11l-1 9 9-12h-6.5z"/>',
  android: '<path d="M7 9h10v7a5 5 0 0 1-10 0z" fill="currentColor" stroke="none"/><path d="m8.2 5.5 1.4 2.4M15.8 5.5l-1.4 2.4" stroke-width="1.6"/><circle cx="9.5" cy="12" r=".9" fill="currentColor" stroke="none"/><circle cx="14.5" cy="12" r=".9" fill="currentColor" stroke="none"/>',
  trash: '<path d="M4 7h16M9 7V4.5h6V7M6.5 7l.8 13h9.4l.8-13"/><path d="M10 11v5M14 11v5"/>',
  terminal: '<path d="m5 7 4 5-4 5"/><path d="M12 17h7"/>',
  folder: '<path d="M3.5 7.5A2.5 2.5 0 0 1 6 5h3l2 2h7a2.5 2.5 0 0 1 2.5 2.5v7A2.5 2.5 0 0 1 18 19H6a2.5 2.5 0 0 1-2.5-2.5z"/>',
  dark: '<path d="M20 14.5A8.5 8.5 0 0 1 9.5 4 8.5 8.5 0 1 0 20 14.5z"/>',
  light: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5 19 19M19 5l-1.5 1.5M6.5 17.5 5 19"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  storage: '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v6c0 1.7 3.6 3 8 3s8-1.3 8-3V6"/><path d="M4 12v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/>',
  cpu: '<rect x="7" y="7" width="10" height="10" rx="2"/><path d="M10 7V4M14 7V4M10 20v-3M14 20v-3M7 10H4M7 14H4M20 10h-3M20 14h-3"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7L11 6"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7L13 18"/>',
};

function icon(name) {
  return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ""}</svg>`;
}

function applyIcons(root = document) {
  root.querySelectorAll("[data-icon]").forEach((el) => {
    el.innerHTML = icon(el.dataset.icon);
  });
}

const BADGES = {
  24: ["#ffe1eb", "#6b1f38"], 25: ["#ffdcea", "#6b1f38"],
  26: ["#dcecff", "#0d3c61"], 27: ["#d8f2fb", "#0b4f6c"],
  28: ["#dcf4e3", "#14532d"], 29: ["#eaf6d8", "#365314"],
  30: ["#fff2cc", "#6d4c00"], 31: ["#ffe7cf", "#7a3e00"],
  32: ["#fbe0dc", "#7a2e1e"], 33: ["#e8ddff", "#4527a0"],
  34: ["#d5f5f8", "#006064"], 35: ["#ffdbe8", "#880e4f"],
  36: ["#dfe3ff", "#283593"], 37: ["#f2ddff", "#6a1b9a"],
};

const BADGE_FALLBACK = ["#eaddff", "#21005d"];
const TAG_PREFERENCE = ["google_apis", "default", "google_apis_playstore"];

const state = {
  status: null,
  catalog: null,
  library: { images: [], avds: [] },
  selectedTag: "all",
  query: "",
  selections: {},
  watching: null,
  lastJob: null,
  lastBytes: { done: 0, at: 0 },
  installing: new Set(),
};

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => Array.from(document.querySelectorAll(sel));

function escapeHTML(value) {
  return String(value == null ? "" : value)
    .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

function formatBytes(n) {
  n = Number(n) || 0;
  if (n < 1024) return `${n} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
  return `${n.toFixed(n >= 100 ? 0 : 1)} ${units[i]}`;
}

function formatSpeed(bytesPerSecond) {
  if (!bytesPerSecond || bytesPerSecond < 1) return "";
  return `${formatBytes(bytesPerSecond)}/s`;
}

function toast(message, isError = false) {
  const el = $("#snackbar");
  el.textContent = message;
  el.classList.toggle("error", isError);
  el.classList.add("show");
  clearTimeout(toast._timer);
  toast._timer = setTimeout(() => el.classList.remove("show"), 3800);
}

async function api(path, options = {}) {
  const opts = { ...options, headers: { ...(options.headers || {}) } };
  if (opts.body && typeof opts.body !== "string") {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(opts.body);
  }
  const res = await fetch(path, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { error: text }; }
  if (!res.ok) throw new Error((data && data.error) || `${res.status} ${res.statusText}`);
  return data;
}

/* ----------------------------------------------------------------- theme */
function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  localStorage.setItem("vom-theme", theme);
  const btn = $("#themeToggle");
  if (btn) btn.innerHTML = icon(theme === "dark" ? "light" : "dark");
}
function initTheme() {
  const saved = localStorage.getItem("vom-theme");
  const prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
  applyTheme(saved || (prefersDark ? "dark" : "light"));
}

/* ------------------------------------------------------------------ views */
const VIEW_META = {
  catalog: ["Catalog", "Android 7.0 through 17, straight from Google"],
  library: ["Library", "Your verified images and virtual devices"],
  setup: ["Setup", "Runtime, acceleration and storage"],
};

function setView(name) {
  $$(".view").forEach((v) => v.classList.add("hidden"));
  const view = $(`#view-${name}`);
  if (view) view.classList.remove("hidden");
  $$(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.view === name));
  const meta = VIEW_META[name] || VIEW_META.catalog;
  $("#pageTitle").textContent = meta[0];
  $("#pageSubtitle").textContent = meta[1];
  if (name === "library") loadLibrary();
  if (name === "setup") loadStatus();
  window.scrollTo({ top: 0, behavior: "smooth" });
}

/* ------------------------------------------------------------------- data */
async function loadStatus() {
  try {
    state.status = await api("/api/status");
    renderStatus();
    renderSetup();
  } catch (err) {
    toast(err.message, true);
  }
}

async function loadCatalog() {
  try {
    state.catalog = await api("/api/catalog");
    renderCatalog();
    renderStatus();
  } catch (err) {
    $("#catalogGrid").innerHTML = `<div class="empty"><h3>Catalog unavailable</h3><p>${escapeHTML(err.message)}</p></div>`;
  }
}

async function loadLibrary() {
  try {
    const lib = await api("/api/library");
    state.library = { images: lib.images || [], avds: lib.avds || [] };
    renderLibrary();
    renderStatus();
    renderCatalog();
  } catch (err) {
    toast(err.message, true);
  }
}

async function refreshAll() {
  await Promise.all([loadStatus(), loadCatalog(), loadLibrary()]);
}

/* ---------------------------------------------------------------- catalog */
function imagesFor(release) {
  return release.images.filter((img) => state.selectedTag === "all" || img.tag === state.selectedTag);
}

function chooseDefault(images) {
  if (!images.length) return null;
  const recommended = state.catalog ? state.catalog.recommendedAbi : "x86_64";
  for (const tag of TAG_PREFERENCE) {
    const hit = images.find((img) => img.tag === tag && img.abi === recommended);
    if (hit) return hit;
  }
  return images.find((img) => img.abi === recommended) || images[0];
}

function selectedFor(release, images) {
  const current = state.selections[release.api];
  const hit = images.find((img) => img.id === current);
  if (hit) return hit;
  const picked = chooseDefault(images);
  if (picked) state.selections[release.api] = picked.id;
  return picked;
}

function installedIDs() {
  return new Set(state.library.images.map((img) => img.id));
}

function avdForImage(id) {
  return state.library.avds.find((a) => a.imageId === id) || null;
}

function releaseMatches(release, images) {
  if (!images.length) return false;
  const q = state.query.trim().toLowerCase();
  if (!q) return true;
  const hay = [
    release.version, release.codename, String(release.api),
    ...images.map((i) => `${i.tagName} ${i.tag} ${i.abi}`),
  ].join(" ").toLowerCase();
  return hay.includes(q);
}

function renderCatalog() {
  const grid = $("#catalogGrid");
  if (!state.catalog) {
    grid.innerHTML = Array.from({ length: 6 }, () => `
      <article class="card skeleton-card">
        <div class="card-head"><div class="version-badge">··</div>
        <div class="card-title"><h2>Loading…</h2><p>Reading the official repository</p></div></div>
        <div class="variants"><div class="variant"><span class="dot"></span><div class="variant-text"><strong>Fetching catalog</strong><small>one moment</small></div></div></div>
      </article>`).join("");
    return;
  }
  const installed = installedIDs();
  let shown = 0;
  const cards = [];
  for (const release of state.catalog.releases) {
    const images = imagesFor(release);
    if (!releaseMatches(release, images)) continue;
    shown++;
    const selected = selectedFor(release, images);
    const badge = BADGES[release.api] || BADGE_FALLBACK;
    const selectedInstalled = selected && installed.has(selected.id);
    const hasAVD = selected ? avdForImage(selected.id) : null;
    const variants = images.map((img) => {
      const active = selected && img.id === selected.id;
      return `<button class="variant ${active ? "active" : ""}" type="button" data-action="select" data-api="${release.api}" data-id="${escapeHTML(img.id)}">
        <span class="dot"></span>
        <span class="variant-text"><strong>${escapeHTML(img.tagName)}</strong><small>${escapeHTML(img.abi)}${img.recommended ? " · recommended" : ""}</small></span>
        ${installed.has(img.id) ? '<span class="installed-dot" title="Installed"></span>' : ""}
        <span class="size">${formatBytes(img.size)}</span>
      </button>`;
    }).join("");
    let action = "";
    if (selectedInstalled) {
      action = `<button class="btn filled" type="button" data-action="launch" data-id="${escapeHTML(selected.id)}">${icon(hasAVD ? "play" : "add")}<span>${hasAVD ? "Launch" : "Create device"}</span></button>`;
    } else if (selected && state.installing.has(selected.id)) {
      action = `<button class="btn filled" type="button" disabled><span class="spinner"></span><span>Installing</span></button>`;
    } else {
      action = `<button class="btn filled" type="button" data-action="install" data-id="${selected ? escapeHTML(selected.id) : ""}" ${selected ? "" : "disabled"}>${icon("download")}<span>Download</span></button>`;
    }
    cards.push(`
      <article class="card" style="animation-delay:${Math.min(shown * 18, 240)}ms">
        <div class="card-head">
          <div class="version-badge" style="--badge:${badge[0]};--badge-ink:${badge[1]}">${release.api}</div>
          <div class="card-title">
            <h2>Android ${escapeHTML(release.version)}</h2>
            <p>${release.codename ? `${escapeHTML(release.codename)} · ` : ""}API ${escapeHTML(String(release.api))}</p>
          </div>
          ${selectedInstalled ? `<span class="chip ok">${icon("check")}Installed</span>` : `<span class="chip">${images.length} image${images.length === 1 ? "" : "s"}</span>`}
        </div>
        <div class="variants">${variants}</div>
        <div class="card-actions">
          ${action}
          <button class="btn text" type="button" data-action="details" data-id="${selected ? escapeHTML(selected.id) : ""}" ${selected ? "" : "disabled"}>Details</button>
        </div>
      </article>`);
  }
  grid.innerHTML = cards.join("");
  $("#catalogEmpty").classList.toggle("hidden", shown > 0);
  applyIcons(grid);
}

function findImage(id) {
  if (!state.catalog) return null;
  for (const release of state.catalog.releases) {
    const hit = release.images.find((img) => img.id === id);
    if (hit) return hit;
  }
  return null;
}

/* ------------------------------------------------------------------ status */
function renderStatus() {
  if (!state.catalog) return;
  let count = 0;
  for (const r of state.catalog.releases) count += r.images.length;
  $("#statVersions").textContent = state.catalog.releases.length;
  $("#statImages").textContent = count;
  $("#statInstalled").textContent = state.library.images.length;
}

function statusRow(label, value, ok) {
  const cls = ok === true ? "status-ok" : ok === false ? "status-bad" : "";
  return `<div class="status-row"><span>${escapeHTML(label)}</span><strong class="${cls}">${value}</strong></div>`;
}

function renderSetup() {
  const s = state.status;
  if (!s) return;
  const emu = s.tools && s.tools.emulator ? ` ${escapeHTML(s.tools.emulator.revision)}` : "";
  const pt = s.tools && s.tools.platformTools ? ` ${escapeHTML(s.tools.platformTools.revision)}` : "";
  $("#setupStatus").innerHTML = [
    statusRow("Android emulator", s.emulator ? `Installed${emu}` : "Not installed", s.emulator),
    statusRow("Platform-tools (adb)", s.platformTools ? `Installed${pt}` : "Not installed", s.platformTools),
    statusRow("System images", `${s.installedImages} installed`, s.installedImages > 0),
    statusRow("Virtual devices", `${s.avds}`, null),
  ].join("");
  $("#hostStatus").innerHTML = [
    statusRow("Host", `${escapeHTML(s.os)} / ${escapeHTML(s.arch)}`, null),
    statusRow("Acceleration", `${escapeHTML(s.acceleration)}${s.accelOk ? "" : " (slow)"}`, s.accelOk),
    statusRow("Free disk", s.freeDisk ? formatBytes(s.freeDisk) : "unknown", s.freeDisk > 2 * 1024 * 1024 * 1024),
    statusRow("Data directory", `<code>${escapeHTML(s.home)}</code>`, null),
    statusRow("Catalog", `${s.images} images · ${escapeHTML(s.catalogSource)}`, s.images > 0),
  ].join("");
}

/* ----------------------------------------------------------------- library */
function renderLibrary() {
  renderImages();
  renderAVDs();
}

function renderImages() {
  const list = $("#imageList");
  if (!state.library.images.length) {
    list.innerHTML = `<div class="empty">${icon("storage")}<h3>No images installed</h3><p>Pick an Android version in the Catalog and press Download.</p></div>`;
    return;
  }
  list.innerHTML = state.library.images.map((img) => {
    const badge = BADGES[img.api] || BADGE_FALLBACK;
    return `<div class="row">
      <div class="mini-badge" style="background:${badge[0]};color:${badge[1]}">${img.api}</div>
      <div class="grow">
        <h3>Android ${escapeHTML(img.version)} · ${escapeHTML(img.tagName || img.tag)} · ${escapeHTML(img.abi)}</h3>
        <p>${formatBytes(img.bytes)} on disk · ${escapeHTML(img.id)}</p>
      </div>
      <div class="row-actions">
        <button class="btn tonal" type="button" data-action="new-device" data-id="${escapeHTML(img.id)}">${icon("add")}<span>New device</span></button>
        <button class="icon-btn" type="button" title="Delete image" data-action="delete-image" data-id="${escapeHTML(img.id)}">${icon("trash")}</button>
      </div>
    </div>`;
  }).join("");
  applyIcons(list);
}

function renderAVDs() {
  const list = $("#avdList");
  if (!state.library.avds.length) {
    list.innerHTML = `<div class="empty" style="grid-column:1/-1">${icon("devices")}<h3>No virtual devices yet</h3><p>Create one from an installed image and launch it.</p></div>`;
    return;
  }
  list.innerHTML = state.library.avds.map((avd) => {
    const badge = BADGES[avd.api] || BADGE_FALLBACK;
    const running = avd.running;
    return `<article class="card device-card">
      <div class="card-head">
        <div class="version-badge" style="--badge:${badge[0]};--badge-ink:${badge[1]}">${avd.api}</div>
        <div class="card-title">
          <h2>${escapeHTML(avd.name)}</h2>
          <p>Android ${escapeHTML(avd.version)} · ${escapeHTML(avd.tagName || avd.tag)} · ${escapeHTML(avd.abi)}</p>
        </div>
        <span class="chip ${running ? "ok" : ""}"><span class="status-dot ${running ? "live" : ""}"></span>${running ? `Running${avd.headless ? " · headless" : ""}` : "Stopped"}</span>
      </div>
      <div class="device-meta">
        <span class="chip">${icon("memory")}${avd.ramMb} MiB RAM</span>
        <span class="chip">${icon("cpu")}${avd.cores} cores</span>
        <span class="chip">${avd.width}×${avd.height}</span>
      </div>
      <div class="device-actions">
        ${running
          ? `<button class="btn tonal" type="button" data-action="stop-avd" data-name="${escapeHTML(avd.name)}">${icon("stop")}<span>Stop</span></button>`
          : `<button class="btn filled" type="button" data-action="start-avd" data-name="${escapeHTML(avd.name)}" data-headless="false">${icon("play")}<span>Launch</span></button>
             <button class="btn outline" type="button" data-action="start-avd" data-name="${escapeHTML(avd.name)}" data-headless="true">${icon("terminal")}<span>Headless</span></button>`}
        <button class="btn text" type="button" data-action="log-avd" data-name="${escapeHTML(avd.name)}">${icon("folder")}<span>Log</span></button>
        <button class="icon-btn" type="button" title="Delete device" data-action="delete-avd" data-name="${escapeHTML(avd.name)}">${icon("trash")}</button>
      </div>
    </article>`;
  }).join("");
  applyIcons(list);
}

/* ----------------------------------------------------------------- actions */
async function installImage(id) {
  const img = findImage(id);
  if (!img) return toast("That image is not in the catalog", true);
  state.installing.add(id);
  renderCatalog();
  try {
    const job = await api("/api/install", { method: "POST", body: { id } });
    watchJob(job);
    toast(`Downloading Android ${img.version} (${formatBytes(img.size)})`);
  } catch (err) {
    state.installing.delete(id);
    renderCatalog();
    toast(err.message, true);
  }
}

async function launchImage(id) {
  const img = findImage(id);
  if (!img) return;
  if (!state.status || !state.status.emulator) {
    openSetupDialog();
    return;
  }
  let avd = avdForImage(id);
  try {
    if (!avd) {
      const name = uniqueName(`VOM-Android-${img.version}-${img.tag}`);
      avd = await api("/api/avd", { method: "POST", body: { imageId: id, name } });
      toast(`Created ${avd.name}`);
    }
    await api("/api/avd/start", { method: "POST", body: { name: avd.name, headless: false } });
    toast(`Booting ${avd.name}. A window will open on the host.`);
    setTimeout(loadLibrary, 1200);
  } catch (err) {
    toast(err.message, true);
  }
}

function uniqueName(base) {
  const names = new Set(state.library.avds.map((a) => a.name));
  if (!names.has(base)) return base;
  let i = 2;
  while (names.has(`${base} ${i}`)) i++;
  return `${base} ${i}`;
}

async function startAVD(name, headless) {
  if (!state.status || !state.status.emulator) return openSetupDialog();
  try {
    await api("/api/avd/start", { method: "POST", body: { name, headless } });
    toast(headless ? `Started ${name} headless` : `Booting ${name}. A window will open on the host.`);
    setTimeout(loadLibrary, 1200);
  } catch (err) { toast(err.message, true); }
}

async function stopAVD(name) {
  try {
    await api("/api/avd/stop", { method: "POST", body: { name } });
    toast(`Stopped ${name}`);
    setTimeout(loadLibrary, 500);
  } catch (err) { toast(err.message, true); }
}

async function deleteAVD(name) {
  openDialog("Delete device?", `<p>This removes the AVD configuration and its user data. The downloaded
    system image stays installed.</p><p><strong>${escapeHTML(name)}</strong></p>`, [
    { label: "Cancel", cls: "text" },
    { label: "Delete", cls: "danger", onClick: async () => {
      try {
        await api("/api/avd/delete", { method: "POST", body: { name } });
        toast(`Deleted ${name}`);
        loadLibrary();
      } catch (err) { toast(err.message, true); }
    } },
  ]);
}

function deleteImage(id) {
  const img = state.library.images.find((i) => i.id === id);
  if (!img) return;
  const users = state.library.avds.filter((a) => a.imageId === id);
  const warning = users.length
    ? `<p class="status-bad"><strong>${users.length} device(s) use this image.</strong> They will not boot until the image is downloaded again.</p>`
    : "";
  openDialog("Delete system image?", `<p>Android ${escapeHTML(img.version)} · ${escapeHTML(img.tagName || img.tag)} · ${escapeHTML(img.abi)}</p>
    <p>${formatBytes(img.bytes)} will be removed from ${escapeHTML(state.status ? state.status.home : "the data directory")}.</p>${warning}`, [
    { label: "Cancel", cls: "text" },
    { label: "Delete", cls: "danger", onClick: async () => {
      try {
        await api(`/api/images?id=${encodeURIComponent(id)}`, { method: "DELETE" });
        toast("Image deleted");
        refreshAll();
      } catch (err) { toast(err.message, true); }
    } },
  ]);
}

function openDetails(id) {
  const img = findImage(id) || state.library.images.find((i) => i.id === id);
  if (!img) return;
  const rows = [
    ["Package", img.path], ["ABI", img.abi], ["Tag", `${img.tagName || img.tag} (${img.tag})`],
    ["Revision", img.revision], ["Download size", formatBytes(img.size)],
    ["SHA-1", img.sha1], ["Source", img.source], ["URL", img.url],
  ];
  const body = `<div class="status-list">${rows.map(([k, v]) =>
    `<div class="status-row"><span>${escapeHTML(k)}</span><strong style="max-width:62%;word-break:break-all">${escapeHTML(v)}</strong></div>`).join("")}</div>`;
  const actions = [];
  if (state.library.images.some((i) => i.id === id)) {
    actions.push({ label: "New device", cls: "filled", onClick: () => openNewDevice(id) });
  } else {
    actions.push({ label: "Download", cls: "filled", onClick: () => installImage(id) });
  }
  actions.push({ label: "Close", cls: "text" });
  openDialog(`Android ${img.version} · ${img.tagName || img.tag}`, body, actions);
}

function openLog(name) {
  openDialog(`Log · ${name}`, `<pre class="logview" id="logText">Loading…</pre>`, [
    { label: "Refresh", cls: "text", close: false, onClick: async () => {
      try {
        const text = await fetch(`/api/avd/log?name=${encodeURIComponent(name)}`).then((r) => r.text());
        $("#logText").textContent = text || "(no log output yet)";
        $("#logText").scrollTop = $("#logText").scrollHeight;
      } catch (err) { $("#logText").textContent = err.message; }
    } },
    { label: "Close", cls: "text" },
  ]);
  setTimeout(() => {
    const btn = document.querySelector('[data-dialog-action="0"]');
    if (btn) btn.click();
  }, 30);
}

function openNewDevice(imageId) {
  const images = state.library.images;
  if (!images.length) return toast("Download an image first", true);
  const defaultImage = imageId || images[images.length - 1].id;
  const options = images.map((img) => `<option value="${escapeHTML(img.id)}" ${img.id === defaultImage ? "selected" : ""}>Android ${escapeHTML(img.version)} · ${escapeHTML(img.tagName || img.tag)} · ${escapeHTML(img.abi)}</option>`).join("");
  const suggested = uniqueName(`VOM-Android-${(images.find((i) => i.id === defaultImage) || images[0]).version}-Device`);
  const body = `
    <div class="field"><label for="avdImage">System image</label><select id="avdImage">${options}</select></div>
    <div class="field"><label for="avdName">Device name</label><input id="avdName" value="${escapeHTML(suggested)}" maxlength="64" autocomplete="off"></div>
    <div class="field-row">
      <div class="field"><label for="avdRam">Memory</label><select id="avdRam">
        <option value="2048">2 GiB</option><option value="4096" selected>4 GiB</option><option value="6144">6 GiB</option><option value="8192">8 GiB</option>
      </select></div>
      <div class="field"><label for="avdCores">CPU cores</label><select id="avdCores">
        <option value="2">2</option><option value="4" selected>4</option><option value="6">6</option><option value="8">8</option>
      </select></div>
    </div>
    <div class="field"><label for="avdRes">Screen</label><select id="avdRes">
      <option value="1080x1920">1080 × 1920 · 420 dpi</option>
      <option value="1080x2400">1080 × 2400 · 420 dpi</option>
      <option value="1440x2560">1440 × 2560 · 560 dpi</option>
      <option value="720x1280">720 × 1280 · 320 dpi</option>
    </select></div>`;
  openDialog("New virtual device", body, [
    { label: "Cancel", cls: "text" },
    { label: "Create", cls: "filled", close: false, onClick: async () => {
      const [w, h] = $("#avdRes").value.split("x").map(Number);
      const dpi = $("#avdRes").value.startsWith("1440") ? 560 : $("#avdRes").value.startsWith("720") ? 320 : 420;
      try {
        const avd = await api("/api/avd", { method: "POST", body: {
          imageId: $("#avdImage").value, name: $("#avdName").value.trim(),
          ramMb: Number($("#avdRam").value), cores: Number($("#avdCores").value),
          width: w, height: h, dpi,
        } });
        closeDialog();
        toast(`Created ${avd.name}`);
        await loadLibrary();
        setView("library");
      } catch (err) { toast(err.message, true); }
    } },
  ]);
}

function openSetupDialog() {
  openDialog("Emulator runtime needed", `<p>VOM downloads the official Google emulator and platform-tools
    once, then uses them for every Android version. The runtime is about 360 MB.</p>
    <div class="note">${icon("info")}<p>Hardware acceleration is required for a usable device. On Linux,
    make sure <code>/dev/kvm</code> is available; VOM will still run in software mode but it is much slower.</p></div>`,
    [
      { label: "Later", cls: "text" },
      { label: "Install runtime", cls: "filled", onClick: () => { setView("setup"); runSetup(); } },
    ]);
}

async function runSetup(force = false) {
  try {
    const job = await api("/api/setup", { method: "POST", body: { force } });
    watchJob(job);
    toast("Installing the Android emulator runtime");
  } catch (err) { toast(err.message, true); }
}

/* -------------------------------------------------------------------- jobs */
function showJob(job) {
  const bar = $("#jobbar");
  bar.classList.remove("hidden");
  $("#jobLabel").textContent = job.label || "Working";
  $("#jobStage").textContent = (job.stage || "working").replace(/-/g, " ");
  $("#jobState").className = "job-state" + (job.state === "error" ? " error" : job.state === "done" ? " done" : "");
  const pct = job.total > 0 ? Math.min(100, (job.done / job.total) * 100) : 0;
  const progress = $("#jobProgress");
  progress.style.width = job.total > 0 ? `${pct}%` : "38%";
  $(".progress").classList.toggle("indeterminate", !(job.total > 0));
  $("#jobBytes").textContent = job.total > 0 ? `${formatBytes(job.done)} / ${formatBytes(job.total)}` : "";
  const now = Date.now();
  let speed = "";
  if (job.state === "running" && state.lastJob === job.id && job.done > state.lastBytes.done) {
    const dt = (now - state.lastBytes.at) / 1000;
    if (dt > 0.2) speed = formatSpeed((job.done - state.lastBytes.done) / dt);
  }
  $("#jobSpeed").textContent = speed;
  state.lastBytes = { done: job.done, at: now };
  state.lastJob = job.id;
  $("#jobCancel").classList.toggle("hidden", job.state !== "running");
}

function hideJobSoon() {
  setTimeout(() => {
    if (state.watching) return;
    $("#jobbar").classList.add("hidden");
    state.lastJob = null;
  }, 1600);
}

async function watchJob(job) {
  if (!job) return;
  state.watching = job.id;
  showJob(job);
  pollJobs();
}

let polling = false;
async function pollJobs() {
  if (polling) return;
  polling = true;
  try {
    const jobs = await api("/api/jobs");
    const running = jobs.find((j) => j.state === "running");
    if (running) {
      state.watching = running.id;
      showJob(running);
      setTimeout(() => { polling = false; pollJobs(); }, 750);
      return;
    }
    if (state.watching) {
      const finished = jobs.find((j) => j.id === state.watching);
      state.watching = null;
      if (finished) {
        showJob(finished);
        if (finished.state === "error") toast(finished.error || "The job failed", true);
        else toast(finished.kind === "setup" ? "Emulator runtime ready" : "Done");
        await refreshAll();
      }
      hideJobSoon();
    }
    polling = false;
  } catch (err) {
    polling = false;
    if (state.watching) setTimeout(pollJobs, 1500);
  }
}

/* ----------------------------------------------------------------- dialogs */
function openDialog(title, bodyHTML, actions) {
  $("#dialogTitle").textContent = title;
  $("#dialogBody").innerHTML = bodyHTML;
  $("#dialogActions").innerHTML = actions.map((a, i) =>
    `<button class="btn ${a.cls || "text"}" type="button" data-dialog-action="${i}">${a.label}</button>`).join("");
  $("#dialogBackdrop").classList.remove("hidden");
  $("#dialogActions").onclick = (event) => {
    const btn = event.target.closest("[data-dialog-action]");
    if (!btn) return;
    const action = actions[Number(btn.dataset.dialogAction)];
    if (!action) return;
    if (action.onClick) action.onClick();
    if (action.close !== false) closeDialog();
  };
  applyIcons($("#dialogBackdrop"));
}

function closeDialog() {
  $("#dialogBackdrop").classList.add("hidden");
  $("#dialogBody").innerHTML = "";
  $("#dialogActions").innerHTML = "";
}

/* ------------------------------------------------------------------- wiring */
function wire() {
  $$(".nav-item").forEach((b) => b.addEventListener("click", () => setView(b.dataset.view)));
  $("#themeToggle").addEventListener("click", () => {
    applyTheme(document.documentElement.dataset.theme === "dark" ? "light" : "dark");
  });
  $("#refreshBtn").addEventListener("click", async () => {
    toast("Refreshing the catalog");
    try {
      await api("/api/catalog/refresh", { method: "POST" });
      pollJobs();
    } catch (err) { toast(err.message, true); }
  });
  $("#setupBtn").addEventListener("click", () => setView("setup"));
  $("#heroSetup").addEventListener("click", () => { setView("setup"); if (!state.status || !state.status.emulator) runSetup(); });
  $("#heroLibrary").addEventListener("click", () => setView("library"));
  $("#libraryRefresh").addEventListener("click", refreshAll);
  $("#newDeviceBtn").addEventListener("click", () => openNewDevice());
  $("#installRuntimeBtn").addEventListener("click", () => runSetup());
  $("#refreshStatusBtn").addEventListener("click", loadStatus);
  $("#refreshStatusBtn").addEventListener("click", () => toast("Runtime status refreshed"));
  $("#jobCancel").addEventListener("click", async () => {
    if (!state.watching) return;
    try { await api("/api/jobs/cancel", { method: "POST", body: { id: state.watching } }); toast("Canceling…"); }
    catch (err) { toast(err.message, true); }
  });
  $("#dialogClose").addEventListener("click", closeDialog);
  $("#dialogBackdrop").addEventListener("click", (e) => { if (e.target === $("#dialogBackdrop")) closeDialog(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape") closeDialog(); });

  $("#searchInput").addEventListener("input", (e) => {
    state.query = e.target.value;
    renderCatalog();
  });
  $("#tagFilter").addEventListener("click", (e) => {
    const btn = e.target.closest("[data-tag]");
    if (!btn) return;
    state.selectedTag = btn.dataset.tag;
    $$("#tagFilter .seg").forEach((b) => b.classList.toggle("active", b === btn));
    renderCatalog();
  });

  $("#catalogGrid").addEventListener("click", (e) => {
    const btn = e.target.closest("[data-action]");
    if (!btn) return;
    const action = btn.dataset.action;
    if (action === "select") {
      state.selections[Number(btn.dataset.api)] = btn.dataset.id;
      renderCatalog();
    } else if (action === "install") {
      installImage(btn.dataset.id);
    } else if (action === "launch") {
      launchImage(btn.dataset.id);
    } else if (action === "details") {
      openDetails(btn.dataset.id);
    }
  });

  $("#imageList").addEventListener("click", (e) => {
    const btn = e.target.closest("[data-action]");
    if (!btn) return;
    if (btn.dataset.action === "new-device") openNewDevice(btn.dataset.id);
    if (btn.dataset.action === "delete-image") deleteImage(btn.dataset.id);
  });

  $("#avdList").addEventListener("click", (e) => {
    const btn = e.target.closest("[data-action]");
    if (!btn) return;
    const name = btn.dataset.name;
    switch (btn.dataset.action) {
      case "start-avd": startAVD(name, btn.dataset.headless === "true"); break;
      case "stop-avd": stopAVD(name); break;
      case "log-avd": openLog(name); break;
      case "delete-avd": deleteAVD(name); break;
    }
  });
}

function init() {
  initTheme();
  applyIcons();
  wire();
  loadStatus();
  loadCatalog();
  loadLibrary();
  pollJobs();
  setInterval(() => {
    if (document.visibilityState === "visible" && !state.watching) loadStatus();
  }, 20000);
  setInterval(() => {
    if (document.visibilityState === "visible" && document.querySelector("#view-library:not(.hidden)")) loadLibrary();
  }, 5000);
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  }
}

document.addEventListener("DOMContentLoaded", init);
