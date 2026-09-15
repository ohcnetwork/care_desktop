package backup

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

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
	hasKey, err := regularFileExists(s.encKeyPath())
	if err != nil {
		return err
	}
	hasCert, err := regularFileExists(s.certPath())
	if err != nil {
		return err
	}
	if hasCert && !hasKey {
		return fmt.Errorf("the backup certificate exists but its private key is missing; recover %s before continuing", s.encKeyPath())
	}
	if !hasKey {
		encrypted, err := s.hasEncryptedBackups()
		if err != nil {
			return err
		}
		existingKey, err := regularFileExists(filepath.Join(s.BackupDir, s.encKeyName()))
		if err != nil {
			return err
		}
		if encrypted || existingKey {
			return fmt.Errorf("%s contains recovery data from another installation; choose a new backup folder and keep the old folder intact for restores", s.BackupDir)
		}
	}
	if err := s.EnsureKeysDir(); err != nil {
		return err
	}
	if err := s.EnsureImage(); err != nil {
		return err
	}
	if hasKey {
		if err := s.keyUnlocks(passphrase, hasCert); err != nil {
			return err
		}
		if err := s.CopyRecoveryKey(); err != nil {
			return err
		}
		if hasCert {
			return nil
		}
	}
	stage, err := os.MkdirTemp(s.keysDir(), ".new-key-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	keyArgs := "-newkey rsa:4096 -keyout /keys/" + s.encKeyName() + " -passout env:PASS"
	if hasKey {
		key, err := s.readKey()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(stage, s.encKeyName()), key, 0o600); err != nil {
			return err
		}
		keyArgs = "-key /keys/" + s.encKeyName() + " -passin env:PASS"
	}
	s.logln("Preparing the backup encryption certificate...")
	args := []string{"run", "--rm", "-e", "PASS", "-v", stage + ":/keys"}
	if runtime.GOOS != "windows" {
		args = append(args, "--user", strconv.Itoa(os.Geteuid())+":"+strconv.Itoa(os.Getegid()))
	}
	script := "set -e\nopenssl req -x509 -sha256 -days 36500 " + keyArgs +
		" -out /keys/backup-cert.pem -subj /CN=care-backup\nchmod 600 /keys/" + s.encKeyName()
	args = append(args, s.Image, "sh", "-c", script)
	if err := s.run.RunWith([]string{"PASS=" + passphrase}, "docker", args...); err != nil {
		return fmt.Errorf("could not prepare the backup encryption key: %w", err)
	}
	if !hasKey {
		if err := copyFile(filepath.Join(stage, s.encKeyName()), s.encKeyPath()); err != nil {
			return err
		}
	}
	if err := copyFile(filepath.Join(stage, "backup-cert.pem"), s.certPath()); err != nil {
		return err
	}
	if err := s.CopyRecoveryKey(); err != nil {
		return err
	}
	s.logln("Backup encryption enabled; its recovery key is saved with the backups.")
	return nil
}

func (s *Store) keyUnlocks(passphrase string, checkCertificate bool) error {
	script := `set -e
openssl pkey -in /keys/backup-key.pem.enc -passin env:PASS -pubout -out /tmp/key.pub`
	if checkCertificate {
		script += `
openssl x509 -in /keys/backup-cert.pem -pubkey -noout > /tmp/cert.pub
cmp -s /tmp/key.pub /tmp/cert.pub`
	}
	cmd := proc.Command("docker", "run", "--rm",
		"-e", "PASS",
		"-v", s.keysDir()+":/keys:ro",
		s.Image, "sh", "-c", script)
	cmd.Env = append(cmd.Environ(), s.run.Env...)
	cmd.Env = append(cmd.Env, "PASS="+passphrase)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not unlock and match the existing backup key; enter its original password and ensure Docker is running (the key was not changed): %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Store) hasEncryptedBackups() (bool, error) {
	entries, err := os.ReadDir(s.BackupDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not inspect the backup folder: %w", err)
	}
	for _, entry := range entries {
		if safeName.MatchString(entry.Name()) && strings.HasSuffix(entry.Name(), ".enc") {
			return true, nil
		}
	}
	return false, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("expected a regular file at %s", path)
	}
	return true, nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeKeyCopy(dst, b)
}

func (s *Store) CopyRecoveryKey() error {
	b, err := s.readKey()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return err
	}
	return writeKeyCopy(filepath.Join(s.BackupDir, s.encKeyName()), b)
}

func (s *Store) PreserveRecoveryKey() error {
	if err := CheckLocation(s.BackupDir, s.Dir); err != nil {
		return err
	}
	exists, err := regularFileExists(s.encKeyPath())
	if err != nil || !exists {
		return err
	}
	if err := s.CopyRecoveryKey(); err != nil {
		return fmt.Errorf("the installation's recovery key must be preserved before its files can be removed: %w", err)
	}
	return nil
}

func CheckLocation(dir, protected string) error {
	resolve := func(path string) (string, error) {
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("expected an absolute directory path: %s", path)
		}
		for current := filepath.Clean(path); ; current = filepath.Dir(current) {
			resolved, err := filepath.EvalSymlinks(current)
			if err == nil {
				rel, err := filepath.Rel(current, path)
				if err != nil {
					return "", err
				}
				return filepath.Join(resolved, rel), nil
			}
			if !os.IsNotExist(err) || filepath.Dir(current) == current {
				return "", err
			}
		}
	}
	target, err := resolve(dir)
	if err != nil {
		return err
	}
	root, err := resolve(protected)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.VolumeName(target), filepath.VolumeName(root)) {
		return nil
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("backups cannot be kept inside %s because cleanup removes that directory; choose a separate folder", protected)
	}
	return nil
}

func (s *Store) DeleteBackups() error {
	entries, err := os.ReadDir(s.BackupDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var failed []error
	kept := false
	for _, entry := range entries {
		name := entry.Name()
		candidate := strings.Replace(strings.TrimPrefix(name, "."), ".tmp", "", 1)
		if entry.IsDir() || (name != s.encKeyName() && name != ".backup.lock" && !safeName.MatchString(candidate)) {
			kept = true
			continue
		}
		if err := os.Remove(filepath.Join(s.BackupDir, name)); err != nil {
			failed = append(failed, err)
		}
	}
	if err := errors.Join(failed...); err != nil {
		return err
	}
	if kept {
		s.logln("Unrecognized files were kept in " + s.BackupDir)
		return nil
	}
	return os.Remove(s.BackupDir)
}

func (s *Store) readKey() ([]byte, error) {
	b, err := os.ReadFile(s.encKeyPath())
	if errors.Is(err, os.ErrPermission) && runtime.GOOS == "linux" {
		cmd := proc.Command("docker", "run", "--rm", "-v", s.keysDir()+":/keys:ro",
			s.Image, "cat", "/keys/"+s.encKeyName())
		cmd.Env = s.run.Env
		b, err = cmd.Output()
	}
	if err != nil {
		return nil, fmt.Errorf("could not read the recovery key at %s: %w", s.encKeyPath(), err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, fmt.Errorf("the recovery key at %s is empty", s.encKeyPath())
	}
	return b, nil
}

func writeKeyCopy(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("the recovery copy at %s must be an independent regular file, not a link or directory", path)
		}
		existing, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(existing, b) {
			return fmt.Errorf("a different recovery key or certificate already exists at %s; it was not overwritten", path)
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := f.Write(b)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	err = errors.Join(writeErr, f.Close())
	if err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}
