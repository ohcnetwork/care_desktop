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
	if err := e.Backups().EnsureKeysDir(); err != nil { // ./keys bind-mount source (empty = plaintext)
		return err
	}
	e.ensureMDNS()
	e.warnDomainDrift()
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
