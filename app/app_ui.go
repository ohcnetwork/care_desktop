package main

import (
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

// AutostartEnabled reports whether the app is set to launch at login.
func (a *App) AutostartEnabled() bool { return autostart.Enabled() }

// SetAutostart turns launch-at-login on or off.
func (a *App) SetAutostart(on bool) error { return autostart.Set(on) }
