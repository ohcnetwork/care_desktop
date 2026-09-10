package clinic

import (
	"github.com/ohcnetwork/care_desktop/app/internal/backup"
)

// composeProject must match the `name:` key in docker-compose.yml. Volumes are
// named <project>_<volume>; changing it orphans every existing volume.
const composeProject = "care-desktop"

// Backups binds a backup store to this engine's install dir and runner.
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

// BackupDirPath is where backups are written, resolved (config value or default).
func (e *Clinic) BackupDirPath() string { return e.backupDir() }

// RestartBackupSidecar recreates the backup container so a changed BACKUP_DIR
// takes effect: bind mounts are fixed when a container is created, so until it is
// recreated the sidecar keeps writing to the folder it started with.
func (e *Clinic) RestartBackupSidecar() error {
	return e.dc("up", "-d", "--force-recreate", "backup")
}
