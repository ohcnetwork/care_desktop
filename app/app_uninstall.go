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
	e := a.engine()
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
		a.reportUninstall(removeImages)
		wruntime.EventsEmit(a.ctx, "uninstalled", true)
	}()
	return nil
}

// reportUninstall re-scans the computer and names whatever is still on it. The
// scan is the verdict, not the teardown's exit status: the steps that can fail
// are the ones behind an elevation prompt the operator is free to dismiss, and
// they fail silently into the log otherwise. Runs before the "uninstalled"
// event so this dialog is not stacked under the first-run screen.
func (a *App) reportUninstall(removeImages bool) {
	var items string
	for _, t := range a.ScanResidue().Traces {
		if t.ID == "images" && !removeImages {
			continue // kept on purpose
		}
		items += "\n  \u2022 " + t.Label + " \u2014 " + t.Detail
	}
	if items == "" {
		return
	}
	_, _ = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.WarningDialog,
		Title: "Some things are still here",
		Message: "The clinic was removed, but these are still on this computer:" + items +
			"\n\nThey usually need an administrator rights. Setting up a clinic again offers to " +
			"clear them, or ask your IT support to remove them.",
		Buttons: []string{"OK"},
	})
}
