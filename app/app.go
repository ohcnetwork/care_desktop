package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"care-desktop/app/internal/care"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/bcrypt"
)

// App is the Wails bridge: every exported method is callable from the web UI as
// window.go.main.App.<Method>. It owns config persistence and drives the engine.
type App struct {
	ctx  context.Context
	kit  fs.FS         // embedded deployment kit
	stop chan struct{} // closed on shutdown to end the address watcher
}

func NewApp(kit fs.FS) *App {
	care.FixPath() // make docker/git findable when launched from Finder/Explorer
	return &App{kit: kit}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.stop = make(chan struct{})
	go a.watchAddress()
}

// shutdown is wired via Wails OnShutdown. Only the watcher needs stopping: the stack
// runs in Docker and keeps serving whether this window is open or not.
func (a *App) shutdown(context.Context) {
	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
}

// watchAddress keeps the clinic's DNS record pointing here when the router hands this
// computer a different address - a reboot, a new lease, or the clinic moving to a
// different network entirely. Without it a nurse two metres from a healthy server
// simply can't find it, and nothing about that failure looks DNS-shaped.
//
// The local check is cheap and runs often; Cloudflare is only called when the address
// actually changed, which is rare.
func (a *App) watchAddress() {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	last, _ := care.OutboundIP()
	for {
		select {
		case <-a.stop:
			return
		case <-t.C:
			cur, err := care.OutboundIP()
			if err != nil || cur == "" || cur == last {
				continue // no network, or nothing moved
			}
			last = cur
			e := a.engine(nil)
			if !e.Configured() {
				continue // pre-setup; nothing to point anywhere yet
			}
			if err := e.SyncDNS(); err != nil {
				wruntime.EventsEmit(a.ctx, "care-log",
					"note: this computer's address changed to "+cur+
						", but the DNS record couldn't be updated ("+err.Error()+")")
			}
		}
	}
}

// --- persisted config -------------------------------------------------------

type Config struct {
	SetupDone   bool   `json:"setup_done"`
	PublicHost  string `json:"public_host"` // the clinic's domain; the API token lives in tls.env, never here
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
	var cfg Config
	if b, err := os.ReadFile(a.configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
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
// tls.env holds the clinic's own domain and API token - an app update must never
// overwrite those back to blank and silently drop the install to plain HTTP.
var kitUserFiles = map[string]bool{"backend.env": true, "frontend.env": true, "tls.env": true}

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
	}
}

// --- state the installer/panel read on load ---------------------------------

type AppState struct {
	SetupDone bool              `json:"setup_done"`
	Origin    string            `json:"origin"` // https://<clinic domain>, or "" before setup
	Docker    care.DockerStatus `json:"docker"`
}

func (a *App) GetState() AppState {
	e := a.engine(nil)
	return AppState{
		SetupDone: a.loadConfig().SetupDone,
		Origin:    e.PublicOrigin(),
		Docker:    e.DockerCheck(),
	}
}

// PublicOrigin is the address staff open, reflecting the current tls.env. The panel
// re-reads it after saving HTTPS settings so the button and the copyable address
// can't keep pointing at the old origin.
func (a *App) PublicOrigin() string { return a.engine(nil).PublicOrigin() }

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

// CheckAddress is the wizard's live check on the clinic's web address: is the domain
// shaped right, does Cloudflare accept the token, and does the name already point at
// this computer. Called as the user types, and again before Install.
func (a *App) CheckAddress(host, token string) care.NameStatus {
	return a.engine(nil).CheckAddress(host, token)
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
			a.notifyInstalled()
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
func (a *App) notifyInstalled() {
	url := a.engine(nil).PublicOrigin() + "/"
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
func (a *App) RunSetup(publicHost, dnsToken, adminPassword, backupPassword string, rememberBackup bool, installDir, backupDir string) error {
	if err := care.ValidatePassword(adminPassword); err != nil {
		return err
	}
	if backupPassword != "" {
		if err := care.ValidatePassword(backupPassword); err != nil {
			return err
		}
	}

	cfg := a.loadConfig()
	cfg.PublicHost = strings.TrimSpace(publicHost)
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
	// Unpack the kit first: SaveTLSSettings writes into it, and it also validates
	// the address + token so a typo fails here rather than after the long build.
	if _, err := a.ensureKit(); err != nil {
		return err
	}
	if err := a.engine(nil).SaveTLSSettings(publicHost, dnsToken); err != nil {
		return err
	}

	// Best-effort: a keychain failure shouldn't abort the install.
	if backupPassword != "" && rememberBackup {
		if err := care.StoreBackupPassword(backupPassword); err != nil {
			wruntime.EventsEmit(a.ctx, "care-log", "note: couldn't save the backup password to the keychain ("+err.Error()+")")
		}
	}

	env := map[string]string{"CARE_ADMIN_PASSWORD": adminPassword}
	if backupPassword != "" {
		env["CARE_BACKUP_PASSWORD"] = backupPassword
	}
	e := a.engine(env)
	a.run(e, func() error {
		// Check the port before the ~10-min build, so a conflict fails immediately
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
	case "tls":
		return filepath.Join(a.kitDir(), "tls.env"), nil
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

// ListApps returns the optional frontend plugins for the Frontend plugins panel:
// the apps.json catalogue merged with CARE's live plug_config rows, which are the
// only record of what's switched on.
func (a *App) ListApps() ([]care.ClinicApp, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - nothing to offer
	}
	return a.engine(nil).ListApps()
}

// SetAppEnabled switches one catalogue app on or off by writing CARE's plug_config
// table - instant, no rebuild. Staff refresh their browser to see the change.
func (a *App) SetAppEnabled(slug string, enabled bool) error {
	return a.engine(nil).SetAppEnabled(slug, enabled)
}

// ReadFrontendPlugins returns CARE's live frontend plugins (plug_config rows) for
// the Frontend plugins editor - the same set CARE's own Apps page shows.
func (a *App) ReadFrontendPlugins() ([]care.FrontendPlugin, error) {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return []care.FrontendPlugin{}, nil // not set up yet
	}
	return a.engine(nil).ReadFrontendPlugins()
}

// SaveFrontendPlugins writes the whole plugin list to CARE's plug_config table
// (add + edit + remove). Instant - no rebuild, since CARE loads them at runtime.
func (a *App) SaveFrontendPlugins(plugins []care.FrontendPlugin) error {
	if _, err := os.Stat(filepath.Join(a.kitDir(), "docker-compose.yml")); err != nil {
		return errString("not set up yet - run the first-time setup")
	}
	return a.engine(nil).WriteFrontendPlugins(plugins)
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
