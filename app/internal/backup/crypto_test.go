package backup

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ohcnetwork/care_desktop/app/internal/sys/proc"
)

func keyStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	s := &Store{Dir: filepath.Join(root, "install"), BackupDir: filepath.Join(root, "backups")}
	if err := os.MkdirAll(s.keysDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRecoveryKeyCannotBeOverwritten(t *testing.T) {
	s := keyStore(t)
	source := []byte("existing protected key")
	if err := os.WriteFile(s.encKeyPath(), source, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.BackupDir, s.encKeyName())
	if err := s.CopyRecoveryKey(); err != nil {
		t.Fatal(err)
	}
	if err := s.CopyRecoveryKey(); err != nil {
		t.Fatal("copying the same key must be idempotent:", err)
	}
	if err := os.WriteFile(s.encKeyPath(), []byte("a different protected key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.CopyRecoveryKey(); err == nil {
		t.Fatal("a different key overwrote recovery data")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(source) {
		t.Fatalf("old key was changed: %q, %v", got, err)
	}
}

func TestRecoveryKeyCopyCannotBeALinkIntoInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevation")
	}
	s := keyStore(t)
	if err := os.WriteFile(s.encKeyPath(), []byte("protected key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(s.encKeyPath(), filepath.Join(s.BackupDir, s.encKeyName())); err != nil {
		t.Fatal(err)
	}
	if err := s.PreserveRecoveryKey(); err == nil {
		t.Fatal("a link to the file being deleted was accepted as a retained recovery key")
	}
}

func TestFreshSetupRefusesPreservedBackups(t *testing.T) {
	for _, name := range []string{"backup-key.pem.enc", "care-20260101-010101.dump.enc", "files-20260101-010101.tar.gz.enc"} {
		t.Run(name, func(t *testing.T) {
			s := keyStore(t)
			if err := os.WriteFile(filepath.Join(s.BackupDir, name), []byte("recovery data"), 0o600); err != nil {
				t.Fatal(err)
			}
			s.EnsureImage = func() error { t.Fatal("setup reached image work before rejecting the destination"); return nil }
			if err := s.GenBackupKeypair("BackupPass123"); err == nil {
				t.Fatal("setup accepted another installation's recovery folder")
			}
			if _, err := os.Stat(s.encKeyPath()); !os.IsNotExist(err) {
				t.Fatalf("setup created a replacement key: %v", err)
			}
		})
	}
}

func TestWrongBackupPasswordNeverRotatesKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	s := keyStore(t)
	for _, path := range []string{s.encKeyPath(), s.certPath()} {
		if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	s = New(proc.Runner{Env: os.Environ()}, s)
	s.EnsureImage = func() error { return nil }
	if err := s.GenBackupKeypair("wrong"); err == nil {
		t.Fatal("an unverified key was accepted")
	}
	for _, path := range []string{s.encKeyPath(), s.certPath()} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
			t.Fatalf("existing key material changed: %q, %v", data, err)
		}
	}
}

func TestRecoveryKeyPreservationFailsClosed(t *testing.T) {
	s := keyStore(t)
	if err := os.WriteFile(s.encKeyPath(), []byte("protected key"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.BackupDir = filepath.Join(s.Dir, "care-db-backups")
	if err := s.PreserveRecoveryKey(); err == nil {
		t.Fatal("a key was preserved inside the directory being deleted")
	}
	s.BackupDir = filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(s.BackupDir, []byte("personal file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.PreserveRecoveryKey(); err == nil {
		t.Fatal("failed key export was ignored")
	}
	if data, err := os.ReadFile(s.encKeyPath()); err != nil || string(data) != "protected key" {
		t.Fatal("failed export changed the source key")
	}
}

func TestBackupDeletionKeepsUnrelatedFiles(t *testing.T) {
	s := keyStore(t)
	for _, name := range []string{
		"care-20260101-010101.dump.enc", "files-20260101-010101.tar.gz.enc",
		".care-manual-20260101-010101.dump.tmp.enc", ".backup.lock", s.encKeyName(), "notes.txt",
	} {
		if err := os.WriteFile(filepath.Join(s.BackupDir, name), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(s.BackupDir, "personal"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBackups(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "notes.txt" || entries[1].Name() != "personal" {
		t.Fatalf("unexpected retained files: %v", entries)
	}
}

func TestBackupLocationRejectsAliasesIntoInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevation")
	}
	s := keyStore(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(s.Dir, alias); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"care-db-backups", filepath.Join("missing", "nested", "care-db-backups")} {
		if err := CheckLocation(filepath.Join(alias, suffix), s.Dir); err == nil || !strings.Contains(err.Error(), "inside") {
			t.Fatalf("install-directory alias was accepted: %v", err)
		}
	}
}

func TestProtectedKeyCanRecoverItsCertificate(t *testing.T) {
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("the runtime supplies openssl in its backup image")
	}
	s := keyStore(t)
	passphrase := "SyntheticBackupPassword123"
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(openssl, args...)
		cmd.Env = append(os.Environ(), "PASS="+passphrase)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("openssl: %v: %s", err, out)
		}
	}
	run("req", "-x509", "-sha256", "-days", "1", "-newkey", "rsa:2048",
		"-keyout", s.encKeyPath(), "-passout", "env:PASS", "-out", s.certPath(), "-subj", "/CN=care-backup")
	reissued := filepath.Join(s.keysDir(), "reissued.pem")
	run("req", "-x509", "-sha256", "-days", "1", "-key", s.encKeyPath(),
		"-passin", "env:PASS", "-out", reissued, "-subj", "/CN=care-backup")
	readCertificate := func(path string) *x509.Certificate {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		block, _ := pem.Decode(data)
		if block == nil {
			t.Fatal("missing certificate")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		return cert
	}
	if !bytes.Equal(readCertificate(s.certPath()).RawSubjectPublicKeyInfo, readCertificate(reissued).RawSubjectPublicKeyInfo) {
		t.Fatal("recovering a certificate changed its key")
	}
	if err := s.CopyRecoveryKey(); err != nil {
		t.Fatal(err)
	}
	run("pkey", "-in", filepath.Join(s.BackupDir, s.encKeyName()), "-passin", "env:PASS", "-noout")
}

func TestUnusedRecoveryKeyCopyIsDiscardedOnlyWhenProvablyOurs(t *testing.T) {
	s := keyStore(t)
	copyPath := filepath.Join(s.BackupDir, s.encKeyName())
	if err := os.WriteFile(copyPath, []byte("foreign key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.DiscardUnusedRecoveryKey(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyPath); err != nil {
		t.Fatal("a copy that cannot be matched to this installation was removed")
	}
	if err := os.WriteFile(s.encKeyPath(), []byte("foreign key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.BackupDir, "care-20260101-010101.dump.enc"), []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.DiscardUnusedRecoveryKey(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyPath); err != nil {
		t.Fatal("the key protecting an encrypted backup was removed")
	}
	if err := os.Remove(filepath.Join(s.BackupDir, "care-20260101-010101.dump.enc")); err != nil {
		t.Fatal(err)
	}
	if err := s.DiscardUnusedRecoveryKey(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("an unused copy of this installation's key was kept: %v", err)
	}
	if err := s.DiscardUnusedRecoveryKey(); err != nil {
		t.Fatal(err)
	}
}

func TestForeignRecoveryDataIsDetectedBeforeSetup(t *testing.T) {
	s := keyStore(t)
	check := func(want bool, why string) {
		t.Helper()
		foreign, err := s.ForeignRecoveryData()
		if err != nil || foreign != want {
			t.Fatalf("%s: foreign = %v, err = %v", why, foreign, err)
		}
	}
	check(false, "empty folder")
	if err := os.Remove(s.BackupDir); err != nil {
		t.Fatal(err)
	}
	check(false, "missing folder")
	if err := os.MkdirAll(s.BackupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(s.BackupDir, s.encKeyName())
	if err := os.WriteFile(copyPath, []byte("their key"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(true, "key copy with no installation key")
	if err := os.WriteFile(s.encKeyPath(), []byte("our key"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(true, "key copy that differs from ours")
	if err := os.WriteFile(copyPath, []byte("our key"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(false, "our own key copy")
	if err := os.WriteFile(filepath.Join(s.BackupDir, "care-20260101-010101.dump.enc"), []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(false, "our own backups beside our key")
	if err := os.Remove(copyPath); err != nil {
		t.Fatal(err)
	}
	check(true, "encrypted backups without a matching key")
}
