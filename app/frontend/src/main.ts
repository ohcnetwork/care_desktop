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
type Backup = { db_dump: string; files_archive: string; label: string; manual: boolean; encrypted: boolean; size_bytes: number };
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
// Setup lines drive the progress bar (panel lines → devtools console). Buffered so
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

const toast = $<HTMLDivElement>("#toast");
const toastMsg = $<HTMLSpanElement>("#toast-msg");
let toastTimer = 0;
// the engine's errors can be multi-line (e.g. the port-80 message); a toast is one
// line, so show just the headline.
function firstLine(s: string): string {
  return s.split("\n")[0].trim();
}
function showToast(msg: string): void {
  toastMsg.textContent = msg;
  toast.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.hidden = true), 2600);
}

// ===========================================================================
// Wizard — checks
// ===========================================================================
let dockerOk = false, gitOk = false, mdnsOk = false, pwOk = false, bpwOk = false;
let installDir = "", backupDir = "";

const install = $<HTMLButtonElement>("#install");

function setPill(id: string, state: "ok" | "checking" | "fail", label: string): void {
  const pill = $(`#${id}`);
  pill.className = "pill " + state;
  const icon = state === "ok" ? "ph-fill ph-check-circle" : state === "checking" ? "ph ph-circle-notch" : "ph-fill ph-x-circle";
  pill.innerHTML = `<i class="${icon}"></i>${label}`;
}

function gate(): void {
  const checksOk = dockerOk && gitOk && mdnsOk;
  install.disabled = !(checksOk && pwOk && bpwOk);
  $("#install-note").textContent = !checksOk
    ? "Complete the checks above to continue."
    : !pwOk
      ? "Set a strong admin password to continue."
      : !bpwOk
        ? "Set a backup password to continue."
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

const pwHint = $<HTMLDivElement>("#pw-hint");
const pwHintDefault = pwHint.textContent ?? "";
let pwTimer = 0;
async function validatePw(): Promise<void> {
  const pw = pwInput.value;
  if (pw === "") {
    pwOk = false;
    pwHint.className = "pw-hint";
    pwHint.textContent = pwHintDefault;
    gate();
    return;
  }
  const reason = await App.ValidatePassword(pw);
  pwOk = reason === "";
  pwHint.className = "pw-hint " + (pwOk ? "ok" : "bad");
  pwHint.textContent = pwOk ? "Looks good — strong enough." : reason;
  gate();
}
pwInput.addEventListener("input", () => {
  clearTimeout(pwTimer);
  pwTimer = window.setTimeout(() => void validatePw(), 180);
});

// backup password — same strength policy as the admin password (reuses ValidatePassword)
const backupPwInput = $<HTMLInputElement>("#backuppw");
$("#bpw-toggle").addEventListener("click", () => {
  const show = backupPwInput.type === "password";
  backupPwInput.type = show ? "text" : "password";
  $("#bpw-eye").className = show ? "ph ph-eye-slash" : "ph ph-eye";
});
const bpwHint = $<HTMLDivElement>("#bpw-hint");
const bpwHintDefault = bpwHint.textContent ?? "";
let bpwTimer = 0;
async function validateBackupPw(): Promise<void> {
  const pw = backupPwInput.value;
  if (pw === "") {
    bpwOk = false;
    bpwHint.className = "pw-hint";
    bpwHint.textContent = bpwHintDefault;
    gate();
    return;
  }
  const reason = await App.ValidatePassword(pw);
  bpwOk = reason === "";
  bpwHint.className = "pw-hint " + (bpwOk ? "ok" : "bad");
  bpwHint.textContent = bpwOk ? "Looks good. Store it somewhere safe — it can't be recovered." : reason;
  gate();
}
backupPwInput.addEventListener("input", () => {
  clearTimeout(bpwTimer);
  bpwTimer = window.setTimeout(() => void validateBackupPw(), 180);
});

// install
const wizForm = $<HTMLDivElement>("#wiz-form");
const wizInstalling = $<HTMLDivElement>("#wiz-installing");
const wizFailed = $<HTMLDivElement>("#wiz-failed");
const failMsg = $<HTMLPreElement>("#fail-msg");
const installBar = $<HTMLDivElement>("#install-bar");
const installPct = $<HTMLDivElement>("#install-pct");
const installStep = $<HTMLSpanElement>("#install-step-text");
// last "error: …" line the engine streamed — shown on the failed screen.
let lastError = "";

// milestone-based progress derived from real engine log lines (setup emits no %).
// The label replaces the console that used to sit on this screen: one plain-English
// line saying what the 10–20 minute wait is doing right now.
const MILESTONES: [RegExp, number, string][] = [
  [/secret key/i, 6, "Preparing the configuration…"],
  [/Building the backup image/i, 9, "Preparing encrypted backups…"],
  [/Generating the backup encryption key/i, 11, "Securing your backups…"],
  [/Building the Caddy/i, 13, "Building the security proxy…"],
  [/Cloning care backend/i, 16, "Downloading CARE (backend)…"],
  [/Cloning care frontend|Cloning care_fe|frontend \(/i, 25, "Downloading CARE (app)…"],
  [/Building the backend image/i, 40, "Building the backend — this is the long one…"],
  [/Building the frontend image/i, 62, "Building the app…"],
  [/Starting CARE/i, 80, "Starting the services…"],
  [/database migrations/i, 88, "Setting up the database…"],
  // 100% only on the post-health "CARE is up" line — NOT "Setup done." (which
  // Setup() logs before Start() has even brought the stack up).
  [/become healthy/i, 94, "Waiting for CARE to answer…"],
  [/CARE is up/i, 100, "Ready."],
];
function bumpInstallProgress(line: string): void {
  for (const [re, pct, label] of MILESTONES) {
    if (re.test(line)) {
      const cur = parseInt(installBar.style.width) || 0;
      if (pct > cur) {
        $(".progress").classList.remove("indet");
        installBar.style.width = pct + "%";
        installPct.textContent = pct + "%";
        installStep.textContent = label;
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
    if (!(dockerOk && gitOk && mdnsOk && pwOk && bpwOk)) {
      $("#install-note").textContent = "A requirement is no longer met — fix the red step and try again.";
      return;
    }
    install.disabled = true;
    phase = "setup";
    wizForm.hidden = true;
    wizInstalling.hidden = false;
    $(".progress").classList.add("indet");
    installPct.textContent = "Working…";
    installStep.textContent = "Getting started…";
    append("Starting one-time setup… (clones + builds the images; several minutes)");
    // rememberBackup=true: installer is the only place the password is entered, so
    // restore reads it from the keychain. A sync rejection needs the fail screen too.
    void App.RunSetup(
      "care.local", pwInput.value, backupPwInput.value, true, installDir, backupDir,
    ).catch((e) => {
      lastError = String(e);
      append(`error: ${String(e)}`);
      showInstallFailed();
    });
  })();
});

// Fail screen: headline + tail of real build output (not just "exit status 1").
function showInstallFailed(): void {
  wizInstalling.hidden = true;
  wizForm.hidden = true;
  const tail = setupLog.slice(-40).join("\n").trim();
  const headline = (lastError || "Setup did not complete.").trim();
  failMsg.textContent = tail ? `${headline}\n\n———— last output ————\n${tail}` : headline;
  wizFailed.hidden = false;
}

$("#fail-retry").addEventListener("click", () => {
  lastError = "";
  setupLog.length = 0;
  installBar.style.width = "0%";
  installPct.textContent = "Working…";
  installStep.textContent = "Getting started…";
  $(".progress").classList.remove("indet");
  wizFailed.hidden = true;
  wizInstalling.hidden = true;
  wizForm.hidden = false;
  // re-run the prerequisite checks; gate() re-enables Install if they pass.
  void Promise.all([checkDocker(), checkGit(), checkMDNS()]);
});

// ===========================================================================
// Panel — tabs
// ===========================================================================
const TAB_META: Record<string, [string, string]> = {
  overview: ["Overview", "Your clinic server at a glance."],
  settings: ["Settings", "Configuration for the backend and frontend."],
  backups: ["Backups", "Restore points for patient data and files."],
  danger: ["Advanced", "Rebuild and uninstall."],
};
function showTab(id: string): void {
  document.querySelectorAll<HTMLElement>(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.tab === id));
  for (const t of ["overview", "settings", "backups", "danger"]) $(`#tab-${t}`).hidden = t !== id;
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
  for (const id of ["#be-save", "#fe-save", "#be-add", "#fe-add", "#plugin-picker", "#plugins-save",
    "#btn-backup-now", "#btn-rebuild-frontend", "#uninstall-run", "#backups-refresh"]) {
    ($(id) as HTMLButtonElement).disabled = busy;
  }
  btnStop.disabled = busy || stopped;
}

async function refresh(): Promise<void> {
  if (busy || panel.hidden) return;
  let state: State;
  try {
    const h: Health = await App.CareHealth();
    // Health alone can't tell "still coming up" from "not running at all", so when
    // it's down we ask compose whether anything exists — the only thing the
    // container list is still needed for.
    if (h.active) state = "running";
    else {
      let ps = "";
      try { ps = await App.CareStatus(); } catch { ps = ""; }
      state = ps.trim() ? "partial" : "stopped";
    }
  } catch { state = "stopped"; }
  lastState = state;
  applyState(state);
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

// System/infrastructure keys — hidden under "Advanced" so the everyday view stays clean.
const ADV_PREFIXES = ["POSTGRES_", "MINIO_", "BUCKET_", "CELERY_", "SNS_", "DJANGO_SECURE_"];
const ADV_KEYS = new Set([
  "DATABASE_URL", "REDIS_URL", "DJANGO_SECRET_KEY", "DJANGO_SETTINGS_MODULE", "PYTHONPATH",
  "DJANGO_ALLOWED_HOSTS", "DJANGO_DEBUG", "DJANGO_ADMIN_URL", "CSRF_TRUSTED_ORIGINS",
  "FILE_UPLOAD_BUCKET", "FACILITY_S3_BUCKET",
]);
function isAdvancedKey(key: string): boolean {
  return ADV_KEYS.has(key) || ADV_PREFIXES.some((p) => key.startsWith(p));
}

class EnvEditor {
  entries: Entry[] = [];
  constructor(private name: "backend" | "frontend", private container: HTMLElement) {}

  async load(): Promise<void> {
    try { this.entries = parseEnv(await App.ReadEnv(this.name)); }
    catch (e) { this.entries = [{ kind: "comment", raw: `# could not read ${this.name}.env: ${String(e)}` }]; }
    this.render();
  }
  private makeRow(e: Entry, idx: number): HTMLDivElement {
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
    return row;
  }
  render(): void {
    this.container.innerHTML = "";
    const advanced = document.createElement("div");
    advanced.className = "advanced-body"; advanced.hidden = true;
    let advCount = 0;
    this.entries.forEach((e, idx) => {
      if (e.kind !== "kv") return;
      if (e.key === "ADDITIONAL_PLUGS") return; // managed by the Plugins subsection
      const row = this.makeRow(e, idx);
      if (!e.isNew && isAdvancedKey(e.key ?? "")) { advanced.appendChild(row); advCount++; }
      else this.container.appendChild(row);
    });
    if (advCount > 0) {
      const toggle = document.createElement("button");
      toggle.className = "btn ghost adv-toggle";
      toggle.innerHTML = `<i class="ph ph-caret-right"></i>Advanced — system configuration (${advCount})`;
      toggle.addEventListener("click", () => {
        advanced.hidden = !advanced.hidden;
        toggle.querySelector("i")!.className = advanced.hidden ? "ph ph-caret-right" : "ph ph-caret-down";
      });
      this.container.appendChild(toggle);
      this.container.appendChild(advanced);
    }
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
// Panel — plugins (ADDITIONAL_PLUGS). Saving rebuilds the backend.
// ===========================================================================
// CarePlugin: local mirror of the bridge type (renamed to dodge the DOM's Plugin).
type CarePlugin = { name: string; package_name: string; version?: string; configs?: Record<string, unknown> };
type PluginConfigRow = { key: string; value: string };
type PluginRow = { name: string; package_name: string; version: string; configs: PluginConfigRow[] };

// Curated catalog of known plugins with a working config baked in, so a clinic can
// add one without knowing its package URL or required settings. "Custom" stays for
// anything not listed.
type CatalogEntry = { label: string; name: string; package_name: string; version: string; configs: PluginConfigRow[] };
const PLUGIN_CATALOG: CatalogEntry[] = [
  {
    label: "Notifications (care_notifications)",
    name: "care_notifications",
    package_name: "git+https://github.com/ohcnetwork/care_notifications_be.git",
    version: "@main",
    // web push needs VAPID keys the clinic doesn't have — off by default (typed false).
    configs: [{ key: "WEBPUSH_NOTIFICATIONS_ENABLED", value: "false" }],
  },
];

// Config values keep their JSON type: false/true → boolean, digits → number, else string.
// A boolean false actually disables a plugin flag; the string "false" would be truthy.
function parseConfigValue(v: string): unknown {
  const t = v.trim();
  if (t === "true") return true;
  if (t === "false") return false;
  if (/^-?\d+$/.test(t)) return parseInt(t, 10);
  if (/^-?\d*\.\d+$/.test(t)) return parseFloat(t);
  return v;
}

class PluginEditor {
  plugins: PluginRow[] = [];
  constructor(private container: HTMLElement) {}

  async load(): Promise<void> {
    try {
      const raw = await App.ReadPlugins();
      this.plugins = raw.map((p) => ({
        name: p.name ?? "",
        package_name: p.package_name ?? "",
        version: p.version ?? "",
        configs: Object.entries(p.configs ?? {}).map(([key, value]) => ({ key, value: String(value) })),
      }));
    } catch { this.plugins = []; }
    this.render();
  }

  private field(label: string, placeholder: string, value: string, onInput: (v: string) => void): HTMLElement {
    const wrap = document.createElement("div");
    wrap.className = "plugin-field";
    const lab = document.createElement("label");
    lab.textContent = label;
    const inp = document.createElement("input");
    inp.type = "text"; inp.placeholder = placeholder; inp.value = value; inp.spellcheck = false;
    inp.addEventListener("input", () => onInput(inp.value));
    wrap.append(lab, inp);
    return wrap;
  }

  render(): void {
    this.container.innerHTML = "";
    if (this.plugins.length === 0) {
      const empty = document.createElement("div");
      empty.className = "plugins-empty";
      empty.textContent = "No plugins yet. Click “Add plugin” to extend CARE.";
      this.container.appendChild(empty);
      return;
    }
    this.plugins.forEach((p, pi) => {
      const card = document.createElement("div");
      card.className = "plugin-card";

      const head = document.createElement("div");
      head.className = "plugin-head";
      const name = document.createElement("input");
      name.className = "plugin-name"; name.placeholder = "plugin name (e.g. hcx)"; name.value = p.name; name.spellcheck = false;
      name.addEventListener("input", () => (this.plugins[pi].name = name.value));
      const rm = document.createElement("button");
      rm.className = "btn ghost env-remove"; rm.textContent = "×"; rm.title = "remove plugin";
      rm.addEventListener("click", () => { this.plugins.splice(pi, 1); this.render(); });
      head.append(name, rm);
      card.appendChild(head);

      card.appendChild(this.field("Package URL", "git+https://github.com/ohcnetwork/care_hcx.git",
        p.package_name, (v) => (this.plugins[pi].package_name = v)));
      card.appendChild(this.field("Version (optional)", "@master or ==1.2.3",
        p.version, (v) => (this.plugins[pi].version = v)));

      const cfgLabel = document.createElement("div");
      cfgLabel.className = "plugin-cfg-label"; cfgLabel.textContent = "Configuration";
      card.appendChild(cfgLabel);

      const cfgForm = document.createElement("div");
      cfgForm.className = "env-form";
      p.configs.forEach((c, ci) => {
        const row = document.createElement("div");
        row.className = "env-row";
        const k = document.createElement("input");
        k.type = "text"; k.placeholder = "CONFIG_KEY"; k.value = c.key; k.className = "env-key-input";
        k.addEventListener("input", () => (this.plugins[pi].configs[ci].key = k.value));
        const v = document.createElement("input");
        v.type = "text"; v.placeholder = "value"; v.value = c.value; v.spellcheck = false;
        v.addEventListener("input", () => (this.plugins[pi].configs[ci].value = v.value));
        const crm = document.createElement("button");
        crm.className = "btn ghost env-remove"; crm.textContent = "×"; crm.title = "remove";
        crm.addEventListener("click", () => { this.plugins[pi].configs.splice(ci, 1); this.render(); });
        row.append(k, v, crm);
        cfgForm.appendChild(row);
      });
      card.appendChild(cfgForm);

      const addCfg = document.createElement("button");
      addCfg.className = "btn ghost tiny plugin-add-cfg";
      addCfg.innerHTML = `<i class="ph ph-plus"></i>Add config`;
      addCfg.addEventListener("click", () => { this.plugins[pi].configs.push({ key: "", value: "" }); this.render(); });
      card.appendChild(addCfg);

      this.container.appendChild(card);
    });
  }

  add(): void {
    this.plugins.push({ name: "", package_name: "", version: "", configs: [] });
    this.render();
  }
  addFromCatalog(c: CatalogEntry): void {
    this.plugins.push({
      name: c.name, package_name: c.package_name, version: c.version,
      configs: c.configs.map((cf) => ({ ...cf })),
    });
    this.render();
  }

  serialize(): CarePlugin[] {
    return this.plugins
      .filter((p) => p.name.trim() !== "" && p.package_name.trim() !== "")
      .map((p) => {
        const out: CarePlugin = { name: p.name.trim(), package_name: p.package_name.trim() };
        if (p.version.trim() !== "") out.version = p.version.trim();
        const configs: Record<string, unknown> = {};
        for (const c of p.configs) if (c.key.trim() !== "") configs[c.key.trim()] = parseConfigValue(c.value);
        if (Object.keys(configs).length) out.configs = configs;
        return out;
      });
  }

  async save(): Promise<void> { await App.SavePlugins(this.serialize()); }
}

const pluginEditor = new PluginEditor($("#plugins-list"));
// Picker: catalog entries + a "Custom" escape hatch. Selecting one adds it, then resets.
const pluginPicker = $<HTMLSelectElement>("#plugin-picker");
pluginPicker.innerHTML =
  `<option value="">+ Add a plugin…</option>` +
  PLUGIN_CATALOG.map((c, i) => `<option value="cat:${i}">${c.label}</option>`).join("") +
  `<option value="custom">Custom — add by URL</option>`;
pluginPicker.addEventListener("change", () => {
  const v = pluginPicker.value;
  if (v === "custom") pluginEditor.add();
  else if (v.startsWith("cat:")) pluginEditor.addFromCatalog(PLUGIN_CATALOG[Number(v.slice(4))]);
  pluginPicker.value = "";
});
$("#plugins-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    try {
      await pluginEditor.save();
      append("\nsaved plugins (ADDITIONAL_PLUGS)");
      showToast("Rebuilding backend with new plugins");
      await run("rebuild-backend", "rebuild backend with new plugins");
    } catch (e) { append(`error saving plugins: ${String(e)}`); showToast("Couldn't save plugins"); }
  })();
});

// ===========================================================================
// Panel — backups + restore
// ===========================================================================
async function loadBackups(): Promise<void> {
  try { backups = await App.ListBackups(); } catch { backups = []; }
  const list = $("#backups-list");
  $("#backups-summary").textContent = backups.length
    ? `${backups.length} kept · last ${shortDate(backups[0].label)}`
    : "";
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
  // "" passphrase → Go reads it from the keychain (set at install).
  try { await App.RestoreBackup(b.db_dump, b.files_archive, "", false); }
  catch (e) { append(`error: ${String(e)}`); setBusy(false); }
}

$("#backups-refresh").addEventListener("click", () => void loadBackups());

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
on("care-log", (line: string) => {
  if (line.startsWith("error:")) lastError = line.slice("error:".length).trim();
  append(line);
});
on("care-done", (code: number) => {
  if (phase === "setup") {
    if (code !== 0) {
      append(`\n✖ Setup failed (exit ${code}).`);
      showInstallFailed();
    }
    return; // success handled by setup-done
  }
  append(`— done (exit ${code}) —`);
  // A native error dialog already popped from the Go side; leave an in-app trace
  // too, since that dialog is gone once dismissed and the log tab may be closed.
  if (code !== 0) showToast(lastError ? firstLine(lastError) : "That action didn't complete.");
  setBusy(false);
  void refresh();
  void loadBackups();
});
on("setup-done", () => {
  append("\n✔ Setup complete — opening the control panel…");
  installBar.style.width = "100%";
  installPct.textContent = "100%";
  installStep.textContent = "Done — opening the control panel…";
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
  await pluginEditor.load();
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
