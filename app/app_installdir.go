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

func (a *App) installDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.UserHomeDir()
	}
	if runtime.GOOS == "windows" {
		if home, herr := os.UserHomeDir(); herr == nil {
			base = home
		}
	}
	return filepath.Join(base, appDirName, installSubdir)
}

const installSubdir = "install"

var installUserFiles = map[string]bool{"backend.env": true, "frontend.env": true}

const gitkeepPlaceholder = ".gitkeep"

func (a *App) ensureInstallDir() (string, error) {
	dest := a.installDir()
	err := fs.WalkDir(a.installFS, "install", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, "install")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || rel == gitkeepPlaceholder {
			return nil
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if installUserFiles[rel] {
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

func (a *App) engine() *clinic.Clinic {
	cfg := a.loadConfig()
	return &clinic.Clinic{
		InstallDir: a.installDir(),
		MDNSName:   strings.TrimSuffix(strings.TrimSpace(cfg.MDNSName), ".local"),
		BackupDir:  cfg.BackupDir,
		Pins:       a.pins,
		Log:        a.logln,
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
