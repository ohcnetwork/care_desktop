package clinic

import (
	"os"
	"path/filepath"
)

// TeardownProject force-removes this app's containers, volumes, and network by label
// (no compose file/install dir needed). Used by the retry cleanup to release a leftover
// container's bind mounts before wiping the install dir.
func (e *Clinic) TeardownProject() { e.forceRemoveProject() }

// forceRemoveProject removes any containers, then volumes, then the network still
// carrying our compose project label - the backstop for when `compose down` didn't
// reach them. Talks to docker directly (no compose file / no install dir needed), so it
// works even after a broken or partial install. Best-effort throughout.
func (e *Clinic) forceRemoveProject() {
	label := "label=com.docker.compose.project=" + composeProject

	if ids := e.captureLines("docker", "ps", "-aq", "--filter", label); len(ids) > 0 {
		e.logln("Force-removing leftover containers...")
		_ = e.run(nil, "docker", append([]string{"rm", "-f"}, ids...)...)
	}
	if vols := e.captureLines("docker", "volume", "ls", "-q", "--filter", label); len(vols) > 0 {
		_ = e.run(nil, "docker", append([]string{"volume", "rm", "-f"}, vols...)...)
	}
	if nets := e.captureLines("docker", "network", "ls", "-q", "--filter", label); len(nets) > 0 {
		_ = e.run(nil, "docker", append([]string{"network", "rm"}, nets...)...)
	}
}

// looksLikeSourceRepo guards against deleting the developer's git checkout when the
// install dir points at the repo root (e.g. `care uninstall` from the source tree). A
// managed install dir - unpacked config - has none of these.
func looksLikeSourceRepo(dir string) bool {
	for _, marker := range []string{".git", "app", "docs"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	// Also walk up: running from a subdir (e.g. app/) means the .git marker sits
	// in a parent, not dir itself - a managed install dir never has a .git ancestor.
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
