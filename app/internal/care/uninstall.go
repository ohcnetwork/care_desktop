package care

import (
	"os"
	"path/filepath"
	"strings"
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
	// Safety net: compose down can silently fail to reach the project (e.g. a wrong
	// working dir, or an interpolation error parsing the file - seen on Windows),
	// leaving containers running. Force-remove anything still tagged with our
	// compose project label, so uninstall always stops the stack.
	e.forceRemoveProject()

	// 2. images (optional): everything we built, plus the base images we pulled.
	if opts.RemoveImages {
		e.logln("Removing Docker images...")
		for _, img := range e.uninstallImages() {
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
	e.removeHostsEntry()   // drop the care.local hosts line we added
	e.undoNetworkChanges() // Windows: revert profile to Public + drop the rules the Fix added

	e.logln("Uninstall complete.")
	return nil
}

// Tags come from the accessors that built them; hardcoding them here is what made
// `--images` match nothing once versions.env pinned real versions.
func (e *Engine) uninstallImages() []string {
	return []string{
		e.backendImage(), e.frontendImage(), e.wafCaddyImage(), e.backupImage(),
		e.postgresImage(), e.redisImage(), e.minioImage(),
		e.caddyImage(), e.caddyImage() + "-builder", // xcaddy build stage
	}
}

// removeImage deletes one image quietly, ignoring "not found" / "still in use".
func (e *Engine) removeImage(tag string) {
	cmd := newCmd(e.containerBin(), "image", "rm", tag)
	cmd.Env = e.baseEnv()
	cmd.Dir = e.workdir()
	if cmd.Run() != nil {
		e.logln("  skipped " + tag + " (not present or still in use)")
		return
	}
	e.logln("  removed " + tag)
}

// TeardownProject force-removes this app's containers, volumes, and network by label
// (no compose file/kit needed). Used by the retry cleanup to release a leftover
// container's bind mounts before wiping the kit.
func (e *Engine) TeardownProject() { e.forceRemoveProject() }

// forceRemoveProject removes any containers, then volumes, then the network still
// carrying our compose project label - the backstop for when `compose down` didn't
// reach them. Talks to the container engine directly (no compose file / no kit dir
// needed), so it works even after a broken or partial install. Best-effort throughout.
// The label itself is docker-compose's; podman compose sets the same label for
// docker-compatibility, so this matches under Podman too.
func (e *Engine) forceRemoveProject() {
	label := "label=com.docker.compose.project=" + composeProject

	if ids := e.captureLines(e.containerBin(), "ps", "-aq", "--filter", label); len(ids) > 0 {
		e.logln("Force-removing leftover containers...")
		_ = e.run(nil, e.containerBin(), append([]string{"rm", "-f"}, ids...)...)
	}
	if vols := e.captureLines(e.containerBin(), "volume", "ls", "-q", "--filter", label); len(vols) > 0 {
		_ = e.run(nil, e.containerBin(), append([]string{"volume", "rm", "-f"}, vols...)...)
	}
	if nets := e.captureLines(e.containerBin(), "network", "ls", "-q", "--filter", label); len(nets) > 0 {
		_ = e.run(nil, e.containerBin(), append([]string{"network", "rm"}, nets...)...)
	}
}

// captureLines runs a command and returns its non-empty output lines (trimmed).
func (e *Engine) captureLines(name string, args ...string) []string {
	out, err := e.capture(name, args...)
	if err != nil || out == "" {
		return nil
	}
	var lines []string
	for _, ln := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
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
	// Also walk up: running from a subdir (e.g. app/) means the .git marker sits
	// in a parent, not dir itself - a managed kit never has a .git ancestor.
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
