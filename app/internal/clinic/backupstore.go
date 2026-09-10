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
		Passphrase: e.backupPassword(),
		Image:      e.backupImage(),
		Host:       e.host(),
		Log:        e.Log,
		Migrate:    e.migrate,
	}
	s.EnsureImage = e.Builder().EnsureBackupImage
	return backup.New(e.Runner(), s)
}
