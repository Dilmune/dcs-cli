package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// The server response shape: the installedSoftware map carries true or a
// version, and five runtimes keep columns of their own.
func softwareFixtureServer(extra map[string]any) map[string]any {
	server := map[string]any{
		"id": "srv-1", "name": "fixture-web", "status": "active",
		"installedSoftware": map[string]any{"docker": true, "fail2ban": true, "git": true, "pm2": true},
		"phpVersions":       []string{},
		"nodeVersion":       "22",
		"installMySQL":      false,
		"installPostgres":   true,
		"installRedis":      true,
	}
	for key, value := range extra {
		server[key] = value
	}
	return server
}

func TestSoftwareListReadsTheServerResponse(t *testing.T) {
	tests := []struct {
		name      string
		server    map[string]any
		wantLines []string
		wantJSON  string
	}{
		{
			name:      "map entries and runtime columns",
			server:    softwareFixtureServer(nil),
			wantLines: []string{"docker", "fail2ban", "git", "node 22", "pm2", "postgres", "redis"},
			wantJSON:  `["docker","fail2ban","git","node","pm2","postgres","redis"]`,
		},
		{
			name: "versions, several PHP releases and a removed entry",
			server: softwareFixtureServer(map[string]any{
				"installedSoftware": map[string]any{"sqlite": true, "bun": "1.2", "gone": false, "blank": ""},
				"phpVersions":       []string{"8.3", "8.2"},
				"nodeVersion":       "",
				"installMySQL":      true,
				"installPostgres":   false,
				"installRedis":      false,
			}),
			wantLines: []string{"bun 1.2", "mysql", "php 8.2", "php 8.3", "sqlite"},
			wantJSON:  `["bun","mysql","php","sqlite"]`,
		},
		{
			name: "nothing installed",
			server: softwareFixtureServer(map[string]any{
				"installedSoftware": map[string]any{}, "nodeVersion": "", "installPostgres": false, "installRedis": false,
			}),
			wantJSON: `[]`,
		},
	}
	for _, tt := range tests {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tt.name, jsonMode), func(t *testing.T) {
				api := newMockAPI()
				api.on(http.MethodGet, client.PathServers, http.StatusOK, []any{tt.server})
				api.on(http.MethodGet, client.PathServer("srv-1"), http.StatusOK, tt.server)
				t.Cleanup(setupTest(t, api))
				if jsonMode {
					setJSONOutput()
				}
				out := executeContractCommand(t, newSoftwareListCmd(), "--server", "fixture-web")
				if jsonMode {
					assert.JSONEq(t, tt.wantJSON, out)
					return
				}
				if len(tt.wantLines) == 0 {
					assert.Contains(t, out, "No software installed")
					return
				}
				var got []string
				for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
					got = append(got, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*")))
				}
				assert.Equal(t, tt.wantLines, got)
			})
		}
	}
}

func TestServersInfoShowsInstalledSoftware(t *testing.T) {
	server := softwareFixtureServer(nil)
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, []any{server})
	api.on(http.MethodGet, client.PathServer("srv-1"), http.StatusOK, server)
	t.Cleanup(setupTest(t, api))

	out := executeContractCommand(t, newServersInfoCmd(), "fixture-web")
	assert.Contains(t, out, "docker, fail2ban, git, node, pm2, postgres, redis")
}
