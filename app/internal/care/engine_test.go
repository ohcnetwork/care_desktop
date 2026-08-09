package care

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// genSecret must replace the placeholder with a real, long key - exactly once.
func TestGenSecret(t *testing.T) {
	dir := t.TempDir()
	be := filepath.Join(dir, "backend.env")
	if err := os.WriteFile(be, []byte("A=1\nDJANGO_SECRET_KEY=CHANGE_ME\nB=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Kit: dir}
	if err := e.genSecret(); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(be)
	got := string(out)
	if strings.Contains(got, "CHANGE_ME") {
		t.Fatal("placeholder not replaced")
	}
	var key string
	for _, l := range strings.Split(got, "\n") {
		if v, ok := strings.CutPrefix(l, "DJANGO_SECRET_KEY="); ok {
			key = v
		}
	}
	if len(key) < 40 {
		t.Fatalf("key too short: %q", key)
	}
	// idempotent: a second run (no placeholder left) must not change the file.
	before, _ := os.ReadFile(be)
	if err := e.genSecret(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(be)
	if string(before) != string(after) {
		t.Fatal("genSecret not idempotent")
	}
}

// get must resolve Env override > versions.env > default.
func TestGetResolution(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "versions.env"), []byte("# c\nBACKEND_IMAGE=pinned:v1\n"), 0o644)
	e := &Engine{Kit: dir}
	if got := e.backendImage(); got != "pinned:v1" {
		t.Fatalf("versions.env not honored: %q", got)
	}
	if got := e.frontendImage(); got != "care_fe:clinic" {
		t.Fatalf("default not used: %q", got)
	}
	e.Env = map[string]string{"BACKEND_IMAGE": "override:v2"}
	if got := e.backendImage(); got != "override:v2" {
		t.Fatalf("Env override not honored: %q", got)
	}
}

// ListBackups must pair a files archive to its same-timestamp dump, flag manual
// dumps as DB-only, ignore junk, and return newest first.
func TestListBackups(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"care-20260101-010000.dump",        // daily, has files
		"files-20260101-010000.tar.gz",     // its pair
		"care-manual-20260215-120000.dump", // manual, DB only
		"files-20260630-030000.tar.gz",     // orphan archive (no dump) - ignored
		"care-20260630-030000.dump",        // daily, has files (newest)
		"notes.txt",                        // junk - ignored
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := &Engine{Env: map[string]string{"BACKUP_DIR": dir}}
	got, err := e.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 backups, got %d: %+v", len(got), got)
	}
	// newest first
	if got[0].DBDump != "care-20260630-030000.dump" {
		t.Fatalf("not sorted newest-first: %q", got[0].DBDump)
	}
	if got[0].FilesArchive != "files-20260630-030000.tar.gz" {
		t.Fatalf("files archive not paired: %q", got[0].FilesArchive)
	}
	// the manual dump is DB-only
	var manual Backup
	for _, b := range got {
		if b.Manual {
			manual = b
		}
	}
	if manual.DBDump != "care-manual-20260215-120000.dump" || manual.FilesArchive != "" {
		t.Fatalf("manual backup mishandled: %+v", manual)
	}
}

// ListBackups must handle encrypted (.enc) and plaintext backups side by side -
// pairing each dump to its same-timestamp files archive regardless of suffix, and
// flagging the encrypted ones. This is the back-compat guarantee: a folder that
// mixes old plaintext dumps with new encrypted ones still lists cleanly.
func TestListBackupsEncrypted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"care-20260101-010000.dump",            // legacy plaintext, has files
		"files-20260101-010000.tar.gz",         // its plaintext pair
		"care-20260701-020000.dump.enc",        // encrypted, has files (newest)
		"files-20260701-020000.tar.gz.enc",     // its encrypted pair
		"care-manual-20260215-120000.dump.enc", // encrypted manual, DB only
		"backup-key.pem.enc",                   // the recovery key copy - must be ignored
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := &Engine{Env: map[string]string{"BACKUP_DIR": dir}}
	got, err := e.ListBackups()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 backups, got %d: %+v", len(got), got)
	}
	newest := got[0]
	if newest.DBDump != "care-20260701-020000.dump.enc" || !newest.Encrypted {
		t.Fatalf("newest encrypted dump mishandled: %+v", newest)
	}
	if newest.FilesArchive != "files-20260701-020000.tar.gz.enc" {
		t.Fatalf("encrypted files archive not paired: %q", newest.FilesArchive)
	}
	// the legacy plaintext dump is still listed, paired, and not flagged encrypted.
	var legacy Backup
	for _, b := range got {
		if b.DBDump == "care-20260101-010000.dump" {
			legacy = b
		}
	}
	if legacy.FilesArchive != "files-20260101-010000.tar.gz" || legacy.Encrypted {
		t.Fatalf("legacy plaintext backup mishandled: %+v", legacy)
	}
}

// Restore must reject anything that isn't one of our own backup filenames, before
// it can reach a shell.
func TestRestoreRejectsBadNames(t *testing.T) {
	e := &Engine{Kit: t.TempDir()}
	for _, bad := range []string{"care.dump; rm -rf /", "files-20260101-010000.tar.gz", "evil"} {
		if err := e.Restore(bad, "", ""); err == nil {
			t.Fatalf("expected rejection for %q", bad)
		}
	}
}

// looksLikeSourceRepo must flag a dev checkout (so uninstall never nukes it) but
// not a managed kit.
func TestLooksLikeSourceRepo(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !looksLikeSourceRepo(repo) {
		t.Fatal("source repo not detected")
	}
	if looksLikeSourceRepo(t.TempDir()) {
		t.Fatal("empty (managed-kit-like) dir flagged as source repo")
	}
	// Running from a subdir (e.g. app/) - the .git marker is in a parent.
	sub := filepath.Join(repo, "app")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if !looksLikeSourceRepo(sub) {
		t.Fatal("source repo not detected from a subdir")
	}
}

// Uninstall with RemoveKit must delete a managed kit but refuse to delete a dir
// that looks like a source checkout - even when asked to. No compose file present,
// so this never touches Docker.
func TestUninstallKitRemovalGuard(t *testing.T) {
	// managed kit -> removed.
	kit := filepath.Join(t.TempDir(), "kit")
	if err := os.MkdirAll(filepath.Join(kit, "care"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(kit, "backend.env"), []byte("x"), 0o644)
	e := &Engine{Kit: kit, Env: map[string]string{"BACKUP_DIR": t.TempDir()}}
	if err := e.Uninstall(UninstallOptions{RemoveKit: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kit); !os.IsNotExist(err) {
		t.Fatalf("managed kit not removed: %v", err)
	}

	// source checkout -> left in place.
	repo := t.TempDir()
	os.Mkdir(filepath.Join(repo, ".git"), 0o755)
	e2 := &Engine{Kit: repo, Env: map[string]string{"BACKUP_DIR": t.TempDir()}}
	if err := e2.Uninstall(UninstallOptions{RemoveKit: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatalf("source checkout was removed: %v", err)
	}
}

// Real docker plumbing: drive compose for two light services, confirm the engine
// brings them up under the isolated project, then clean up. Skips without docker.
func TestComposeWiring(t *testing.T) {
	if exec.Command("docker", "version").Run() != nil {
		t.Skip("docker not available")
	}
	kit, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(kit, "docker-compose.yml")); err != nil {
		t.Skipf("kit not found at %s", kit)
	}
	var logs strings.Builder
	e := &Engine{Kit: kit, Log: func(s string) { logs.WriteString(s + "\n") }}
	t.Cleanup(func() { _ = e.dc("down") }) // no -v: volumes survive

	if err := e.dc("up", "-d", "db", "redis"); err != nil {
		t.Fatalf("compose up failed: %v\n%s", err, logs.String())
	}
	status, err := e.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "db") || !strings.Contains(status, "redis") {
		t.Fatalf("services not in status:\n%s", status)
	}
}
