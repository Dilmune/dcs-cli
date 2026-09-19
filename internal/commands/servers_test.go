package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func TestServersList(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathServers, 200, sampleServers())
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newServersListCmd()
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "prod-web")
	assert.Contains(t, out, "staging")
}

func TestServersListJSON(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathServers, 200, sampleServers())
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newServersListCmd()
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	var servers []Server
	err := json.Unmarshal([]byte(out), &servers)
	require.NoError(t, err, "output should be valid JSON")
	assert.Len(t, servers, 2)
	assert.Equal(t, "prod-web", servers[0].Name)
	assert.Equal(t, "staging", servers[1].Name)
}

func TestServersListEmpty(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathServers, 200, []Server{})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newServersListCmd()
	cmd.SetArgs([]string{})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "No servers yet")
}

func TestServersInfo(t *testing.T) {
	servers := sampleServers()
	api := newMockAPI()
	api.on("GET", client.PathServers, 200, servers)
	api.on("GET", client.PathServer("srv-1"), 200, servers[0])
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newServersInfoCmd()
	cmd.SetArgs([]string{"prod-web"})

	out := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "prod-web")
	assert.Contains(t, out, "hetzner")
	assert.Contains(t, out, "203.0.113.1")
}

func TestServersDeleteForce(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathServers, 200, sampleServers())
	api.on("DELETE", client.PathServer("srv-1"), 200, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newServersDeleteCmd()
	cmd.SetArgs([]string{"prod-web"})
	cmd.Flags().Set("force", "true")

	out := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, out, "deleted")
}

func TestServersListUnauthenticated(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	apiClient = nil

	cmd := newServersListCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	assert.ErrorIs(t, err, client.ErrNotAuthenticated)
}
