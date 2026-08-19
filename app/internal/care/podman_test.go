package care

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// memoryMiB must convert the bytes-as-string field podman reports, and treat
// anything unparseable or non-positive as "unknown" (0).
func TestPodmanMachineMemoryMiB(t *testing.T) {
	cases := []struct {
		bytes string
		want  int
	}{
		{"2147483648", 2048},
		{"4294967296", 4096},
		{"", 0},
		{"not-a-number", 0},
		{"-1", 0},
		{"0", 0},
	}
	for _, c := range cases {
		if got := (podmanMachine{Memory: c.bytes}).memoryMiB(); got != c.want {
			t.Errorf("memoryMiB(%q) = %d, want %d", c.bytes, got, c.want)
		}
	}
}

// defaultPodmanMachine must prefer the running machine over the first one, and
// fall back to the first when none are running.
func TestDefaultPodmanMachine(t *testing.T) {
	stopped := podmanMachine{Name: "stopped-one", Running: false}
	running := podmanMachine{Name: "running-one", Running: true}

	if got := defaultPodmanMachine([]podmanMachine{stopped, running}); got.Name != "running-one" {
		t.Fatalf("want the running machine preferred, got %q", got.Name)
	}
	if got := defaultPodmanMachine([]podmanMachine{stopped}); got.Name != "stopped-one" {
		t.Fatalf("want the only (stopped) machine as fallback, got %q", got.Name)
	}
}

func TestMachineArg(t *testing.T) {
	if got := machineArg("podman-machine-default"); got != " podman-machine-default" {
		t.Fatalf("got %q", got)
	}
	if got := machineArg(""); got != "" {
		t.Fatalf("want empty for an unnamed machine, got %q", got)
	}
}

// fakePodmanScript writes a `podman` stand-in on a fresh PATH dir that answers
// `version` (reachable, for containerBin()'s probe) and echoes machineListJSON
// for `machine list --format json` - letting PodmanMachineCheck be exercised
// end-to-end without a real podman install.
func fakePodmanScript(t *testing.T, machineListJSON string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$*" in
  *"machine list"*) echo '%s' ;;
  *) exit 0 ;;
esac
`, machineListJSON)
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func withPodmanOnlyPath(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", orig) })
	os.Setenv("PATH", dir)
}

// PodmanMachineCheck must flag a stopped machine as not-OK with a start
// instruction, not fixable (starting isn't a resize).
func TestPodmanMachineCheckStopped(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":false,"Memory":"2147483648"}]`)
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || got.OK || got.Fixable {
		t.Fatalf("want Applicable=true OK=false Fixable=false, got %+v", got)
	}
	if got.How == "" {
		t.Fatal("want a How instruction for a stopped machine")
	}
}

// PodmanMachineCheck must flag a running-but-underpowered machine as not-OK
// and Fixable (FixPodmanMachineMemory can resize it).
func TestPodmanMachineCheckLowMemory(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"2147483648"}]`)
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || got.OK || !got.Fixable {
		t.Fatalf("want Applicable=true OK=false Fixable=true, got %+v", got)
	}
}

// PodmanMachineCheck must report OK for a running machine with enough memory.
func TestPodmanMachineCheckHealthy(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"4294967296"}]`)
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || !got.OK {
		t.Fatalf("want Applicable=true OK=true, got %+v", got)
	}
}

// PodmanMachineCheck must report not-applicable when podman has no machine at
// all (e.g. native Linux podman, or `podman machine init` never run) - an
// empty machine list, not an error.
func TestPodmanMachineCheckNoMachine(t *testing.T) {
	dir := fakePodmanScript(t, `[]`)
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if got.Applicable {
		t.Fatalf("want Applicable=false when there's no machine to check, got %+v", got)
	}
}
