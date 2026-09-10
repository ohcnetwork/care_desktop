package clinic

import (
	"os"
)

// Setup is the one-time bootstrap: secret, backup dir, clone+build both images,
// then mDNS. Mirrors `care.sh setup`.
func (e *Clinic) Setup() error {
	if err := e.genSecret(); err != nil {
		return err
	}
	if err := e.applyDomain(); err != nil {
		return err
	}
	if err := os.MkdirAll(e.backupDir(), 0o755); err != nil {
		return err
	}
	e.logln("Backups will go to: " + e.backupDir())
	// Backup image + keypair + WAF image first: small build contexts, fast failures.
	if err := e.Backups().EnsureKeysDir(); err != nil {
		return err
	}
	if err := e.Builder().BuildBackup(); err != nil {
		return err
	}
	if err := e.Backups().GenBackupKeypair(e.BackupPassword); err != nil {
		return err
	}
	if err := e.Builder().BuildCaddy(); err != nil {
		return err
	}
	if err := e.Builder().BuildBackend(); err != nil {
		return err
	}
	if err := e.Builder().BuildFrontend(); err != nil {
		return err
	}
	// Hosts entry + cert trust wait for Start (ensureLocalAccess): one approval, and
	// the CA doesn't exist until Caddy runs.
	e.logln("Setup done.")
	return nil
}
