package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The read-only branch is what this check exists for, but it needs a read-only
// mount to exercise; see TestValidateBackupDirReadOnly, which is skipped unless
// one is present.
func TestValidateBackupDir(t *testing.T) {
	a := &App{}

	t.Run("empty means default", func(t *testing.T) {
		if got := a.ValidateBackupDir("  "); got != "" {
			t.Fatalf("want no problem for the default, got %q", got)
		}
	})

	t.Run("writable dir", func(t *testing.T) {
		if got := a.ValidateBackupDir(t.TempDir()); got != "" {
			t.Fatalf("want no problem, got %q", got)
		}
	})

	t.Run("leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		if got := a.ValidateBackupDir(dir); got != "" {
			t.Fatalf("want no problem, got %q", got)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("probe file was not cleaned up: %v", entries)
		}
	})

	t.Run("missing dir", func(t *testing.T) {
		got := a.ValidateBackupDir(filepath.Join(t.TempDir(), "gone"))
		if !strings.Contains(got, "isn't there any more") {
			t.Fatalf("want a missing-folder message, got %q", got)
		}
	})

	t.Run("file not dir", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "backups")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := a.ValidateBackupDir(f); !strings.Contains(got, "not a folder") {
			t.Fatalf("want a not-a-folder message, got %q", got)
		}
	})

	t.Run("unwritable dir", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores the permission bits")
		}
		dir := filepath.Join(t.TempDir(), "locked")
		if err := os.Mkdir(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		if got := a.ValidateBackupDir(dir); !strings.Contains(got, "isn't allowed to write") {
			t.Fatalf("want a permission message, got %q", got)
		}
	})
}

// CARE_TEST_RO_DIR points at a directory on a read-only mount, e.g. an attached
// read-only disk image. Without it there is nothing to assert against.
func TestValidateBackupDirReadOnly(t *testing.T) {
	dir := os.Getenv("CARE_TEST_RO_DIR")
	if dir == "" {
		t.Skip("set CARE_TEST_RO_DIR to a directory on a read-only mount")
	}
	got := (&App{}).ValidateBackupDir(dir)
	if !strings.Contains(got, "read-only") {
		t.Fatalf("want a read-only message, got %q", got)
	}
}
