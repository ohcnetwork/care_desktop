package clinic

import (
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/health"
)

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
	e.logln("Starting CARE...")
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300", "db", "redis", "backend"); err != nil {
		return err
	}
	e.logln("Applying database migrations...")
	if err := e.migrate(); err != nil {
		return err
	}
	if err := e.dc("up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		return err
	}
	e.createAdmin()
	e.logln("Waiting for CARE to become healthy...")
	if err := health.Wait(e.Log, 3*time.Minute); err != nil {
		return err
	}
	e.logln("")
	e.logln("CARE is up -> https://" + e.host() + "/   (login: admin)")
	e.writeDeviceSetupScripts()
	e.setUpThisComputer()
	return nil
}
