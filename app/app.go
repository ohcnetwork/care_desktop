package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"care-desktop/app/internal/care"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/bcrypt"
)

// App is the Wails bridge: every exported method is callable from the web UI as
// window.go.main.App.<Method>. It owns config persistence and drives the engine.
type App struct {
	ctx context.Context
	kit fs.FS // embedded deployment kit

	advMu   sync.Mutex       // guards adv
	adv     *care.Advertiser // the running mDNS responder (advertise mode), if any
	advStop chan struct{}    // closed on shutdown to end the DHCP watcher
}

func NewApp(kit fs.FS) *App {
	care.FixPath() // make docker/git findable when launched from Finder/Explorer
	return &App{kit: kit}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Advertise care.local right away (host process; no rename, no sudo). Doing it
	// before setup means the installer's step-3 check goes green immediately, and
	// leaving it up while the app runs keeps the clinic reachable by name.
	a.startAdvertise()
	a.advStop = make(chan struct{})
	go a.watchAdvertise()
}

// shutdown stops the responder when the app quits (wired via Wails OnShutdown).
func (a *App) shutdown(context.Context) {
	if a.advStop != nil {
		close(a.advStop)
	}
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
}

// startAdvertise brings up the mDNS responder for the configured name, unless mDNS
// is in "rename"/"off" mode. Best-effort: a failure is logged, never fatal.
func (a *App) startAdvertise() {
	if a.engine(nil).MDNSMode() != "advertise" {
		return
	}
	a.advMu.Lock()
	defer a.advMu.Unlock()
	if a.adv != nil {
		return
	}
	name := a.loadConfig().MDNSName
	adv, err := care.Advertise(name)
	if err != nil {
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, "care-log", "mDNS: couldn't advertise "+name+".local ("+err.Error()+")")
		}
		return
	}
	a.adv = adv
}

// restartAdvertise re-advertises with the current config (after the name changes,
// or the LAN IP changes).
func (a *App) restartAdvertise() {
	a.advMu.Lock()
	a.adv.Stop()
	a.adv = nil
	a.advMu.Unlock()
	a.startAdvertise()
}

// advRunning reports whether we're actively answering <name>.local.
func (a *App) advRunning() bool {
	a.advMu.Lock()
	defer a.advMu.Unlock()
	return a.adv != nil
}

// watchAdvertise re-advertises when the host's LAN IP changes (e.g. DHCP renew)
// or when care.local stops resolving (responder silently died - sleep/wake,
// network flap, mDNSResponder dropped our record). Cheap: a lookup every 30s.
func (a *App) watchAdvertise() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	misses := 0
	for {
		select {
		case <-a.advStop:
			return
		case <-t.C:
			a.advMu.Lock()
			adv := a.adv
			a.advMu.Unlock()
			if adv == nil {
				continue
			}
			if adv.IPsChanged() {
				misses = 0
				a.restartAdvertise()
				continue
			}
			// ponytail: debounce two misses so one flaky lookup doesn't churn
			// the responder; a genuinely dead responder never recovers on its own.
			if adv.Resolves() {
				misses = 0
				continue
			}
			misses++
			if misses >= 2 {
				misses = 0
				if a.ctx != nil {
					wruntime.EventsEmit(a.ctx, "care-log", "mDNS: care.local stopped resolving - re-advertising.")
				}
				a.restartAdvertise()
			}
		}
	}
}

// --- persisted config -------------------------------------------------------

type Config struct {
	SetupDone   bool   `json:"setup_done"`
	MDNSName    string `json:"mdns_name"`
	InstallDir  string `json:"install_dir"`
	BackupDir   string `json:"backup_dir"`
	AdminPwHash string `json:"admin_pw_hash,omitempty"` // bcrypt of the install-time admin password; gates Advanced
}

func (a *App) configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	dir = filepath.Join(dir, "care-desktop")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "config.json")
}

func (a *App) loadConfig() Config {
	cfg := Config{MDNSName: "care.local"}
	b, err := os.ReadFile(a.configPath())
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.MDNSName == "" {
		cfg.MDNSName = "care.local"
	}
	return cfg
}

func (a *App) saveConfig(cfg Config) error {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(a.configPath(), b, 0o644)
}

// --- kit location + first-run unpack ----------------------------------------

func (a *App) kitDir() string {
	if d := os.Getenv("CARE_DESKTOP_DIR"); d != "" {
		return d
	}
	if cfg := a.loadConfig(); cfg.InstallDir != "" {
		return cfg.InstallDir
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.UserHomeDir()
	}
	return filepath.Join(base, "care-desktop", "kit")
}

// kitUserFiles are preserved on a kit refresh; everything else is app-owned.
var kitUserFiles = map[string]bool{"backend.env": true, "frontend.env": true}

// ensureKit syncs the embedded kit on every setup (not just the first) so an updated
// app delivers new/changed files to an existing install, keeping edited env files.
// Fixes the stale-kit failure where setup can't find a newly added kit file.
func (a *App) ensureKit() (string, error) {
	dest := a.kitDir()
	err := fs.WalkDir(a.kit, "kit", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "kit")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if kitUserFiles[rel] { // keep the user's edits; only seed when absent
			if _, err := os.Stat(target); err == nil {
				return nil
			}
		}
		data, err := fs.ReadFile(a.kit, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
	return dest, err
}

// engine builds an Engine bound to the kit dir, streaming logs to the UI.
func (a *App) engine(extra map[string]string) *care.Engine {
	env := map[string]string{}
	if cfg := a.loadConfig(); cfg.BackupDir != "" {
		env["BACKUP_DIR"] = cfg.BackupDir
	}
	for k, v := range extra {
		env[k] = v
	}
	return &care.Engine{
		Kit: a.kitDir(),
		Env: env,
		Log: func(s string) { wruntime.EventsEmit(a.ctx, "care-log", s) },
		Confirm: func(title, message string) bool {
			sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
				Type:          wruntime.QuestionDialog,
				Title:         title,
				Message:       message,
				Buttons:       []string{"Yes", "No"},
				DefaultButton: "Yes",
				CancelButton:  "No",
			})
			return err == nil && sel == "Yes"
		},
	}
}

// --- state the installer/panel read on load ---------------------------------

type AppState struct {
	SetupDone bool              `json:"setup_done"`
	MDNSName  string            `json:"mdns_name"`
	Docker    care.DockerStatus `json:"docker"`
}

func (a *App) GetState() AppState {
	cfg := a.loadConfig()
	return AppState{
		SetupDone: cfg.SetupDone,
		MDNSName:  cfg.MDNSName,
		Docker:    a.engine(nil).DockerCheck(),
	}
}

func (a *App) DockerStatus() care.DockerStatus { return a.engine(nil).DockerCheck() }
func (a *App) GitStatus() care.DockerStatus    { return a.engine(nil).GitCheck() }
func (a *App) CareHealth() care.Health         { return a.engine(nil).Ping() }

// ValidatePassword lets the wizard check the admin password live as the user types.
// Returns "" when acceptable, otherwise a human-readable reason to show under the field.
func (a *App) ValidatePassword(pw string) string {
	if err := care.ValidatePassword(pw); err != nil {
		return err.Error()
	}
	return ""
}

// VerifyAdminPassword gates the Advanced screen. It checks against the bcrypt hash
// stored at setup. Legacy installs (set up before the hash existed) have none, so
// any non-empty entry passes - a speed bump, not verification.
func (a *App) VerifyAdminPassword(pw string) bool {
	h := a.loadConfig().AdminPwHash
	if h == "" {
		return strings.TrimSpace(pw) != ""
	}
	return bcrypt.CompareHashAndPassword([]byte(h), []byte(pw)) == nil
}

// MDNSStatus is green whenever this app is actively advertising the name (advertise
// mode), since that's exactly what makes care.local resolve. Otherwise it falls back
// to the engine's resolution test + mode-specific guidance.
func (a *App) MDNSStatus() care.NameStatus {
	if a.advRunning() {
		full := care.NameStatus{OK: true}
		full.Message = a.loadConfig().MDNSName + " is being advertised by this app"
		return full
	}
	return a.engine(nil).MDNSCheck()
}

// --- actions (async; stream logs, finish with a care-done event) ------------

var allowed = map[string]bool{
	"start": true, "stop": true, "restart": true,
	"rebuild-backend": true, "rebuild-frontend": true, "backup-now": true,
}

func (a *App) run(e *care.Engine, fn func() error, markSetup bool, label string) {
	go func() {
		code := 0
		if err := fn(); err != nil {
			wruntime.EventsEmit(a.ctx, "care-log", "error: "+err.Error())
			code = 1
			// The installer shows a failed screen; the panel has none, so a failed
			// action (e.g. Start when port 80 is taken) would otherwise be invisible
			// unless the log tab happens to be open. Surface it natively. The engine's
			// message is self-explanatory (the port-80 error even names what to quit).
			if !markSetup {
				a.notifyActionFailed(label, err.Error())
			}
		}
		if markSetup && code == 0 {
			cfg := a.loadConfig()
			cfg.SetupDone = true
			_ = a.saveConfig(cfg)
			wruntime.EventsEmit(a.ctx, "setup-done", true)
			a.notifyInstalled(cfg.MDNSName)
		}
		wruntime.EventsEmit(a.ctx, "care-done", code)
	}()
}

// notifyActionFailed pops a native error dialog when a panel action fails. It's the
// counterpart to notifyInstalled: the panel has no failed screen, so this is how a
// user learns (and why) an action didn't go through. label picks a fitting title.
func (a *App) notifyActionFailed(label, detail string) {
	title := "CARE couldn't finish that"
	switch label {
	case "start":
		title = "CARE couldn't start"
	case "restart":
		title = "CARE couldn't restart"
	case "stop":
		title = "CARE couldn't stop"
	case "restore":
		title = "Restore didn't finish"
	case "rebuild-backend", "rebuild-frontend":
		title = "Rebuild didn't finish"
	}
	_, _ = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:    wruntime.ErrorDialog,
		Title:   title,
		Message: detail,
		Buttons: []string{"OK"},
	})
}

// notifyInstalled shows the one-time native "install complete" pop-up. It's fired
// from the setup-done branch, which only runs after the stack is verified healthy
// (WaitHealthy), so this dialog is a truthful "it's up and reachable" signal.
// Offers to open the app straight away.
func (a *App) notifyInstalled(mdnsName string) {
	if mdnsName == "" {
		mdnsName = "care.local"
	}
	url := "https://" + mdnsName + "/"
	sel, _ := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.InfoDialog,
		Title:         "CARE Desktop installed",
		Message:       "CARE is installed and running.\n\nOpen it at " + url + "\nLogin: admin / admin (change the password in the app).",
		Buttons:       []string{"Open CARE", "Close"},
		DefaultButton: "Open CARE",
	})
	if sel == "Open CARE" {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}

// CareAction runs one whitelisted action against the existing kit.
func (a *App) CareAction(action string) error {
	if !allowed[action] {
		return errString("action not allowed: " + action)
	}
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return errString("not set up yet - run the first-time setup")
	}
	e := a.engine(nil)
	a.run(e, actionFunc(e, action), false, action)
	return nil
}

func actionFunc(e *care.Engine, action string) func() error {
	switch action {
	case "start":
		return e.Start
	case "stop":
		return e.Stop
	case "restart":
		return e.Restart
	case "rebuild-backend":
		return e.RebuildBackend
	case "rebuild-frontend":
		return e.RebuildFrontend
	case "backup-now":
		return e.BackupNow
	}
	return func() error { return errString("unknown action") }
}

// RunSetup persists the wizard's choices, unpacks the kit, then runs setup+start.
// Empty backupPassword = encryption off; rememberBackup saves it to the keychain.
func (a *App) RunSetup(mdnsName, adminPassword, backupPassword string, rememberBackup bool, installDir, backupDir string) error {
	if err := care.ValidatePassword(adminPassword); err != nil {
		return err
	}
	if backupPassword != "" {
		if err := care.ValidatePassword(backupPassword); err != nil {
			return err
		}
	}

	mdns := strings.TrimSpace(mdnsName)
	if mdns == "" {
		mdns = "care.local"
	}
	host := strings.TrimSuffix(mdns, ".local")

	cfg := a.loadConfig()
	cfg.MDNSName = mdns
	if h, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost); err == nil {
		cfg.AdminPwHash = string(h) // so Advanced can be gated behind the admin password, offline
	}
	if strings.TrimSpace(installDir) != "" {
		cfg.InstallDir = filepath.Join(strings.TrimSpace(installDir), "CARE Desktop")
	}
	if strings.TrimSpace(backupDir) != "" {
		cfg.BackupDir = filepath.Join(strings.TrimSpace(backupDir), "care-db-backups")
	}
	if err := a.saveConfig(cfg); err != nil {
		return err
	}
	a.restartAdvertise() // pick up the chosen name (usually still care.local)
	if _, err := a.ensureKit(); err != nil {
		return err
	}

	// Best-effort: a keychain failure shouldn't abort the install.
	if backupPassword != "" && rememberBackup {
		if err := care.StoreBackupPassword(backupPassword); err != nil {
			wruntime.EventsEmit(a.ctx, "care-log", "note: couldn't save the backup password to the keychain ("+err.Error()+")")
		}
	}

	env := map[string]string{
		"CARE_MDNS_NAME":      host,
		"CARE_ADMIN_PASSWORD": adminPassword,
		"CARE_NO_MDNS":        "1", // naming is a verified wizard step; don't retry sudo here
	}
	if backupPassword != "" {
		env["CARE_BACKUP_PASSWORD"] = backupPassword
	}
	e := a.engine(env)
	a.run(e, func() error {
		// Check port 80 before the ~10-min build, so a conflict fails immediately
		// instead of after the wait (Start re-checks in case it's taken meanwhile).
		if err := e.EnsurePortFree(); err != nil {
			return err
		}
		if err := e.Setup(); err != nil {
			return err
		}
		return e.Start()
	}, true, "setup")
	return nil
}

func (a *App) CareStatus() (string, error) { return a.engine(nil).Status() }

// --- restore ----------------------------------------------------------------

// ListBackups returns the restorable points in the backup folder (newest first)
// for the panel's restore dropdown.
func (a *App) ListBackups() ([]care.Backup, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - no backups to offer
	}
	return a.engine(nil).ListBackups()
}

// ConfirmRestore shows a native yes/no dialog before a destructive restore, so
// the (irreversible) data replacement is never a single stray click.
func (a *App) ConfirmRestore(filesIncluded bool) bool {
	what := "the current database"
	if filesIncluded {
		what = "the current database and uploaded files"
	}
	sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.QuestionDialog,
		Title:         "Restore from backup?",
		Message:       "This replaces " + what + " with the selected backup and cannot be undone.\nCARE will be stopped during the restore, then restarted.\n\nContinue?",
		Buttons:       []string{"Restore", "Cancel"},
		DefaultButton: "Cancel",
	})
	return err == nil && sel == "Restore"
}

// RestoreBackup restores async. For an encrypted backup, "" passphrase falls back to
// the keychain; remember saves the one that worked.
func (a *App) RestoreBackup(dbDump, filesArchive, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return errString("not set up yet - run the first-time setup")
	}
	if passphrase == "" {
		passphrase = care.LoadBackupPassword()
	}
	if passphrase != "" && remember {
		_ = care.StoreBackupPassword(passphrase)
	}
	e := a.engine(nil)
	a.run(e, func() error { return e.Restore(dbDump, filesArchive, passphrase) }, false, "restore")
	return nil
}

// BackupEncryptionEnabled reports whether backups are encrypted (a keypair exists),
// so the panel knows a restore will need the backup password.
func (a *App) BackupEncryptionEnabled() bool {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return false
	}
	return a.engine(nil).BackupEncryptionOn()
}

// HasStoredBackupPassword reports whether the backup password is remembered on this
// machine - if so, the restore UI can skip the password prompt.
func (a *App) HasStoredBackupPassword() bool { return care.HasBackupPassword() }

// --- uninstall --------------------------------------------------------------

// ConfirmUninstall shows a stern native warning before the (irreversible) teardown.
func (a *App) ConfirmUninstall(removeBackups bool) bool {
	msg := "This permanently deletes CARE and all of its data:\n" +
		"- every container and data volume (patient records + uploaded files)\n" +
		"- the installed files and downloaded source\n"
	if removeBackups {
		msg += "- your backups - there will be NO way to recover the data\n"
	} else {
		msg += "\nYour backups are kept.\n"
	}
	msg += "\nThis cannot be undone. Continue?"
	sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.WarningDialog,
		Title:         "Uninstall CARE Desktop?",
		Message:       msg,
		Buttons:       []string{"Uninstall", "Cancel"},
		DefaultButton: "Cancel",
	})
	return err == nil && sel == "Uninstall"
}

// RunUninstall tears the install down (async, streaming logs), then clears the
// app's own state - autostart entry and saved config - and signals the UI to
// reset to first-run via an "uninstalled" event.
func (a *App) RunUninstall(removeImages, removeBackups bool) error {
	e := a.engine(nil)
	go func() {
		_ = e.Uninstall(care.UninstallOptions{
			RemoveImages:  removeImages,
			RemoveKit:     true,
			RemoveBackups: removeBackups,
		})
		_ = a.SetAutostart(false)     // remove the login-item, if any
		care.ForgetBackupPassword()   // drop the remembered backup password, if any
		_ = os.Remove(a.configPath()) // forget setup - next launch shows the wizard
		wruntime.EventsEmit(a.ctx, "care-log", "")
		wruntime.EventsEmit(a.ctx, "care-log", "Uninstalled. This computer's name was not changed back.")
		wruntime.EventsEmit(a.ctx, "uninstalled", true)
	}()
	return nil
}

// --- env editing ------------------------------------------------------------

func (a *App) envPath(name string) (string, error) {
	switch name {
	case "backend":
		return filepath.Join(a.kitDir(), "backend.env"), nil
	case "frontend":
		return filepath.Join(a.kitDir(), "frontend.env"), nil
	}
	return "", errString("unknown env file: " + name)
}

func (a *App) ReadEnv(name string) (string, error) {
	p, err := a.envPath(name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	return string(b), err
}

func (a *App) WriteEnv(name, content string) error {
	p, err := a.envPath(name)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// --- backend plugins (ADDITIONAL_PLUGS in backend.env) ----------------------

// ReadPlugins returns the backend plugins configured for this install.
func (a *App) ReadPlugins() ([]care.Plugin, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "backend.env")); err != nil {
		return []care.Plugin{}, nil // not set up yet
	}
	return a.engine(nil).ReadPlugins()
}

// SavePlugins writes the plugin list; the UI follows with a rebuild-backend.
func (a *App) SavePlugins(plugins []care.Plugin) error {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "backend.env")); err != nil {
		return errString("not set up yet - run the first-time setup")
	}
	return a.engine(nil).WritePlugins(plugins)
}

// --- frontend plugins (CARE plug_config table, synced with /admin/apps) ------
//
// Unlike backend plugins, frontend plugins load at runtime from CARE's plug_config
// table, so a toggle is a single database write - no rebuild. The engine reads and
// writes those rows directly, the same ones CARE's own Apps page edits, so the two
// panels stay in sync.

// appsEngine pins the clinic's chosen mDNS name, which the plugin bundle URLs are
// built from. engine(nil) would fall back to "care" on a clinic that picked another
// name.
func (a *App) appsEngine() *care.Engine {
	host := strings.TrimSuffix(strings.TrimSpace(a.loadConfig().MDNSName), ".local")
	if host == "" {
		host = "care"
	}
	return a.engine(map[string]string{"CARE_MDNS_NAME": host})
}

// ListApps returns the optional frontend plugins for the Frontend plugins panel:
// the apps.json catalogue merged with CARE's live plug_config rows, which are the
// only record of what's switched on.
func (a *App) ListApps() ([]care.ClinicApp, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - nothing to offer
	}
	return a.appsEngine().ListApps()
}

// SetAppEnabled switches one catalogue app on or off by writing CARE's plug_config
// table - instant, no rebuild. Staff refresh their browser to see the change.
func (a *App) SetAppEnabled(slug string, enabled bool) error {
	return a.appsEngine().SetAppEnabled(slug, enabled)
}

// ReadFrontendPlugins returns CARE's live frontend plugins (plug_config rows) for
// the Frontend plugins editor - the same set CARE's own Apps page shows.
func (a *App) ReadFrontendPlugins() ([]care.FrontendPlugin, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return []care.FrontendPlugin{}, nil // not set up yet
	}
	return a.appsEngine().ReadFrontendPlugins()
}

// SaveFrontendPlugins writes the whole plugin list to CARE's plug_config table
// (add + edit + remove). Instant - no rebuild, since CARE loads them at runtime.
func (a *App) SaveFrontendPlugins(plugins []care.FrontendPlugin) error {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return errString("not set up yet - run the first-time setup")
	}
	return a.appsEngine().WriteFrontendPlugins(plugins)
}

// --- misc UI helpers --------------------------------------------------------

func (a *App) OpenURL(url string) { wruntime.BrowserOpenURL(a.ctx, url) }

func (a *App) ChooseFolder(title string) string {
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: title})
	if err != nil {
		return ""
	}
	return dir
}

func (a *App) WasAutostartLaunched() bool {
	for _, arg := range os.Args {
		if arg == "--autostart" {
			return true
		}
	}
	return false
}

type errString string

func (e errString) Error() string { return string(e) }
