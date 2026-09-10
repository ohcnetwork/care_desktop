package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

// Backup encryption: an RSA keypair. The sidecar holds only the public cert (can
// seal, can't open); the private key is password-encrypted, needed only at restore.
// A copy of it rides in the backup folder so the folder restores on a fresh machine.

func (s *Store) keysDir() string    { return filepath.Join(s.Dir, "keys") }
func (s *Store) certPath() string   { return filepath.Join(s.keysDir(), "backup-cert.pem") }
func (s *Store) encKeyPath() string { return filepath.Join(s.keysDir(), "backup-key.pem.enc") }
func (s *Store) encKeyName() string { return "backup-key.pem.enc" }

// backupEncryptionOn: encryption is on iff the public cert exists.
func (s *Store) backupEncryptionOn() bool {
	_, err := os.Stat(s.certPath())
	return err == nil
}

func (s *Store) BackupEncryptionOn() bool { return s.backupEncryptionOn() }

// privateKeyLocation prefers the backup-folder copy (pairs with those backups, so a
// carried folder restores anywhere); the install dir copy is the fallback.
func (s *Store) privateKeyLocation() string {
	if p := filepath.Join(s.BackupDir, s.encKeyName()); proc.FileExists(p) {
		return p
	}
	if proc.FileExists(s.encKeyPath()) {
		return s.encKeyPath()
	}
	return ""
}

// ensureKeysDir keeps the ./keys:/keys bind-mount source present (empty = plaintext).
// EnsureKeysDir creates the keys directory.
func (s *Store) EnsureKeysDir() error {
	return os.MkdirAll(s.keysDir(), 0o755)
}

// GenBackupKeypair sets up the backup keypair. On a re-run with a different password
// it reconciles rather than silently diverging from the stored password: reuse if the
// password unlocks the key; regenerate if it differs and no encrypted backups exist;
// refuse if encrypted backups exist (a new key would strand them).
func (s *Store) GenBackupKeypair(passphrase string) error {
	if passphrase == "" {
		return nil // encryption disabled
	}
	if err := s.EnsureKeysDir(); err != nil {
		return err
	}
	if err := s.EnsureImage(); err != nil {
		return err
	}
	if s.backupEncryptionOn() {
		if s.keyUnlocks(passphrase) {
			return nil // same password - keep the existing keypair
		}
		if s.hasEncryptedBackups() {
			return fmt.Errorf("this backup password doesn't match the one your existing encrypted backups were made with - enter the original password (a new key would leave those backups unrecoverable)")
		}
		// No encrypted backups depend on the old key: replace it to match the new password.
		s.logln("Updating the backup encryption key for the new backup password...")
		_ = os.Remove(s.certPath())
		_ = os.Remove(s.encKeyPath())
	}
	s.logln("Generating the backup encryption key...")
	// passphrase via `-e PASS` (no value) -> forwarded from our env, never in argv.
	script := `set -e
openssl req -x509 -newkey rsa:4096 -sha256 -days 36500 \
  -keyout /keys/` + s.encKeyName() + ` -out /keys/backup-cert.pem \
  -subj "/CN=care-backup" -passout env:PASS
chmod 600 /keys/` + s.encKeyName() + ``
	if err := s.run.RunWith([]string{"PASS=" + passphrase}, "docker", "run", "--rm",
		"-e", "PASS",
		"-v", s.keysDir()+":/keys",
		s.Image, "sh", "-c", script); err != nil {
		return err
	}
	// Copy the (password-protected) key beside the backups -> self-contained folder.
	if err := copyFile(s.encKeyPath(), filepath.Join(s.BackupDir, s.encKeyName())); err != nil {
		s.logln("warning: couldn't copy the recovery key into the backup folder: " + err.Error())
	}
	s.logln("Backup encryption enabled.")
	return nil
}

// keyUnlocks reports whether passphrase decrypts the existing private key (quietly).
func (s *Store) keyUnlocks(passphrase string) bool {
	if !proc.FileExists(s.encKeyPath()) {
		return false
	}
	cmd := proc.Command("docker", "run", "--rm",
		"-e", "PASS",
		"-v", s.keysDir()+":/keys:ro",
		s.Image, "sh", "-c",
		`openssl pkey -in /keys/`+s.encKeyName()+` -passin env:PASS -noout 2>/dev/null`)
	cmd.Env = append(s.run.Env, "PASS="+passphrase)
	return cmd.Run() == nil
}

func (s *Store) hasEncryptedBackups() bool {
	backups, err := s.ListBackups()
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
