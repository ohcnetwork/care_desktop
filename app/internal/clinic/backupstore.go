package clinic

import (
	"github.com/ohcnetwork/care_desktop/app/internal/backup"
)

const composeProject = "care-desktop"

func (e *Clinic) Backups() *backup.Store {
	s := &backup.Store{
		Dir:        e.InstallDir,
		BackupDir:  e.backupDir(),
		Passphrase: e.BackupPassword,
		Image:      e.Pins.BackupImage,
		Host:       e.host(),
		Log:        e.Log,
		Migrate:    e.migrate,
	}
	s.EnsureImage = e.Builder().EnsureBackupImage
	return backup.New(e.Runner(), s)
}

func (e *Clinic) BackupDirPath() string { return e.backupDir() }

func (e *Clinic) RestartBackupSidecar() error {
	return e.dc("up", "-d", "--force-recreate", "backup")
}
