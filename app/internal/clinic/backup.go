package clinic

import (
	"errors"
	"path/filepath"
	"time"
)

// BackupNow writes an immediate, encrypted DB dump. There is no plaintext path:
// a manual backup is the one an operator takes before something risky and then
// copies to a USB stick, so it is the last one that should be readable by
// whoever finds the stick. If the keypair is missing we refuse rather than
// quietly writing patient data in the clear.
func (e *Clinic) BackupNow() error {
	if !e.Backups().BackupEncryptionOn() {
		return errors.New("backup encryption is not set up on this install, so a backup " +
			"would be written unencrypted - run setup again and set a backup password")
	}
	ts := time.Now().Format("20060102-150405")
	name := "care-manual-" + ts + ".dump.enc"
	// Same shape as scripts/backup.sh: dump to a temp file, verify it, seal it,
	// then rename into place. Never a pipeline - `pg_dump | openssl` reports
	// openssl's exit status, so a failed dump would look like a written backup.
	script := `set -e
tmp=/backups/.care-manual-` + ts + `.dump.tmp
trap 'rm -f "$tmp" "$tmp.enc"' EXIT
PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -Fc -d "$POSTGRES_DB" -f "$tmp"
pg_restore --list "$tmp" >/dev/null
openssl cms -encrypt -binary -aes-256-cbc -stream -outform DER -in "$tmp" -out "$tmp.enc" /keys/backup-cert.pem
mv "$tmp.enc" /backups/` + name
	if err := e.dc("exec", "-T", "backup", "sh", "-c", script); err != nil {
		return err
	}
	e.logln("Backup written to " + filepath.Join(e.backupDir(), name))
	return nil
}
