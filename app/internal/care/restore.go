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
	DBDump       string `json:"db_dump"`       // e.g. care-20260701-020000.dump
	FilesArchive string `json:"files_archive"` // e.g. files-20260701-020000.tar.gz, or ""
	Label        string `json:"label"`         // human-friendly, for the UI dropdown
	Manual       bool   `json:"manual"`        // a "Backup now" dump (DB only)
	SizeBytes    int64  `json:"size_bytes"`    // size of the DB dump
}

// dumpRe pulls the 20060102-150405 timestamp out of care-<ts>.dump and
// care-manual-<ts>.dump.
var dumpRe = regexp.MustCompile(`^care-(?:manual-)?(\d{8}-\d{6})\.dump$`)

// safeName gates filenames coming from the UI/CLI before they reach a shell:
// only our own backup files — no path separators, no metacharacters.
var safeName = regexp.MustCompile(`^(?:care-(?:manual-)?\d{8}-\d{6}\.dump|files-\d{8}-\d{6}\.tar\.gz)$`)

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
	// index the files-*.tar.gz archives by timestamp so we can pair them to dumps.
	files := map[string]string{}
	for _, en := range entries {
		n := en.Name()
		if ts, ok := strings.CutPrefix(n, "files-"); ok {
			if ts, ok := strings.CutSuffix(ts, ".tar.gz"); ok {
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
		}
		if info, err := en.Info(); err == nil {
			b.SizeBytes = info.Size()
		}
		b.Label = backupLabel(ts, b.Manual, b.FilesArchive != "")
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

func backupLabel(ts string, manual, withFiles bool) string {
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
	return fmt.Sprintf("%s · %s · %s", when, kind, scope)
}

// Restore replaces the live data with a chosen backup. Destructive: the current
// database is dropped and re-created, and (when filesArchive is given) the MinIO
// volume is overwritten. App services are stopped during the swap and brought
// back up afterward. Mirrors the manual steps in docs/backups.md.
func (e *Engine) Restore(dbDump, filesArchive string) error {
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

	e.logln("Restoring from backup — this replaces the current data.")
	// Release DB connections + halt writes so the swap is clean.
	e.logln("Stopping app services...")
	_ = e.dc("stop", "backend", "celery-worker", "celery-beat")
	// The restore runs inside the backup container (has /backups + pg tools);
	// make sure it and the database are up.
	if err := e.dc("up", "-d", "db", "backup"); err != nil {
		return err
	}
	if err := e.restoreDB(dbDump); err != nil {
		return err
	}
	if filesArchive != "" {
		if err := e.restoreFiles(filesArchive); err != nil {
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
	e.migrate()
	e.logln("Bringing CARE back up...")
	if err := e.dc("up", "-d"); err != nil {
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

// restoreDB drops + re-creates the database and pg_restores the dump, from inside
// the backup container (the dump is already mounted there at /backups). The dump
// name is validated by safeName, so embedding it in the script is safe.
func (e *Engine) restoreDB(dump string) error {
	e.waitForDB()
	e.logln("Restoring database from " + dump + " ...")
	script := `set -e
export PGPASSWORD="$POSTGRES_PASSWORD"
DB="${POSTGRES_DB:-care}"; H="${POSTGRES_HOST:-db}"; U="${POSTGRES_USER:-postgres}"
psql -h "$H" -U "$U" -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$DB' AND pid<>pg_backend_pid();"
dropdb -h "$H" -U "$U" --if-exists "$DB"
createdb -h "$H" -U "$U" "$DB"
pg_restore -h "$H" -U "$U" -d "$DB" --no-owner --no-privileges "/backups/` + dump + `"`
	if err := e.dc("exec", "-T", "backup", "sh", "-c", script); err != nil {
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
// the caller's `up -d` restarts it. Uses the postgres image (already present, and
// its busybox has tar+gzip) so this works fully offline.
func (e *Engine) restoreFiles(archive string) error {
	e.logln("Restoring uploaded files from " + archive + " ...")
	_ = e.dc("stop", "minio")
	vol := composeProject + "_minio-data"
	// clear the volume (incl. dotfiles) then extract the archive into it.
	script := `set -e
cd /minio-data
rm -rf ./* ./.[!.]* ./..?* 2>/dev/null || true
tar xzf "/backups/` + archive + `" -C /minio-data`
	if err := e.run(nil, "docker", "run", "--rm",
		"-v", vol+":/minio-data",
		"-v", e.backupDir()+":/backups:ro",
		"postgres:17-alpine", "sh", "-c", script); err != nil {
		return fmt.Errorf("file restore failed: %w", err)
	}
	return nil
}
