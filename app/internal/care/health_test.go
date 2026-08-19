package care

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeEngineScript writes a `docker` (or another name) stand-in on a fresh PATH
// dir that answers `version` (reachable) and echoes psNames for a `ps ...`
// invocation - letting caddyRunning()/EnsurePortFree() be exercised without a
// real container engine.
func fakeEngineScript(t *testing.T, binName, psNames string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
  ps) echo '%s' ;;
esac
exit 0
`, psNames)
	if err := os.WriteFile(filepath.Join(dir, binName), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func withEnginePath(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir)
}

// caddyRunning must recognize our own already-running caddy via the engine's
// native `ps --filter label=...` (not `compose ps --services --filter ...`,
// which is docker-compose-v2-only syntax podman-compose's CLI rejects outright -
// the bug that produced a false "port 80 already in use by gvproxy" on an
// idempotent restart under podman).
func TestCaddyRunningTrue(t *testing.T) {
	dir := fakeEngineScript(t, "docker", "care-desktop_caddy_1")
	withEnginePath(t, dir)

	if !(&Engine{}).caddyRunning() {
		t.Fatal("want caddyRunning=true when the engine's ps lists our caddy container")
	}
}

// caddyRunning must report false when nothing matches (caddy isn't up).
func TestCaddyRunningFalse(t *testing.T) {
	dir := fakeEngineScript(t, "docker", "")
	withEnginePath(t, dir)

	if (&Engine{}).caddyRunning() {
		t.Fatal("want caddyRunning=false when ps returns nothing")
	}
}

// EnsurePortFree must not flag a conflict when our own caddy is already up -
// an idempotent restart of our own stack must never be treated as "someone
// else is using port 80".
func TestEnsurePortFreeOwnCaddyIsNotAConflict(t *testing.T) {
	dir := fakeEngineScript(t, "docker", "care-desktop_caddy_1")
	withEnginePath(t, dir)

	if err := (&Engine{}).EnsurePortFree(); err != nil {
		t.Fatalf("want no error when our own caddy is running, got: %v", err)
	}
}
