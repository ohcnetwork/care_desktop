package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/backup"
)

// --- restore ----------------------------------------------------------------

// ListBackups returns the restorable points in the backup folder (newest first)
// for the panel's restore dropdown.
func (a *App) ListBackups() ([]backup.Backup, error) {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return nil, nil // not set up yet - no backups to offer
	}
	return a.engine(nil).Backups().ListBackups()
}

// RestoreBackup restores async. For an encrypted backup, "" passphrase falls back to
// the keychain; remember saves the one that worked.
func (a *App) RestoreBackup(dbDump, filesArchive, passphrase string, remember bool) error {
	if _, err := os.Stat(filepath.Join(a.installDir(), "docker-compose.yml")); err != nil {
		return errors.New("not set up yet - run the first-time setup")
	}
	if passphrase == "" {
		passphrase = backup.LoadPassword()
	}
	if passphrase != "" && remember {
		_ = backup.StorePassword(passphrase)
	}
	e := a.engine(nil)
	a.run(func() error { return e.Backups().Restore(dbDump, filesArchive, passphrase) }, false, "restore")
	return nil
}
