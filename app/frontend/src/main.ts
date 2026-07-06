// CARE Desktop control app — installer + control panel, driven by the Go bridge
// (window.go.main.App) and Wails events (window.runtime).
//
// Fonts + icons are bundled locally (not from a CDN) so the app works offline.
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-sans/600.css";
import "@fontsource/ibm-plex-sans/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "@fontsource/ibm-plex-mono/600.css";
import "@phosphor-icons/web/regular";
import "@phosphor-icons/web/bold";
import "@phosphor-icons/web/fill";

const App = window.go.main.App;
const on = (event: string, cb: (...data: any[]) => void) => window.runtime.EventsOn(event, cb);

const $ = <T extends HTMLElement>(sel: string): T => {
  const el = document.querySelector<T>(sel);
  if (!el) throw new Error(`missing element: ${sel}`);
  return el;
};

type DockerStatus = { ok: boolean; message: string };
type NameStatus = { ok: boolean; message: string; how: string };
type Health = { active: boolean; code: number; detail: string };
type AppState = { setup_done: boolean; mdns_name: string; docker: DockerStatus };
type Backup = { db_dump: string; files_archive: string; label: string; manual: boolean; size_bytes: number };
type State = "running" | "partial" | "stopped" | "unknown";

let phase: "setup" | "panel" = "setup";
let busy = false;
let lastState: State = "unknown";
let mdnsName = "care.local";
let backups: Backup[] = [];

const wizard = $<HTMLDivElement>("#wizard");
const panel = $<HTMLDivElement>("#panel");
function showView(v: "wizard" | "panel"): void {
  wizard.hidden = v !== "wizard";
  panel.hidden = v !== "panel";
}

// ===========================================================================
// Log + toast
// ===========================================================================
const wlog = $<HTMLPreElement>("#wizard-log");
const logEl = $<HTMLPreElement>("#log");
function append(line: string): void {
  const el = phase === "setup" ? wlog : logEl;
  el.textContent += line + "\n";
  el.scrollTop = el.scrollHeight;
  if (phase === "setup") bumpInstallProgress(line);
}

const toast = $<HTMLDivElement>("#toast");
const toastMsg = $<HTMLSpanElement>("#toast-msg");
let toastTimer = 0;
function showToast(msg: string): void {
  toastMsg.textContent = msg;
  toast.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.hidden = true), 2600);
}

// ===========================================================================
// Wizard — checks
// ===========================================================================
let dockerOk = false, gitOk = false, mdnsOk = false;
let installDir = "", backupDir = "";

const install = $<HTMLButtonElement>("#install");

function setPill(id: string, state: "ok" | "checking" | "fail", label: string): void {
  const pill = $(`#${id}`);
  pill.className = "pill " + state;
  const icon = state === "ok" ? "ph-fill ph-check-circle" : state === "checking" ? "ph ph-circle-notch" : "ph-fill ph-x-circle";
  pill.innerHTML = `<i class="${icon}"></i>${label}`;
}

function gate(): void {
  install.disabled = !(dockerOk && gitOk && mdnsOk);
  $("#install-note").textContent = install.disabled
    ? "Complete the checks above to continue."
    : "Ready — this takes 10–20 minutes.";
}

async function checkDocker(): Promise<void> {
  setPill("docker-pill", "checking", "Checking…");
  const d: DockerStatus = await App.DockerStatus();
  dockerOk = d.ok;
  setPill("docker-pill", d.ok ? "ok" : "fail", d.ok ? "Ready" : "Not found");
  gate();
}

async function checkGit(): Promise<void> {
  setPill("git-pill", "checking", "Checking…");
  const d: DockerStatus = await App.GitStatus();
  gitOk = d.ok;
  setPill("git-pill", d.ok ? "ok" : "fail", d.ok ? "Ready" : "Not found");
  gate();
}

async function checkMDNS(): Promise<void> {
  setPill("mdns-pill", "checking", "Checking…");
  const d: NameStatus = await App.MDNSStatus();
  mdnsOk = d.ok;
  setPill("mdns-pill", d.ok ? "ok" : "fail", d.ok ? "Ready" : "Not found");
  const how = $("#mdns-how");
  how.hidden = d.ok;
  $("#mdns-how-text").textContent = d.how || "";
  gate();
}

$("#check-docker").addEventListener("click", () => void checkDocker());
$("#check-git").addEventListener("click", () => void checkGit());
$("#check-mdns").addEventListener("click", () => void checkMDNS());

$("#choose-install").addEventListener("click", () => {
  void (async () => {
    const sel = await App.ChooseFolder("Choose install folder");
    if (sel) { installDir = sel; $("#install-path").textContent = sel; }
  })();
});
$("#choose-backup").addEventListener("click", () => {
  void (async () => {
    const sel = await App.ChooseFolder("Choose backup folder");
    if (sel) { backupDir = sel; $("#backup-path").textContent = sel; }
  })();
});

// password show/hide
const pwInput = $<HTMLInputElement>("#adminpw");
$("#pw-toggle").addEventListener("click", () => {
  const show = pwInput.type === "password";
  pwInput.type = show ? "text" : "password";
  $("#pw-eye").className = show ? "ph ph-eye-slash" : "ph ph-eye";
});

// install
const wizForm = $<HTMLDivElement>("#wiz-form");
const wizInstalling = $<HTMLDivElement>("#wiz-installing");
const installBar = $<HTMLDivElement>("#install-bar");
const installPct = $<HTMLDivElement>("#install-pct");

// milestone-based progress derived from real engine log lines (setup emits no %).
const MILESTONES: [RegExp, number][] = [
  [/secret key/i, 8],
  [/Cloning care backend/i, 15],
  [/Cloning care frontend|Cloning care_fe|frontend \(/i, 25],
  [/Building the backend image/i, 40],
  [/Building the frontend image/i, 62],
  [/Starting CARE/i, 80],
  [/database migrations/i, 90],
  [/CARE is up|Setup done/i, 100],
];
function bumpInstallProgress(line: string): void {
  for (const [re, pct] of MILESTONES) {
    if (re.test(line)) {
      const cur = parseInt(installBar.style.width) || 0;
      if (pct > cur) {
        $(".progress").classList.remove("indet");
        installBar.style.width = pct + "%";
        installPct.textContent = pct + "%";
      }
      return;
    }
  }
}

install.addEventListener("click", () => {
  if (install.disabled) return;
  void (async () => {
    install.disabled = true;
    $("#install-note").textContent = "Re-checking requirements…";
    await Promise.all([checkDocker(), checkGit(), checkMDNS()]);
    if (!(dockerOk && gitOk && mdnsOk)) {
      $("#install-note").textContent = "A requirement is no longer met — fix the red step and try again.";
      return;
    }
    install.disabled = true;
    phase = "setup";
    wizForm.hidden = true;
    wizInstalling.hidden = false;
    $(".progress").classList.add("indet");
    installPct.textContent = "Working…";
    append("Starting one-time setup… (clones + builds the images; several minutes)");
    void App.RunSetup("care.local", pwInput.value, installDir, backupDir).catch((e) => append(`error: ${String(e)}`));
  })();
});

// ===========================================================================
// Panel — tabs
// ===========================================================================
const TAB_META: Record<string, [string, string]> = {
  overview: ["Overview", "Your clinic server at a glance."],
  settings: ["Settings", "Configuration for the backend and frontend."],
  backups: ["Backups", "Restore points for patient data and files."],
  logs: ["Activity log", "Live output from the CARE engine."],
  danger: ["Advanced", "Rebuild and uninstall."],
};
function showTab(id: string): void {
  document.querySelectorAll<HTMLElement>(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.tab === id));
  for (const t of ["overview", "settings", "backups", "logs", "danger"]) $(`#tab-${t}`).hidden = t !== id;
  const [title, sub] = TAB_META[id];
  $("#tab-title").textContent = title;
  $("#tab-sub").textContent = sub;
  if (id === "backups") void loadBackups();
}
document.querySelectorAll<HTMLElement>(".nav-item").forEach((b) =>
  b.addEventListener("click", () => showTab(b.dataset.tab!)),
);

// ===========================================================================
// Panel — status / overview
// ===========================================================================
const statusHero = $<HTMLDivElement>(".status-hero");
const btnStart = $<HTMLButtonElement>("#btn-start");
const btnStop = $<HTMLButtonElement>("#btn-stop");
const btnRestart = $<HTMLButtonElement>("#btn-restart");
const autostartCb = $<HTMLInputElement>("#autostart");

const SYS: Record<Exclude<State, "unknown">, { label: string; sub: string; icon: string; cls: string }> = {
  running: { label: "Running", sub: "The clinic system is live and reachable.", icon: "ph-fill ph-pulse", cls: "" },
  stopped: { label: "Stopped", sub: "The clinic system is not running.", icon: "ph-fill ph-stop-circle", cls: "stopped" },
  partial: { label: "Starting…", sub: "Some services are still coming up.", icon: "ph-fill ph-circle-notch", cls: "partial" },
};

function applyState(state: State): void {
  const s = state === "unknown" ? SYS.stopped : SYS[state];
  statusHero.className = "status-hero " + s.cls;
  $("#sys-icon").className = s.icon;
  $("#sys-label").textContent = state === "unknown" ? "checking…" : s.label;
  $("#sys-sub").textContent = state === "unknown" ? "" : s.sub;

  const running = state === "running", partial = state === "partial", stopped = state === "stopped";
  btnStart.disabled = busy || running || partial;
  btnStart.classList.toggle("primary", stopped && !busy);
  btnStop.disabled = busy || stopped;
  btnRestart.disabled = busy || stopped;

  $("#mini-dot").className = "mini-dot " + (running ? "running" : partial ? "partial" : stopped ? "stopped" : "");
  $("#mini-label").textContent = running ? "Running" : partial ? "Starting…" : stopped ? "Stopped" : "checking…";

  // panel-wide busy lock
  for (const id of ["#be-save", "#fe-save", "#be-add", "#fe-add", "#btn-backup-now", "#btn-rebuild-frontend",
    "#uninstall-run", "#backups-refresh"]) {
    ($(id) as HTMLButtonElement).disabled = busy;
  }
  btnStop.disabled = busy || stopped;
}

async function refresh(): Promise<void> {
  if (busy || panel.hidden) return;
  let state: State;
  try {
    const h: Health = await App.CareHealth();
    if (h.active) state = "running";
    else {
      let ps = "";
      try { ps = await App.CareStatus(); } catch { ps = ""; }
      state = ps.trim() ? "partial" : "stopped";
      renderServices(ps);
    }
  } catch { state = "stopped"; }
  if (state === "running") {
    try { renderServices(await App.CareStatus()); } catch { /* ignore */ }
  }
  lastState = state;
  applyState(state);
}

function renderServices(ps: string): void {
  const grid = $("#services-grid");
  const lines = ps.split("\n").map((l) => l.trim()).filter(Boolean);
  if (lines.length === 0) {
    grid.innerHTML = `<div class="svc"><span class="svc-dot down"></span><span class="svc-state">not running</span></div>`;
    $("#stat-services").textContent = "0 up";
    return;
  }
  let up = 0;
  grid.innerHTML = lines
    .map((l) => {
      const [name, ...rest] = l.split(/\s+/);
      const state = rest.join(" ") || "";
      const ok = /running|healthy|up/i.test(state);
      if (ok) up++;
      return `<div class="svc"><span class="svc-dot ${ok ? "up" : "down"}"></span><span class="svc-name">${name}</span><span class="svc-state">${state}</span></div>`;
    })
    .join("");
  $("#stat-services").textContent = `${up} / ${lines.length} up`;
}

function setBusy(b: boolean): void {
  busy = b;
  applyState(lastState);
}

async function run(action: string, note?: string): Promise<void> {
  if (busy) return;
  setBusy(true);
  append(`\n$ care ${action}${note ? `   # ${note}` : ""}`);
  try {
    await App.CareAction(action);
  } catch (e) {
    append(`error: ${String(e)}`);
    setBusy(false);
  }
}

btnStart.addEventListener("click", () => void run("start"));
btnStop.addEventListener("click", () => void run("stop"));
btnRestart.addEventListener("click", () => void run("restart"));
$("#btn-rebuild-frontend").addEventListener("click", () => void run("rebuild-frontend"));
$("#btn-backup-now").addEventListener("click", () => void run("backup-now"));

// address open + copy
function openClinic(): void { void App.OpenURL(`http://${mdnsName}/`); }
$("#open-link").addEventListener("click", openClinic);
$("#open-link2").addEventListener("click", openClinic);
$("#copy-addr").addEventListener("click", () => {
  void navigator.clipboard.writeText(mdnsName).then(
    () => showToast("Address copied to clipboard"),
    () => showToast(mdnsName),
  );
});

// autostart
$("#autostart-label").addEventListener("click", (e) => {
  e.preventDefault();
  void (async () => {
    const want = !autostartCb.checked;
    try {
      await App.SetAutostart(want);
      showToast(want ? "Start at login: on" : "Start at login: off");
    } catch (err) {
      append(`autostart error: ${String(err)}`);
    }
    await syncAutostart();
  })();
});
async function syncAutostart(): Promise<void> {
  try {
    const enabled = await App.AutostartEnabled();
    autostartCb.checked = enabled;
    $("#autostart-label").classList.toggle("on", enabled);
  } catch { /* ignore */ }
}

// ===========================================================================
// Panel — .env editors
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
  const out = entries
    .map((e) => {
      if (e.kind === "comment") return e.raw ?? "";
      if (e.kind === "blank") return "";
      if (!e.key || e.key.trim() === "") return null;
      return `${e.key}=${e.value ?? ""}`;
    })
    .filter((l): l is string => l !== null);
  return out.join("\n") + "\n";
}

class EnvEditor {
  entries: Entry[] = [];
  constructor(private name: "backend" | "frontend", private container: HTMLElement) {}

  async load(): Promise<void> {
    try { this.entries = parseEnv(await App.ReadEnv(this.name)); }
    catch (e) { this.entries = [{ kind: "comment", raw: `# could not read ${this.name}.env: ${String(e)}` }]; }
    this.render();
  }
  render(): void {
    this.container.innerHTML = "";
    this.entries.forEach((e, idx) => {
      if (e.kind !== "kv") return;
      const row = document.createElement("div");
      row.className = "env-row";
      if (e.isNew) {
        const k = document.createElement("input");
        k.type = "text"; k.placeholder = "NEW_KEY"; k.value = e.key ?? ""; k.className = "env-key-input";
        k.addEventListener("input", () => (this.entries[idx].key = k.value));
        row.appendChild(k);
      } else {
        const label = document.createElement("label");
        label.className = "env-key"; label.textContent = e.key ?? "";
        row.appendChild(label);
      }
      const v = document.createElement("input");
      v.type = "text"; v.value = e.value ?? ""; v.spellcheck = false;
      v.addEventListener("input", () => (this.entries[idx].value = v.value));
      row.appendChild(v);
      if (e.isNew) {
        const rm = document.createElement("button");
        rm.className = "btn ghost env-remove"; rm.textContent = "×"; rm.title = "remove";
        rm.addEventListener("click", () => { this.entries.splice(idx, 1); this.render(); });
        row.appendChild(rm);
      }
      this.container.appendChild(row);
    });
  }
  add(): void { this.entries.push({ kind: "kv", key: "", value: "", isNew: true }); this.render(); }
  async save(): Promise<void> { await App.WriteEnv(this.name, serializeEnv(this.entries)); }
}

const beEditor = new EnvEditor("backend", $("#be-form"));
const feEditor = new EnvEditor("frontend", $("#fe-form"));
$("#be-add").addEventListener("click", () => beEditor.add());
$("#fe-add").addEventListener("click", () => feEditor.add());
$("#be-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    try { await beEditor.save(); append("\nsaved backend.env"); showToast("Backend settings applied"); await run("start", "recreate backend with new settings"); }
    catch (e) { append(`error saving backend.env: ${String(e)}`); }
  })();
});
$("#fe-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    try { await feEditor.save(); append("\nsaved frontend.env"); showToast("Rebuilding app with new settings"); await run("rebuild-frontend", "rebuild app image with new settings"); }
    catch (e) { append(`error saving frontend.env: ${String(e)}`); }
  })();
});

// ===========================================================================
// Panel — backups + restore
// ===========================================================================
async function loadBackups(): Promise<void> {
  try { backups = await App.ListBackups(); } catch { backups = []; }
  const list = $("#backups-list");
  $("#stat-backups").textContent = String(backups.length);
  $("#stat-lastbackup").textContent = backups.length ? shortDate(backups[0].label) : "—";
  if (backups.length === 0) {
    list.innerHTML = `<div class="backups-empty">No backups yet. Click <b>Backup now</b> or wait for the daily backup.</div>`;
    return;
  }
  list.innerHTML = backups
    .map((b, i) => {
      const size = b.size_bytes ? (b.size_bytes / 1e6).toFixed(1) + " MB" : "";
      const files = b.files_archive ? "DB + files" : "DB only";
      const badge = b.manual ? `<span class="badge manual">Manual</span>` : `<span class="badge">Automatic</span>`;
      return `<div class="backup-row">
        <div class="backup-ic"><i class="ph ph-database"></i></div>
        <div class="backup-info"><div class="backup-label">${b.label}</div><div class="backup-meta">${[size, files].filter(Boolean).join(" · ")}</div></div>
        ${badge}
        <button class="btn" data-restore="${i}"><i class="ph ph-arrow-counter-clockwise"></i>Restore</button>
      </div>`;
    })
    .join("");
  list.querySelectorAll<HTMLButtonElement>("[data-restore]").forEach((btn) =>
    btn.addEventListener("click", () => void runRestore(Number(btn.dataset.restore))),
  );
}

function shortDate(label: string): string {
  // labels look like "2026-07-04 15:04 · daily · DB + files"
  const m = label.match(/^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}:\d{2})/);
  return m ? `${m[3]}/${m[2]} ${m[4]}` : label.split(" · ")[0] || label;
}

async function runRestore(idx: number): Promise<void> {
  if (busy) return;
  const b = backups[idx];
  if (!b) return;
  const ok = await App.ConfirmRestore(b.files_archive !== "");
  if (!ok) return;
  setBusy(true);
  append(`\n$ care restore ${b.db_dump}${b.files_archive ? ` ${b.files_archive}` : ""}   # replaces current data`);
  showToast("Restore started — data will be replaced");
  try { await App.RestoreBackup(b.db_dump, b.files_archive); }
  catch (e) { append(`error: ${String(e)}`); setBusy(false); }
}

$("#backups-refresh").addEventListener("click", () => void loadBackups());
$("#clear-log").addEventListener("click", () => { logEl.textContent = ""; });

// ===========================================================================
// Panel — uninstall
// ===========================================================================
$("#uninstall-run").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const rmBackups = ($("#uninstall-backups") as HTMLInputElement).checked;
    const rmImages = ($("#uninstall-images") as HTMLInputElement).checked;
    const ok = await App.ConfirmUninstall(rmBackups);
    if (!ok) return;
    setBusy(true);
    append(`\n$ care uninstall${rmImages ? " --images" : ""}${rmBackups ? " --backups" : ""} --yes   # removes everything`);
    try { await App.RunUninstall(rmImages, rmBackups); }
    catch (e) { append(`error: ${String(e)}`); setBusy(false); }
  })();
});

// ===========================================================================
// Events
// ===========================================================================
on("care-log", (line: string) => append(line));
on("care-done", (code: number) => {
  if (phase === "setup") {
    if (code !== 0) {
      append(`\n✖ Setup failed (exit ${code}). Fix the issue above and try again.`);
      wizInstalling.hidden = true;
      wizForm.hidden = false;
      install.disabled = false;
    }
    return; // success handled by setup-done
  }
  append(`— done (exit ${code}) —`);
  setBusy(false);
  void refresh();
  void loadBackups();
});
on("setup-done", () => {
  append("\n✔ Setup complete — opening the control panel…");
  installBar.style.width = "100%";
  installPct.textContent = "100%";
  phase = "panel";
  showView("panel");
  void bootPanel();
});
on("uninstalled", () => {
  append("\n✔ Uninstalled — resetting to first-run setup…");
  showToast("Uninstalled");
  setTimeout(() => window.location.reload(), 1800);
});

// ===========================================================================
// Boot
// ===========================================================================
async function bootPanel(): Promise<void> {
  const state = await App.GetState();
  mdnsName = state.mdns_name || "care.local";
  $("#addr-name").textContent = mdnsName;
  $("#open-name").textContent = mdnsName;
  showTab("overview");
  await beEditor.load();
  await feEditor.load();
  await loadBackups();
  await refresh();
  await syncAutostart();
  try {
    if (await App.WasAutostartLaunched()) {
      const h = await App.CareHealth();
      if (!h.active && !busy) { append("\nLaunched at startup — starting CARE…"); await run("start"); }
    }
  } catch { /* ignore */ }
}

async function boot(): Promise<void> {
  const state: AppState = await App.GetState();
  if (state.setup_done) {
    phase = "panel";
    showView("panel");
    await bootPanel();
  } else {
    phase = "setup";
    showView("wizard");
    await Promise.all([checkDocker(), checkGit(), checkMDNS()]);
  }
}

void boot();
setInterval(() => void refresh(), 5_000);
