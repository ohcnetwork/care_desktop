package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
	"github.com/ohcnetwork/care_desktop/app/internal/clinic"
	"github.com/ohcnetwork/care_desktop/app/internal/health"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/bcrypt"
)

// --- actions (async; stream logs, finish with a care-done event) ------------

// run executes fn on a goroutine, streaming its log lines to the UI and ending
// with a care-done event carrying the exit code.
func (a *App) run(fn func() error, markSetup bool, label string) {
	go func() {
		code := 0
		// Without this a panic anywhere in Setup, Start, Restore or Uninstall takes
		// the whole process down instantly: no dialog, no care-done, no trace, and
		// from the operator's side indistinguishable from the app simply vanishing.
		// Recovering turns the worst failure mode into the best-documented one.
		defer func() {
			if r := recover(); r != nil {
				code = 1
				a.log.Writef("PANIC in %s: %v\n%s", label, r, debug.Stack())
				detail := fmt.Sprintf("CARE hit an internal error during %s: %v", label, r)
				a.logln("error: " + detail)
				a.notifyActionFailed(label, detail)
				wruntime.EventsEmit(a.ctx, "care-done", code)
			}
		}()
		if err := fn(); err != nil {
			a.logln("error: " + err.Error())
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
		Message:       "",
		Buttons:       []string{"Open CARE", "Close"},
		DefaultButton: "Open CARE",
	})
	if sel == "Open CARE" {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}

// CareAction runs one named action against the existing install dir. The set of
// names actionFunc recognises is the whitelist; there is no second list to keep
// in step with it.
func (a *App) CareAction(action string) error {
	fn := actionFunc(a.engine(), action)
	if fn == nil {
		return errors.New("action not allowed: " + action)
	}
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	a.run(fn, false, action)
	return nil
}

func actionFunc(e *clinic.Clinic, action string) func() error {
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
	return nil
}

// RunSetup persists the wizard's choices, unpacks the install dir, then runs setup+start.
// rememberBackup saves the backup password to the keychain.
//
// Both passwords are required. Every backup is encrypted, so a blank backup
// password is not "encryption off" any more - it is an install that cannot write
// a backup at all. Rejected here rather than minutes later inside
// GenBackupKeypair, so the operator is told at the form they are still looking at.
func (a *App) RunSetup(mdnsName, adminPassword, backupPassword string, rememberBackup bool, installDir, backupDir string) error {
	if err := ValidatePassword(adminPassword); err != nil {
		return err
	}
	if err := ValidatePassword(backupPassword); err != nil {
		return err
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
	// Refuse here rather than minutes in: the engine's first act is to mkdir the
	// backup folder. Checked against the folder this run will actually use, so a
	// bad one carried over from an earlier attempt is caught too.
	backupParent := strings.TrimSpace(backupDir)
	if backupParent == "" && cfg.BackupDir != "" {
		backupParent = filepath.Dir(cfg.BackupDir)
	}
	if problem := a.ValidateBackupDir(backupParent); problem != "" {
		return errors.New(problem)
	}
	if err := a.saveConfig(cfg); err != nil {
		return err
	}
	a.restartAdvertise() // pick up the chosen name (usually still care.local)
	if _, err := a.ensureInstallDir(); err != nil {
		return err
	}

	// Best-effort: a keychain failure shouldn't abort the install.
	if backupPassword != "" && rememberBackup {
		if err := backup.StorePassword(backupPassword); err != nil {
			a.logln("note: couldn't save the backup password to the keychain (" + err.Error() + ")")
		}
	}

	e := a.engine()
	e.MDNSName = host
	e.AdminPassword = adminPassword
	e.BackupPassword = backupPassword
	a.run(func() error {
		// Check port 80 before the ~10-min build, so a conflict fails immediately
		// instead of after the wait (Start re-checks in case it's taken meanwhile).
		if err := health.EnsurePortFree(e.Runner(), e.Host()); err != nil {
			return err
		}
		if err := e.Setup(); err != nil {
			return err
		}
		return e.Start()
	}, true, "setup")
	return nil
}

// CleanupFailedInstall lets Retry start clean: tear down the leftover Docker project
// (a crash-looping container keeps re-creating the deleted install dir files), then wipe the
// staged install dir and this app's config folders. Safe on a failed first-run - patient data
// lives in Docker volumes, not here.
//
// This used to be Windows-only, which left the retry on macOS and Linux running
// against the config the failed attempt had already written. RunSetup only
// overwrites a saved backup folder when a new one is picked, so a folder that
// broke the install - a read-only drive, say - survived into every retry and
// failed it again, with no way to clear it from the wizard.
func (a *App) CleanupFailedInstall() error {
	a.engine().TeardownProject()

	var firstErr error
	remove := func(target string) {
		if target == "" {
			return
		}
		a.logln("cleanup: removing " + target)
		if err := os.RemoveAll(target); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	remove(a.installDir())               // before the config wipe below, since installDir reads config
	remove(filepath.Dir(a.configPath())) // our own folder, named exactly
	if runtime.GOOS == "windows" {
		// The installer can land our data under any of several spellings
		// ("care-desktop", "CARE Desktop", ...), so %AppData% is swept by name.
		// Only on Windows: elsewhere UserConfigDir is a shared directory
		// (~/Library/Application Support, ~/.config) where a prefix match could
		// hit another vendor's "care..." folder.
		if base, err := os.UserConfigDir(); err == nil {
			if entries, err := os.ReadDir(base); err == nil {
				for _, ent := range entries {
					if careAppDataName(ent.Name()) {
						remove(filepath.Join(base, ent.Name()))
					}
				}
			}
		}
	}
	return firstErr
}

// careAppDataName matches this app's %AppData% folders (care-desktop, "CARE Desktop",
// ...), case- and separator-insensitively, so cleanup only deletes our own.
func careAppDataName(name string) bool {
	n := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(name))
	return strings.HasPrefix(n, "care")
}

func (a *App) CareStatus() (string, error) { return a.engine().Status() }
