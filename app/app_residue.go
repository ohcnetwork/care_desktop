package main

import (
	"errors"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
	"github.com/ohcnetwork/care_desktop/app/internal/clinic"
	"github.com/ohcnetwork/care_desktop/app/internal/residue"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) ScanResidue() residue.Report {
	e := a.engine()
	return residue.Scan(residue.Options{
		Runner:       e.Runner(),
		Project:      e.Project(),
		InstallDir:   a.residueInstallDir(e),
		ConfigPath:   a.configPath(),
		Images:       e.Images(),
		StoredSecret: backup.HasPassword(),
	})
}

func (a *App) PurgeResidue() error {
	if a.loadConfig().SetupDone {
		return errors.New("this computer already has a clinic set up - use Uninstall in the panel instead")
	}

	before := a.ScanResidue()
	if before.Clean {
		return nil
	}

	items := ""
	for _, t := range before.Traces {
		items += "\n  • " + t.Label + " — " + t.Detail
	}

	kept := "\n\nYour backups are NOT touched."
	if dir := a.loadConfig().BackupDir; dir != "" {
		kept = "\n\nYour backups in " + dir + " are NOT touched."
	}
	if backup.HasPassword() {
		kept += " Restoring them later needs your backup password, and this removes " +
			"the saved copy from this computer - make sure you know it before continuing."
	}
	sel, err := wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.QuestionDialog,
		Title: "Remove the earlier CARE Desktop?",
		Message: "This computer still has these from an earlier CARE Desktop:\n" + items +
			"\n\nEverything above is deleted - clinic data, images, settings, installed files, " +
			"and this app's log files. " +
			"This cannot be undone." + kept +
			"\n\nYour computer needs to be clean before a new clinic can be set up.",
		Buttons:       []string{"Remove everything", "Cancel"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil || sel != "Remove everything" {
		return nil
	}

	a.logln("Removing the earlier CARE Desktop from this computer...")
	e := a.engine()
	e.InstallDir = a.residueInstallDir(e)
	if err := e.Purge(); err != nil {
		return err
	}
	backup.ForgetPassword()
	a.forgetConfig()

	if err := a.log.PurgeFolder(); err != nil {
		a.logln("note: couldn't remove the old log files (" + err.Error() + ")")
	}

	after := a.ScanResidue()
	a.reportPurge(after)
	return nil
}

func (a *App) residueInstallDir(e *clinic.Clinic) string {
	return residue.InstallDirFrom(e.Runner(), e.Project(), e.InstallDir)
}

func (a *App) reportPurge(after residue.Report) {
	if after.Clean {
		_, _ = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
			Type:    wruntime.InfoDialog,
			Title:   "This computer is clean",
			Message: "Everything from the earlier CARE Desktop has been removed. You can go ahead and set up the clinic.",
			Buttons: []string{"Continue"},
		})
		return
	}

	left := ""
	for _, t := range after.Traces {
		left += "\n  • " + t.Label + " — " + t.Detail
	}
	_, _ = wruntime.MessageDialog(a.ctx, wruntime.MessageDialogOptions{
		Type:  wruntime.WarningDialog,
		Title: "Some things are still here",
		Message: "Most of the earlier CARE Desktop was removed, but not these:" + left +
			"\n\nThey usually need an administrator. Try again, or ask your IT support to remove them.",
		Buttons: []string{"OK"},
	})
}
