package proc

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLinesDistinguishesFailureFromEmptyOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"failed": "#!/bin/sh\nexit 1\n",
		"empty":  "#!/bin/sh\nexit 0\n",
		"lines":  "#!/bin/sh\nprintf ' first \\n\\nsecond\\n'\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	r := Runner{}
	if _, err := r.Lines(filepath.Join(dir, "failed")); err == nil {
		t.Fatal("a failed command became an empty successful result")
	}
	if lines, err := r.Lines(filepath.Join(dir, "empty")); err != nil || len(lines) != 0 {
		t.Fatalf("empty output: %v, %v", lines, err)
	}
	if lines, err := r.Lines(filepath.Join(dir, "lines")); err != nil || len(lines) != 2 || lines[0] != "first" || lines[1] != "second" {
		t.Fatalf("line parsing: %v, %v", lines, err)
	}
}

func TestRunWithPreservesInheritedEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	t.Setenv("CARE_INHERITED", "kept")
	if err := (Runner{}).RunWith([]string{"CARE_EXTRA=added"}, "/bin/sh", "-c",
		`test "$CARE_INHERITED" = kept && test "$CARE_EXTRA" = added`); err != nil {
		t.Fatal(err)
	}
}
