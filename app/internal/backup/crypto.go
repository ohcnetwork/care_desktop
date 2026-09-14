package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func (s *Store) keysDir() string    { return filepath.Join(s.Dir, "keys") }
func (s *Store) certPath() string   { return filepath.Join(s.keysDir(), "backup-cert.pem") }
func (s *Store) encKeyPath() string { return filepath.Join(s.keysDir(), "backup-key.pem.enc") }
func (s *Store) encKeyName() string { return "backup-key.pem.enc" }

func (s *Store) BackupEncryptionOn() bool {
	_, err := os.Stat(s.certPath())
	return err == nil
}

func (s *Store) EnsureKeysDir() error {
	return os.MkdirAll(s.keysDir(), 0o755)
}

func (s *Store) GenBackupKeypair(passphrase string) error {
	if passphrase == "" {
		return fmt.Errorf("a backup password is required - every backup is encrypted, and without one none can be written")
	}
	if err := s.EnsureKeysDir(); err != nil {
		return err
	}
	if err := s.EnsureImage(); err != nil {
		return err
	}
	if s.BackupEncryptionOn() {
		if s.keyUnlocks(passphrase) {
			return nil
		}
		if s.hasEncryptedBackups() {
			return fmt.Errorf("this backup password doesn't match the one your existing encrypted backups were made with - enter the original password (a new key would leave those backups unrecoverable)")
		}
		s.logln("Updating the backup encryption key for the new backup password...")
		_ = os.Remove(s.certPath())
		_ = os.Remove(s.encKeyPath())
	}
	s.logln("Generating the backup encryption key...")
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
	if err := s.CopyRecoveryKey(); err != nil {
		s.logln("warning: couldn't copy the recovery key into the backup folder: " + err.Error())
	}
	s.logln("Backup encryption enabled.")
	return nil
}

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

func (s *Store) CopyRecoveryKey() error {
	if !proc.FileExists(s.encKeyPath()) {
		return fmt.Errorf("no recovery key at %s", s.encKeyPath())
	}
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return err
	}
	return copyFile(s.encKeyPath(), filepath.Join(s.BackupDir, s.encKeyName()))
}
