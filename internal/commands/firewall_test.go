package commands

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func TestFirewallList(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathFirewall("srv-1"), http.StatusOK, FirewallStatus{
		Enabled: true,
		Rules: []FirewallRule{
			{Number: 1, Action: "allow", To: "22/tcp", From: "anywhere", Comment: "SSH"},
		},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newFirewallListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "enabled")
	assert.Contains(t, output, "allow")
	assert.Contains(t, output, "SSH")
}

func TestFirewallListEmpty(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathFirewall("srv-1"), http.StatusOK, FirewallStatus{
		Enabled: false,
		Rules:   nil,
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newFirewallListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "disabled")
}

func TestFirewallAddRule(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodPost, client.PathFirewallRules("srv-1"), http.StatusOK, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newFirewallAddCmd()
	cmd.SetArgs([]string{"--server", "prod-web", "--port", "443", "--protocol", "tcp", "--action", "allow"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestFirewallRemoveForce(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodDelete, client.PathFirewallRule("srv-1", 1), http.StatusOK, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newFirewallRemoveCmd()
	cmd.SetArgs([]string{"1", "--server", "prod-web", "--force"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestFirewallListJSON(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathFirewall("srv-1"), http.StatusOK, FirewallStatus{
		Enabled: true,
		Rules: []FirewallRule{
			{Number: 1, Action: "allow", To: "22/tcp", From: "anywhere", Comment: "SSH"},
		},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()
	cmd := newFirewallListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.True(t, json.Valid([]byte(output)), "expected valid JSON output")
}
