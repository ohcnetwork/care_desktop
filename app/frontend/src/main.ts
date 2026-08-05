// CARE Desktop control app - installer + control panel, driven by the Go bridge
// (window.go.main.App) and Wails events. Fonts + icons are bundled locally (offline).
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
import "./style.css";

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
type AppState = { setup_done: boolean; origin: string; docker: DockerStatus };
type Backup = { db_dump: string; files_archive: string; label: string; manual: boolean; encrypted: boolean; size_bytes: number };
type CarePlugin = { name: string; package_name: string; version?: string; configs?: Record<string, unknown> };
type State = "running" | "partial" | "stopped" | "unknown";

let phase: "setup" | "panel" = "setup";
let busy = false, busyLabel = "";
let lastState: State = "unknown";
// Where the clinic opens - https://<the clinic's domain>. Read from the engine so
// the Open button and the copyable address always follow the live tls.env.
let clinicOrigin = "";
let clinicHost = "";
let backups: Backup[] = [];

function showView(v: "wizard" | "panel"): void {
  $("#wizard").hidden = v !== "wizard";
  $("#panel").hidden = v !== "panel";
}

// ===========================================================================
// Toast + setup log buffer
// ===========================================================================
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
// Wizard - requirement checks
// ===========================================================================
let runtimeOk = false, addrOk = false, pwOk = false, bpwOk = false;
let backupDir = "";
const install = $<HTMLButtonElement>("#install");

function setReq(id: string, state: "ok" | "wait" | "bad", label: string, how = ""): void {
  const ico = $(`#${id}-ico`);
  ico.className = "req-ico " + state;
  ico.innerHTML = `<i class="${state === "ok" ? "ph-fill ph-check-circle" : state === "wait" ? "ph ph-circle-notch spin" : "ph-fill ph-x-circle"}"></i>`;
  const badge = $(`#${id}-badge`);
  badge.className = "req-badge " + state;
  badge.textContent = label;
  const howEl = $(`#${id}-how`);
  howEl.hidden = !how;
  howEl.textContent = how;
}

function gate(): void {
  install.disabled = !(runtimeOk && addrOk && pwOk && bpwOk);
  $("#install-note").textContent = !runtimeOk
    ? "Waiting for your computer to be ready..."
    : !addrOk ? "Enter your clinic's web address and check it to continue."
      : !pwOk ? "Set a strong admin password to continue."
        : !bpwOk ? "Set a backup password to continue."
          : "Ready. This takes about 10 to 20 minutes.";
}

async function checkRuntime(): Promise<void> {
  setReq("runtime", "wait", "Checking...");
  const d: DockerStatus = await App.DockerStatus();
  const g: DockerStatus = await App.GitStatus();
  runtimeOk = d.ok && g.ok;
  if (runtimeOk) setReq("runtime", "ok", "Ready");
  else setReq("runtime", "bad", "Not ready", !d.ok ? d.message : g.message);
  gate();
}
// checkAddress asks the engine whether the domain is shaped right, whether Cloudflare
// accepts the token, and whether the name already points at this computer. It talks to
// Cloudflare and to DNS, so it runs on demand rather than on every keystroke.
async function checkAddress(): Promise<void> {
  const host = $<HTMLInputElement>("#publichost").value.trim();
  const token = $<HTMLInputElement>("#dnstoken").value.trim();
  if (!host && !token) { addrOk = false; setReq("addr", "wait", "Not set"); gate(); return; }
  setReq("addr", "wait", "Checking...");
  const d: NameStatus = await App.CheckAddress(host, token);
  addrOk = d.ok;
  setReq("addr", d.ok ? "ok" : "bad", d.ok ? "Ready" : "Not ready", d.ok ? "" : (d.how || d.message));
  gate();
}
function recheck(): void { void checkRuntime(); void checkAddress(); }
$("#check-reqs").addEventListener("click", recheck);
$("#addr-check").addEventListener("click", () => void checkAddress());
// Editing either field invalidates the last result - the badge must never keep
// saying Ready for a value that is no longer what would be installed.
for (const id of ["#publichost", "#dnstoken"]) {
  $(id).addEventListener("input", () => { addrOk = false; setReq("addr", "wait", "Not checked"); gate(); });
}
$("#tok-toggle").addEventListener("click", () => {
  const t = $<HTMLInputElement>("#dnstoken");
  const show = t.type === "password";
  t.type = show ? "text" : "password";
  $("#tok-eye").className = show ? "ph ph-eye-slash" : "ph ph-eye";
});

$("#choose-backup").addEventListener("click", () => void (async () => {
  const sel = await App.ChooseFolder("Choose backup folder");
  if (sel) { backupDir = sel; $("#backup-path").textContent = sel; }
})());

// password fields (admin + backup), sharing one validator
function wirePw(input: HTMLInputElement, toggle: string, eye: string, hint: HTMLDivElement, okMsg: string, set: (ok: boolean) => void): void {
  const def = hint.textContent ?? "";
  $(`#${toggle}`).addEventListener("click", () => {
    const show = input.type === "password";
    input.type = show ? "text" : "password";
    $(`#${eye}`).className = show ? "ph ph-eye-slash" : "ph ph-eye";
  });
  let t = 0;
  const validate = async () => {
    if (input.value === "") { set(false); hint.className = "hint"; hint.textContent = def; gate(); return; }
    const reason = await App.ValidatePassword(input.value);
    const ok = reason === "";
    set(ok); hint.className = "hint " + (ok ? "ok" : "bad"); hint.textContent = ok ? okMsg : reason; gate();
  };
  input.addEventListener("input", () => { clearTimeout(t); t = window.setTimeout(() => void validate(), 180); });
}
wirePw($("#adminpw"), "pw-toggle", "pw-eye", $("#pw-hint"), "Looks good - strong enough.", (v) => (pwOk = v));
wirePw($("#backuppw"), "bpw-toggle", "bpw-eye", $("#bpw-hint"), "Looks good. Store it somewhere safe - it can't be recovered.", (v) => (bpwOk = v));

// ===========================================================================
// Wizard - install flow
// ===========================================================================
const wizForm = $("#wiz-form"), wizInstalling = $("#wiz-installing"), wizFailed = $("#wiz-failed");
const installBar = $<HTMLDivElement>("#install-bar"), installPct = $("#install-pct"), installStep = $("#install-step-text");
let lastError = "";

const MILESTONES: [RegExp, number, string][] = [
  [/secret key/i, 6, "Preparing the configuration..."],
  [/Building the backup image/i, 9, "Preparing encrypted backups..."],
  [/Generating the backup encryption key/i, 11, "Securing your backups..."],
  [/Building the Caddy/i, 13, "Building the secure gateway..."],
  [/Cloning care backend/i, 16, "Downloading CARE..."],
  [/Cloning care frontend|Cloning care_fe|frontend \(/i, 25, "Downloading the app..."],
  [/Building the backend image/i, 42, "Building the backend - this is the long one..."],
  [/Building the frontend image/i, 64, "Building the app..."],
  [/Starting CARE/i, 82, "Starting the services..."],
  [/database migrations/i, 90, "Setting up the database..."],
  [/become healthy/i, 95, "Waiting for CARE to answer..."],
  [/CARE is up/i, 100, "Ready."],
];
function bumpInstallProgress(line: string): void {
  for (const [re, pct, label] of MILESTONES) {
    if (re.test(line)) {
      const cur = parseInt(installBar.style.width) || 0;
      if (pct > cur) {
        $("#install-progress").classList.remove("indet");
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
    $("#install-note").textContent = "Re-checking your computer...";
    await checkRuntime();
    await checkAddress();
    if (!(runtimeOk && addrOk && pwOk && bpwOk)) {
      $("#install-note").textContent = "A step is no longer met - fix it and try again.";
      return;
    }
    phase = "setup";
    wizForm.hidden = true; wizInstalling.hidden = false;
    $("#install-progress").classList.add("indet");
    installPct.textContent = "Working...";
    installStep.textContent = "Getting started...";
    append("Starting one-time setup...");
    void App.RunSetup(
      $<HTMLInputElement>("#publichost").value.trim(),
      $<HTMLInputElement>("#dnstoken").value.trim(),
      $<HTMLInputElement>("#adminpw").value,
      $<HTMLInputElement>("#backuppw").value,
      true, "", backupDir,
    ).catch((e) => {
      lastError = String(e); append(`error: ${String(e)}`); showInstallFailed();
    });
  })();
});

function showInstallFailed(): void {
  wizInstalling.hidden = true; wizForm.hidden = true;
  const tail = setupLog.slice(-40).join("\n").trim();
  const headline = (lastError || "Setup did not complete.").trim();
  $("#fail-msg").textContent = tail ? `${headline}\n\n---- last output ----\n${tail}` : headline;
  wizFailed.hidden = false;
}
$("#fail-retry").addEventListener("click", () => {
  lastError = ""; setupLog.length = 0;
  installBar.style.width = "0%"; installPct.textContent = "Working..."; installStep.textContent = "Getting started...";
  $("#install-progress").classList.remove("indet");
  wizFailed.hidden = true; wizInstalling.hidden = true; wizForm.hidden = false;
  recheck();
});

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

const SYS: Record<Exclude<State, "unknown">, { label: string; sub: string; icon: string; cls: string }> = {
  running: { label: "Running", sub: "The clinic system is live and reachable.", icon: "ph-fill ph-pulse", cls: "" },
  stopped: { label: "Stopped", sub: "The clinic system is not running.", icon: "ph-fill ph-stop-circle", cls: "stopped" },
  partial: { label: "Starting...", sub: "Some services are still coming up.", icon: "ph ph-circle-notch", cls: "partial" },
};
function applyState(state: State): void {
  const hero = $(".status-hero");
  if (busy) {
    hero.className = "status-hero partial";
    $("#sys-icon").className = "ph ph-circle-notch spin";
    $("#sys-label").textContent = busyLabel + "...";
    $("#sys-sub").textContent = "Please wait a moment.";
    $("#hero-actions").hidden = true; $("#busy-banner").hidden = false;
    $("#busy-text").textContent = `${busyLabel} the clinic system. This usually takes a few seconds, please wait.`;
    $("#mini-dot").className = "mini-dot busy"; $("#mini-label").textContent = busyLabel + "...";
    lockPanel(true);
    return;
  }
  $("#hero-actions").hidden = false; $("#busy-banner").hidden = true;
  const s = state === "unknown" ? SYS.stopped : SYS[state];
  hero.className = "status-hero " + s.cls;
  $("#sys-icon").className = s.icon + (state === "partial" ? " spin" : "");
  $("#sys-label").textContent = state === "unknown" ? "checking..." : s.label;
  $("#sys-sub").textContent = state === "unknown" ? "" : s.sub;
  const running = state === "running", partial = state === "partial", stopped = !running && !partial;
  ($("#btn-start") as HTMLButtonElement).disabled = running || partial;
  $("#btn-start").classList.toggle("primary", stopped);
  ($("#btn-stop") as HTMLButtonElement).disabled = stopped;
  ($("#btn-restart") as HTMLButtonElement).disabled = stopped;
  $("#mini-dot").className = "mini-dot " + (running ? "running" : partial ? "busy" : "stopped");
  $("#mini-label").textContent = running ? "Running" : partial ? "Starting..." : "Stopped";
  lockPanel(false);
}
function lockPanel(b: boolean): void {
  for (const id of ["#btn-backup-now", "#backups-refresh", "#env-save", "#env-add", "#plugins-save", "#plugin-picker", "#btn-rebuild", "#uninstall-run"]) {
    const el = document.querySelector<HTMLButtonElement>(id);
    if (el) el.disabled = b;
  }
}

async function refresh(): Promise<void> {
  if (busy || $("#panel").hidden) return;
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
$("#btn-rebuild").addEventListener("click", () => void run("rebuild-frontend"));

// address open + copy
function openClinic(): void { void App.OpenURL(`${clinicOrigin}/`); }
$("#open-link").addEventListener("click", openClinic);
$("#open-link2").addEventListener("click", openClinic);
$("#copy-addr").addEventListener("click", () => void navigator.clipboard.writeText(clinicHost).then(() => showToast("Address copied"), () => showToast(clinicHost)));

// refreshOrigin re-reads the live origin after HTTPS settings change, so the panel
// never keeps offering an address that now just redirects.
async function refreshOrigin(origin?: string): Promise<void> {
  try {
    clinicOrigin = origin ?? await App.PublicOrigin();
    clinicHost = clinicOrigin.replace(/^https?:\/\//, "");
    $("#addr-name").textContent = clinicHost || "not set up yet";
    $("#open-name").textContent = clinicHost || "CARE";
    $("#addr-secure").hidden = !clinicOrigin.startsWith("https://");
  } catch { /* keep showing the current address */ }
}
$("#view-backups").addEventListener("click", () => showTab("backups"));

// autostart
const autostartCb = $<HTMLInputElement>("#autostart");
$("#autostart-label").addEventListener("click", (e) => {
  e.preventDefault();
  void (async () => {
    const want = !autostartCb.checked;
    try { await App.SetAutostart(want); showToast(want ? "Start at login: on" : "Start at login: off"); }
    catch (err) { append(`autostart error: ${String(err)}`); }
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
const ADV_KEYS = new Set(["DATABASE_URL", "REDIS_URL", "DJANGO_SECRET_KEY", "DJANGO_SETTINGS_MODULE", "PYTHONPATH", "DJANGO_ALLOWED_HOSTS", "DJANGO_DEBUG", "DJANGO_ADMIN_URL", "CSRF_TRUSTED_ORIGINS", "FILE_UPLOAD_BUCKET", "FACILITY_S3_BUCKET", "CARE_ACME_CA"]);
function isAdvancedKey(key: string): boolean { return ADV_KEYS.has(key) || ADV_PREFIXES.some((p) => key.startsWith(p)); }

type EnvName = "backend" | "frontend" | "tls";

class EnvEditor {
  entries: Entry[] = [];
  name: EnvName = "backend";
  constructor(private everydayEl: HTMLElement, private advEl: HTMLElement) {}
  async load(name: EnvName): Promise<void> {
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
      rm.className = "btn ghost env-remove"; rm.textContent = "x"; rm.title = "remove";
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
  $("#adv-env-caret").className = body.hidden ? "ph ph-caret-right" : "ph ph-caret-down";
});
$("#env-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const name = envEditor.name;
    try {
      await envEditor.save();
      if (name === "tls") {
        // start re-reads tls.env, swaps in the matching Caddyfile, obtains the
        // certificate, and rebuilds the frontend when the origin changed (Vite
        // bakes the API URL in at build time, so a restart alone isn't enough).
        showToast("Applying HTTPS settings — this can take several minutes");
        await run("start");
        await refreshOrigin();
      } else if (name === "backend") {
        showToast("Backend settings applied");
        await run("start");
      } else {
        showToast("Rebuilding app with new settings");
        await run("rebuild-frontend");
      }
    } catch (e) { append(`error saving env: ${String(e)}`); showToast("Couldn't save settings"); }
  })();
});

// ===========================================================================
// Plugins (backend only - ADDITIONAL_PLUGS)
// ===========================================================================
type PluginConfigRow = { key: string; value: string };
type PluginRow = { name: string; package_name: string; version: string; configs: PluginConfigRow[] };
type CatalogEntry = { label: string; name: string; package_name: string; version: string; configs: PluginConfigRow[] };
const PLUGIN_CATALOG: CatalogEntry[] = [
  {
    label: "Notifications (care_notifications)",
    name: "care_notifications",
    package_name: "git+https://github.com/ohcnetwork/care_notifications_be.git",
    version: "@main",
    configs: [{ key: "WEBPUSH_NOTIFICATIONS_ENABLED", value: "false" }],
  },
];
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
        name: p.name ?? "", package_name: p.package_name ?? "", version: p.version ?? "",
        configs: Object.entries(p.configs ?? {}).map(([key, value]) => ({ key, value: String(value) })),
      }));
    } catch { this.plugins = []; }
    this.render();
  }
  private field(label: string, ph: string, value: string, onInput: (v: string) => void): HTMLElement {
    const wrap = document.createElement("div"); wrap.className = "plugin-field";
    const lab = document.createElement("label"); lab.textContent = label;
    const inp = document.createElement("input");
    inp.type = "text"; inp.placeholder = ph; inp.value = value; inp.spellcheck = false;
    inp.addEventListener("input", () => onInput(inp.value));
    wrap.append(lab, inp);
    return wrap;
  }
  render(): void {
    this.container.innerHTML = "";
    if (this.plugins.length === 0) {
      const empty = document.createElement("div"); empty.className = "plugins-empty";
      empty.textContent = "No plugins yet.";
      this.container.appendChild(empty); return;
    }
    this.plugins.forEach((p, pi) => {
      const card = document.createElement("div"); card.className = "plugin-card";
      const head = document.createElement("div"); head.className = "plugin-head";
      const pz = document.createElement("i"); pz.className = "ph ph-puzzle-piece pz"; head.appendChild(pz);
      const name = document.createElement("input");
      name.className = "plugin-name"; name.placeholder = "plugin name (e.g. hcx)"; name.value = p.name; name.spellcheck = false;
      name.addEventListener("input", () => (this.plugins[pi].name = name.value));
      const rm = document.createElement("button");
      rm.className = "btn ghost env-remove"; rm.textContent = "x"; rm.title = "remove plugin"; rm.style.color = "var(--red)";
      rm.addEventListener("click", () => { this.plugins.splice(pi, 1); this.render(); });
      head.append(name, rm); card.appendChild(head);
      card.appendChild(this.field("Package URL", "git+https://github.com/ohcnetwork/care_hcx.git", p.package_name, (v) => (this.plugins[pi].package_name = v)));
      card.appendChild(this.field("Version (optional)", "@master or ==1.2.3", p.version, (v) => (this.plugins[pi].version = v)));
      const cl = document.createElement("div"); cl.className = "plugin-cfg-label"; cl.textContent = "Configuration"; card.appendChild(cl);
      const cf = document.createElement("div"); cf.className = "env-form";
      p.configs.forEach((c, ci) => {
        const row = document.createElement("div"); row.className = "env-row";
        const k = document.createElement("input"); k.type = "text"; k.placeholder = "CONFIG_KEY"; k.value = c.key; k.className = "env-key-input";
        k.addEventListener("input", () => (this.plugins[pi].configs[ci].key = k.value));
        const v = document.createElement("input"); v.type = "text"; v.placeholder = "value"; v.value = c.value; v.spellcheck = false;
        v.addEventListener("input", () => (this.plugins[pi].configs[ci].value = v.value));
        const crm = document.createElement("button"); crm.className = "btn ghost env-remove"; crm.textContent = "x";
        crm.addEventListener("click", () => { this.plugins[pi].configs.splice(ci, 1); this.render(); });
        row.append(k, v, crm); cf.appendChild(row);
      });
      card.appendChild(cf);
      const addCfg = document.createElement("button");
      addCfg.className = "btn ghost tiny"; addCfg.style.marginTop = "8px"; addCfg.innerHTML = `<i class="ph ph-plus"></i>Add config`;
      addCfg.addEventListener("click", () => { this.plugins[pi].configs.push({ key: "", value: "" }); this.render(); });
      card.appendChild(addCfg);
      this.container.appendChild(card);
    });
  }
  add(): void { this.plugins.push({ name: "", package_name: "", version: "", configs: [] }); this.render(); }
  addFromCatalog(c: CatalogEntry): void {
    this.plugins.push({ name: c.name, package_name: c.package_name, version: c.version, configs: c.configs.map((cf) => ({ ...cf })) });
    this.render();
  }
  serialize(): CarePlugin[] {
    return this.plugins.filter((p) => p.name.trim() !== "" && p.package_name.trim() !== "").map((p) => {
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
const pluginPicker = $<HTMLSelectElement>("#plugin-picker");
pluginPicker.innerHTML = `<option value="">+ Add a plugin...</option>` +
  PLUGIN_CATALOG.map((c, i) => `<option value="cat:${i}">${c.label}</option>`).join("") +
  `<option value="custom">Custom - add by URL</option>`;
pluginPicker.addEventListener("change", () => {
  const v = pluginPicker.value;
  if (v === "custom") pluginEditor.add();
  else if (v.startsWith("cat:")) pluginEditor.addFromCatalog(PLUGIN_CATALOG[Number(v.slice(4))]);
  pluginPicker.value = "";
});
$("#plugins-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    try { await pluginEditor.save(); showToast("Rebuilding backend with new plugins"); await run("rebuild-backend"); }
    catch (e) { append(`error saving plugins: ${String(e)}`); showToast("Couldn't save plugins"); }
  })();
});

// ===========================================================================
// Frontend plugins (CARE plug_config — edited here, loaded by CARE at runtime)
// ===========================================================================
// A frontend plugin is one plug_config row: a slug + a meta blob whose meta.url is
// the remoteEntry.js the browser loads. Adding, editing, or removing one is a
// database write — no rebuild, nothing to download. This editor loads the live rows
// (the same set CARE's own Apps page shows) and saves the whole set back, so the two
// never drift apart. Structured fields stand in for CARE admin's raw meta JSON.
type FeConfigRow = { key: string; value: string };
type FrontendPlugin = { slug: string; meta: Record<string, unknown> };
type FePluginRow = { slug: string; url: string; configs: FeConfigRow[]; meta: Record<string, unknown> };
type FeCatalogEntry = { label: string; slug: string; url: string };
const FE_PLUGIN_CATALOG: FeCatalogEntry[] = [
  {
    label: "Hello World (sample)",
    slug: "care_hello_fe",
    url: "https://ohcnetwork.github.io/care_hello_fe/assets/remoteEntry.js",
  },
];

class FrontendPluginEditor {
  plugins: FePluginRow[] = [];
  private loaded = false;
  private loadError = "";
  constructor(private container: HTMLElement) {}

  private empty(text: string): void {
    this.container.textContent = "";
    const box = document.createElement("div");
    box.className = "plugins-empty";
    box.textContent = text;
    this.container.appendChild(box);
  }

  async load(): Promise<void> {
    this.loaded = false;
    this.loadError = "";
    this.empty("Checking…");
    let raw: FrontendPlugin[];
    try {
      raw = (await App.ReadFrontendPlugins()) ?? [];
    } catch (e) {
      this.loadError = firstLine(String(e));
      this.empty(this.loadError);
      return;
    }
    this.plugins = raw.map((p) => {
      const meta = (p.meta ?? {}) as Record<string, unknown>;
      const cfg = (meta.config ?? {}) as Record<string, unknown>;
      return {
        slug: p.slug,
        url: typeof meta.url === "string" ? meta.url : "",
        configs: Object.entries(cfg).map(([key, value]) => ({ key, value: String(value) })),
        meta,
      };
    });
    this.loaded = true;
    this.render();
  }

  private field(label: string, ph: string, value: string, onInput: (v: string) => void): HTMLElement {
    const wrap = document.createElement("div"); wrap.className = "plugin-field";
    const lab = document.createElement("label"); lab.textContent = label;
    const inp = document.createElement("input");
    inp.type = "text"; inp.placeholder = ph; inp.value = value; inp.spellcheck = false;
    inp.addEventListener("input", () => onInput(inp.value));
    wrap.append(lab, inp);
    return wrap;
  }

  render(): void {
    this.container.textContent = "";
    if (this.plugins.length === 0) {
      this.empty("No frontend plugins yet. Add one with the picker below.");
      return;
    }
    this.plugins.forEach((p, pi) => {
      const card = document.createElement("div"); card.className = "plugin-card";
      const head = document.createElement("div"); head.className = "plugin-head";
      const pz = document.createElement("i"); pz.className = "ph ph-puzzle-piece pz"; head.appendChild(pz);
      const slug = document.createElement("input");
      slug.className = "plugin-name"; slug.placeholder = "plugin name (e.g. care_hello_fe)"; slug.value = p.slug; slug.spellcheck = false;
      slug.addEventListener("input", () => (this.plugins[pi].slug = slug.value));
      const rm = document.createElement("button");
      rm.className = "btn ghost env-remove"; rm.textContent = "x"; rm.title = "remove plugin"; rm.style.color = "var(--red)";
      rm.addEventListener("click", () => { this.plugins.splice(pi, 1); this.render(); });
      head.append(slug, rm); card.appendChild(head);

      card.appendChild(this.field("Remote entry URL", "https://…/assets/remoteEntry.js", p.url, (v) => (this.plugins[pi].url = v)));

      const cl = document.createElement("div"); cl.className = "plugin-cfg-label"; cl.textContent = "Configuration (optional)"; card.appendChild(cl);
      const cf = document.createElement("div"); cf.className = "env-form";
      p.configs.forEach((c, ci) => {
        const row = document.createElement("div"); row.className = "env-row";
        const k = document.createElement("input"); k.type = "text"; k.placeholder = "CONFIG_KEY"; k.value = c.key; k.className = "env-key-input";
        k.addEventListener("input", () => (this.plugins[pi].configs[ci].key = k.value));
        const v = document.createElement("input"); v.type = "text"; v.placeholder = "value"; v.value = c.value; v.spellcheck = false;
        v.addEventListener("input", () => (this.plugins[pi].configs[ci].value = v.value));
        const crm = document.createElement("button"); crm.className = "btn ghost env-remove"; crm.textContent = "x";
        crm.addEventListener("click", () => { this.plugins[pi].configs.splice(ci, 1); this.render(); });
        row.append(k, v, crm); cf.appendChild(row);
      });
      card.appendChild(cf);
      const addCfg = document.createElement("button");
      addCfg.className = "btn ghost tiny"; addCfg.style.marginTop = "8px"; addCfg.innerHTML = `<i class="ph ph-plus"></i>Add config`;
      addCfg.addEventListener("click", () => { this.plugins[pi].configs.push({ key: "", value: "" }); this.render(); });
      card.appendChild(addCfg);
      this.container.appendChild(card);
    });
  }

  add(): void { this.plugins.push({ slug: "", url: "", configs: [], meta: {} }); this.render(); }
  addFromCatalog(c: FeCatalogEntry): void {
    this.plugins.push({ slug: c.slug, url: c.url, configs: [], meta: {} }); this.render();
  }

  serialize(): FrontendPlugin[] {
    return this.plugins.filter((p) => p.slug.trim() !== "" && p.url.trim() !== "").map((p) => {
      const meta: Record<string, unknown> = { ...p.meta, name: p.slug.trim(), url: p.url.trim() };
      const config: Record<string, unknown> = {};
      for (const c of p.configs) if (c.key.trim() !== "") config[c.key.trim()] = parseConfigValue(c.value);
      if (Object.keys(config).length) meta.config = config; else delete meta.config;
      return { slug: p.slug.trim(), meta };
    });
  }

  async save(): Promise<void> {
    if (!this.loaded) {
      // The panel opened before CARE was ready, so we never got a clean snapshot.
      // Read one now — this is also the data-loss guard: without a baseline a save
      // could delete rows we simply failed to read. Then keep the user's edits on top.
      const pending = this.plugins;
      await this.load();
      if (!this.loaded) {
        throw new Error(this.loadError || "Couldn't read the current plugins from CARE — is it running?");
      }
      const bySlug = new Map(this.plugins.map((p) => [p.slug, p]));
      for (const p of pending) if (p.slug.trim() || p.url.trim()) bySlug.set(p.slug, p);
      this.plugins = [...bySlug.values()];
      this.render();
    }
    await App.SaveFrontendPlugins(this.serialize());
  }
}

const feEditor = new FrontendPluginEditor($("#plugins-list"));
const fePicker = $<HTMLSelectElement>("#fe-plugin-picker");
fePicker.innerHTML = `<option value="">+ Add a plugin...</option>` +
  FE_PLUGIN_CATALOG.map((c, i) => `<option value="cat:${i}">${c.label}</option>`).join("") +
  `<option value="custom">Custom - add by URL</option>`;
fePicker.addEventListener("change", () => {
  const v = fePicker.value;
  if (v === "custom") feEditor.add();
  else if (v.startsWith("cat:")) feEditor.addFromCatalog(FE_PLUGIN_CATALOG[Number(v.slice(4))]);
  fePicker.value = "";
});
$("#fe-plugins-save").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    try { await feEditor.save(); showToast("Frontend plugins saved — staff refresh their browser"); await feEditor.load(); }
    catch (e) { append(`error saving frontend plugins: ${String(e)}`); showToast(firstLine(String(e))); }
  })();
});

// ===========================================================================
// System configuration - backend / frontend section switch
// ===========================================================================
const SECTION_FILE: Record<EnvName, string> = { backend: "backend.env", frontend: "frontend.env", tls: "tls.env" };
const SECTION_HINT: Record<EnvName, string> = {
  backend: "Everyday settings for the backend, such as email delivery.",
  frontend: "Everyday settings for the app that staff and patients see.",
  tls: "Serve the clinic over HTTPS on your own domain. Leave the host blank to stay on plain http://care.local. Needs the domain's DNS on Cloudflare and an API token — see docs/tls.md.",
};
const SECTION_SAVE: Record<EnvName, string> = { backend: "Save & apply", frontend: "Save & rebuild app", tls: "Save & apply HTTPS" };

async function selectSection(section: EnvName): Promise<void> {
  $("#sel-backend").classList.toggle("on", section === "backend");
  $("#sel-frontend").classList.toggle("on", section === "frontend");
  $("#sel-tls").classList.toggle("on", section === "tls");
  $("#cfg-file").textContent = SECTION_FILE[section];
  $("#cfg-hint").textContent = SECTION_HINT[section];
  $("#env-save").textContent = SECTION_SAVE[section];
  $("#adv-env-body").hidden = true; $("#adv-env-caret").className = "ph ph-caret-right";
  await envEditor.load(section);
  showPluginsFor(section);
}
function showPluginsFor(section: EnvName): void {
  const be = $("#be-plugins-actions"), fe = $("#fe-plugins-actions");
  const tag = $("#plugins-card .tag");
  $("#plugins-card").hidden = section === "tls"; // no plugins to configure for HTTPS
  if (section === "tls") return;
  if (section === "backend") {
    $("#plugins-hint").textContent = "Extra features for the backend. Adding or changing plugins rebuilds it, which needs the internet.";
    if (tag) { tag.textContent = "rebuilds on save"; tag.classList.add("warn"); }
    be.hidden = false; fe.hidden = true;
    void pluginEditor.load();
  } else {
    $("#plugins-hint").textContent = "Optional apps for staff. Add one by its published URL — changes are instant, no rebuild.";
    if (tag) { tag.textContent = "instant · no rebuild"; tag.classList.remove("warn"); }
    be.hidden = true; fe.hidden = false;
    void feEditor.load();
  }
}
$("#sel-backend").addEventListener("click", () => void selectSection("backend"));
$("#sel-frontend").addEventListener("click", () => void selectSection("frontend"));
$("#sel-tls").addEventListener("click", () => void selectSection("tls"));

// ===========================================================================
// Advanced - password gate + navigation + uninstall
// ===========================================================================
let advUnlocked = false;
let advScreen: "home" | "sysconfig" = "home";
function showAdvanced(): void {
  $("#adv-gate").hidden = advUnlocked;
  $("#adv-home").hidden = !(advUnlocked && advScreen === "home");
  $("#adv-sys").hidden = !(advUnlocked && advScreen === "sysconfig");
  $("#adv-lock-icon").className = "ph " + (advUnlocked ? "ph-lock-key-open" : "ph-lock-simple") + " lock";
}
const advPw = $<HTMLInputElement>("#advpw");
$("#advpw-toggle").addEventListener("click", () => {
  const show = advPw.type === "password";
  advPw.type = show ? "text" : "password";
  $("#advpw-eye").className = show ? "ph ph-eye-slash" : "ph ph-eye";
});
async function tryUnlock(): Promise<void> {
  const ok = await App.VerifyAdminPassword(advPw.value);
  if (ok) { advUnlocked = true; advScreen = "home"; advPw.value = ""; $("#adv-gate-err").hidden = true; showAdvanced(); }
  else { $("#adv-gate-err").hidden = false; $("#adv-gate-err").textContent = "That password doesn't match. Try again."; }
}
$("#adv-unlock").addEventListener("click", () => void tryUnlock());
advPw.addEventListener("keydown", (e) => { if (e.key === "Enter") void tryUnlock(); });
$("#open-sysconfig").addEventListener("click", () => { advScreen = "sysconfig"; showAdvanced(); void selectSection("backend"); });
$("#close-sysconfig").addEventListener("click", () => { advScreen = "home"; showAdvanced(); });

$("#uninstall-run").addEventListener("click", () => { $("#uninstall-idle").hidden = true; $("#uninstall-confirm").hidden = false; });
$("#uninstall-cancel").addEventListener("click", () => { $("#uninstall-idle").hidden = false; $("#uninstall-confirm").hidden = true; });
$("#uninstall-yes").addEventListener("click", () => {
  if (busy) return;
  void (async () => {
    const rmBackups = ($("#uninstall-backups") as HTMLInputElement).checked;
    const rmImages = ($("#uninstall-images") as HTMLInputElement).checked;
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
async function loadBackups(): Promise<void> {
  try { backups = await App.ListBackups(); } catch { backups = []; }
  $("#backups-summary").textContent = backups.length ? `${backups.length} kept - last ${shortDate(backups[0].label)}` : "";
  // overview strip
  if (backups.length) {
    $("#backup-strip-title").textContent = "Backups are up to date";
    $("#backup-strip-sub").textContent = `Last backup ${shortDate(backups[0].label)}${backups[0].encrypted ? " - encrypted" : ""}.`;
  } else {
    $("#backup-strip-title").textContent = "No backups yet";
    $("#backup-strip-sub").textContent = 'Click "Back up now", or wait for the daily backup.';
  }
  const list = $("#backups-list");
  if (backups.length === 0) {
    list.innerHTML = `<div class="backups-empty">No backups yet. Click <b>Back up now</b> or wait for the daily backup.</div>`;
    return;
  }
  list.innerHTML = backups.map((b, i) => {
    const size = b.size_bytes ? (b.size_bytes / 1e6).toFixed(1) + " MB" : "";
    const files = b.files_archive ? "DB + files" : "DB only";
    const meta = [size, files, b.encrypted ? "encrypted" : ""].filter(Boolean).join(" - ");
    const badge = b.manual ? `<span class="badge manual">Manual</span>` : `<span class="badge">Automatic</span>`;
    const confirm = i === confirmingRestore ? `
      <div class="confirm-strip">
        <i class="ph-fill ph-warning"></i>
        <div class="msg">Replace all current data with the backup from <b>${b.label}</b>? This can't be undone.</div>
        <button class="btn" data-cancel>Cancel</button>
        <button class="btn danger" data-confirm="${i}">Yes, restore</button>
      </div>` : "";
    return `<div class="backup-item">
      <div class="backup-row">
        <div class="backup-ico"><i class="ph ph-database"></i></div>
        <div class="backup-info"><div class="backup-label">${b.label}</div><div class="backup-meta">${meta}</div></div>
        ${badge}
        <button class="btn" data-restore="${i}"><i class="ph ph-arrow-counter-clockwise"></i>Restore</button>
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
function shortDate(label: string): string {
  const m = label.match(/^(\d{4})-(\d{2})-(\d{2})\s+(\d{2}:\d{2})/);
  return m ? `${m[3]}/${m[2]} ${m[4]}` : label.split(" - ")[0] || label;
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
  installBar.style.width = "100%"; installPct.textContent = "100%"; installStep.textContent = "Done - opening the control panel...";
  phase = "panel"; showView("panel"); void bootPanel();
});
on("uninstalled", () => { showToast("Uninstalled"); setTimeout(() => window.location.reload(), 1800); });

// ===========================================================================
// Boot
// ===========================================================================
async function bootPanel(): Promise<void> {
  const state = await App.GetState();
  await refreshOrigin(state.origin);
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
  const state: AppState = await App.GetState();
  if (state.setup_done) { phase = "panel"; showView("panel"); await bootPanel(); }
  else { phase = "setup"; showView("wizard"); recheck(); }
}
void boot();
setInterval(() => void refresh(), 5_000);
