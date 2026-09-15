package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	for _, value := range []string{`{"state":"old"}`, `{"state":"new"}`} {
		if err := Write(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != value {
			t.Fatalf("replacement failed: %s, %v", data, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("incorrect permissions: %v, %v", info, err)
		}
	}
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Write(blocked, []byte("data"), 0o600); err == nil {
		t.Fatal("replacing a directory unexpectedly succeeded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary files were left behind: %v, %v", entries, err)
	}
}
