package care

import "github.com/zalando/go-keyring"

// Backup password in the OS secret store (Keychain / Credential Manager / Secret
// Service) so restores don't re-prompt - at the cost of putting it back on disk.

const (
	keychainService = "care-desktop"
	keychainAccount = "backup-password"
)

func StoreBackupPassword(pw string) error {
	return keyring.Set(keychainService, keychainAccount, pw)
}

// LoadBackupPassword returns the stored password, or "" if none / store unavailable.
func LoadBackupPassword() string {
	pw, err := keyring.Get(keychainService, keychainAccount)
	if err != nil {
		return ""
	}
	return pw
}

func HasBackupPassword() bool {
	_, err := keyring.Get(keychainService, keychainAccount)
	return err == nil
}

func ForgetBackupPassword() {
	_ = keyring.Delete(keychainService, keychainAccount)
}
