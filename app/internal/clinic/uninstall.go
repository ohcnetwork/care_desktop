package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/hostname"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/netfix"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

// UninstallOptions controls how much of a CARE install to tear down. Containers,
// the private network, and the data volumes are always removed; the rest is opt-in.
type UninstallOptions struct {
	RemoveImages     bool // also delete the built + base Docker images (re-downloaded next install)
	RemoveInstallDir bool // also delete the install dir (unpacked config + the care/care_fe clones)
	RemoveBackups    bool // also delete the backup folder - DESTROYS the recovery data
}

// Uninstall tears down the stack. Destructive: `compose down -v` removes the data
// volumes (all patient data). Best-effort throughout - a failure in one step is
// logged and the rest still runs, so even a half-finished install can be cleaned up.
func (e *Clinic) Uninstall(opts UninstallOptions) error {
	// 0. Grab the root CA before teardown - it lives in the caddy-data volume that
	//    `compose down -v` destroys, so capture it now to untrust it at the end.
	rootPEM := e.caddyRootPEM()
	// Same reason: the machine's original name is recorded in the install dir, which
	// step 4 may delete.
	prevHostname := hostname.Previous(e.InstallDir)

	// 1. containers + private network + data volumes. Needs the compose file, so
	//    do this first, while the install dir still exists.
	if _, err := os.Stat(filepath.Join(e.InstallDir, "docker-compose.yml")); err == nil {
		e.logln("Removing containers, network, and data volumes...")
		if err := e.dc("down", "-v", "--remove-orphans"); err != nil {
			e.logln("  (compose down reported an error - continuing cleanup)")
		}
		// Safety net: compose down can silently fail to reach the project (e.g. a
		// wrong working dir, or an interpolation error parsing the file - seen on
		// Windows), leaving containers running. Force-remove anything still tagged
		// with our compose project label, so uninstall always stops the stack.
		//
		// Inside the compose-file check on purpose: forceRemoveProject matches on
		// the project *label*, not on this install dir, so it would delete the data volumes
		// of an install somewhere else on the machine. Only an install dir that owns a
		// compose file is entitled to tear the project down.
		e.forceRemoveProject()
	}

	// 2. images (optional): everything we built, plus the base images we pulled.
	if opts.RemoveImages {
		e.logln("Removing Docker images...")
		for _, img := range e.uninstallImages() {
			e.removeImage(img) // best-effort; in-use base images are simply skipped
		}
		e.pruneBuildCache()
	}

	// 3. the git clones - always safe to delete, and the biggest downloads.
	for _, dir := range []string{e.beDir(), e.feDir()} {
		if _, err := os.Stat(dir); err == nil {
			e.logln("Removing " + dir)
			_ = os.RemoveAll(dir)
		}
	}

	// 4. the install dir (unpacked config). Guarded: never delete a source checkout -
	//    the CLI's install dir can be the repo root itself.
	if opts.RemoveInstallDir {
		if looksLikeSourceRepo(e.InstallDir) {
			e.logln("Install dir looks like a source checkout - left in place: " + e.InstallDir)
		} else if _, err := os.Stat(e.InstallDir); err == nil {
			e.logln("Removing installed files " + e.InstallDir)
			_ = os.RemoveAll(e.InstallDir)
		}
	}

	// 5. backups (optional) - the recovery data. Only when explicitly asked.
	if opts.RemoveBackups {
		if dir := e.backupDir(); dirExists(dir) {
			e.logln("Removing backups in " + dir)
			_ = os.RemoveAll(dir)
		}
	}

	// 6. the trusted root on THIS machine (best-effort; matched by fingerprint so
	//    it only ever removes the cert we installed).
	var failed []string
	if s := trust.Untrust(e.Log, e.Confirm, rootPEM); s != "" {
		failed = append(failed, s)
	}
	if s := hosts.Remove(e.Log, e.Confirm, e.host()); s != "" { // drop the care.local hosts line we added
		failed = append(failed, s)
	}
	if s := hostname.Restore(e.Runner(), e.Log, e.InstallDir, e.mdnsName(), prevHostname); s != "" { // undo a "rename"-mode install
		failed = append(failed, s)
	}
	netfix.Undo(e.Log) // Windows: revert profile to Public + drop the rules Fix added

	e.reportLeftovers(opts, failed)
	return nil
}
