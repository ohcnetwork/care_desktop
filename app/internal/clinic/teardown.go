package clinic

import (
	"os"
	"path/filepath"
)

func (e *Clinic) TeardownProject() {
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
