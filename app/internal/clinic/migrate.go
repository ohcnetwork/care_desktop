package clinic

import (
	"fmt"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func (e *Clinic) migrate() error {
	for n := 1; n <= 20; n++ {
		if err := e.dc("exec", "-T", "backend", "python", "manage.py", "migrate", "--noinput"); err == nil {
			return nil
		}
		e.logln(fmt.Sprintf("  waiting for backend/db... (%d)", n))
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database migrations did not complete - backend/db not ready")
}

func (e *Clinic) createAdmin() {
	if e.AdminPassword == "" {
		return
	}
	cmd := proc.Command("docker", "compose", "exec", "-T",
		"-e", "DJANGO_SUPERUSER_PASSWORD="+e.AdminPassword,
		"backend", "python", "manage.py", "createsuperuser", "--noinput",
		"--username", "admin", "--email", "admin@care.local")
	cmd.Env = e.baseEnv()
	cmd.Dir = e.workdir()
	out, err := cmd.CombinedOutput()
	switch {
	case err == nil:
		e.logln("Created admin login (username: admin).")
	case strings.Contains(string(out), "already taken"):
		e.logln("Admin login already exists (left unchanged).")
	default:
		e.logln("Admin login not created: " + strings.TrimSpace(string(out)))
	}
}
