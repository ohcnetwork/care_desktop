// CARE Desktop control app - installer + control panel, driven by the Go bridge
// (window.go.main.App) and Wails events. Fonts are bundled locally (offline);
// icons are inline SVG in index.html, so there is no icon font to ship.
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-sans/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "@fontsource/ibm-plex-mono/600.css";
import "./style.css";

const App = window.go.main.App;
const on = (event: string, cb: (...data: any[]) => void) => window.runtime.EventsOn(event, cb);
const $ = <T extends HTMLElement>(sel: string): T => {
  const el = document.querySelector<T>(sel);
  if (!el) throw new Error(`missing element: ${sel}`);
  return el;
};
const DB_ICON = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><ellipse cx="12" cy="6" rx="8" ry="3"></ellipse><path d="M4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6"></path></svg>`;
const esc = (s: string): string => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));

type DockerStatus = { ok: boolean; message: string };
type NameStatus = { ok: boolean; message: string; how: string };
type NetworkStatus = { applicable: boolean; ok: boolean; message: string; how: string; fixable: boolean };
type Health = { active: boolean; code: number; detail: string };
type AppState = { setup_done: boolean; mdns_name: string; docker: DockerStatus };
type Backup = { db_dump: string; files_archive: string; label: string; manual: boolean; encrypted: boolean; size_bytes: number };
type CarePlugin = { name: string; package_name: string; version?: string; configs?: Record<string, unknown> };
type State = "running" | "partial" | "stopped" | "unknown";
type Section = "backend" | "frontend";

let phase: "setup" | "panel" = "setup";
let busy = false, busyLabel = "";
let lastState: State = "unknown";
let mdnsName = "care.local";
let backups: Backup[] = [];

// ===========================================================================
// Shell - views, rail, toast, log buffer
// ===========================================================================
type View = "setup" | "installing" | "failed" | "panel";
function showView(v: View): void {
  for (const name of ["setup", "installing", "failed", "panel"] as View[]) $(`#view-${name}`).hidden = name !== v;
  const inPanel = v === "panel";
  $("#brand-kicker").textContent = inPanel ? "Control panel" : "First-time setup";
  $("#rail-steps").hidden = inPanel;
  $("#rail-nav").hidden = !inPanel;
  $("#rail-status").hidden = !inPanel;
  railStep(v === "installing" || v === "failed" ? "install" : openSection);
}

const toast = $("#toast"), toastMsg = $("#toast-msg");
let toastTimer = 0;
function firstLine(s: string): string { return s.split("\n")[0].trim(); }
function showToast(msg: string): void {
  toastMsg.textContent = msg;
  toast.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.hidden = true), 2600);
}

// Setup lines drive the progress bar (panel lines -> devtools console). Buffered so
// the fail screen can show the real build error, not just "exit status 1".
const setupLog: string[] = [];
function append(line: string): void {
  if (phase === "setup") {
    setupLog.push(line);
    if (setupLog.length > 300) setupLog.shift();
    bumpInstallProgress(line);
    return;
  }
  console.log(line);
}

// ===========================================================================
// Accordions
// ===========================================================================
// Setup sections open one at a time (the design's guided flow); the advanced
// sections in the panel are independent.
type SetupSection = "checks" | "backup" | "admin" | "install";
let openSection: SetupSection = "checks";

function setAcc(id: string, open: boolean): void {
  const body = document.getElementById(`${id}-body`);
  const head = document.querySelector<HTMLElement>(`[data-acc="${id}"]`);
  if (!body || !head) return;
  body.hidden = !open;
  head.parentElement!.classList.toggle("open", open);
}
function railStep(active: SetupSection): void {
  document.querySelectorAll<HTMLElement>(".rail-step").forEach((el) =>
    el.classList.toggle("active", el.dataset.step === active));
}
function railDone(step: SetupSection, done: boolean): void {
  const el = document.querySelector<HTMLElement>(`.rail-step[data-step="${step}"]`);
  if (!el) return;
  el.classList.toggle("done", done);
  el.querySelector(".rail-dot")!.innerHTML = done ? "&#10003;" : { checks: "1", backup: "2", admin: "3", install: "4" }[step];
}
function openSetupSection(id: SetupSection): void {
  openSection = id;
  for (const s of ["checks", "backup", "admin"] as SetupSection[]) setAcc(s, s === id);
  railStep(id);
}
document.querySelectorAll<HTMLElement>("[data-acc]").forEach((head) => {
  const id = head.dataset.acc!;
  head.addEventListener("click", () => {
    const closed = !!document.getElementById(`${id}-body`)?.hidden;
    const setup = id === "checks" || id === "backup" || id === "admin";
    if (setup && closed) openSetupSection(id as SetupSection);
    else setAcc(id, closed);
  });
});
document.querySelectorAll<HTMLElement>("[data-info]").forEach((btn) => {
  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    const box = $(`#${btn.dataset.info}`);
    box.hidden = !box.hidden;
  });
});

// ===========================================================================
// Setup - requirement checks
// ===========================================================================
type Check = { id: string; title: string; detail: string; state: "wait" | "ok" | "bad"; how: string; fixable: boolean };
const CHECKS: Check[] = [
  { id: "runtime", title: "Runtime engine", detail: "Docker, Compose and Git", state: "wait", how: "", fixable: false },
  { id: "mdns", title: "Network name", detail: "care.local on the clinic WiFi", state: "wait", how: "", fixable: false },
];
let networkCheck: Check | null = null; // Windows-only gate
let pwOk = false, pwcOk = false, bpwOk = false, bpwcOk = false;
let backupDir = "";
const install = $<HTMLButtonElement>("#install");

function visibleChecks(): Check[] { return networkCheck ? [...CHECKS, networkCheck] : CHECKS; }
function checksState(): "wait" | "ok" | "bad" {
  const all = visibleChecks();
  if (all.some((c) => c.state === "bad")) return "bad";
  return all.some((c) => c.state === "wait") ? "wait" : "ok";
}

function renderChecks(): void {
  const host = $("#check-rows");
  host.textContent = "";
  for (const c of visibleChecks()) {
    const row = document.createElement("div");
    row.className = "row " + (c.state === "bad" ? "bad" : "");
    row.innerHTML = `
      <span class="row-dot ${c.state}">${c.state === "ok" ? "&#10003;" : c.state === "bad" ? "&#10007;" : "&middot;"}</span>
      <div class="row-txt"><div class="t">${esc(c.title)}</div><div class="s">${esc(c.detail)}</div></div>
      <span class="pill ${c.state === "wait" ? "" : c.state}">${c.state === "wait" ? "Checking" : c.state === "ok" ? "Ready" : "Not ready"}</span>`;
    host.appendChild(row);
    if (c.state === "bad" && c.how) {
      const fix = document.createElement("div");
      fix.className = "row-fix";
      fix.innerHTML = `<span>${esc(c.how)}</span>`;
      if (c.fixable) {
        const btn = document.createElement("button");
        btn.className = "btn danger";
        btn.textContent = "Fix automatically";
        btn.addEventListener("click", () => void fixNetwork(btn));
        fix.appendChild(btn);
      }
      host.appendChild(fix);
    }
  }
  const state = checksState();
  const dot = $("#checks-dot");
  dot.className = "acc-dot" + (state === "ok" ? " done" : "");
  dot.innerHTML = state === "ok" ? "&#10003;" : "1";
  $("#checks-summary").textContent = state === "wait"
    ? "Checking this computer"
    : state === "ok" ? "Everything this clinic needs is ready" : "Open to see what failed";
  const badge = $("#checks-badge");
  badge.className = "pill " + (state === "wait" ? "" : state);
  const issues = visibleChecks().filter((c) => c.state === "bad").length;
  badge.textContent = state === "wait" ? "Checking" : state === "ok" ? "All good" : `${issues} issue${issues > 1 ? "s" : ""}`;
  $("#sec-checks").classList.toggle("bad", state === "bad");
  railDone("checks", state === "ok");
  gate();
}

function gate(): void {
  const ready = checksState() === "ok" && pwOk && pwcOk && bpwOk && bpwcOk;
  install.disabled = !ready;
  $("#install-note").textContent = checksState() !== "ok"
    ? "Waiting for your computer to be ready…"
    : !(bpwOk && bpwcOk) ? "Set and confirm the backup password to continue."
      : !(pwOk && pwcOk) ? "Set and confirm the admin password to continue."
        : "Ready. This takes about 10 to 20 minutes.";
}

async function checkRuntime(): Promise<void> {
  CHECKS[0].state = "wait"; renderChecks();
  const d: DockerStatus = await App.DockerStatus();
  const g: DockerStatus = await App.GitStatus();
  CHECKS[0].state = d.ok && g.ok ? "ok" : "bad";
  CHECKS[0].how = d.ok && g.ok ? "" : (!d.ok ? d.message : g.message);
  renderChecks();
}
async function checkMDNS(): Promise<void> {
  CHECKS[1].state = "wait"; renderChecks();
  const d: NameStatus = await App.MDNSStatus();
  CHECKS[1].state = d.ok ? "ok" : "bad";
  CHECKS[1].how = d.ok ? "" : (d.how || d.message);
  renderChecks();
}
// Windows-only Private-network gate; hidden (and non-blocking) when not applicable.
async function checkNetwork(): Promise<void> {
  const s: NetworkStatus = await App.NetworkStatus();
  networkCheck = s.applicable
    ? {
      id: "network", title: "Network profile", detail: "WiFi set to Private",
      state: s.ok ? "ok" : "bad", how: s.ok ? "" : (s.how || s.message), fixable: s.fixable,
    }
    : null;
  renderChecks();
}
async function fixNetwork(btn: HTMLButtonElement): Promise<void> {
  btn.disabled = true;
  btn.textContent = "Fixing…";
  try { await App.FixNetwork(); showToast("Network set to Private."); }
  catch (e) { showToast("Couldn't change the network: " + firstLine(String(e))); }
  await checkNetwork();
}
function recheck(): void { void Promise.all([checkRuntime(), checkMDNS(), checkNetwork()]); }
$("#check-reqs").addEventListener("click", (e) => { e.stopPropagation(); recheck(); });

$("#choose-backup").addEventListener("click", () => void (async () => {
  const sel = await App.ChooseFolder("Choose backup folder");
  if (sel) { backupDir = sel; $("#backup-path").textContent = sel; syncBackupSection(); }
})());

// ---------------------------------------------------------------------------
// Passwords: strength comes from the Go validator (single source of truth),
// the confirm field is a plain local comparison.
// ---------------------------------------------------------------------------
const DEFAULT_PW_MSG = "Use 8 to 20 characters with an uppercase letter, a lowercase letter and a number.";
function setMsg(el: HTMLElement, text: string, tone: "" | "ok" | "bad"): void {
  el.className = tone;
  el.textContent = text;
  el.hidden = text === "";
}
function wirePw(
  ids: { pw: string; confirm: string; wrap: string; cwrap: string; toggle: string; msg: string; cmsg: string; note: string },
  set: (ok: boolean, confirmOk: boolean) => void,
  after: () => void,
): void {
  const pw = $<HTMLInputElement>(`#${ids.pw}`), cf = $<HTMLInputElement>(`#${ids.confirm}`);
  $(`#${ids.toggle}`).addEventListener("click", () => {
    const show = pw.type === "password";
    pw.type = cf.type = show ? "text" : "password";
    $(`#${ids.toggle}`).textContent = show ? "Hide" : "Show";
  });
  let t = 0, strong = false;
  const paint = () => {
    const confirmOk = cf.value !== "" && cf.value === pw.value;
    $(`#${ids.wrap}`).className = "pw " + (pw.value === "" ? "" : strong ? "ok" : "bad");
    $(`#${ids.cwrap}`).className = "pw " + (cf.value === "" ? "" : confirmOk ? "ok" : "bad");
    const note = $(`#${ids.note}`);
    note.className = "pw-note " + (cf.value === "" ? "" : confirmOk ? "ok" : "bad");
    note.textContent = cf.value === "" ? "" : confirmOk ? "Match" : "No match";
    setMsg($(`#${ids.cmsg}`), cf.value === "" ? "" : confirmOk
      ? "Both passwords match."
      : "The two passwords are different. Retype the confirmation to match exactly, including capital letters.",
      confirmOk ? "ok" : "bad");
    set(strong, confirmOk);
    after();
    gate();
  };
  const validate = async () => {
    if (pw.value === "") {
      strong = false;
      setMsg($(`#${ids.msg}`), DEFAULT_PW_MSG, "");
    } else {
      const reason = await App.ValidatePassword(pw.value);
      strong = reason === "";
      setMsg($(`#${ids.msg}`), strong ? "Strong password." : reason, strong ? "ok" : "bad");
    }
    paint();
  };
  pw.addEventListener("input", () => { clearTimeout(t); t = window.setTimeout(() => void validate(), 180); });
  cf.addEventListener("input", paint);
}

function sectionDone(id: "backup" | "admin", done: boolean, n: string, summary: string): void {
  const dot = $(`#${id}-dot`);
  dot.className = "acc-dot" + (done ? " done" : "");
  dot.innerHTML = done ? "&#10003;" : n;
  const badge = $(`#${id}-badge`);
  badge.className = "pill" + (done ? " ok" : "");
  badge.textContent = done ? "Done" : "To do";
  $(`#${id}-summary`).textContent = summary;
  railDone(id, done);
}
function syncBackupSection(): void {
  const done = bpwOk && bpwcOk;
  sectionDone("backup", done, "2", done
    ? `${backupDir || "Desktop (default)"}, password set`
    : "Drive and password for daily backups");
}
function syncAdminSection(): void {
  const done = pwOk && pwcOk;
  sectionDone("admin", done, "3", done ? "admin, password set" : "The first login for this clinic");
}
wirePw(
  { pw: "backuppw", confirm: "backuppwc", wrap: "bpw-wrap", cwrap: "bpwc-wrap", toggle: "bpw-toggle", msg: "bpw-msg", cmsg: "bpwc-msg", note: "bpwc-note" },
  (ok, cok) => { bpwOk = ok; bpwcOk = cok; },
  () => { syncBackupSection(); },
);
wirePw(
  { pw: "adminpw", confirm: "adminpwc", wrap: "apw-wrap", cwrap: "apwc-wrap", toggle: "pw-toggle", msg: "apw-msg", cmsg: "apwc-msg", note: "apwc-note" },
  (ok, cok) => { pwOk = ok; pwcOk = cok; },
  syncAdminSection,
);

// ===========================================================================
// Setup - install flow
// ===========================================================================
type RunStep = { re: RegExp; pct: number; label: string };
const RUN_STEPS: RunStep[] = [
  { re: /secret key/i, pct: 8, label: "Preparing the configuration" },
  { re: /backup image|backup encryption key/i, pct: 15, label: "Securing the backups" },
  { re: /Building the Caddy/i, pct: 22, label: "Building the secure gateway" },
  { re: /Cloning care|care_fe|frontend \(/i, pct: 36, label: "Downloading CARE" },
  { re: /Building the backend image/i, pct: 62, label: "Building the backend" },
  { re: /Building the frontend image/i, pct: 80, label: "Building the app" },
  { re: /Starting CARE/i, pct: 90, label: "Starting the services" },
  { re: /database migrations/i, pct: 97, label: "Setting up the database" },
  { re: /become healthy|CARE is up/i, pct: 100, label: "Waiting for CARE to answer" },
];
let stepIdx = 0, pct = 0, elapsedTimer = 0, startedAt = 0;
let lastError = "";

function renderSteps(): void {
  const done = pct >= 100;
  $("#steps-list").innerHTML = RUN_STEPS.map((s, i) => {
    const state = done || i < stepIdx ? "done" : i === stepIdx ? "now" : "todo";
    const dot = state === "now"
      ? `<span class="step-dot now"><span class="ring"></span></span>`
      : `<span class="step-dot ${state}">${state === "done" ? "&#10003;" : "&middot;"}</span>`;
    return `<div class="step-row ${state}">${dot}<span class="label">${esc(s.label)}</span><span class="n">${state === "done" ? "done" : state === "now" ? "working" : ""}</span></div>`;
  }).join("");
  $("#details-summary").textContent = done ? `All ${RUN_STEPS.length} steps finished` : `Step ${stepIdx + 1} of ${RUN_STEPS.length}`;
  $("#step-label").textContent = done ? "Done" : RUN_STEPS[stepIdx].label;
  $("#install-pct").textContent = `${Math.floor(pct)}%`;
  $<HTMLDivElement>("#install-bar").style.width = `${pct}%`;
}
function bumpInstallProgress(line: string): void {
  for (let i = 0; i < RUN_STEPS.length; i++) {
    if (!RUN_STEPS[i].re.test(line)) continue;
    if (RUN_STEPS[i].pct <= pct) return;
    stepIdx = i;
    pct = RUN_STEPS[i].pct;
    renderSteps();
    return;
  }
}
function tickElapsed(): void {
  const s = Math.floor((Date.now() - startedAt) / 1000);
  $("#elapsed").textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}
function startInstalling(): void {
  stepIdx = 0; pct = 0;
  renderSteps();
  $("#run-ico").className = "run-ico";
  $("#run-ico").innerHTML = `<span class="ring"></span>`;
  $("#run-title").textContent = "Setting up your clinic";
  $("#run-sub").textContent = "This takes about 10 to 20 minutes.";
  $("#run-foot").hidden = true;
  startedAt = Date.now();
  tickElapsed();
  clearInterval(elapsedTimer);
  elapsedTimer = window.setInterval(tickElapsed, 1000);
  showView("installing");
}

install.addEventListener("click", () => {
  if (install.disabled) return;
  void (async () => {
    install.disabled = true;
    $("#install-note").textContent = "Re-checking your computer…";
    await Promise.all([checkRuntime(), checkMDNS(), checkNetwork()]);
    if (checksState() !== "ok" || !(pwOk && pwcOk && bpwOk && bpwcOk)) {
      $("#install-note").textContent = "A step is no longer met - fix it and try again.";
      return;
    }
    phase = "setup";
    startInstalling();
    append("Starting one-time setup...");
    void App.RunSetup("care.local", $<HTMLInputElement>("#adminpw").value, $<HTMLInputElement>("#backuppw").value, true, "", backupDir).catch((e) => {
      lastError = String(e); append(`error: ${String(e)}`); showInstallFailed();
    });
  })();
});

function showInstallFailed(): void {
  clearInterval(elapsedTimer);
  const tail = setupLog.slice(-40).join("\n").trim();
  const headline = (lastError || "Setup did not complete.").trim();
  $("#fail-msg").textContent = tail ? `${headline}\n\n---- last output ----\n${tail}` : headline;
  setAcc("fail", false);
  showView("failed");
}
function backToSetup(): void {
  lastError = ""; setupLog.length = 0;
  pct = 0; stepIdx = 0;
  renderSteps();
  showView("setup");
  openSetupSection("checks");
  recheck();
}
$("#fail-retry").addEventListener("click", () => void (async () => {
  const btn = $<HTMLButtonElement>("#fail-retry");
  btn.disabled = true;
  // Windows: wipe the half-staged kit so the retry re-stages clean. No-op elsewhere.
  try { await App.CleanupFailedInstall(); } catch (e) { console.log("cleanup: " + String(e)); }
  btn.disabled = false;
  backToSetup();
})());
$("#fail-back").addEventListener("click", backToSetup);
$("#open-panel").addEventListener("click", () => { phase = "panel"; showView("panel"); void bootPanel(); });

// ===========================================================================
// Panel - tabs + status
// ===========================================================================
const TAB_META: Record<string, [string, string]> = {
  overview: ["Overview", "Your clinic server at a glance."],
  backups: ["Backups", "Safe copies of your patient data."],
  advanced: ["Advanced", "Technical options for this clinic."],
};
function showTab(id: string): void {
  document.querySelectorAll<HTMLElement>(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.tab === id));
  for (const t of ["overview", "backups", "advanced"]) $(`#tab-${t}`).hidden = t !== id;
  const [title, sub] = TAB_META[id];
  $("#tab-title").textContent = title; $("#tab-sub").textContent = sub;
  if (id === "backups") void loadBackups();
  if (id === "advanced") showAdvanced();
}
document.querySelectorAll<HTMLElement>(".nav-item").forEach((b) => b.addEventListener("click", () => showTab(b.dataset.tab!)));

const SYS: Record<Exclude<State, "unknown">, { label: string; sub: string; glyph: string; cls: string }> = {
  running: { label: "Running", sub: "Live and reachable on the clinic WiFi.", glyph: "\u25cf", cls: "running" },
  stopped: { label: "Stopped", sub: "The clinic system is not running.", glyph: "\u25a0", cls: "" },
  partial: { label: "Starting…", sub: "Some services are still coming up.", glyph: "\u25a0", cls: "busy" },
};
function applyState(state: State): void {
  const ico = $("#sys-ico");
  if (busy) {
    ico.className = "status-ico busy";
    ico.innerHTML = `<span class="ring"></span>`;
    $("#sys-label").textContent = busyLabel + "…";
    $("#sys-sub").textContent = "Please wait a moment.";
    for (const id of ["#btn-start", "#btn-stop", "#btn-restart"]) $<HTMLButtonElement>(id).disabled = true;
    $("#rail-status-dot").className = "rail-status-dot busy";
    $("#rail-status-label").textContent = busyLabel + "…";
    lockPanel(true);
    return;
  }
  const s = state === "unknown" ? SYS.stopped : SYS[state];
  ico.className = "status-ico " + s.cls;
  ico.textContent = s.glyph;
  $("#sys-label").textContent = state === "unknown" ? "checking…" : s.label;
  $("#sys-sub").textContent = state === "unknown" ? "" : s.sub;
  const running = state === "running", partial = state === "partial", stopped = !running && !partial;
  const start = $<HTMLButtonElement>("#btn-start");
  start.disabled = running || partial;
  start.classList.toggle("primary", stopped);
  $<HTMLButtonElement>("#btn-stop").disabled = stopped;
  $<HTMLButtonElement>("#btn-restart").disabled = stopped;
  $("#rail-status-dot").className = "rail-status-dot " + (running ? "running" : partial ? "busy" : "stopped");
  $("#rail-status-label").textContent = running ? "Running" : partial ? "Starting…" : "Stopped";
  lockPanel(false);
}
function lockPanel(b: boolean): void {
  for (const id of ["#btn-backup-now", "#btn-backup-now2", "#backups-refresh", "#env-save", "#env-add", "#plugins-save", "#plugin-picker", "#btn-rebuild", "#uninstall-run"]) {
    const el = document.querySelector<HTMLButtonElement>(id);
    if (el) el.disabled = b;
  }
}

async function refresh(): Promise<void> {
  if (busy || $("#view-panel").hidden) return;
  let state: State;
  try {
    const h: Health = await App.CareHealth();
    if (h.active) state = "running";
    else { let ps = ""; try { ps = await App.CareStatus(); } catch { ps = ""; } state = ps.trim() ? "partial" : "stopped"; }
  } catch { state = "stopped"; }
  lastState = state;
  applyState(state);
}
function setBusy(b: boolean, label = ""): void { busy = b; busyLabel = label; applyState(lastState); }

async function run(action: string): Promise<void> {
  if (busy) return;
  const labels: Record<string, string> = { start: "Starting", stop: "Stopping", restart: "Restarting", "rebuild-frontend": "Rebuilding", "rebuild-backend": "Rebuilding", "backup-now": "Backing up" };
  setBusy(true, labels[action] || "Working");
  append(`\n$ care ${action}`);
  try { await App.CareAction(action); }
  catch (e) { append(`error: ${String(e)}`); setBusy(false); }
}
$("#btn-start").addEventListener("click", () => void run("start"));
$("#btn-stop").addEventListener("click", () => void run("stop"));
$("#btn-restart").addEventListener("click", () => void run("restart"));
$("#btn-backup-now").addEventListener("click", () => void run("backup-now"));
$("#btn-backup-now2").addEventListener("click", () => void run("backup-now"));
$("#btn-rebuild").addEventListener("click", () => void run("rebuild-frontend"));

// address open + copy
function openClinic(): void { void App.OpenURL(`https://${mdnsName}/`); }
$("#open-link").addEventListener("click", openClinic);
$("#open-link2").addEventListener("click", openClinic);
$("#open-device-setup").addEventListener("click", () => void App.OpenURL(`https://${mdnsName}/setup`));
$("#copy-addr").addEventListener("click", () => void navigator.clipboard.writeText(mdnsName).then(() => showToast("Address copied"), () => showToast(mdnsName)));
$("#view-backups").addEventListener("click", () => showTab("backups"));

// autostart
let autostartOn = false;
$("#autostart-label").addEventListener("click", () => void (async () => {
  const want = !autostartOn;
  try { await App.SetAutostart(want); showToast(want ? "Start at login on" : "Start at login off"); }
  catch (err) { append(`autostart error: ${String(err)}`); }
  await syncAutostart();
})());
async function syncAutostart(): Promise<void> {
  try {
    autostartOn = await App.AutostartEnabled();
    $("#autostart-label").classList.toggle("on", autostartOn);
  } catch { /* ignore */ }
}

// ===========================================================================
// Env editors (backend/frontend) - everyday + Advanced split
// ===========================================================================
type Entry = { kind: "comment" | "blank" | "kv"; raw?: string; key?: string; value?: string; isNew?: boolean };
function parseEnv(text: string): Entry[] {
  const lines = text.split(/\r?\n/);
  if (lines.length && lines[lines.length - 1] === "") lines.pop();
  return lines.map((line): Entry => {
    if (line.trim() === "") return { kind: "blank" };
    if (line.trimStart().startsWith("#")) return { kind: "comment", raw: line };
    const m = line.match(/^([A-Za-z_][A-Za-z0-9_]*)=(.*)$/);
    if (m) return { kind: "kv", key: m[1], value: m[2] };
    return { kind: "comment", raw: line };
  });
}
function serializeEnv(entries: Entry[]): string {
  return entries.map((e) => {
    if (e.kind === "comment") return e.raw ?? "";
    if (e.kind === "blank") return "";
    if (!e.key || e.key.trim() === "") return null;
    return `${e.key}=${e.value ?? ""}`;
  }).filter((l): l is string => l !== null).join("\n") + "\n";
}

const ADV_PREFIXES = ["POSTGRES_", "MINIO_", "BUCKET_", "CELERY_", "SNS_", "DJANGO_SECURE_", "REACT_PUBLIC", "REACT_SENTRY", "REACT_APP_META"];
const ADV_KEYS = new Set(["DATABASE_URL", "REDIS_URL", "DJANGO_SECRET_KEY", "DJANGO_SETTINGS_MODULE", "PYTHONPATH", "DJANGO_ALLOWED_HOSTS", "DJANGO_DEBUG", "DJANGO_ADMIN_URL", "CSRF_TRUSTED_ORIGINS", "FILE_UPLOAD_BUCKET", "FACILITY_S3_BUCKET"]);
function isAdvancedKey(key: string): boolean { return ADV_KEYS.has(key) || ADV_PREFIXES.some((p) => key.startsWith(p)); }

class EnvEditor {
  entries: Entry[] = [];
  name: Section = "backend";
  constructor(private everydayEl: HTMLElement, private advEl: HTMLElement) {}
  async load(name: Section): Promise<void> {
    this.name = name;
    try { this.entries = parseEnv(await App.ReadEnv(name)); }
    catch (e) { this.entries = [{ kind: "comment", raw: `# could not read ${name}.env: ${String(e)}` }]; }
    this.render();
  }
  private makeRow(e: Entry, idx: number): HTMLDivElement {
    const row = document.createElement("div"); row.className = "env-row";
    if (e.isNew) {
      const k = document.createElement("input");
      k.type = "text"; k.placeholder = "NEW_KEY"; k.value = e.key ?? ""; k.className = "env-key-input";
      k.addEventListener("input", () => (this.entries[idx].key = k.value));
      row.appendChild(k);
    } else {
      const l = document.createElement("label"); l.className = "env-key"; l.textContent = e.key ?? "";
      row.appendChild(l);
    }
    const v = document.createElement("input");
    v.type = "text"; v.value = e.value ?? ""; v.spellcheck = false;
    v.addEventListener("input", () => (this.entries[idx].value = v.value));
    row.appendChild(v);
    if (e.isNew) {
      const rm = document.createElement("button");
      rm.className = "x-btn"; rm.innerHTML = "&#215;"; rm.title = "remove";
      rm.addEventListener("click", () => { this.entries.splice(idx, 1); this.render(); });
      row.appendChild(rm);
    }
    return row;
  }
  render(): void {
    this.everydayEl.innerHTML = ""; this.advEl.innerHTML = "";
    let advCount = 0;
    this.entries.forEach((e, idx) => {
      if (e.kind !== "kv") return;
      if (e.key === "ADDITIONAL_PLUGS") return;
      const row = this.makeRow(e, idx);
      if (!e.isNew && isAdvancedKey(e.key ?? "")) { this.advEl.appendChild(row); advCount++; }
      else this.everydayEl.appendChild(row);
    });
    $("#adv-env-count").textContent = String(advCount);
  }
  add(): void { this.entries.push({ kind: "kv", key: "", value: "", isNew: true }); this.render(); }
  async save(): Promise<void> { await App.WriteEnv(this.name, serializeEnv(this.entries)); }
}
const envEditor = new EnvEditor($("#env-form"), $("#adv-env-form"));
$("#env-add").addEventListener("click", () => envEditor.add());
$("#adv-env-toggle").addEventListener("click", () => {
  const body = $("#adv-env-body");
  body.hidden = !body.hidden;
  $("#adv-env-toggle").classList.toggle("open", !body.hidden);
});
$("#env-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const be = envEditor.name === "backend";
    try {
      await envEditor.save();
      showToast(be ? "Settings applied" : "Rebuilding the app with new settings");
      await run(be ? "start" : "rebuild-frontend");
    } catch (e) { append(`error saving env: ${String(e)}`); showToast("Couldn't save settings"); }
  })();
});

// ===========================================================================
// Plugins - one table for both kinds
// ===========================================================================
// Backend plugins are pip packages baked into the backend image (ADDITIONAL_PLUGS,
// rebuild on save). Frontend plugins are CARE plug_config rows loaded by the
// browser at runtime (a database write, no rebuild). Same table, two adapters.
type ConfigRow = { key: string; value: string };
type Row = { name: string; url: string; version: string; configs: ConfigRow[]; meta?: Record<string, unknown> };
type CatalogEntry = { value: string; label: string; row: Row };
type FrontendPlugin = { slug: string; meta: Record<string, unknown> };

const BE_CATALOG: CatalogEntry[] = [{
  value: "care_notifications",
  label: "Notifications (care_notifications)",
  row: {
    name: "care_notifications",
    url: "git+https://github.com/ohcnetwork/care_notifications_be.git",
    version: "@main",
    configs: [{ key: "WEBPUSH_NOTIFICATIONS_ENABLED", value: "false" }],
  },
}];
const FE_CATALOG: CatalogEntry[] = [{
  value: "care_hello_fe",
  label: "Hello World (sample)",
  row: { name: "care_hello_fe", url: "https://ohcnetwork.github.io/care_hello_fe/assets/remoteEntry.js", version: "", configs: [], meta: {} },
}];

function parseConfigValue(v: string): unknown {
  const t = v.trim();
  if (t === "true") return true;
  if (t === "false") return false;
  if (/^-?\d+$/.test(t)) return parseInt(t, 10);
  if (/^-?\d*\.\d+$/.test(t)) return parseFloat(t);
  return v;
}

class PluginTable {
  rows: Row[] = [];
  kind: Section = "backend";
  private expanded = new Set<number>();
  // Frontend saves overwrite the whole plug_config set, so a save without a clean
  // baseline read could silently delete rows. Track whether we ever got one.
  private feLoaded = false;
  private feError = "";
  constructor(private el: HTMLElement) {}

  async load(kind: Section): Promise<void> {
    this.kind = kind;
    this.expanded.clear();
    if (kind === "backend") {
      try {
        const raw = await App.ReadPlugins();
        this.rows = raw.map((p) => ({
          name: p.name ?? "", url: p.package_name ?? "", version: p.version ?? "",
          configs: Object.entries(p.configs ?? {}).map(([key, value]) => ({ key, value: String(value) })),
        }));
      } catch { this.rows = []; }
      this.render();
      return;
    }
    this.feLoaded = false; this.feError = "";
    this.el.innerHTML = `<div class="plugins-table"><div class="list-empty">Checking…</div></div>`;
    let raw: FrontendPlugin[];
    try { raw = (await App.ReadFrontendPlugins()) ?? []; }
    catch (e) {
      this.feError = firstLine(String(e));
      this.rows = [];
      this.el.innerHTML = `<div class="plugins-table"><div class="list-empty">${esc(this.feError)}</div></div>`;
      return;
    }
    this.rows = raw.map((p) => {
      const meta = (p.meta ?? {}) as Record<string, unknown>;
      const cfg = (meta.config ?? {}) as Record<string, unknown>;
      return {
        name: p.slug, url: typeof meta.url === "string" ? meta.url : "", version: "",
        configs: Object.entries(cfg).map(([key, value]) => ({ key, value: String(value) })),
        meta,
      };
    });
    this.feLoaded = true;
    this.render();
  }

  private cell(cls: string, value: string, ph: string, onInput: (v: string) => void): HTMLInputElement {
    const inp = document.createElement("input");
    inp.className = cls; inp.type = "text"; inp.value = value; inp.placeholder = ph; inp.spellcheck = false;
    inp.addEventListener("input", () => onInput(inp.value));
    return inp;
  }

  render(): void {
    this.el.textContent = "";
    const head = document.createElement("div");
    head.className = "plugins-head";
    head.innerHTML = `<span class="c-name">Name</span><span class="c-url">Source</span><span class="c-version">Version</span><span class="c-configs">Configs</span><span style="width:26px; flex:none;"></span>`;
    this.el.appendChild(head);

    const table = document.createElement("div");
    table.className = "plugins-table";
    if (this.rows.length === 0) {
      const empty = document.createElement("div");
      empty.className = "list-empty";
      empty.textContent = "No plugins yet";
      table.appendChild(empty);
    }
    this.rows.forEach((p, i) => {
      const row = document.createElement("div");
      row.className = "plugin-row";
      row.appendChild(this.cell("c-name", p.name, this.kind === "backend" ? "care_hcx" : "care_hello_fe", (v) => (this.rows[i].name = v)));
      row.appendChild(this.cell("c-url", p.url, this.kind === "backend" ? "git+https://github.com/org/repo.git" : "https://host/assets/remoteEntry.js", (v) => (this.rows[i].url = v)));
      if (this.kind === "backend") {
        row.appendChild(this.cell("c-version", p.version, "@main", (v) => (this.rows[i].version = v)));
      } else {
        const dash = document.createElement("span");
        dash.className = "c-version faint mono"; dash.textContent = "latest";
        row.appendChild(dash);
      }
      const cfgBtn = document.createElement("button");
      cfgBtn.className = "c-configs";
      cfgBtn.textContent = `${p.configs.length} ${this.expanded.has(i) ? "▾" : "▸"}`;
      cfgBtn.title = "Show configuration";
      cfgBtn.addEventListener("click", () => {
        this.expanded.has(i) ? this.expanded.delete(i) : this.expanded.add(i);
        this.render();
      });
      row.appendChild(cfgBtn);
      const rm = document.createElement("button");
      rm.className = "x-btn"; rm.innerHTML = "&#215;"; rm.title = "remove plugin";
      rm.addEventListener("click", () => { this.rows.splice(i, 1); this.expanded.clear(); this.render(); });
      row.appendChild(rm);
      table.appendChild(row);

      if (!this.expanded.has(i)) return;
      const cfg = document.createElement("div");
      cfg.className = "plugin-cfg";
      p.configs.forEach((c, ci) => {
        const r = document.createElement("div"); r.className = "env-row";
        const k = document.createElement("input");
        k.type = "text"; k.className = "env-key-input"; k.placeholder = "CONFIG_KEY"; k.value = c.key; k.spellcheck = false;
        k.addEventListener("input", () => (this.rows[i].configs[ci].key = k.value));
        const v = document.createElement("input");
        v.type = "text"; v.placeholder = "value"; v.value = c.value; v.spellcheck = false;
        v.addEventListener("input", () => (this.rows[i].configs[ci].value = v.value));
        const crm = document.createElement("button");
        crm.className = "x-btn"; crm.innerHTML = "&#215;";
        crm.addEventListener("click", () => { this.rows[i].configs.splice(ci, 1); this.render(); });
        r.append(k, v, crm);
        cfg.appendChild(r);
      });
      const add = document.createElement("button");
      add.className = "btn"; add.textContent = "Add config"; add.style.alignSelf = "flex-start";
      add.addEventListener("click", () => { this.rows[i].configs.push({ key: "", value: "" }); this.render(); });
      cfg.appendChild(add);
      table.appendChild(cfg);
    });
    this.el.appendChild(table);
    $("#plugins-summary").textContent = `${this.rows.length} ${this.kind} plugin${this.rows.length === 1 ? "" : "s"}`;
  }

  addFromCatalog(entry: CatalogEntry): void {
    this.rows.push({ ...entry.row, configs: entry.row.configs.map((c) => ({ ...c })) });
    this.render();
  }
  addCustom(): void {
    this.rows.push({ name: "", url: "", version: "", configs: [], meta: this.kind === "frontend" ? {} : undefined });
    this.render();
  }

  private serializeBackend(): CarePlugin[] {
    return this.rows.filter((p) => p.name.trim() !== "" && p.url.trim() !== "").map((p) => {
      const out: CarePlugin = { name: p.name.trim(), package_name: p.url.trim() };
      if (p.version.trim() !== "") out.version = p.version.trim();
      const configs: Record<string, unknown> = {};
      for (const c of p.configs) if (c.key.trim() !== "") configs[c.key.trim()] = parseConfigValue(c.value);
      if (Object.keys(configs).length) out.configs = configs;
      return out;
    });
  }
  private serializeFrontend(): FrontendPlugin[] {
    return this.rows.filter((p) => p.name.trim() !== "" && p.url.trim() !== "").map((p) => {
      const meta: Record<string, unknown> = { ...(p.meta ?? {}), name: p.name.trim(), url: p.url.trim() };
      const config: Record<string, unknown> = {};
      for (const c of p.configs) if (c.key.trim() !== "") config[c.key.trim()] = parseConfigValue(c.value);
      if (Object.keys(config).length) meta.config = config; else delete meta.config;
      return { slug: p.name.trim(), meta };
    });
  }

  async save(): Promise<void> {
    if (this.kind === "backend") { await App.SavePlugins(this.serializeBackend()); return; }
    if (!this.feLoaded) {
      // The panel opened before CARE was ready, so we never got a clean snapshot.
      // Read one now - without a baseline a save could delete rows we failed to
      // read. Then keep the user's edits on top.
      const pending = this.rows;
      await this.load("frontend");
      if (!this.feLoaded) throw new Error(this.feError || "Couldn't read the current plugins from CARE - is it running?");
      const bySlug = new Map(this.rows.map((p) => [p.name, p]));
      for (const p of pending) if (p.name.trim() || p.url.trim()) bySlug.set(p.name, p);
      this.rows = [...bySlug.values()];
      this.render();
    }
    await App.SaveFrontendPlugins(this.serializeFrontend());
  }
}
const plugins = new PluginTable($("#plugins-list"));
const pluginPicker = $<HTMLSelectElement>("#plugin-picker");
function fillPicker(section: Section): void {
  const catalog = section === "backend" ? BE_CATALOG : FE_CATALOG;
  pluginPicker.innerHTML = `<option value="">Add a plugin</option>` +
    catalog.map((c) => `<option value="${c.value}">${esc(c.label)}</option>`).join("") +
    `<option value="custom">Custom, add by URL</option>`;
}
pluginPicker.addEventListener("change", () => {
  const v = pluginPicker.value;
  pluginPicker.value = "";
  if (!v) return;
  if (v === "custom") { plugins.addCustom(); return; }
  const entry = (plugins.kind === "backend" ? BE_CATALOG : FE_CATALOG).find((c) => c.value === v);
  if (entry) plugins.addFromCatalog(entry);
});
$("#plugins-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const be = plugins.kind === "backend";
    try {
      await plugins.save();
      if (be) { showToast("Rebuilding the backend"); await run("rebuild-backend"); }
      else { showToast("Frontend plugins saved - staff refresh their browser"); await plugins.load("frontend"); }
    } catch (e) { append(`error saving plugins: ${String(e)}`); showToast(firstLine(String(e))); }
  })();
});

// ===========================================================================
// Advanced - section switch, password gate, uninstall
// ===========================================================================
async function selectSection(section: Section): Promise<void> {
  const be = section === "backend";
  for (const id of ["#sel-backend", "#psel-backend"]) $(id).classList.toggle("on", be);
  for (const id of ["#sel-frontend", "#psel-frontend"]) $(id).classList.toggle("on", !be);
  $("#cfg-file").textContent = be ? "backend.env" : "frontend.env";
  $("#env-save").textContent = be ? "Save and apply" : "Save and rebuild app";
  $("#plugins-save").textContent = be ? "Save and rebuild backend" : "Save frontend plugins";
  const tag = $("#plugins-tag");
  tag.className = "pill " + (be ? "warn" : "ok");
  tag.textContent = be ? "rebuilds on save" : "instant";
  $("#adv-env-body").hidden = true;
  $("#adv-env-toggle").classList.remove("open");
  fillPicker(section);
  await envEditor.load(section);
  await plugins.load(section);
}
for (const id of ["#sel-backend", "#psel-backend"]) $(id).addEventListener("click", () => void selectSection("backend"));
for (const id of ["#sel-frontend", "#psel-frontend"]) $(id).addEventListener("click", () => void selectSection("frontend"));

let advUnlocked = false;
function showAdvanced(): void {
  $("#adv-gate").hidden = advUnlocked;
  $("#adv-home").hidden = !advUnlocked;
}
const advPw = $<HTMLInputElement>("#advpw");
$("#advpw-toggle").addEventListener("click", () => {
  const show = advPw.type === "password";
  advPw.type = show ? "text" : "password";
  $("#advpw-toggle").textContent = show ? "Hide" : "Show";
});
async function tryUnlock(): Promise<void> {
  if (advPw.value === "") {
    $("#adv-gate-err").hidden = false;
    $("#adv-gate-err").textContent = "Enter the admin password.";
    return;
  }
  const ok = await App.VerifyAdminPassword(advPw.value);
  if (!ok) {
    $("#advpw-wrap").className = "pw bad";
    $("#adv-gate-err").hidden = false;
    $("#adv-gate-err").textContent = "That password does not match the admin password.";
    return;
  }
  advUnlocked = true;
  advPw.value = "";
  $("#advpw-wrap").className = "pw";
  $("#adv-gate-err").hidden = true;
  showAdvanced();
  await selectSection("backend");
}
$("#adv-unlock").addEventListener("click", () => void tryUnlock());
advPw.addEventListener("keydown", (e) => { if (e.key === "Enter") void tryUnlock(); });

$("#uninstall-run").addEventListener("click", () => { $("#uninstall-idle").hidden = true; $("#uninstall-confirm").hidden = false; });
$("#uninstall-cancel").addEventListener("click", () => { $("#uninstall-idle").hidden = false; $("#uninstall-confirm").hidden = true; });
$("#uninstall-yes").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const rmBackups = $<HTMLInputElement>("#uninstall-backups").checked;
    const rmImages = $<HTMLInputElement>("#uninstall-images").checked;
    $("#uninstall-idle").hidden = false; $("#uninstall-confirm").hidden = true;
    setBusy(true, "Uninstalling");
    append(`\n$ care uninstall${rmImages ? " --images" : ""}${rmBackups ? " --backups" : ""} --yes`);
    try { await App.RunUninstall(rmImages, rmBackups); }
    catch (e) { append(`error: ${String(e)}`); setBusy(false); }
  })();
});

// ===========================================================================
// Backups + inline restore
// ===========================================================================
let confirmingRestore = -1;
function shortDate(label: string): string {
  const m = label.match(/^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}:\d{2})/);
  return m ? `${m[3]}/${m[2]} ${m[4]}` : label.split(" - ")[0] || label;
}
async function loadBackups(): Promise<void> {
  try { backups = await App.ListBackups(); } catch { backups = []; }
  $("#backups-summary").textContent = backups.length ? `${backups.length} kept` : "";
  if (backups.length) {
    $("#backup-strip-title").textContent = "Up to date";
    $("#backup-strip-sub").textContent = `Last ${shortDate(backups[0].label)}${backups[0].encrypted ? ", encrypted" : ""}`;
  } else {
    $("#backup-strip-title").textContent = "No backups yet";
    $("#backup-strip-sub").textContent = "Run one now or wait for the daily backup";
  }
  const list = $("#backups-list");
  if (backups.length === 0) {
    list.innerHTML = `<div class="list-empty">No backups yet. Click <b>Back up now</b> or wait for the daily backup.</div>`;
    return;
  }
  list.innerHTML = backups.map((b, i) => {
    const size = b.size_bytes ? (b.size_bytes / 1e6).toFixed(1) + " MB" : "";
    const meta = [size, b.files_archive ? "database and files" : "database only", b.encrypted ? "encrypted" : ""].filter(Boolean).join("  ·  ");
    const confirm = i === confirmingRestore ? `
      <div class="confirm">
        <span class="grow">Replace current data with this copy? This cannot be undone.</span>
        <button class="btn" data-cancel>Cancel</button>
        <button class="btn danger" data-confirm="${i}">Yes, restore</button>
      </div>` : "";
    return `<div class="backup-item">
      <div class="backup-row">
        <span class="backup-ico">${DB_ICON}</span>
        <div class="grow"><div class="backup-label">${esc(b.label)}</div><div class="backup-meta">${esc(meta)}</div></div>
        <span class="badge${b.manual ? " manual" : ""}">${b.manual ? "Manual" : "Automatic"}</span>
        <button class="btn" data-restore="${i}">Restore</button>
      </div>${confirm}
    </div>`;
  }).join("");
  list.querySelectorAll<HTMLButtonElement>("[data-restore]").forEach((btn) =>
    btn.addEventListener("click", () => { confirmingRestore = Number(btn.dataset.restore); void loadBackups(); }));
  list.querySelectorAll<HTMLButtonElement>("[data-cancel]").forEach((btn) =>
    btn.addEventListener("click", () => { confirmingRestore = -1; void loadBackups(); }));
  list.querySelectorAll<HTMLButtonElement>("[data-confirm]").forEach((btn) =>
    btn.addEventListener("click", () => void doRestore(Number(btn.dataset.confirm))));
}
async function doRestore(idx: number): Promise<void> {
  const b = backups[idx]; if (!b || busy) return;
  confirmingRestore = -1;
  setBusy(true, "Restoring");
  append(`\n$ care restore ${b.db_dump}${b.files_archive ? ` ${b.files_archive}` : ""}`);
  showToast("Restore started - data will be replaced");
  try { await App.RestoreBackup(b.db_dump, b.files_archive, "", false); }
  catch (e) { append(`error: ${String(e)}`); setBusy(false); }
}
$("#backups-refresh").addEventListener("click", () => { confirmingRestore = -1; void loadBackups(); });

// ===========================================================================
// Events
// ===========================================================================
on("care-log", (line: string) => { if (line.startsWith("error:")) lastError = line.slice("error:".length).trim(); append(line); });
on("care-done", (code: number) => {
  if (phase === "setup") { if (code !== 0) { append(`\nx Setup failed (exit ${code}).`); showInstallFailed(); } return; }
  append(`- done (exit ${code}) -`);
  if (code !== 0) showToast(lastError ? firstLine(lastError) : "That action didn't complete.");
  setBusy(false); void refresh(); void loadBackups();
});
on("setup-done", () => {
  clearInterval(elapsedTimer);
  pct = 100; stepIdx = RUN_STEPS.length - 1;
  renderSteps();
  $("#run-ico").className = "run-ico done";
  $("#run-ico").innerHTML = "&#10003;";
  $("#run-title").textContent = "Your clinic is ready";
  $("#run-sub").textContent = "CARE is running on this computer and reachable on the clinic WiFi.";
  $("#run-addr").textContent = mdnsName;
  $("#run-foot").hidden = false;
  railStep("install");
  railDone("install", true);
});
on("uninstalled", () => { showToast("Uninstalled"); setTimeout(() => window.location.reload(), 1800); });

// ===========================================================================
// Boot
// ===========================================================================
async function bootPanel(): Promise<void> {
  const state = await App.GetState();
  mdnsName = state.mdns_name || "care.local";
  $("#addr-name").textContent = mdnsName;
  $("#open-name").textContent = mdnsName;
  showTab("overview");
  await loadBackups();
  await refresh();
  await syncAutostart();
  try {
    if (await App.WasAutostartLaunched()) {
      const h = await App.CareHealth();
      if (!h.active && !busy) { append("\nLaunched at startup - starting CARE..."); await run("start"); }
    }
  } catch { /* ignore */ }
}
async function boot(): Promise<void> {
  renderSteps();
  const state: AppState = await App.GetState();
  if (state.setup_done) { phase = "panel"; showView("panel"); await bootPanel(); }
  else {
    phase = "setup";
    showView("setup");
    openSetupSection("checks");
    recheck();
  }
}
void boot();
setInterval(() => void refresh(), 5_000);
