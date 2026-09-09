package care

import (
	"strings"
	"testing"
)

// newReportEngine returns an engine whose log is captured, pointed at a backup
// dir that exists (so "kept" has something real to report).
func newReportEngine(t *testing.T, backupDir string) (*Engine, *strings.Builder) {
	t.Helper()
	var log strings.Builder
	e := &Engine{
		Env: map[string]string{"BACKUP_DIR": backupDir},
		Log: func(s string) { log.WriteString(s + "\n") },
	}
	return e, &log
}

func TestReportLeftoversNothingLeft(t *testing.T) {
	// A missing backup dir plus RemoveImages means there is genuinely nothing
	// left, which is the only case allowed to claim a full revert.
	e, log := newReportEngine(t, t.TempDir()+"/gone")
	e.reportLeftovers(UninstallOptions{RemoveImages: true, RemoveBackups: true}, nil)

	got := log.String()
	if !strings.Contains(got, "Every change CARE made to this computer has been reverted") {
		t.Fatalf("want the full-revert line, got:\n%s", got)
	}
	if strings.Contains(got, "could NOT be reverted") {
		t.Fatalf("reported failures when there were none:\n%s", got)
	}
}

func TestReportLeftoversKeptOnPurpose(t *testing.T) {
	dir := t.TempDir()
	e, log := newReportEngine(t, dir)
	e.reportLeftovers(UninstallOptions{}, nil)

	got := log.String()
	if strings.Contains(got, "Every change CARE made") {
		t.Fatalf("claimed a full revert while keeping things:\n%s", got)
	}
	if !strings.Contains(got, "Backups in "+dir) {
		t.Fatalf("kept backups not named:\n%s", got)
	}
	if !strings.Contains(got, "Docker images") {
		t.Fatalf("kept images not named:\n%s", got)
	}
}

func TestReportLeftoversFailuresAreNamed(t *testing.T) {
	e, log := newReportEngine(t, t.TempDir()+"/gone")
	e.reportLeftovers(
		UninstallOptions{RemoveImages: true, RemoveBackups: true},
		[]string{"the hosts line is still there", "the certificate is still trusted"},
	)

	got := log.String()
	if !strings.Contains(got, "could NOT be reverted") {
		t.Fatalf("want the failure heading, got:\n%s", got)
	}
	for _, want := range []string{"hosts line is still there", "certificate is still trusted"} {
		if !strings.Contains(got, want) {
			t.Fatalf("failure %q not reported:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Every change CARE made") {
		t.Fatalf("claimed a full revert despite failures:\n%s", got)
	}
}
