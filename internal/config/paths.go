package config

import (
	"os"
	"path/filepath"
)

// Dir returns the DCS config directory.
// Priority: $DCS_CONFIG_DIR > $XDG_CONFIG_HOME/dcs > ~/.dcs
func Dir() string {
	if dir := os.Getenv("DCS_CONFIG_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "dcs")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".dcs")
	}
	return filepath.Join(home, ".dcs")
}

// FilePath returns the full path to the config file.
func FilePath() string {
	return filepath.Join(Dir(), "config.json")
}

// ProjectFilePath returns the path to .dcs.json in the given directory.
func ProjectFilePath(dir string) string {
	return filepath.Join(dir, ".dcs.json")
}
