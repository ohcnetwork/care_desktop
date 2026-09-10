package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ohcnetwork/care_desktop/app/internal/clinic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- install dir location + first-run unpack ----------------------------------------

func (a *App) installDir() string {
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
	if runtime.GOOS == "windows" {
		// Docker Desktop can't read files under %AppData% live on Windows, so install dir bind
		// mounts arrive as empty dirs; stage under the home dir, which it reads live.
		// (config.json stays in %AppData% - Docker never reads it.)
		if home, herr := os.UserHomeDir(); herr == nil {
			base = home
		}
	}
	return filepath.Join(base, "care-desktop", "install")
}

// installUserFiles are preserved on an install-dir refresh; everything else is app-owned.
var installUserFiles = map[string]bool{"backend.env": true, "frontend.env": true}

// ensureInstallDir syncs the embedded install dir on every setup (not just the first) so an updated
// app delivers new/changed files to an existing install, keeping edited env files.
// Fixes the stale-install dir failure where setup can't find a newly added install dir file.
func (a *App) ensureInstallDir() (string, error) {
	dest := a.installDir()
	err := fs.WalkDir(a.installFS, "install", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "install")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if installUserFiles[rel] { // keep the user's edits; only seed when absent
			if _, err := os.Stat(target); err == nil {
				return nil
			}
		}
		data, err := fs.ReadFile(a.installFS, p)
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

// engine builds an Engine bound to the install dir, streaming logs to the UI.
func (a *App) engine(extra map[string]string) *clinic.Clinic {
	env := map[string]string{}
	if cfg := a.loadConfig(); cfg.BackupDir != "" {
		env["BACKUP_DIR"] = cfg.BackupDir
	}
	for k, v := range extra {
		env[k] = v
	}
	return &clinic.Clinic{
		InstallDir: a.installDir(),
		Env:        env,
		Log:        func(s string) { wruntime.EventsEmit(a.ctx, "care-log", s) },
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
