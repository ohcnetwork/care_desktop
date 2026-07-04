package care

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Start brings the stack up: ensure both images, mDNS, `compose up -d`, migrate,
// then a default admin. Mirrors `care.sh start`.
func (e *Engine) Start() error {
	if err := e.ensureBackendImage(); err != nil {
		return err
	}
	if err := e.ensureFrontendImage(); err != nil {
		return err
	}
	e.ensureMDNS()
	e.logln("Starting CARE...")
	// Migrate with a SINGLE migrator: bring up the api backend (its start.sh does
	// NOT migrate) + its deps, migrate to completion, then start the rest.
	// celery-beat's entrypoint also runs `migrate`, so starting everything at once
	// races two migrators and fails with "column ... already exists" whenever
	// migrations are pending. See migrate() below.
	if err := e.dc("up", "-d", "db", "redis", "backend"); err != nil {
		return err
	}
	e.logln("Applying database migrations...")
	e.migrate()
	if err := e.dc("up", "-d"); err != nil {
		return err
	}
	e.createAdmin()
	e.logln("")
	e.logln("CARE is up → http://" + e.mdnsName() + ".local/   (login: admin / admin)")
	return nil
}

func (e *Engine) Stop() error    { e.logln("Stopping CARE (data kept)..."); return e.dc("stop") }
func (e *Engine) Restart() error { e.logln("Restarting CARE..."); return e.dc("restart") }

// RebuildBackend rebuilds the backend image (after new backend code) and restarts
// the Django + celery services, then migrates.
func (e *Engine) RebuildBackend() error {
	if err := e.buildBackend(); err != nil {
		return err
	}
	// Recreate + migrate the api backend BEFORE celery-beat (which also migrates),
	// so the two don't race on the freshly-built code's new migrations.
	if err := e.dc("up", "-d", "backend"); err != nil {
		return err
	}
	e.migrate()
	if err := e.dc("up", "-d", "celery-worker", "celery-beat"); err != nil {
		return err
	}
	e.logln("Backend rebuilt and restarted.")
	return nil
}

// RebuildFrontend rebuilds the FE image (Vite bakes REACT_* at build time) and
// restarts the frontend service.
func (e *Engine) RebuildFrontend() error {
	if err := e.buildFrontend(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "frontend"); err != nil {
		return err
	}
	e.logln("Frontend rebuilt and restarted.")
	return nil
}

// Status returns `docker compose ps` as "<service> <state>" lines.
func (e *Engine) Status() (string, error) {
	return e.capture("docker", "compose", "ps", "--format", "{{.Service}} {{.State}}")
}

// BackupNow writes an immediate database dump into the backup folder.
func (e *Engine) BackupNow() error {
	ts := time.Now().Format("20060102-150405")
	script := fmt.Sprintf(
		`PGPASSWORD=$POSTGRES_PASSWORD pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -Fc -d "$POSTGRES_DB" -f /backups/care-manual-%s.dump`, ts)
	if err := e.dc("exec", "-T", "backup", "sh", "-c", script); err != nil {
		return err
	}
	e.logln("Backup written to " + e.backupDir() + "/care-manual-" + ts + ".dump")
	return nil
}

// migrate runs migrations, retrying until the backend/db are ready.
//
// The api container's start.sh does NOT migrate, so we do (idempotent). But
// celery-beat's entrypoint DOES run `migrate` on boot — so callers must run this
// while celery-beat is stopped (bring up only db+redis+backend first), or the two
// migrate processes race and fail with "column ... already exists" on any pending
// migration. Start/RebuildBackend/Restore all order things that way.
func (e *Engine) migrate() {
	for n := 1; n <= 20; n++ {
		if err := e.dc("exec", "-T", "backend", "python", "manage.py", "migrate", "--noinput"); err == nil {
			return
		}
		e.logln(fmt.Sprintf("  waiting for backend/db... (%d)", n))
		time.Sleep(5 * time.Second)
	}
	e.logln("backend not ready — run start again shortly")
}

// createAdmin makes a default admin/admin superuser. Idempotent: on an existing
// install createsuperuser exits 1 with "username already taken", which is the
// normal case — we report it plainly. Output is captured (not streamed) so the
// raw "CommandError ... exit status 1" never leaks into the log.
func (e *Engine) createAdmin() {
	cmd := exec.Command("docker", "compose", "exec", "-T",
		"-e", "DJANGO_SUPERUSER_PASSWORD="+e.adminPassword(),
		"backend", "python", "manage.py", "createsuperuser", "--noinput",
		"--username", "admin", "--email", "admin@care.local")
	cmd.Env = e.baseEnv()
	cmd.Dir = e.workdir()
	out, err := cmd.CombinedOutput()
	switch {
	case err == nil:
		e.logln("Created admin login (username: admin) — change the password in the app.")
	case strings.Contains(string(out), "already taken"):
		e.logln("Admin login already exists (left unchanged).")
	default:
		e.logln("Admin login not created: " + strings.TrimSpace(string(out)))
	}
}
