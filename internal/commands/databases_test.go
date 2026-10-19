package commands

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
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

func TestDBRestoreResolvesTheBackupID(t *testing.T) {
	backups := []map[string]string{
		{"id": "b0000000-1111-4000-8000-000000000001", "status": "completed"},
		{"id": "b0000000-2222-4000-8000-000000000002", "status": "completed"},
		{"id": "c0ffee00-3333-4000-8000-000000000003", "status": "completed"},
	}
	tests := []struct {
		name       string
		backupID   string
		wantTarget string
		wantErr    error
		wantListed bool
	}{
		{name: "the short ID from the table reaches the full ID", backupID: "c0ffee00", wantTarget: "c0ffee00-3333-4000-8000-000000000003", wantListed: true},
		{name: "a full ID", backupID: "b0000000-2222-4000-8000-000000000002", wantTarget: "b0000000-2222-4000-8000-000000000002", wantListed: true},
		{name: "a short ID two backups share is ambiguous", backupID: "b0000000", wantErr: errLookupAmbiguous, wantListed: true},
		{name: "fewer than 8 characters is not a lookup", backupID: "c0ffee", wantErr: errLookupNotFound, wantListed: true},
		{name: "a backup of another database is not found", backupID: "d0000000-4444-4000-8000-000000000004", wantErr: errLookupNotFound, wantListed: true},
		{name: "a blank ID is refused before any lookup", backupID: "", wantErr: errLookupInputRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var restores []string
			var listed atomic.Int32
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == client.PathServers:
					writeContractResponse(w, sampleServers())
				case r.Method == http.MethodGet && r.URL.Path == client.PathDatabases("srv-1"):
					writeContractResponse(w, sampleDatabases())
				case r.Method == http.MethodGet && r.URL.Path == client.PathDatabaseBackups("srv-1", "db-1"):
					listed.Add(1)
					writeContractResponse(w, backups)
				case r.Method == http.MethodPost:
					restores = append(restores, r.URL.Path)
					writeContractResponse(w, nil)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			cmd := newDBRestoreCmd()
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			cmd.SetArgs([]string{"myapp_prod", tt.backupID, "--server", "srv-1", "--force"})
			var err error
			captureStderr(t, func() {
				captureStdout(t, func() { err = cmd.Execute() })
			})
			if tt.wantTarget != "" {
				require.NoError(t, err)
				assert.Equal(t, []string{client.PathDatabaseBackupRestore("srv-1", "db-1", tt.wantTarget)}, restores)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, restores, "no restore may be requested")
			}
			assert.Equal(t, tt.wantListed, listed.Load() > 0)
		})
	}
}
