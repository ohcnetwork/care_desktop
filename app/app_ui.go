package main

import (
	"errors"
	"net/url"
	"os"
	"slices"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"

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
	dir := a.log.Folder()
	if dir == "" {
		return errors.New("this run isn't writing a log file - the log folder couldn't be opened for writing")
	}
	wruntime.BrowserOpenURL(a.ctx, (&url.URL{Scheme: "file", Path: dir}).String())
	return nil
}

func (a *App) AutostartEnabled() bool { return autostart.Enabled() }

func (a *App) SetAutostart(on bool) error { return autostart.Set(on) }
