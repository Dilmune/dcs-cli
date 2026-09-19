package config

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "dcs-cli"
	keychainUser    = "api-key"
)

// SaveToKeychain stores the API key in the OS keychain.
func SaveToKeychain(apiKey string) error {
	if err := keyring.Set(keychainService, keychainUser, apiKey); err != nil {
		return fmt.Errorf("save to keychain: %w", err)
	}
	return nil
}

// LoadFromKeychain retrieves the API key from the OS keychain.
func LoadFromKeychain() (string, error) {
	key, err := keyring.Get(keychainService, keychainUser)
	if err != nil {
		return "", fmt.Errorf("read from keychain: %w", err)
	}
	return key, nil
}

// DeleteFromKeychain removes the API key from the OS keychain.
func DeleteFromKeychain() error {
	if err := keyring.Delete(keychainService, keychainUser); err != nil {
		return fmt.Errorf("delete from keychain: %w", err)
	}
	return nil
}
