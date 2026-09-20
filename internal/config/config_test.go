package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DCS_CONFIG_DIR", t.TempDir())
	t.Setenv("DCS_API_KEY", "")
	t.Setenv("DCS_API_URL", "")

	cfg := Load()
	assert.Equal(t, DefaultAPIURL, cfg.APIURL)
	assert.Empty(t, cfg.APIKey)
	assert.Nil(t, cfg.User)
	assert.False(t, cfg.IsAuthenticated())
}

func TestLoad_FromFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)
	t.Setenv("DCS_API_KEY", "")
	t.Setenv("DCS_API_URL", "")

	content := `{"api_url":"https://custom.api.com","api_key":"dcs_test123","user":{"id":"u1","email":"test@example.com","name":"Test User"}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0600))

	cfg := Load()
	assert.Equal(t, "https://custom.api.com", cfg.APIURL)
	assert.Equal(t, "dcs_test123", cfg.APIKey)
	assert.True(t, cfg.IsAuthenticated())
	require.NotNil(t, cfg.User)
	assert.Equal(t, "test@example.com", cfg.User.Email)
	assert.Equal(t, "Test User", cfg.User.Name)
}

func TestLoad_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)

	content := `{"api_url":"https://file.api.com","api_key":"dcs_from_file"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0600))

	t.Setenv("DCS_API_KEY", "dcs_from_env")
	t.Setenv("DCS_API_URL", "https://env.api.com")

	cfg := Load()
	assert.Equal(t, "https://env.api.com", cfg.APIURL)
	assert.Equal(t, "dcs_from_env", cfg.APIKey)
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)
	t.Setenv("DCS_API_KEY", "")
	t.Setenv("DCS_API_URL", "")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("not json"), 0600))

	cfg := Load()
	assert.Equal(t, DefaultAPIURL, cfg.APIURL)
}

func TestSave_And_Reload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)
	t.Setenv("DCS_API_KEY", "")
	t.Setenv("DCS_API_URL", "")

	cfg := &Config{
		APIURL: "https://saved.api.com",
		APIKey: "dcs_saved_key",
		User:   &UserInfo{ID: "u1", Email: "saved@test.com", Name: "Saved User"},
	}

	require.NoError(t, cfg.Save())

	loaded := Load()
	assert.Equal(t, "https://saved.api.com", loaded.APIURL)
	assert.Equal(t, "dcs_saved_key", loaded.APIKey)
	require.NotNil(t, loaded.User)
	assert.Equal(t, "saved@test.com", loaded.User.Email)
}

func TestIsAuthenticated(t *testing.T) {
	assert.False(t, (&Config{}).IsAuthenticated())
	assert.False(t, (&Config{APIKey: ""}).IsAuthenticated())
	assert.True(t, (&Config{APIKey: "dcs_abc"}).IsAuthenticated())
}

func TestProjectConfig_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	pc := &ProjectConfig{
		ServerID:   "srv-123",
		ServerName: "production",
		SiteID:     "site-456",
		SiteDomain: "app.example.com",
	}

	require.NoError(t, SaveProjectConfig(dir, pc))

	loaded, err := LoadProjectConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, "srv-123", loaded.ServerID)
	assert.Equal(t, "production", loaded.ServerName)
	assert.Equal(t, "site-456", loaded.SiteID)
	assert.Equal(t, "app.example.com", loaded.SiteDomain)
}

func TestLoadProjectConfig_NotFound(t *testing.T) {
	_, err := LoadProjectConfig(t.TempDir())
	assert.Error(t, err)
}

func TestUserInfoDisplayNameFallsBackToEmail(t *testing.T) {
	for _, tt := range []struct {
		name          string
		user          UserInfo
		hasName       bool
		display, line string
	}{
		{"named", UserInfo{Name: "Test User", Email: "test@example.com"}, true, "Test User", "Test User · test@example.com"},
		{"empty name", UserInfo{Email: "test@example.com"}, false, "test@example.com", "test@example.com"},
		{"blank name", UserInfo{Name: "  ", Email: "test@example.com"}, false, "test@example.com", "test@example.com"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.hasName, tt.user.HasDisplayName())
			assert.Equal(t, tt.display, tt.user.DisplayName())
			assert.Equal(t, tt.line, tt.user.AccountLine())
		})
	}
}
