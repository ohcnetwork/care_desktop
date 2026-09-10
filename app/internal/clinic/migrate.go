package clinic

import (
	"fmt"
	"strings"
	"time"
)

// migrate runs migrations, retrying until the backend/db are ready. It returns an
// error if they never come up, so callers don't proceed to report success on a
// half-migrated stack.
//
// The api container's start.sh does NOT migrate, so we do (idempotent). But
// celery-beat's entrypoint DOES run `migrate` on boot - so callers must run this
// while celery-beat is stopped (bring up only db+redis+backend first), or the two
// migrate processes race and fail with "column ... already exists" on any pending
// migration. Start/RebuildBackend/Restore all order things that way.
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

// createAdmin makes the "admin" superuser with the password chosen during setup.
// Idempotent: on an existing
// install createsuperuser exits 1 with "username already taken", which is the
// normal case - we report it plainly. Output is captured (not streamed) so the
// raw "CommandError ... exit status 1" never leaks into the log.
func (e *Clinic) createAdmin() {
	cmd := newCmd("docker", "compose", "exec", "-T",
		"-e", "DJANGO_SUPERUSER_PASSWORD="+e.adminPassword(),
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
