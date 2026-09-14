package backup

import "github.com/zalando/go-keyring"

const (
	keychainService = "care-desktop"
	keychainAccount = "backup-password"
)

func StorePassword(pw string) error {
	return keyring.Set(keychainService, keychainAccount, pw)
}

func LoadPassword() string {
	pw, err := keyring.Get(keychainService, keychainAccount)
	if err != nil {
		return ""
	}
	return pw
}

func HasPassword() bool {
	_, err := keyring.Get(keychainService, keychainAccount)
	return err == nil
}

func ForgetPassword() {
	_ = keyring.Delete(keychainService, keychainAccount)
}
