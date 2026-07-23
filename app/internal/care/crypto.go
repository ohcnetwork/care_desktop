package care

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Backup encryption: an RSA keypair. The sidecar holds only the public cert (can
// seal, can't open); the private key is password-encrypted, needed only at restore.
// A copy of it rides in the backup folder so the folder restores on a fresh machine.

func (e *Engine) keysDir() string    { return filepath.Join(e.Kit, "keys") }
func (e *Engine) certPath() string   { return filepath.Join(e.keysDir(), "backup-cert.pem") }
func (e *Engine) encKeyPath() string { return filepath.Join(e.keysDir(), "backup-key.pem.enc") }
func (e *Engine) encKeyName() string { return "backup-key.pem.enc" }

// backupEncryptionOn: encryption is on iff the public cert exists.
func (e *Engine) backupEncryptionOn() bool {
	_, err := os.Stat(e.certPath())
	return err == nil
}

func (e *Engine) BackupEncryptionOn() bool { return e.backupEncryptionOn() }

// privateKeyLocation prefers the backup-folder copy (pairs with those backups, so a
// carried folder restores anywhere); the kit copy is the fallback.
func (e *Engine) privateKeyLocation() string {
	if p := filepath.Join(e.backupDir(), e.encKeyName()); fileExists(p) {
		return p
	}
	if fileExists(e.encKeyPath()) {
		return e.encKeyPath()
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ensureKeysDir keeps the ./keys:/keys bind-mount source present (empty = plaintext).
func (e *Engine) ensureKeysDir() error {
	return os.MkdirAll(e.keysDir(), 0o755)
}

// GenBackupKeypair sets up the backup keypair. On a re-run with a different password
// it reconciles rather than silently diverging from the stored password: reuse if the
// password unlocks the key; regenerate if it differs and no encrypted backups exist;
// refuse if encrypted backups exist (a new key would strand them).
func (e *Engine) GenBackupKeypair(passphrase string) error {
	if passphrase == "" {
		return nil // encryption disabled
	}
	if err := e.ensureKeysDir(); err != nil {
		return err
	}
	if err := e.ensureBackupImage(); err != nil {
		return err
	}
	if e.backupEncryptionOn() {
		if e.keyUnlocks(passphrase) {
			return nil // same password - keep the existing keypair
		}
		if e.hasEncryptedBackups() {
			return fmt.Errorf("this backup password doesn't match the one your existing encrypted backups were made with - enter the original password (a new key would leave those backups unrecoverable)")
		}
		// No encrypted backups depend on the old key: replace it to match the new password.
		e.logln("Updating the backup encryption key for the new backup password...")
		_ = os.Remove(e.certPath())
		_ = os.Remove(e.encKeyPath())
	}
	e.logln("Generating the backup encryption key...")
	// passphrase via `-e PASS` (no value) -> forwarded from our env, never in argv.
	script := `set -e
openssl req -x509 -newkey rsa:4096 -sha256 -days 36500 \
  -keyout /keys/` + e.encKeyName() + ` -out /keys/backup-cert.pem \
  -subj "/CN=care-backup" -passout env:PASS
chmod 600 /keys/` + e.encKeyName() + ``
	if err := e.run([]string{"PASS=" + passphrase}, "docker", "run", "--rm",
		"-e", "PASS",
		"-v", e.keysDir()+":/keys",
		e.backupImage(), "sh", "-c", script); err != nil {
		return err
	}
	// Copy the (password-protected) key beside the backups -> self-contained folder.
	if err := copyFile(e.encKeyPath(), filepath.Join(e.backupDir(), e.encKeyName())); err != nil {
		e.logln("warning: couldn't copy the recovery key into the backup folder: " + err.Error())
	}
	e.logln("Backup encryption enabled.")
	return nil
}

// keyUnlocks reports whether passphrase decrypts the existing private key (quietly).
func (e *Engine) keyUnlocks(passphrase string) bool {
	if !fileExists(e.encKeyPath()) {
		return false
	}
	cmd := exec.Command("docker", "run", "--rm",
		"-e", "PASS",
		"-v", e.keysDir()+":/keys:ro",
		e.backupImage(), "sh", "-c",
		`openssl pkey -in /keys/`+e.encKeyName()+` -passin env:PASS -noout 2>/dev/null`)
	cmd.Env = append(e.baseEnv(), "PASS="+passphrase)
	return cmd.Run() == nil
}

func (e *Engine) hasEncryptedBackups() bool {
	backups, err := e.ListBackups()
	if err != nil {
		return false
	}
	for _, b := range backups {
		if b.Encrypted {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o600)
}
