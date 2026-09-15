package clinic

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

func (e *Clinic) BackupNow() error {
	if !e.Backups().BackupEncryptionOn() {
		return errors.New("backup encryption is not set up on this install, so a backup " +
			"would be written unencrypted - run setup again and set a backup password")
	}
	ts := time.Now().Format("20060102-150405")
	name := "care-manual-" + ts + ".dump.enc"
	if err := e.dc("exec", "-T", "backup", "sh", "/backup.sh", "once", "manual-"+ts); err != nil {
		return fmt.Errorf("could not write the backup - CARE must be running to take one (%w)", err)
	}
	e.logln("Backup written (database only, not uploaded files): " +
		filepath.Join(e.backupDir(), name))
	return nil
}
