package backup

import (
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Store struct {
	Dir       string
	BackupDir string
	Image     string
	Host      string
	Project   string
	Log       func(string)

	EnsureImage func() error
	Migrate     func() error

	run proc.Runner
}

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
