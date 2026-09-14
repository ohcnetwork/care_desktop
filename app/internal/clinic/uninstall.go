package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/netfix"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

type UninstallOptions struct {
	RemoveImages     bool
	RemoveInstallDir bool
	RemoveBackups    bool
}

func (e *Clinic) Uninstall(opts UninstallOptions) error {
	rootPEM := e.caddyRootPEM()

	if _, err := os.Stat(filepath.Join(e.InstallDir, "docker-compose.yml")); err == nil {
		e.logln("Removing containers, network, and data volumes...")
		if err := e.dc("down", "-v", "--remove-orphans"); err != nil {
			e.logln("  (compose down reported an error - continuing cleanup)")
		}
		e.TeardownProject()
	}

	if opts.RemoveImages {
		e.logln("Removing Docker images...")
		for _, img := range e.uninstallImages() {
			e.removeImage(img)
		}
		e.pruneBuildCache()
	}

	if opts.RemoveInstallDir {
		e.removeInstallFiles()
	}

	if opts.RemoveBackups {
		if dir := e.backupDir(); dirExists(dir) {
			e.logln("Removing backups in " + dir)
			_ = os.RemoveAll(dir)
		}
	}

	e.reportLeftovers(opts, e.revertSystemChanges(rootPEM))
	return nil
}

func (e *Clinic) removeInstallFiles() {
	if looksLikeSourceRepo(e.InstallDir) {
		e.logln("Install dir looks like a source checkout - left in place: " + e.InstallDir)
		return
	}
	if _, err := os.Stat(e.InstallDir); err == nil {
		e.logln("Removing installed files " + e.InstallDir)
		_ = os.RemoveAll(e.InstallDir)
	}
}

func (e *Clinic) revertSystemChanges(rootPEM string) []string {
	var failed []string
	if s := trust.Untrust(e.Log, e.Confirm, rootPEM); s != "" {
		failed = append(failed, s)
	}
	if s := hosts.Remove(e.Log, e.Confirm, e.host()); s != "" {
		failed = append(failed, s)
	}
	netfix.Undo(e.Log)
	return failed
}
