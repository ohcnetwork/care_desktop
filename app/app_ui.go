package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) OpenURL(url string) { wruntime.BrowserOpenURL(a.ctx, url) }

func (a *App) ChooseFolder(title string) string {
	opts := wruntime.OpenDialogOptions{Title: title}
	if home, err := os.UserHomeDir(); err == nil {
		opts.DefaultDirectory = home
	}
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil {
		return ""
	}
	return dir
}

func (a *App) WasAutostartLaunched() bool {
	return slices.Contains(os.Args, "--autostart")
}

func (a *App) LogPath() string { return a.log.Path() }

func (a *App) OpenLogFolder() error {
	path := a.log.Path()
	if path == "" {
		return errors.New("this run isn't writing a log file - the log folder couldn't be opened for writing")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = proc.Command("open", "-R", path)
	case "windows":
		cmd = proc.Command("explorer.exe", "/select,"+path)
	default:
		cmd = proc.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't open the log folder: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (a *App) AutostartEnabled() bool { return autostart.Enabled() }

func (a *App) SetAutostart(on bool) error {
	return a.withJob(func() error { return autostart.Set(on) })
}
