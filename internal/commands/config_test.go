package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigView(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newConfigViewCmd()
	out := captureStdout(t, func() {
		cmd.Run(cmd, nil)
	})

	assert.Contains(t, out, "Configuration")
	assert.Contains(t, out, cfg.APIURL)
	assert.Contains(t, out, "test@example.com")
}

func TestConfigViewJSON(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newConfigViewCmd()
	out := captureStdout(t, func() {
		cmd.Run(cmd, nil)
	})

	assert.True(t, json.Valid([]byte(out)), "output should be valid JSON")

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Contains(t, result, "api_url")
	assert.Contains(t, result, "authenticated")
}

func TestConfigSet(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)

	cmd := newConfigSetCmd()
	err := cmd.RunE(cmd, []string{"api_url", "https://custom.api.com"})
	require.NoError(t, err)

	assert.Equal(t, "https://custom.api.com", cfg.APIURL)
}

func TestConfigPath(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newConfigPathCmd()
	out := captureStdout(t, func() {
		cmd.Run(cmd, nil)
	})

	assert.NotEmpty(t, out)
	assert.Contains(t, out, "dcs")
}

func TestConfigSetInvalidKey(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newConfigSetCmd()
	err := cmd.RunE(cmd, []string{"invalid_key", "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown config key")
}

func TestConfigView_APIKeyShowsPrefixAndEllipsisOnly(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	tests := []struct {
		name, key, want string
	}{
		{"long key shows first 12 cells then the ellipsis", "dcs_live_0123456789abcdef", "     API key  dcs_live_012…"},
		{"twelve-cell key is fully hidden", "test-key-123", "     API key  ***"},
		{"short key is fully hidden", "short", "     API key  ***"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg.APIKey = tc.key
			cmd := newConfigViewCmd()
			out := captureStdout(t, func() {
				cmd.Run(cmd, nil)
			})
			assert.Contains(t, out, tc.want+"\n")
			assert.NotContains(t, out, tc.key)
		})
	}
}
