package main

import (
	"errors"
	"net/url"
	"os"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- misc UI helpers --------------------------------------------------------

func (a *App) OpenURL(url string) { wruntime.BrowserOpenURL(a.ctx, url) }

func (a *App) ChooseFolder(title string) string {
	opts := wruntime.OpenDialogOptions{Title: title}
	// Without a starting point the panel opens wherever the app was launched
	// from — which, run straight off the DMG, is a read-only volume, and the
	// operator only finds out mid-install.
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
	for _, arg := range os.Args {
		if arg == "--autostart" {
			return true
		}
	}
	return false
}

// LogPath is the diagnostic log this run is writing to, for the panel to show.
func (a *App) LogPath() string { return a.log.Path() }

// OpenLogFolder reveals the log directory in Finder/Explorer. The file is only
// useful if the operator can find it without being told a path over the phone.
func (a *App) OpenLogFolder() error {
	dir := a.log.Folder()
	if dir == "" {
		return errors.New("this run isn't writing a log file - the log folder couldn't be opened for writing")
	}
	// url.URL rather than "file://"+dir: the operator's home or chosen parent can
	// contain spaces, and an unescaped one makes the URL a no-op on Windows.
	wruntime.BrowserOpenURL(a.ctx, (&url.URL{Scheme: "file", Path: dir}).String())
	return nil
}

// AutostartEnabled reports whether the app is set to launch at login.
func (a *App) AutostartEnabled() bool { return autostart.Enabled() }

// SetAutostart turns launch-at-login on or off.
func (a *App) SetAutostart(on bool) error { return autostart.Set(on) }
