package care

import (
	"os"
	"os/exec"
	"path/filepath"
)

// UninstallOptions controls how much of a CARE install to tear down. Containers,
// the private network, and the data volumes are always removed; the rest is opt-in.
type UninstallOptions struct {
	RemoveImages  bool // also delete the built + base Docker images (re-downloaded next install)
	RemoveKit     bool // also delete the kit dir (unpacked config + the care/care_fe clones)
	RemoveBackups bool // also delete the backup folder - DESTROYS the recovery data
}

// Uninstall tears down the stack. Destructive: `compose down -v` removes the data
// volumes (all patient data). Best-effort throughout - a failure in one step is
// logged and the rest still runs, so even a half-finished install can be cleaned up.
func (e *Engine) Uninstall(opts UninstallOptions) error {
	// 0. Grab the root CA before teardown - it lives in the caddy-data volume that
	//    `compose down -v` destroys, so capture it now to untrust it at the end.
	rootPEM := e.caddyRootPEM()

	// 1. containers + private network + data volumes. Needs the compose file, so
	//    do this first, while the kit still exists.
	if _, err := os.Stat(filepath.Join(e.Kit, "docker-compose.yml")); err == nil {
		e.logln("Removing containers, network, and data volumes...")
		if err := e.dc("down", "-v", "--remove-orphans"); err != nil {
			e.logln("  (compose down reported an error - continuing cleanup)")
		}
	}

	// 2. images (optional): the two we built, plus the base images we pulled.
	if opts.RemoveImages {
		e.logln("Removing Docker images...")
		for _, img := range []string{
			e.backendImage(), e.frontendImage(),
			"postgres:17-alpine", "redis:8-alpine", "minio/minio:latest", "caddy:2",
		} {
			e.removeImage(img) // best-effort; in-use base images are simply skipped
		}
	}

	// 3. the git clones - always safe to delete, and the biggest downloads.
	for _, dir := range []string{e.beDir(), e.feDir()} {
		if _, err := os.Stat(dir); err == nil {
			e.logln("Removing " + dir)
			_ = os.RemoveAll(dir)
		}
	}

	// 4. the kit dir (unpacked config). Guarded: never delete a source checkout -
	//    the CLI's kit can be the repo root itself.
	if opts.RemoveKit {
		if looksLikeSourceRepo(e.Kit) {
			e.logln("Kit dir looks like a source checkout - left in place: " + e.Kit)
		} else if _, err := os.Stat(e.Kit); err == nil {
			e.logln("Removing installed files " + e.Kit)
			_ = os.RemoveAll(e.Kit)
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
	e.untrustLocalCA(rootPEM)

	e.logln("Uninstall complete.")
	return nil
}

// removeImage deletes one image quietly, ignoring "not found" / "still in use".
func (e *Engine) removeImage(tag string) {
	cmd := exec.Command("docker", "image", "rm", tag)
	cmd.Env = e.baseEnv()
	cmd.Dir = e.workdir()
	if cmd.Run() != nil {
		e.logln("  skipped " + tag + " (not present or still in use)")
		return
	}
	e.logln("  removed " + tag)
}

// looksLikeSourceRepo guards against deleting the developer's git checkout when the
// kit dir points at the repo root (e.g. `care uninstall` from the source tree). A
// managed kit - unpacked config plus the care/care_fe clones - has none of these.
func looksLikeSourceRepo(dir string) bool {
	for _, marker := range []string{".git", "app", "docs"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
