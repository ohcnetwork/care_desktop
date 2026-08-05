package care

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Start brings the stack up: ensure both images, mDNS, `compose up -d`, migrate,
// then a default admin. Mirrors `care.sh start`.
func (e *Engine) Start() error {
	if err := e.ValidateTLS(); err != nil {
		return err
	}
	if err := e.EnsurePortFree(); err != nil {
		return err
	}
	e.logln(e.tlsBanner())
	// Point the clinic's DNS record at wherever the router put us this time. Done
	// early so the change has the whole startup to propagate, and best-effort: an
	// unreachable Cloudflare must not stop a clinic that is otherwise fine from
	// coming up, and the record is usually already correct anyway.
	if err := e.SyncDNS(); err != nil {
		e.logln("note: couldn't update the DNS record (" + err.Error() + ")")
		e.logln("      devices can only reach CARE once " + e.publicHost() + " points at this computer")
	}
	if err := e.ensureBackendImage(); err != nil {
		return err
	}
	if err := e.ensureFrontendImage(); err != nil {
		return err
	}
	if err := e.ensureBackupImage(); err != nil {
		return err
	}
	if err := e.ensureCaddyImage(); err != nil {
		return err
	}
	if err := e.ensureKeysDir(); err != nil { // ./keys bind-mount source (empty = plaintext)
		return err
	}
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
	if err := e.migrate(); err != nil {
		return err
	}
	if err := e.dc("up", "-d"); err != nil {
		return err
	}
	e.createAdmin()
	// Don't report success until the app actually answers - "up -d" only means the
	// containers were created. This is the gate the installer relies on to mark the
	// install complete.
	// A first start also waits on the ACME DNS-01 round trip: write the TXT record,
	// wait for it to propagate, then issuance. Well under 5 minutes normally, but
	// slow DNS propagation can push it - hence the generous budget.
	e.logln("Waiting for CARE to become healthy (a first run also obtains the certificate)...")
	if err := e.WaitHealthy(8 * time.Minute); err != nil {
		return fmt.Errorf("%w\n\nIf this is the first start, check the certificate step:\n"+
			"  docker compose logs caddy | grep -i acme\n"+
			"Most failures here are the API token lacking Zone:DNS:Edit, or the domain "+
			"not being on Cloudflare's nameservers yet. See docs/tls.md", err)
	}
	e.logln("")
	e.logln("CARE is up -> " + e.PublicOrigin() + "/   (login: admin / admin)")
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
	if err := e.migrate(); err != nil {
		return err
	}
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

// BackupNow writes an immediate DB dump (encrypted -> .enc when encryption is on).
func (e *Engine) BackupNow() error {
	ts := time.Now().Format("20060102-150405")
	name := "care-manual-" + ts + ".dump"
	dump := `PGPASSWORD=$POSTGRES_PASSWORD pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -Fc -d "$POSTGRES_DB"`
	var script string
	if e.backupEncryptionOn() {
		name += ".enc"
		script = "set -e; " + dump +
			" | openssl cms -encrypt -binary -aes-256-cbc -stream -outform DER -out /backups/" + name + " /keys/backup-cert.pem"
	} else {
		script = dump + " -f /backups/" + name
	}
	if err := e.dc("exec", "-T", "backup", "sh", "-c", script); err != nil {
		return err
	}
	e.logln("Backup written to " + filepath.Join(e.backupDir(), name))
	return nil
}

// migrate runs migrations, retrying until the backend/db are ready. It returns an
// error if they never come up, so callers don't proceed to report success on a
// half-migrated stack.
//
// The api container's start.sh does NOT migrate, so we do (idempotent). But
// celery-beat's entrypoint DOES run `migrate` on boot - so callers must run this
// while celery-beat is stopped (bring up only db+redis+backend first), or the two
// migrate processes race and fail with "column ... already exists" on any pending
// migration. Start/RebuildBackend/Restore all order things that way.
func (e *Engine) migrate() error {
	for n := 1; n <= 20; n++ {
		if err := e.dc("exec", "-T", "backend", "python", "manage.py", "migrate", "--noinput"); err == nil {
			return nil
		}
		e.logln(fmt.Sprintf("  waiting for backend/db... (%d)", n))
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("database migrations did not complete - backend/db not ready")
}

// createAdmin makes a default admin/admin superuser. Idempotent: on an existing
// install createsuperuser exits 1 with "username already taken", which is the
// normal case - we report it plainly. Output is captured (not streamed) so the
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
		e.logln("Created admin login (username: admin) - change the password in the app.")
	case strings.Contains(string(out), "already taken"):
		e.logln("Admin login already exists (left unchanged).")
	default:
		e.logln("Admin login not created: " + strings.TrimSpace(string(out)))
	}
}
