package residue

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func TestUnavailableDockerIsNotClean(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX command fixtures")
	}
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, name := range []string{"docker", "security"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", root)
	run := proc.Runner{Env: os.Environ()}
	report, err := Scan(Options{
		Runner: run, Project: "care-desktop", InstallDir: filepath.Join(root, "install"),
	})
	if err == nil || report.Clean {
		t.Fatalf("unavailable Docker was called clean: %+v, %v", report, err)
	}
	if _, err := InstallDirFrom(run, "care-desktop", filepath.Join(root, "missing")); err == nil {
		t.Fatal("failed installation discovery was ignored")
	}
}
