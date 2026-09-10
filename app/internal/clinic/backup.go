package clinic

import (
	"path/filepath"
	"time"
)

// BackupNow writes an immediate DB dump (encrypted -> .enc when encryption is on).
func (e *Clinic) BackupNow() error {
	ts := time.Now().Format("20060102-150405")
	name := "care-manual-" + ts + ".dump"
	dump := `PGPASSWORD=$POSTGRES_PASSWORD pg_dump -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -Fc -d "$POSTGRES_DB"`
	var script string
	if e.Backups().BackupEncryptionOn() {
		name += ".enc"
		script = "set -e; " + dump +
			" | openssl cms -encrypt -binary -aes-256-cbc -stream -outform DER -out /backups/" + name + " /keys/backup-cert.pem"
	} else {
		script = dump + " -f /backups/" + name
	}
	if err := e.dc("exec", "-T", "backup", "sh", "-c", script); err != nil {
		return err
	}
	e.logln("Backup written to " + filepath.Join(e.backupDir(), name))
	return nil
}
