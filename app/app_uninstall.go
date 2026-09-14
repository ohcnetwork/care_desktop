package main

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
	"github.com/ohcnetwork/care_desktop/app/internal/clinic"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) RunUninstall(removeImages, removeBackups bool) error {
	e := a.engine()
	go func() {
		_ = e.Uninstall(clinic.UninstallOptions{
			RemoveImages:     removeImages,
			RemoveInstallDir: true,
			RemoveBackups:    removeBackups,
		})
		_ = a.SetAutostart(false)
		backup.ForgetPassword()
		_ = os.RemoveAll(filepath.Dir(a.configPath()))

		a.reportUninstall(removeImages)
		wruntime.EventsEmit(a.ctx, "uninstalled", true)
	}()
	return nil
}

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
