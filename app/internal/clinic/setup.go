package clinic

import (
	"os"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
)

func (e *Clinic) Setup() error {
	if err := backup.CheckLocation(e.backupDir(), e.InstallDir); err != nil {
		return err
	}
	if err := e.genSecret(); err != nil {
		return err
	}
	if err := e.ApplyDomain(); err != nil {
		return err
	}
	if err := os.MkdirAll(e.backupDir(), 0o755); err != nil {
		return err
	}
	e.logln("Backups will go to: " + e.backupDir())
	if err := e.Backups().EnsureKeysDir(); err != nil {
		return err
	}
	if err := e.Builder().EnsureBackupImage(); err != nil {
		return err
	}
	if err := e.Backups().GenBackupKeypair(e.BackupPassword); err != nil {
		return err
	}
	if err := e.Builder().EnsureCaddyImage(); err != nil {
		return err
	}
	if err := e.Builder().EnsureBackendImage(); err != nil {
		return err
	}
	if err := e.Builder().EnsureFrontendImage(); err != nil {
		return err
	}
	e.logln("Setup done.")
	return nil
}
