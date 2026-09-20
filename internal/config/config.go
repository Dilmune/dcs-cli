package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultAPIURL = "https://api.dilmune.com"

type UserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

const accountSeparator = " · "

func (u UserInfo) HasDisplayName() bool {
	return strings.TrimSpace(u.Name) != ""
}

// DisplayName is what every surface prints as the account's name; the API
// leaves Name empty for accounts that never set one, so the email stands in.
func (u UserInfo) DisplayName() string {
	if u.HasDisplayName() {
		return u.Name
	}
	return u.Email
}

// AccountLine is the one-line identity shown where there is no room for
// separate name and email rows.
func (u UserInfo) AccountLine() string {
	if u.HasDisplayName() {
		return u.Name + accountSeparator + u.Email
	}
	return u.Email
}

type ProjectConfig struct {
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	SiteID     string `json:"site_id"`
	SiteDomain string `json:"site_domain"`
}

type Config struct {
	APIURL          string    `json:"api_url"`
	APIKey          string    `json:"api_key"`
	User            *UserInfo `json:"user,omitempty"`
	DefaultServerID string    `json:"default_server_id,omitempty"`
	UseKeychain     bool      `json:"use_keychain,omitempty"`
}

// Load reads the config from disk, with env var and keychain overrides.
func Load() *Config {
	cfg := &Config{
		APIURL: DefaultAPIURL,
	}

	data, err := os.ReadFile(FilePath())
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to parse config file: %v\n", err)
		}
	}

	if cfg.UseKeychain && cfg.APIKey == "" {
		if key, err := LoadFromKeychain(); err == nil {
			cfg.APIKey = key
		}
	}

	if v := os.Getenv("DCS_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("DCS_API_URL"); v != "" {
		cfg.APIURL = v
	}

	return cfg
}

// Save writes the config to disk. If keychain is enabled, stores the API key
// in the OS keychain and omits it from the config file.
func (c *Config) Save() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	toSave := *c
	if c.UseKeychain && c.APIKey != "" {
		if err := SaveToKeychain(c.APIKey); err != nil {
			return fmt.Errorf("save to keychain: %w", err)
		}
		toSave.APIKey = ""
	}

	data, err := json.MarshalIndent(&toSave, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(FilePath(), data, 0600); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// IsAuthenticated returns true if an API key is configured.
func (c *Config) IsAuthenticated() bool {
	return c.APIKey != ""
}

// LoadProjectConfig reads .dcs.json from the given directory.
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	data, err := os.ReadFile(ProjectFilePath(dir))
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}
	var pc ProjectConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parse .dcs.json: %w", err)
	}
	return &pc, nil
}

// SaveProjectConfig writes .dcs.json to the given directory.
func SaveProjectConfig(dir string, pc *ProjectConfig) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal .dcs.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".dcs.json"), data, 0600); err != nil {
		return fmt.Errorf("save project config: %w", err)
	}
	return nil
}
