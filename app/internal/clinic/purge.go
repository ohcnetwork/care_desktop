package clinic

import (
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/autostart"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/hostname"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/hosts"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/netfix"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/trust"
)

// Project is the compose project name every container, volume, and network of an
// install is labelled with.
func (e *Clinic) Project() string { return composeProject }

// CloneDirs are the source checkouts an install downloads.
func (e *Clinic) CloneDirs() []string { return []string{e.beDir(), e.feDir()} }

// Images is every image tag an install builds or pulls.
func (e *Clinic) Images() []string { return e.uninstallImages() }

// Purge removes every trace of an earlier CARE Desktop from this computer, so a
// fresh install starts from nothing. It differs from Uninstall in two ways, both
// deliberate:
//
//  1. It force-removes the compose project by label without first checking that
//     an install dir with a compose file exists. Uninstall guards that check
//     because it is torn down on behalf of one install dir, and the label is
//     machine-wide - so an unguarded teardown there would destroy a *different*
//     install's data. Purge is the opposite case: it is invoked from the
//     first-run wizard, before this app owns an install, and its whole purpose is
//     to remove whatever carries the label. There is nothing else it could hit.
//
//  2. It never touches the backup folder. Backups are the recovery data. They do
//     not interfere with a fresh install, and deleting them to "clean up" would
//     destroy the only copy of a clinic's history.
//
// Best-effort throughout: one failed step is logged and the rest still runs, so
// a half-broken leftover install can still be cleared.
func (e *Clinic) Purge() error {
	// Capture both before teardown: the root CA lives in the caddy-data volume
	// that is about to be destroyed, and the machine's original name is recorded
	// inside the install dir this deletes.
	rootPEM := e.caddyRootPEM()
	prevHostname := hostname.Previous(e.InstallDir)

	if _, err := os.Stat(filepath.Join(e.InstallDir, "docker-compose.yml")); err == nil {
		e.logln("Removing containers, network, and data volumes...")
		if err := e.dc("down", "-v", "--remove-orphans"); err != nil {
			e.logln("  (compose down reported an error - continuing cleanup)")
		}
	}
	e.forceRemoveProject()

	e.logln("Removing Docker images...")
	for _, img := range e.uninstallImages() {
		e.removeImage(img)
	}
	e.pruneBuildCache()

	for _, dir := range e.CloneDirs() {
		if _, err := os.Stat(dir); err == nil {
			e.logln("Removing " + dir)
			_ = os.RemoveAll(dir)
		}
	}

	if looksLikeSourceRepo(e.InstallDir) {
		e.logln("Install dir looks like a source checkout - left in place: " + e.InstallDir)
	} else if _, err := os.Stat(e.InstallDir); err == nil {
		e.logln("Removing installed files " + e.InstallDir)
		_ = os.RemoveAll(e.InstallDir)
	}

	var failed []string
	if s := trust.Untrust(e.Log, e.Confirm, rootPEM); s != "" {
		failed = append(failed, s)
	}
	if s := hosts.Remove(e.Log, e.Confirm, e.host()); s != "" {
		failed = append(failed, s)
	}
	if s := hostname.Restore(e.Runner(), e.Log, e.InstallDir, e.mdnsName(), prevHostname); s != "" {
		failed = append(failed, s)
	}
	netfix.Undo(e.Log)

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
