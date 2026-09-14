package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ohcnetwork/care_desktop/app/internal/health"
	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

type Backup struct {
	DBDump       string `json:"db_dump"`
	FilesArchive string `json:"files_archive"`
	Label        string `json:"label"`
	Manual       bool   `json:"manual"`
	Encrypted    bool   `json:"encrypted"`
	SizeBytes    int64  `json:"size_bytes"`
}

var dumpRe = regexp.MustCompile(`^care-(?:manual-)?(\d{8}-\d{6})\.dump(?:\.enc)?$`)

var safeName = regexp.MustCompile(`^(?:care-(?:manual-)?\d{8}-\d{6}\.dump(?:\.enc)?|files-\d{8}-\d{6}\.tar\.gz(?:\.enc)?)$`)

func (s *Store) ListBackups() ([]Backup, error) {
	dir := s.BackupDir
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
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
	label := fmt.Sprintf("%s - %s - %s", when, kind, scope)
	if encrypted {
		label += " - encrypted"
	}
	return label
}

func (s *Store) Restore(dbDump, filesArchive, passphrase string) error {
	return s.RestoreFrom(s.BackupDir, dbDump, filesArchive, passphrase)
}

func (s *Store) RestoreFrom(srcDir, dbDump, filesArchive, passphrase string) error {
	if srcDir == "" {
		srcDir = s.BackupDir
	}
	dbDump = filepath.Base(dbDump)
	if !safeName.MatchString(dbDump) || !strings.HasPrefix(dbDump, "care-") {
		return fmt.Errorf("not a database dump: %q", dbDump)
	}
	if err := s.mustExist(srcDir, dbDump); err != nil {
		return err
	}
	if filesArchive != "" {
		filesArchive = filepath.Base(filesArchive)
		if !safeName.MatchString(filesArchive) || !strings.HasPrefix(filesArchive, "files-") {
			return fmt.Errorf("not a files archive: %q", filesArchive)
		}
		if err := s.mustExist(srcDir, filesArchive); err != nil {
			return err
		}
	}

	encrypted := strings.HasSuffix(dbDump, ".enc") || strings.HasSuffix(filesArchive, ".enc")
	if encrypted {
		if passphrase == "" {
			return fmt.Errorf("this backup is encrypted - the backup password is required to restore it")
		}
		if s.keyFor(srcDir) == "" {
			return fmt.Errorf("backup encryption key not found - restore on the original computer, or copy %s into the backup folder next to the dumps", s.encKeyName())
		}
	}

	s.logln("Restoring from backup - this replaces the current data.")
	s.logln("Stopping app services...")
	_ = s.dc("stop", "backend", "celery-worker", "celery-beat")
	if err := s.dc("up", "-d", "db", "backup"); err != nil {
		return err
	}
	if err := s.restoreDB(srcDir, dbDump, passphrase); err != nil {
		return err
	}
	if filesArchive != "" {
		if err := s.restoreFiles(srcDir, filesArchive, passphrase); err != nil {
			return err
		}
	}
	s.logln("Applying database migrations...")
	if err := s.dc("up", "-d", "--wait", "--wait-timeout", "300", "db", "redis", "backend"); err != nil {
		return err
	}
	if err := s.Migrate(); err != nil {
		return err
	}
	s.logln("Bringing CARE back up...")
	if err := s.dc("up", "-d", "--wait", "--wait-timeout", "300"); err != nil {
		return err
	}
	s.logln("Waiting for CARE to become healthy...")
	if err := health.Wait(s.Log, 3*time.Minute); err != nil {
		return err
	}
	s.logln("")
	s.logln("Restore complete -> https://" + s.Host + "/")
	return nil
}

func (s *Store) mustExist(dir, name string) error {
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("backup not found in %s: %s", dir, name)
	}
	return nil
}

func (s *Store) keyFor(dir string) string {
	if p := filepath.Join(dir, s.encKeyName()); proc.FileExists(p) {
		return p
	}
	if proc.FileExists(s.encKeyPath()) {
		return s.encKeyPath()
	}
	return ""
}

func (s *Store) restoreDB(srcDir, dump, passphrase string) error {
	if err := s.waitForDB(); err != nil {
		return err
	}
	s.logln("Restoring database from " + dump + " ...")
	script := `set -e
export PGPASSWORD="$POSTGRES_PASSWORD"
DB="${POSTGRES_DB:-care}"; H="${POSTGRES_HOST:-db}"; U="${POSTGRES_USER:-postgres}"
` + s.plaintextOf(dump, "RESTORE_FILE", "/tmp/care-restore.dump") + `
# Validate the archive BEFORE dropping anything. Decrypting proves the password
# was right, not that the dump is intact; without this a corrupt-but-decryptable
# file gets as far as dropdb+createdb and then fails, leaving an empty database
# where a working clinic used to be. scripts/backup.sh runs the same check before
# it writes, so a dump that fails here was damaged in storage or in transit.
pg_restore --list "$RESTORE_FILE" >/dev/null
psql -h "$H" -U "$U" -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$DB' AND pid<>pg_backend_pid();"
dropdb -h "$H" -U "$U" --if-exists "$DB"
createdb -h "$H" -U "$U" "$DB"
pg_restore -h "$H" -U "$U" -d "$DB" --no-owner --no-privileges "$RESTORE_FILE"`
	if err := s.runInBackupImage(srcDir, passphrase, script); err != nil {
		return fmt.Errorf("database restore failed: %w", err)
	}
	return nil
}

func (s *Store) plaintextOf(name, v, tmp string) string {
	src := "/backups/" + name
	if !strings.HasSuffix(name, ".enc") {
		return v + `="` + src + `"`
	}
	return `KEY=/backups/` + s.encKeyName() + `
[ -f "$KEY" ] || KEY=/keys/` + s.encKeyName() + `
` + v + `=` + tmp + `
trap 'rm -f "$` + v + `"' EXIT
openssl cms -decrypt -binary -inform DER -in "` + src + `" -out "$` + v + `" -inkey "$KEY" -passin env:BACKUP_PASS`
}

func (s *Store) runInBackupImage(srcDir, passphrase, script string, extra ...string) error {
	args := []string{"run", "--rm", "--network", s.Project,
		"--env-file", filepath.Join(s.Dir, "backend.env")}
	if passphrase != "" {
		args = append(args, "-e", "BACKUP_PASS="+passphrase)
	}
	args = append(args, extra...)
	args = append(args,
		"-v", srcDir+":/backups:ro",
		"-v", s.keysDir()+":/keys:ro",
		s.Image, "sh", "-c", script)
	return s.run.Run("docker", args...)
}

func (s *Store) waitForDB() error {
	for n := 1; n <= 20; n++ {
		if s.dc("exec", "-T", "backup", "sh", "-c",
			`pg_isready -h "${POSTGRES_HOST:-db}" -U "${POSTGRES_USER:-postgres}" -q`) == nil {
			return nil
		}
		s.logln(fmt.Sprintf("  waiting for database... (%d)", n))
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("the database did not start, so nothing was changed - start CARE and try the restore again")
}

func (s *Store) restoreFiles(srcDir, archive, passphrase string) error {
	s.logln("Restoring uploaded files from " + archive + " ...")
	_ = s.dc("stop", "minio")
	script := `set -e
` + s.plaintextOf(archive, "ARCHIVE", "/tmp/files.tar.gz") + `
tar -tzf "$ARCHIVE" >/dev/null
cd /minio-data
rm -rf ./* ./.[!.]* ./..?* 2>/dev/null || true
tar xzf "$ARCHIVE" -C /minio-data`
	if err := s.runInBackupImage(srcDir, passphrase, script,
		"-v", s.Project+"_minio-data:/minio-data"); err != nil {
		return fmt.Errorf("file restore failed: %w", err)
	}
	return nil
}
