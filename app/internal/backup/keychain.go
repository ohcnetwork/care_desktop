package backup

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "care-desktop"
	keychainAccount = "backup-password"
)

func StorePassword(pw string) error {
	return keyring.Set(keychainService, keychainAccount, pw)
}

func LoadPassword() (string, error) {
	pw, err := keyring.Get(keychainService, keychainAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return pw, err
}

func HasPassword() (bool, error) {
	_, err := keyring.Get(keychainService, keychainAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func ForgetPassword() error {
	err := keyring.Delete(keychainService, keychainAccount)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
