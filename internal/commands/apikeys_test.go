package commands

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func TestAPIKeysList(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAPIKeys, http.StatusOK, []APIKey{
		{ID: "key-1", Name: "ci-key", KeyPrefix: "dcs_abc", CreatedAt: "2026-03-01"},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newAPIKeysListCmd()

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "ci-key")
	assert.Contains(t, output, "dcs_abc")
}

func TestAPIKeysListEmpty(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAPIKeys, http.StatusOK, []APIKey{})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newAPIKeysListCmd()

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No API keys found")
}

func TestAPIKeysListJSON(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAPIKeys, http.StatusOK, []APIKey{
		{ID: "key-1", Name: "ci-key", KeyPrefix: "dcs_abc", CreatedAt: "2026-03-01"},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()
	cmd := newAPIKeysListCmd()

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.True(t, json.Valid([]byte(output)), "expected valid JSON output")
}

func TestAPIKeysCreate(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodPost, client.PathAPIKeys, http.StatusOK, APIKeyCreated{
		APIKey: APIKey{ID: "key-2", Name: "new-key", KeyPrefix: "dcs_xyz"},
		Key:    "dcs_full_secret_key",
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newAPIKeysCreateCmd()
	cmd.SetArgs([]string{"--name", "new-key"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "created")
	assert.Contains(t, output, "dcs_full_secret_key")
}

func TestAPIKeysDeleteForce(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAPIKeys, http.StatusOK, []APIKey{
		{ID: "key-1", Name: "ci-key", KeyPrefix: "dcs_abc", CreatedAt: "2026-03-01"},
	})
	api.on(http.MethodDelete, client.PathAPIKey("key-1"), http.StatusOK, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newAPIKeysDeleteCmd()
	cmd.SetArgs([]string{"ci-key", "--force"})

	err := cmd.Execute()
	require.NoError(t, err)
}
