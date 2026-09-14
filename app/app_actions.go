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
	"github.com/ohcnetwork/care_desktop/app/internal/sys/mdns"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/bcrypt"
)

func (a *App) run(fn func() error, markSetup bool, label string) error {
	if !a.jobMu.TryLock() {
		return errors.New("something else is still running - wait for it to finish")
	}
	go func() {
		defer a.jobMu.Unlock()
		code := 0
		defer func() {
			if r := recover(); r != nil {
				code = 1
				a.log.Writef("PANIC in %s: %v\n%s", label, r, debug.Stack())
				detail := fmt.Sprintf("CARE hit an internal error during %s: %v", label, r)
				a.logln("error: " + detail)
				if !markSetup {
					a.notifyActionFailed(label, detail)
				}
				wruntime.EventsEmit(a.ctx, "care-done", code)
			}
		}()
		if err := fn(); err != nil {
			a.logln("error: " + err.Error())
			code = 1
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
	return nil
}

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
	case "backup-now":
		title = "Backup didn't finish"
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

func (a *App) notifyInstalled(mdnsName string) {
	if mdnsName == "" {
		mdnsName = "care.local"
	}
	url := "https://" + mdnsName + "/"
	sel, _ := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:          wruntime.InfoDialog,
		Title:         "CARE Desktop installed",
		Message:       "Staff can open the clinic at " + url,
		Buttons:       []string{"Open CARE", "Close"},
		DefaultButton: "Open CARE",
	})
	if sel == "Open CARE" {
		wruntime.BrowserOpenURL(a.ctx, url)
	}
}

func (a *App) ClinicAction(action string) error {
	fn := actionFunc(a.engine(), action)
	if fn == nil {
		return errors.New("action not allowed: " + action)
	}
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	return a.run(fn, false, action)
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

func (a *App) RunSetup(mdnsName, adminPassword, backupPassword, backupDir string) error {
	if err := ValidatePassword(adminPassword); err != nil {
		return err
	}
	if err := ValidatePassword(backupPassword); err != nil {
		return err
	}

	if err := backup.StorePassword(backupPassword); err != nil {
		return fmt.Errorf("couldn't save the backup password to this computer's keychain: %w", err)
	}

	host := mdns.Label(mdnsName)
	if host == "" {
		host = "care"
	}

	cfg := a.loadConfig()
	cfg.MDNSName = host + ".local"
	h, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("couldn't secure the admin password: %w", err)
	}
	cfg.AdminPwHash = string(h)
	if strings.TrimSpace(backupDir) != "" {
		cfg.BackupDir = filepath.Join(strings.TrimSpace(backupDir), "care-db-backups")
	}
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
	a.restartAdvertise()
	if _, err := a.ensureInstallDir(); err != nil {
		return err
	}

	e := a.engine()
	e.MDNSName = host
	e.AdminPassword = adminPassword
	e.BackupPassword = backupPassword
	return a.run(func() error {
		if err := health.EnsurePortFree(e.Runner(), e.Host()); err != nil {
			return err
		}
		if err := e.Setup(); err != nil {
			return err
		}
		return e.Start()
	}, true, "setup")
}

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

	remove(a.installDir())
	remove(filepath.Dir(a.configPath())) // our own folder, named exactly
	if runtime.GOOS == "windows" {
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

func careAppDataName(name string) bool {
	n := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToLower(name))
	return strings.HasPrefix(n, "care")
}

func (a *App) ClinicStatus() (string, error) { return a.engine().Status() }
