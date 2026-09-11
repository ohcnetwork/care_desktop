package applog

import (
	"os"
	"strings"
	"sync"
	"testing"
)

// Redaction is a security property and it fails silently: a log that leaks a
// clinic's database password looks exactly like one that does not.
func TestRedact(t *testing.T) {
	secret := map[string]string{
		`MINIO_SECRET_KEY=hunter2seekrit`:                   "hunter2seekrit",
		`POSTGRES_PASSWORD: sup3rs3cret`:                    "sup3rs3cret",
		`"DJANGO_SECRET_KEY": "abc123def"`:                  "abc123def",
		`BACKUP_PASSPHRASE=correcthorse`:                    "correcthorse",
		`MINIO_ACCESS_KEY=AKIAEXAMPLE`:                      "AKIAEXAMPLE",
		`API_TOKEN=ghp_xxxxxxxxxxxx`:                        "ghp_xxxxxxxxxxxx",
		`cloning https://bob:tok3nvalue@github.com/x/y.git`: "tok3nvalue",
	}
	for line, leak := range secret {
		got := Redact(line)
		if strings.Contains(got, leak) {
			t.Errorf("secret survived redaction:\n  in:  %s\n  out: %s", line, got)
		}
		if !strings.Contains(got, mask) {
			t.Errorf("expected %s in output: %s", mask, got)
		}
	}

	// Over-eager is fine; mangling ordinary output is not.
	for _, line := range []string{
		`FILE_UPLOAD_BUCKET=patient-bucket`,
		`POSTGRES_IMAGE=postgres:17.10-alpine`,
		`Building the backend image (care:clinic)...`,
		`CARE is up -> https://care.local/   (login: admin)`,
	} {
		if got := Redact(line); got != line {
			t.Errorf("untouched line was altered:\n  in:  %s\n  out: %s", line, got)
		}
	}
}

// The ceiling is what stops a retrying build filling a clinic's disk.
func TestRotationHoldsCeiling(t *testing.T) {
	defer func(s int64, n int) { maxSize, maxFiles = s, n }(maxSize, maxFiles)
	maxSize, maxFiles = 16<<10, 3

	dir := t.TempDir()
	l := Open(dir)
	line := strings.Repeat("x", 1000)
	for i := 0; i < 500; i++ { // ~500KB against a 48KB ceiling
		l.Write(line)
	}
	l.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, e := range entries {
		fi, _ := e.Info()
		total += fi.Size()
	}
	if len(entries) > maxFiles {
		t.Errorf("kept %d files, want at most %d", len(entries), maxFiles)
	}
	if ceiling := maxSize * int64(maxFiles); total > ceiling {
		t.Errorf("total %d bytes exceeds ceiling %d", total, ceiling)
	}
}

// Rule 3: safe from anywhere. proc.Runner alone streams stdout and stderr into
// this from two goroutines.
func TestConcurrentWrites(t *testing.T) {
	l := Open(t.TempDir())
	defer l.Close()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				l.Write("concurrent line")
			}
		}()
	}
	wg.Wait()
}

// Rule 1: losing logs must never stop the app. A nil logger and an unopenable
// directory both have to be ordinary, silent no-ops.
func TestNeverFailsTheCaller(t *testing.T) {
	var nilLogger *Logger
	nilLogger.Write("must not panic")
	nilLogger.Header("v", "i", "c")
	nilLogger.Close()
	if nilLogger.Path() != "" || nilLogger.Folder() != "" {
		t.Error("nil logger should report no path")
	}

	disabled := Open("\x00 not a directory")
	disabled.Write("discarded, not fatal")
	if disabled.Path() != "" {
		t.Errorf("expected a disabled logger, got path %q", disabled.Path())
	}
	disabled.Close()
}
