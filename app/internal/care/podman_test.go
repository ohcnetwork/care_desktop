package care

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// `version` (reachable, for containerBin()'s probe), echoes machineListJSON for
// `machine list --format json`, and echoes rootful (e.g. "true"/"false"; empty
// means answer nothing, simulating "couldn't determine") for
// `machine inspect --format {{.Rootful}}` - letting PodmanMachineCheck be
// exercised end-to-end without a real podman install.
func fakePodmanScript(t *testing.T, machineListJSON, rootful string) string {
	t.Helper()
	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
case "$*" in
  *"machine list"*) echo '%s' ;;
  *"machine inspect"*) echo '%s' ;;
  *) exit 0 ;;
esac
`, machineListJSON, rootful)
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
// instruction, not fixable (starting isn't a resize), and Blocking (nothing
// works until it's started).
func TestPodmanMachineCheckStopped(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":false,"Memory":"2147483648"}]`, "false")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || got.OK || got.Fixable || !got.Blocking {
		t.Fatalf("want Applicable=true OK=false Fixable=false Blocking=true, got %+v", got)
	}
	if got.How == "" {
		t.Fatal("want a How instruction for a stopped machine")
	}
}

// PodmanMachineCheck must flag a running rootless machine as not-OK, Fixable
// (FixPodmanMachineRootful can switch it), and Blocking - rootless podman
// can't bind CARE's privileged ports 80/443 (a certainty, not a risk), so
// this must be caught before setup ever reaches `compose up`.
func TestPodmanMachineCheckRootless(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"4294967296"}]`, "false")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || got.OK || !got.Fixable || !got.Blocking {
		t.Fatalf("want Applicable=true OK=false Fixable=true Blocking=true, got %+v", got)
	}
	if !strings.Contains(got.Message, "rootless") {
		t.Fatalf("want the message to mention rootless, got %+v", got)
	}
}

// PodmanMachineCheck must flag a running-but-underpowered machine as not-OK,
// Fixable (FixPodmanMachineMemory can resize it), but NOT Blocking - the
// stack may still come up fine, only builds are at risk of crashing.
// Checked only once rootful, since a rootless machine fails outright
// regardless of memory.
func TestPodmanMachineCheckLowMemory(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"2147483648"}]`, "true")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || got.OK || !got.Fixable || got.Blocking {
		t.Fatalf("want Applicable=true OK=false Fixable=true Blocking=false, got %+v", got)
	}
}

// PodmanMachineCheck must report OK for a running, rootful machine with enough
// memory.
func TestPodmanMachineCheckHealthy(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"4294967296"}]`, "true")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || !got.OK {
		t.Fatalf("want Applicable=true OK=true, got %+v", got)
	}
}

// PodmanMachineCheck must not flag rootless-ness it couldn't actually confirm
// (e.g. an older podman without machine inspect --format support) - unknown
// is treated as "don't block", falling through to the memory check.
func TestPodmanMachineCheckRootfulUnknown(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"4294967296"}]`, "")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if !got.Applicable || !got.OK {
		t.Fatalf("want an unknown rootful status to fall through to OK, got %+v", got)
	}
}

// PodmanMachineCheck must report not-applicable when podman has no machine at
// all (e.g. native Linux podman, or `podman machine init` never run) - an
// empty machine list, not an error.
func TestPodmanMachineCheckNoMachine(t *testing.T) {
	dir := fakePodmanScript(t, `[]`, "true")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).PodmanMachineCheck()
	if got.Applicable {
		t.Fatalf("want Applicable=false when there's no machine to check, got %+v", got)
	}
}

// DockerCheck must actually block (OK: false) on a rootless podman machine,
// not just warn - unlike low memory, a rootless machine is a *certain* future
// failure (compose can never bind :80/:443 as-is), so the wizard shouldn't
// show green and let the user hit that error later, mid-setup.
func TestDockerCheckBlocksOnRootlessPodman(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"4294967296"}]`, "false")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).DockerCheck()
	if got.OK {
		t.Fatalf("want OK=false for a rootless podman machine, got %+v", got)
	}
	if !strings.Contains(got.Message, "rootless") {
		t.Fatalf("want the message to mention rootless, got %+v", got)
	}
}

// DockerCheck must still report OK (with an inline warning) for a running,
// rootful-but-underpowered machine - low memory is a risk, not a certainty.
func TestDockerCheckWarnsOnLowMemoryPodman(t *testing.T) {
	dir := fakePodmanScript(t, `[{"Name":"podman-machine-default","Running":true,"Memory":"2147483648"}]`, "true")
	withPodmanOnlyPath(t, dir)

	got := (&Engine{}).DockerCheck()
	if !got.OK {
		t.Fatalf("want OK=true (just a warning) for low memory, got %+v", got)
	}
	if !strings.Contains(got.Message, "warning") || !strings.Contains(got.Message, "memory") {
		t.Fatalf("want an inline memory warning, got %+v", got)
	}
}
