package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"
)

func (e *Clinic) Project() string { return composeProject }

func (e *Clinic) Images() []string { return e.uninstallImages() }

func (e *Clinic) Purge() error {
	rootPEM := e.caddyRootPEM()

	if _, err := os.Stat(filepath.Join(e.InstallDir, "docker-compose.yml")); err == nil {
		e.logln("Removing containers, network, and data volumes...")
		if err := e.dc("down", "-v", "--remove-orphans"); err != nil {
			e.logln("  (compose down reported an error - continuing cleanup)")
		}
	}
	e.TeardownProject()

	e.logln("Removing Docker images...")
	for _, img := range e.uninstallImages() {
		e.removeImage(img)
	}
	e.pruneBuildCache()

	e.removeInstallFiles()

	failed := e.revertSystemChanges(rootPEM)

	if autostart.Enabled() {
		e.logln("Removing the start-at-login entry...")
		if err := autostart.Set(false); err != nil {
			failed = append(failed, "CARE Desktop is still set to open at login ("+err.Error()+").")
		}
	}

	for _, s := range failed {
		e.logln("note: " + s)
	}
	return nil
}
