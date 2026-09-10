package main

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
	"github.com/ohcnetwork/care_desktop/app/internal/clinic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// --- uninstall --------------------------------------------------------------

// RunUninstall tears the install down (async, streaming logs), then clears the
// app's own state - autostart entry and saved config - and signals the UI to
// reset to first-run via an "uninstalled" event.
func (a *App) RunUninstall(removeImages, removeBackups bool) error {
	e := a.engine(nil)
	go func() {
		_ = e.Uninstall(clinic.UninstallOptions{
			RemoveImages:     removeImages,
			RemoveInstallDir: true,
			RemoveBackups:    removeBackups,
		})
		_ = a.SetAutostart(false) // remove the login-item, if any
		backup.ForgetPassword()   // drop the remembered backup password, if any
		// The whole app-support folder, not just config.json: the install dir is unpacked
		// underneath it, and an orphaned directory is still a change we made.
		_ = os.RemoveAll(filepath.Dir(a.configPath()))
		// No blanket "the name was not changed back" note any more: the default
		// mode never renames the machine, and when a rename-mode install did,
		// Uninstall reverts it and reports anything it genuinely could not.
		wruntime.EventsEmit(a.ctx, "uninstalled", true)
	}()
	return nil
}
