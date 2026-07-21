package care

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// composeProject is the compose `name:` — volumes are named "<project>_<volume>".
const composeProject = "care-desktop"

// Backup is one restorable point in the backup folder: a database dump and, for
// daily backups, the matching uploaded-files archive (same timestamp). Manual
// ("Backup now") dumps are DB-only, so FilesArchive is empty.
type Backup struct {
	DBDump       string `json:"db_dump"`       // e.g. care-20260701-020000.dump[.enc]
	FilesArchive string `json:"files_archive"` // e.g. files-20260701-020000.tar.gz[.enc], or ""
	Label        string `json:"label"`         // human-friendly, for the UI dropdown
	Manual       bool   `json:"manual"`        // a "Backup now" dump (DB only)
	Encrypted    bool   `json:"encrypted"`     // written by an encrypted sidecar (.enc)
	SizeBytes    int64  `json:"size_bytes"`    // size of the DB dump
}

// dumpRe pulls the 20060102-150405 timestamp out of care-<ts>.dump and
// care-manual-<ts>.dump, with or without the encrypted .enc suffix.
var dumpRe = regexp.MustCompile(`^care-(?:manual-)?(\d{8}-\d{6})\.dump(?:\.enc)?$`)

// safeName gates filenames coming from the UI/CLI before they reach a shell:
// only our own backup files (optionally .enc) — no path separators, no metacharacters.
var safeName = regexp.MustCompile(`^(?:care-(?:manual-)?\d{8}-\d{6}\.dump(?:\.enc)?|files-\d{8}-\d{6}\.tar\.gz(?:\.enc)?)$`)

// ListBackups returns the restorable points in the backup folder, newest first.
// Missing folder (nothing backed up yet) is not an error — it returns nil.
func (e *Engine) ListBackups() ([]Backup, error) {
	dir := e.backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// index the files-*.tar.gz[.enc] archives by timestamp so we can pair them to dumps.
	files := map[string]string{}
	for _, en := range entries {
		n := en.Name()
		if core, ok := strings.CutPrefix(n, "files-"); ok {
			core = strings.TrimSuffix(core, ".enc")
			if ts, ok := strings.CutSuffix(core, ".tar.gz"); ok {
				files[ts] = n
			}
		}
	}
	var out []Backup
	for _, en := range entries {
		m := dumpRe.FindStringSubmatch(en.Name())
		if m == nil {
			continue
		}
		ts := m[1]
		b := Backup{
			DBDump:       en.Name(),
			FilesArchive: files[ts],
			Manual:       strings.HasPrefix(en.Name(), "care-manual-"),
			Encrypted:    strings.HasSuffix(en.Name(), ".enc"),
		}
		if info, err := en.Info(); err == nil {
			b.SizeBytes = info.Size()
		}
		b.Label = backupLabel(ts, b.Manual, b.FilesArchive != "", b.Encrypted)
		out = append(out, b)
	}
	// sort by the embedded timestamp (newest first) — the "manual-" infix means
	// the raw filename doesn't sort chronologically.
	tsOf := func(name string) string {
		if m := dumpRe.FindStringSubmatch(name); m != nil {
			return m[1]
		}
		return name
	}
	sort.Slice(out, func(i, j int) bool { return tsOf(out[i].DBDump) > tsOf(out[j].DBDump) })
	return out, nil
}

func backupLabel(ts string, manual, withFiles, encrypted bool) string {
	when := ts
	if t, err := time.Parse("20060102-150405", ts); err == nil {
		when = t.Format("2006-01-02 15:04")
	}
	kind := "daily"
	if manual {
		kind = "manual"
	}
	scope := "DB only"
	if withFiles {
		scope = "DB + files"
	}
	label := fmt.Sprintf("%s · %s · %s", when, kind, scope)
	if encrypted {
		label += " · encrypted"
	}
	return label
}

// Restore replaces the live data with a chosen backup. Destructive: the current
// database is dropped and re-created, and (when filesArchive is given) the MinIO
// volume is overwritten. App services are stopped during the swap and brought
// back up afterward. Mirrors the manual steps in docs/backups.md.
func (e *Engine) Restore(dbDump, filesArchive, passphrase string) error {
	dbDump = filepath.Base(dbDump) // tolerate a pasted path; we only ever read from the backup dir
	if !safeName.MatchString(dbDump) || !strings.HasPrefix(dbDump, "care-") {
		return fmt.Errorf("not a database dump: %q", dbDump)
	}
	if err := e.mustExist(dbDump); err != nil {
		return err
	}
	if filesArchive != "" {
		filesArchive = filepath.Base(filesArchive)
		if !safeName.MatchString(filesArchive) || !strings.HasPrefix(filesArchive, "files-") {
			return fmt.Errorf("not a files archive: %q", filesArchive)
		}
		if err := e.mustExist(filesArchive); err != nil {
			return err
		}
	}

	// Check password + key up front, before we stop services and drop the database.
	encrypted := strings.HasSuffix(dbDump, ".enc") || strings.HasSuffix(filesArchive, ".enc")
	if encrypted {
		if passphrase == "" {
			return fmt.Errorf("this backup is encrypted — the backup password is required to restore it")
		}
		if e.privateKeyLocation() == "" {
			return fmt.Errorf("backup encryption key not found — restore on the original computer, or copy %s into the backup folder next to the dumps", e.encKeyName())
		}
	}

	e.logln("Restoring from backup — this replaces the current data.")
	// Release DB connections + halt writes so the swap is clean.
	e.logln("Stopping app services...")
	_ = e.dc("stop", "backend", "celery-worker", "celery-beat")
	// The restore runs inside the backup container (has /backups + pg tools);
	// make sure it and the database are up.
	if err := e.dc("up", "-d", "db", "backup"); err != nil {
		return err
	}
	if err := e.restoreDB(dbDump, passphrase); err != nil {
		return err
	}
	if filesArchive != "" {
		if err := e.restoreFiles(filesArchive, passphrase); err != nil {
			return err
		}
	}
	// Migrate with a SINGLE migrator, before celery-beat (which also migrates on
	// boot): bring up the api backend + deps, migrate the dump up to the current
	// code, then start the rest. Starting everything at once would race two
	// migrators on the dump's pending migrations ("column ... already exists").
	e.logln("Applying database migrations...")
	if err := e.dc("up", "-d", "db", "redis", "backend"); err != nil {
		return err
	}
	if err := e.migrate(); err != nil {
		return err
	}
	e.logln("Bringing CARE back up...")
	if err := e.dc("up", "-d"); err != nil {
		return err
	}
	e.logln("Waiting for CARE to become healthy...")
	if err := e.WaitHealthy(3 * time.Minute); err != nil {
		return err
	}
	e.logln("")
	e.logln("Restore complete → http://" + e.mdnsName() + ".local/")
	return nil
}

func (e *Engine) mustExist(name string) error {
	if _, err := os.Stat(filepath.Join(e.backupDir(), name)); err != nil {
		return fmt.Errorf("backup not found in %s: %s", e.backupDir(), name)
	}
	return nil
}

// restoreDB drops + re-creates the DB and pg_restores, inside the backup container.
// dump is validated by safeName, so embedding it is safe. A .enc dump is decrypted
// to a tmpfile first — before the drop, so a wrong password fails harmlessly.
func (e *Engine) restoreDB(dump, passphrase string) error {
	e.waitForDB()
	e.logln("Restoring database from " + dump + " ...")
	encrypted := strings.HasSuffix(dump, ".enc")
	src := "/backups/" + dump
	prep := `RESTORE_FILE="` + src + `"`
	if encrypted {
		// Key from the backup folder (travels with it); fall back to /keys.
		prep = `KEY=/backups/` + e.encKeyName() + `
[ -f "$KEY" ] || KEY=/keys/` + e.encKeyName() + `
RESTORE_FILE=/tmp/care-restore.dump
trap 'rm -f "$RESTORE_FILE"' EXIT
openssl cms -decrypt -binary -inform DER -in "` + src + `" -out "$RESTORE_FILE" -inkey "$KEY" -passin env:BACKUP_PASS`
	}
	script := `set -e
export PGPASSWORD="$POSTGRES_PASSWORD"
DB="${POSTGRES_DB:-care}"; H="${POSTGRES_HOST:-db}"; U="${POSTGRES_USER:-postgres}"
` + prep + `
psql -h "$H" -U "$U" -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$DB' AND pid<>pg_backend_pid();"
dropdb -h "$H" -U "$U" --if-exists "$DB"
createdb -h "$H" -U "$U" "$DB"
pg_restore -h "$H" -U "$U" -d "$DB" --no-owner --no-privileges "$RESTORE_FILE"`
	args := []string{"exec", "-T"}
	if encrypted {
		// passphrase briefly visible in host argv — acceptable on a single-op box.
		args = append(args, "-e", "BACKUP_PASS="+passphrase)
	}
	args = append(args, "backup", "sh", "-c", script)
	if err := e.dc(args...); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}
	return nil
}

// waitForDB blocks until postgres accepts connections (or gives up after ~100s).
func (e *Engine) waitForDB() {
	for n := 1; n <= 20; n++ {
		if e.dc("exec", "-T", "backup", "sh", "-c",
			`pg_isready -h "${POSTGRES_HOST:-db}" -U "${POSTGRES_USER:-postgres}" -q`) == nil {
			return
		}
		e.logln(fmt.Sprintf("  waiting for database... (%d)", n))
		time.Sleep(5 * time.Second)
	}
}

// restoreFiles overwrites the MinIO volume with the archive. The long-running
// backup container mounts minio-data read-only on purpose, so we use a throwaway
// container to mount it read-write. minio is stopped first so nothing is mid-write;
// the caller's `up -d` restarts it. Uses the backup image (postgres + openssl, and
// its busybox has tar+gzip) so decrypt + extract both work fully offline.
func (e *Engine) restoreFiles(archive, passphrase string) error {
	e.logln("Restoring uploaded files from " + archive + " ...")
	_ = e.dc("stop", "minio")
	vol := composeProject + "_minio-data"
	encrypted := strings.HasSuffix(archive, ".enc")
	src := "/backups/" + archive
	extract := `tar xzf "` + src + `" -C /minio-data`
	if encrypted {
		extract = `KEY=/backups/` + e.encKeyName() + `
[ -f "$KEY" ] || KEY=/keys/` + e.encKeyName() + `
openssl cms -decrypt -binary -inform DER -in "` + src + `" -out /tmp/files.tar.gz -inkey "$KEY" -passin env:BACKUP_PASS
tar xzf /tmp/files.tar.gz -C /minio-data
rm -f /tmp/files.tar.gz`
	}
	// clear the volume (incl. dotfiles) then extract the archive into it.
	script := `set -e
cd /minio-data
rm -rf ./* ./.[!.]* ./..?* 2>/dev/null || true
` + extract
	args := []string{"run", "--rm"}
	if encrypted {
		args = append(args, "-e", "BACKUP_PASS="+passphrase)
	}
	args = append(args,
		"-v", vol+":/minio-data",
		"-v", e.backupDir()+":/backups:ro",
		"-v", e.keysDir()+":/keys:ro",
		e.backupImage(), "sh", "-c", script)
	if err := e.run(nil, "docker", args...); err != nil {
		return fmt.Errorf("file restore failed: %w", err)
	}
	return nil
}
