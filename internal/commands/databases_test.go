package commands

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func registerServerAndDBRoutes(api *mockAPI) {
	api.on(http.MethodGet, "/api/v1/servers", http.StatusOK, sampleServers())
	api.on(http.MethodGet, "/api/v1/servers/srv-1/databases", http.StatusOK, sampleDatabases())
}

func TestDBList(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newDBListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "myapp_prod")
	assert.Contains(t, output, "postgres")
}

func TestDBListJSON(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newDBListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.True(t, json.Valid([]byte(output)), "expected valid JSON, got: %s", output)
}

func TestDBListEmpty(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, "/api/v1/servers", http.StatusOK, sampleServers())
	api.on(http.MethodGet, "/api/v1/servers/srv-1/databases", http.StatusOK, []Database{})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newDBListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No databases yet")
}

func TestDBDeleteForce(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	api.on(http.MethodDelete, "/api/v1/servers/srv-1/databases/db-1", http.StatusOK, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newDBDeleteCmd()
	cmd.SetArgs([]string{"myapp_prod", "--server", "prod-web"})
	cmd.Flags().Set("force", "true")

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "deleted")
}

func TestDBQuery(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	api.on(http.MethodPost, "/api/v1/servers/srv-1/databases/db-1/query", http.StatusOK, QueryResult{
		Columns:      []string{"id", "name"},
		Rows:         []map[string]any{{"id": "1", "name": "test"}},
		RowsAffected: 1,
		ExecutionMs:  5,
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newDBQueryCmd()
	cmd.SetArgs([]string{"myapp_prod", "--server", "prod-web", "--sql", "SELECT * FROM users"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "test")
}

func TestDBBackups(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	api.on(http.MethodGet, "/api/v1/servers/srv-1/databases/db-1/backups", http.StatusOK, []DBBackup{
		{ID: "bk-1", Status: "completed", SizeBytes: 1048576, CreatedAt: "2026-03-01"},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newDBBackupsCmd()
	cmd.SetArgs([]string{"myapp_prod", "--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "bk-1")
	assert.Contains(t, output, "completed")
}

func TestDBSchemaJSON(t *testing.T) {
	api := newMockAPI()
	registerServerAndDBRoutes(api)
	api.on(http.MethodGet, "/api/v1/servers/srv-1/databases/db-1/schema", http.StatusOK, map[string]any{
		"tables": []map[string]any{
			{"name": "users", "columns": []string{"id", "email"}},
		},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newDBSchemaCmd()
	cmd.SetArgs([]string{"myapp_prod", "--server", "prod-web"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatBytes(tt.bytes), "formatBytes(%d)", tt.bytes)
	}
}
