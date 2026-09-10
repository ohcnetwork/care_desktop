package clinic

import (
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/health"
)

// Start brings the stack up: ensure both images, mDNS, `compose up -d`, migrate,
// then a default admin. Mirrors `care.sh start`.
func (e *Clinic) Start() error {
	if err := health.EnsurePortFree(e.Runner(), e.host()); err != nil {
		return err
	}
	if err := e.Builder().EnsureBackendImage(); err != nil {
		return err
	}
	if err := e.Builder().EnsureFrontendImage(); err != nil {
		return err
	}
	if err := e.Builder().EnsureBackupImage(); err != nil {
		return err
	}
	if err := e.Builder().EnsureCaddyImage(); err != nil {
		return err
	}
	if err := e.Backups().EnsureKeysDir(); err != nil { // ./keys bind-mount source (rule R3)
		return err
	}
	e.warnDomainDrift()
	e.logln("Starting CARE...")
	// Migrate with a SINGLE migrator: bring up the api backend (its start.sh does
	// NOT migrate) + its deps, migrate to completion, then start the rest.
	// celery-beat's entrypoint also runs `migrate`, so starting everything at once
	// races two migrators and fails with "column ... already exists" whenever
	// migrations are pending. See migrate() below.
	// --wait blocks until these are *healthy*, not merely created, so migrate
	// below runs against a database and an app server that are actually ready.
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300", "db", "redis", "backend"); err != nil {
		return err
	}
	e.logln("Applying database migrations...")
	if err := e.migrate(); err != nil {
		return err
	}
	// --wait is what turns "containers created" into "every service reports
	// healthy". Without it the panel can show Running while minio, celery or the
	// backup sidecar are dead - none of them sit on the /ping/ path that
	// health.Wait probes. See docker-compose.yml: all nine now have a healthcheck.
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		return err
	}
	e.createAdmin()
	// Don't report success until the app actually answers on :80 - "up -d" only
	// means the containers were created. This is the gate the installer relies on
	// to mark the install complete.
	e.logln("Waiting for CARE to become healthy...")
	if err := health.Wait(e.Log, e.host(), 3*time.Minute); err != nil {
		return err
	}
	e.logln("")
	e.logln("CARE is up -> https://" + e.host() + "/   (login: admin)")
	// Only now: these need Caddy up (the CA lives in its volume).
	e.writeCertInstallers()
	e.ensureLocalAccess()
	return nil
}
