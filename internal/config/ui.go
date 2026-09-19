package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const uiWelcomeFile = "ui-welcome-v1"

func UIWelcomeSeen(dir string) (bool, error) {
	info, err := os.Lstat(filepath.Join(dir, uiWelcomeFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read UI welcome preference: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("UI welcome preference is not a regular file")
	}
	return true, nil
}

// A separate empty marker never rewrites config.json or touches the keychain.
func RememberUIWelcome(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create UI preference directory: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, uiWelcomeFile), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		if _, err := UIWelcomeSeen(dir); err != nil {
			return fmt.Errorf("check existing UI welcome preference: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("save UI welcome preference: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close UI welcome preference: %w", err)
	}
	return nil
}
