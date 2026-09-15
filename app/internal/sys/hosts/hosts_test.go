package hosts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestHasEntryRequiresLocalAddress(t *testing.T) {
	for _, tc := range []struct {
		data string
		want bool
	}{
		{"127.0.0.1 care.local # care-desktop\n", true},
		{"::1 localhost CARE.LOCAL\n", true},
		{"127.0.0.1 localhost care.local # other\n", true},
		{"192.0.2.1 care.local\n", false},
		{"127.0.0.1 care.local\n192.0.2.1 care.local\n", false},
		{"127.0.0.1 other.local # care.local\n", false},
		{"# 127.0.0.1 care.local\n", false},
		{"care.local\n", false},
	} {
		if got := hasEntry(tc.data, "care.local"); got != tc.want {
			t.Errorf("hasEntry(%q) = %v, want %v", tc.data, got, tc.want)
		}
	}
}

func TestRemoveScriptPreservesUnownedLinesAndHandlesEmptyResult(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{"mixed", "127.0.0.1 localhost\n" + line("care.local") + "\n", "127.0.0.1 localhost\n"},
		{"owned only", line("care.local") + "\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "hosts file")
			if err := os.WriteFile(p, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := proc.Command("sh", "-c", removeSh(p))
			cmd.Env = append(os.Environ(), "TMPDIR="+dir)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("remove: %v: %s", err, out)
			}
			data, err := os.ReadFile(p)
			if err != nil || string(data) != tc.want {
				t.Fatalf("remaining hosts = %q, %v; want %q", data, err, tc.want)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("scratch file was not removed: %v, %v", entries, err)
			}
		})
	}
}

func TestRemoveScriptPreservesWriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell fixture")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "hosts")
	if err := os.WriteFile(p, []byte("127.0.0.1 localhost\n"+line("care.local")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cat"), []byte("#!/bin/sh\nexit 47\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd := proc.Command("sh", "-c", removeSh(p))
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 47 {
		t.Fatalf("write failure was lost: %v, %s", err, out)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("scratch file was not removed: %v, %v", entries, err)
	}
}

func TestRemoveUnixUsesRemainingState(t *testing.T) {
	for _, tc := range []struct {
		name             string
		removedInitially bool
		approve          bool
		removeElevated   bool
		wantCalls        []bool
		wantLeftover     bool
	}{
		{"successful command left entry", false, true, true, []bool{false, true}, false},
		{"failed command removed entry", true, true, false, []bool{false}, false},
		{"elevation left entry", false, true, false, []bool{false, true}, true},
		{"approval declined", false, false, false, []bool{false}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "hosts")
			if err := os.WriteFile(p, []byte(line("care.local")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			var calls []bool
			got := removeUnix(func(string, string) bool { return tc.approve }, "care.local", p,
				func(_ string, elevated bool) error {
					calls = append(calls, elevated)
					if !elevated && tc.removedInitially || elevated && tc.removeElevated {
						if err := os.WriteFile(p, nil, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					if tc.removedInitially {
						return errors.New("command reported a failure")
					}
					return nil
				})
			if !slices.Equal(calls, tc.wantCalls) || (got != "") != tc.wantLeftover {
				t.Fatalf("calls = %v, leftover = %q", calls, got)
			}
		})
	}
}

func TestLeftoverDoesNotTreatReadFailureAsRemoval(t *testing.T) {
	if got := leftover("care.local", t.TempDir()); !strings.Contains(got, "Could not check") {
		t.Fatalf("read failure became successful removal: %q", got)
	}
}

func TestInspectDistinguishesAbsentFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	present, err := inspect(filepath.Join(dir, "missing"))
	if err != nil || present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
	p := filepath.Join(dir, "hosts")
	if err := os.WriteFile(p, []byte("127.0.0.1 localhost\n"+line("care.local")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	present, err = inspect(p)
	if err != nil || !present {
		t.Fatalf("present = %v, err = %v", present, err)
	}
	present, err = inspect(dir)
	if err == nil || present {
		t.Fatalf("directory read became a definite answer: present = %v, err = %v", present, err)
	}
}
