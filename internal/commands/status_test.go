package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
)

func sampleStats() map[string]any {
	return map[string]any{"totalServers": 3, "totalSites": 1, "totalDatabases": 0, "totalDeployments": 7}
}

// NewRootCmd rebinds the persistent flags, which resets the output globals,
// so JSON mode is switched on after the tree exists.
func runStatus(t *testing.T, jsonMode bool) string {
	t.Helper()
	root := NewRootCmd()
	if jsonMode {
		setJSONOutput()
	}
	cmd, _, err := root.Find([]string{"status"})
	require.NoError(t, err)
	return captureStdout(t, func() { require.NoError(t, cmd.RunE(cmd, nil)) })
}

func TestStatusRendersWorkspaceOverview(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathDashboardStats, 200, sampleStats())
	cleanup := setupTest(t, api)
	defer cleanup()

	out := runStatus(t, false)

	assert.Contains(t, out, "  Dilmune Cloud  /  dcs")
	assert.Contains(t, out, "v"+client.Version)
	assert.Contains(t, out, "  Test User\n")
	assert.NotContains(t, out, "test@example.com", "the email row is omitted when a name exists")
	for _, want := range []string{"1  Servers", "2  Sites & deploys", "3  Databases", "4  Storage", "5  Access", "6  Operations",
		"3 servers", "1 site", "0 databases", "Reference: buckets and files", "Reference: keys and account", "Reference: logs, environment, processes"} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "╭", "no boxes in command output")
	assert.NotContains(t, out, "Dashboard")
}

func TestStatusAccountFallsBackToEmail(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathDashboardStats, 200, sampleStats())
	cleanup := setupTest(t, api)
	defer cleanup()
	cfg.User = &config.UserInfo{ID: "user-1", Email: "test@example.com", Name: "  "}

	assert.Contains(t, runStatus(t, false), "  test@example.com\n")
}

func TestStatusEmptyCachedUserReadsTheLiveIdentity(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathDashboardStats, 200, sampleStats())
	api.on("GET", client.PathAuthMe, 200, map[string]any{"user": map[string]any{"id": "user-1", "email": "live@example.com", "name": "Live Name"}})
	cleanup := setupTest(t, api)
	defer cleanup()
	cfg.User = &config.UserInfo{}

	out := runStatus(t, false)
	assert.Contains(t, out, "  Live Name\n")
	assert.NotContains(t, out, "live@example.com")
}

func TestStatusWithoutIdentityOmitsAccountLine(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathDashboardStats, 200, sampleStats())
	cleanup := setupTest(t, api)
	defer cleanup()
	cfg.User = nil

	out := runStatus(t, false)
	assert.True(t, strings.HasPrefix(out, "\n  Dilmune Cloud  /  dcs"), out)
	assert.Contains(t, out, "v"+client.Version+"\n\n    1  Servers", "brand line, one blank, then the areas")
	assert.NotContains(t, out, "test@example.com")
}

func TestStatusJSONIsTheRawStatsPayload(t *testing.T) {
	api := newMockAPI()
	api.on("GET", client.PathDashboardStats, 200, sampleStats())
	cleanup := setupTest(t, api)
	defer cleanup()
	out := runStatus(t, true)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, map[string]any{"totalServers": float64(3), "totalSites": float64(1), "totalDatabases": float64(0), "totalDeployments": float64(7)}, got,
		"fields the overview does not read still pass through untouched")
	assert.NotContains(t, out, "Dilmune Cloud")
}
