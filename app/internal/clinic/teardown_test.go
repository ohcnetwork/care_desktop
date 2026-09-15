package clinic

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/release"
)

func TestTeardownInspectsBeforeDeleting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	root := t.TempDir()
	trace := filepath.Join(root, "calls")
	t.Setenv("CARE_COMMAND_TRACE", trace)
	t.Setenv("PATH", root)
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CARE_COMMAND_TRACE"
case "$1 $2" in
  'ps -aq') printf 'container-id\n' ;;
  'volume ls') exit 1 ;;
  'network ls') printf 'network-id\n' ;;
  *) exit 99 ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "docker"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	e := &Clinic{InstallDir: root, BackupDir: filepath.Join(root, "backups"), Pins: &release.Pins{}}
	if err := e.TeardownProject(); err == nil {
		t.Fatal("teardown ignored a failed resource inspection")
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "rm ") {
		t.Fatalf("teardown deleted resources after an incomplete inspection: %s", data)
	}
}

func TestInstallRemovalRejectsUnownedDirectories(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "personal.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Clinic{InstallDir: root}).removeInstallFiles(); err == nil {
		t.Fatal("an unowned directory was accepted for removal")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
		t.Fatal("unrelated files were changed")
	}
}
