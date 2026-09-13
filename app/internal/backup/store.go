// Package backup lists, creates, encrypts, and restores CARE's database dumps
// and file archives. See docs/backups.md.
package backup

import (
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Store operates on one install directory's backups.
type Store struct {
	Dir       string       // install dir (holds docker-compose.yml and keys/)
	BackupDir string       // where dumps and archives are written
	Image     string       // the backup helper image, e.g. care-backup:clinic
	Host      string       // clinic address, e.g. care.local
	Project   string       // compose project name; volumes are <project>_<volume>
	Log       func(string) // optional line sink

	// EnsureImage builds the backup image if it is missing; Migrate runs the
	// backend's database migrations. Both are owned by the compose layer.
	EnsureImage func() error
	Migrate     func() error

	run proc.Runner
}

// New binds a Store to a command runner.
func New(run proc.Runner, s *Store) *Store {
	s.run = run
	return s
}

func (s *Store) logln(line string) {
	if s.Log != nil {
		s.Log(line)
	}
}

func (s *Store) dc(args ...string) error {
	return s.run.Run("docker", append([]string{"compose"}, args...)...)
}
