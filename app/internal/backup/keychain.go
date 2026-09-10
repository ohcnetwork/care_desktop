package backup

import "github.com/zalando/go-keyring"

// Backup password in the OS secret store (Keychain / Credential Manager / Secret
// Service) so restores don't re-prompt - at the cost of putting it back on disk.

const (
	keychainService = "care-desktop"
	keychainAccount = "backup-password"
)

// StorePassword saves the backup password in the OS secret store.
func StorePassword(pw string) error {
	return keyring.Set(keychainService, keychainAccount, pw)
}

// LoadPassword returns the stored password, or "" if none / store unavailable.
func LoadPassword() string {
	pw, err := keyring.Get(keychainService, keychainAccount)
	if err != nil {
		return ""
	}
	return pw
}

// HasPassword reports whether a password is stored.
func HasPassword() bool {
	_, err := keyring.Get(keychainService, keychainAccount)
	return err == nil
}

// ForgetPassword removes the stored password.
func ForgetPassword() {
	_ = keyring.Delete(keychainService, keychainAccount)
}
