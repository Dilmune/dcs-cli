package commands

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func setupContractAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	t.Cleanup(setupTest(t, newMockAPI()))
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	apiClient = client.New(server.URL, "fixture-key")
	cfg.APIURL = server.URL
	cfg.DefaultServerID = "srv-1"
}

func writeContractResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func executeContractCommand(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	cmd.SetArgs(args)
	var runErr error
	output := captureStdout(t, func() { runErr = cmd.Execute() })
	require.NoError(t, runErr)
	return output
}

func linkContractProject(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, config.SaveProjectConfig(dir, &config.ProjectConfig{
		ServerID: "srv-1", SiteID: "site-1", SiteDomain: "fixture.example.test",
	}))
	t.Chdir(dir)
}

func TestStatusUsesDashboardContract(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathDashboardStats, http.StatusOK, map[string]any{
		"totalServers": 7, "totalSites": 8, "totalDatabases": 9,
		"activeServers": 7, "totalSshKeys": 2, "hasPaymentMethod": true,
	})
	t.Cleanup(setupTest(t, api))
	output := executeContractCommand(t, newStatusCmd())
	assert.Contains(t, output, "7 servers")
	assert.Contains(t, output, "8 sites")
	assert.Contains(t, output, "9 databases")
	assert.NotContains(t, output, "Billing:")
}

func TestServerCreatedAtUsesAPIContractAndPreservesJSONKey(t *testing.T) {
	wire := map[string]string{"id": "srv-1", "name": "fixture", "status": "active", "createdAt": "2026-01-02T03:04:05Z"}
	data, err := json.Marshal(wire)
	require.NoError(t, err)
	var server Server
	require.NoError(t, json.Unmarshal(data, &server))
	assert.Equal(t, "2026-01-02T03:04:05Z", server.CreatedAt)
	output, err := json.Marshal(server)
	require.NoError(t, err)
	var serialized map[string]any
	require.NoError(t, json.Unmarshal(output, &serialized))
	assert.Equal(t, "2026-01-02T03:04:05Z", serialized["created_at"])
	assert.NotContains(t, serialized, "createdAt")
	var legacy Server
	require.NoError(t, json.Unmarshal(output, &legacy))
	assert.Equal(t, server, legacy)
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, []any{wire})
	api.on(http.MethodGet, client.PathServer("srv-1"), http.StatusOK, wire)
	t.Cleanup(setupTest(t, api))
	assert.Contains(t, executeContractCommand(t, newServersInfoCmd(), "srv-1"), "2026-01-02T03:04:05Z")
}

func TestSitesCreateUsesAPIContractAndPreservesJSONKeys(t *testing.T) {
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, client.PathSites("srv-1"), r.URL.Path)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"domain":"fixture.example.test","projectType":"node","gitRepoUrl":"https://example.test/fixture.git","gitBranch":"release"}`, string(body))
		writeContractResponse(w, map[string]any{
			"id": "site-1", "domain": "fixture.example.test", "projectType": "node",
			"status": "active", "sslEnabled": true, "gitRepoUrl": "https://example.test/fixture.git",
			"gitBranch": "release", "createdAt": "2026-01-02T03:04:05Z",
		})
	})
	setJSONOutput()
	output := executeContractCommand(t, newSitesCreateCmd(), "--domain", "fixture.example.test", "--type", "node", "--repo", "https://example.test/fixture.git", "--branch", "release")
	assert.JSONEq(t, `{"id":"site-1","domain":"fixture.example.test","project_type":"node","status":"active","ssl_enabled":true,"git_repo":"https://example.test/fixture.git","git_branch":"release","created_at":"2026-01-02T03:04:05Z"}`, output)
}

func TestSitesInfoUsesAPIContract(t *testing.T) {
	api := newMockAPI()
	site := map[string]any{
		"id": "site-1", "domain": "fixture.example.test", "projectType": "node", "status": "active",
		"sslEnabled": true, "gitRepoUrl": "https://example.test/fixture.git", "gitBranch": "release",
		"createdAt": "2026-01-02T03:04:05Z",
	}
	api.on(http.MethodGet, client.PathSites("srv-1"), http.StatusOK, []any{site})
	api.on(http.MethodGet, client.PathSite("srv-1", "site-1"), http.StatusOK, site)
	t.Cleanup(setupTest(t, api))
	cfg.DefaultServerID = "srv-1"
	output := executeContractCommand(t, newSitesInfoCmd(), "fixture.example.test")
	for _, want := range []string{"node", "https://example.test/fixture.git", "release", "2026-01-02T03:04:05Z"} {
		assert.Contains(t, output, want)
	}
	assert.NotContains(t, output, "disabled")
}

func TestKeysAddUsesAPIContractAndJSON(t *testing.T) {
	const publicKey = "ssh-ed25519 fixture-public-key"
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, client.PathSSHKeys, r.URL.Path)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"name":"fixture","publicKey":"ssh-ed25519 fixture-public-key"}`, string(body))
		writeContractResponse(w, map[string]string{"id": "key-1", "name": "fixture"})
	})
	keyPath := filepath.Join(t.TempDir(), "fixture.pub")
	require.NoError(t, os.WriteFile(keyPath, []byte(publicKey+"\n"), 0600))
	setJSONOutput()
	output := executeContractCommand(t, newKeysAddCmd(), "--name", "fixture", "--file", keyPath)
	assert.JSONEq(t, `{"id":"key-1","name":"fixture"}`, output)
}

func TestDatabaseTypeAliasesUseBackendValues(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"mysql", "mysql8"}, {"postgres", "postgres16"}, {"mysql8", "mysql8"},
		{"postgres16", "postgres16"}, {"postgres15", "postgres15"}, {"mariadb", "mariadb"},
		{"mongodb7", "mongodb7"}, {"redis7", "redis7"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, client.PathDatabases("srv-1"), r.URL.Path)
				var body map[string]string
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, map[string]string{"name": "fixture", "type": tt.want}, body)
				writeContractResponse(w, map[string]string{
					"id": "db-1", "name": "fixture", "type": tt.want, "status": "creating", "createdAt": "2026-01-02T03:04:05Z",
				})
			})
			setJSONOutput()
			output := executeContractCommand(t, newDBCreateCmd(), "--name", "fixture", "--type", tt.input)
			var body map[string]string
			require.NoError(t, json.Unmarshal([]byte(output), &body))
			assert.Equal(t, tt.want, body["type"])
			assert.Equal(t, "2026-01-02T03:04:05Z", body["created_at"])
			assert.NotContains(t, body, "createdAt")
		})
	}
}

func TestAPIKeyExpiry(t *testing.T) {
	now := time.Date(2026, time.September, 16, 12, 30, 0, 0, time.FixedZone("fixture", 3*60*60))
	for _, value := range []string{"1d", "30d", "90d"} {
		t.Run(value, func(t *testing.T) {
			actual, err := apiKeyExpiry(value, now)
			require.NoError(t, err)
			parsed, err := time.Parse(time.RFC3339, actual)
			require.NoError(t, err)
			assert.True(t, parsed.After(now))
			assert.Equal(t, time.UTC, parsed.Location())
		})
	}
	actual, err := apiKeyExpiry("90d", now)
	require.NoError(t, err)
	assert.Equal(t, now.Add(90*24*time.Hour).UTC().Format(time.RFC3339), actual)
	for _, value := range []string{"", "0d", "-1d", "1.5d", "90", "garbage", "999999999999999999999d", "106752d"} {
		t.Run("invalid_"+value, func(t *testing.T) {
			_, err := apiKeyExpiry(value, now)
			require.Error(t, err)
		})
	}
}

func TestAPIKeyCreationSendsAbsoluteExpiry(t *testing.T) {
	before := time.Now().UTC().Add(90 * 24 * time.Hour).Truncate(time.Second)
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, client.PathAPIKeys, r.URL.Path)
		var body map[string]string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.NotContains(t, body, "expires_at")
		expiry, err := time.Parse(time.RFC3339, body["expiresAt"])
		assert.NoError(t, err)
		assert.False(t, expiry.Before(before))
		assert.False(t, expiry.After(time.Now().UTC().Add(90*24*time.Hour)))
		writeContractResponse(w, map[string]any{
			"id": "key-1", "name": body["name"], "keyPrefix": "fixture_", "key": "fixture-secret",
			"lastUsedAt": nil, "expiresAt": body["expiresAt"], "createdAt": "2026-09-16T09:30:00Z",
		})
	})
	setJSONOutput()
	output := executeContractCommand(t, newAPIKeysCreateCmd(), "--name", "fixture", "--expires", "90d")
	var created map[string]string
	require.NoError(t, json.Unmarshal([]byte(output), &created))
	assert.Equal(t, "fixture-secret", created["key"])
	assert.Equal(t, "fixture_", created["key_prefix"])
	assert.NotEmpty(t, created["expires_at"])
	assert.Empty(t, created["last_used_at"])
	assert.Equal(t, "2026-09-16T09:30:00Z", created["created_at"])
	assert.NotContains(t, created, "expiresAt")
}

func TestAPIKeyCreationRejectsInvalidExpiryBeforeRequest(t *testing.T) {
	for _, value := range []string{"", "0d", "-1d", "overflow", "106752d"} {
		t.Run("invalid_"+value, func(t *testing.T) {
			var requests atomic.Int64
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				writeContractResponse(w, nil)
			})
			cmd := newAPIKeysCreateCmd()
			cmd.SetArgs([]string{"--name", "fixture", "--expires=" + value})
			require.ErrorContains(t, cmd.Execute(), "invalid --expires")
			assert.Zero(t, requests.Load())
		})
	}
}

func TestAPIKeyCreationWithoutExpiryOmitsField(t *testing.T) {
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"name": "fixture"}, body)
		writeContractResponse(w, map[string]string{"id": "key-1", "name": "fixture", "key": "fixture-secret"})
	})
	setJSONOutput()
	executeContractCommand(t, newAPIKeysCreateCmd(), "--name", "fixture")
}

func TestDeployJSONUsesAPIContractWithoutPreamble(t *testing.T) {
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, client.PathSiteDeploy("srv-1", "site-1"), r.URL.Path)
		var body map[string]string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"branch": "release"}, body)
		writeContractResponse(w, map[string]string{
			"id": "deploy-1", "status": "queued", "commitHash": "fixture-hash",
			"trigger": "manual", "createdAt": "2026-01-02T03:04:05Z",
		})
	})
	linkContractProject(t)
	setJSONOutput()
	output := executeContractCommand(t, newDeployCmd(), "--branch", "release")
	assert.JSONEq(t, `{"id":"deploy-1","status":"queued","commit_sha":"fixture-hash","trigger":"manual","created_at":"2026-01-02T03:04:05Z"}`, output)
}

func TestLogoutClearsStoredAuthentication(t *testing.T) {
	for _, useKeychain := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "keychain"}[useKeychain], func(t *testing.T) {
			t.Cleanup(setupTest(t, newMockAPI()))
			keyring.MockInit()
			t.Cleanup(keyring.MockInit)
			t.Setenv("DCS_CONFIG_DIR", t.TempDir())
			t.Setenv("DCS_API_KEY", "")
			t.Setenv("DCS_API_URL", "")
			cfg.UseKeychain = useKeychain
			cfg.DefaultServerID = "srv-1"
			require.NoError(t, cfg.Save())
			require.True(t, config.Load().IsAuthenticated())
			require.NoError(t, clearStoredAuthentication())
			loaded := config.Load()
			assert.False(t, loaded.IsAuthenticated())
			assert.Nil(t, loaded.User)
			assert.Nil(t, apiClient)
			assert.Equal(t, "srv-1", loaded.DefaultServerID)
			assert.Equal(t, cfg.APIURL, loaded.APIURL)
			assert.Equal(t, useKeychain, loaded.UseKeychain)
			_, err := config.LoadFromKeychain()
			assert.ErrorIs(t, err, keyring.ErrNotFound)
		})
	}
}

func TestLogoutMissingKeychainEntryIsAlreadyCleared(t *testing.T) {
	t.Cleanup(setupTest(t, newMockAPI()))
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	t.Setenv("DCS_CONFIG_DIR", t.TempDir())
	t.Setenv("DCS_API_KEY", "")
	cfg.UseKeychain = true
	require.NoError(t, clearStoredAuthentication())
	assert.False(t, config.Load().IsAuthenticated())
}

func TestLogoutKeychainFailurePreservesConfig(t *testing.T) {
	t.Cleanup(setupTest(t, newMockAPI()))
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	t.Setenv("DCS_CONFIG_DIR", t.TempDir())
	cfg.UseKeychain = true
	require.NoError(t, cfg.Save())
	before, err := os.ReadFile(config.FilePath())
	require.NoError(t, err)
	keyring.MockInitWithError(errors.New("fixture locked keychain"))
	require.ErrorContains(t, clearStoredAuthentication(), "remove keychain credentials")
	after, err := os.ReadFile(config.FilePath())
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.True(t, cfg.IsAuthenticated())
	assert.NotNil(t, apiClient)
}

func TestAPIKeyListDisplaysCurrentResponseFields(t *testing.T) {
	api := newMockAPI()
	wire := map[string]any{
		"id": "key-1", "name": "fixture", "keyPrefix": "fixture_",
		"lastUsedAt": "2026-09-15T09:30:00Z", "expiresAt": "2026-12-15T09:30:00Z",
		"createdAt": "2026-09-14T09:30:00Z",
	}
	data, err := json.Marshal(wire)
	require.NoError(t, err)
	var decoded APIKey
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "2026-09-15T09:30:00Z", decoded.LastUsedAt)
	assert.Equal(t, "2026-12-15T09:30:00Z", decoded.ExpiresAt)
	assert.Equal(t, "2026-09-14T09:30:00Z", decoded.CreatedAt)
	api.on(http.MethodGet, client.PathAPIKeys, http.StatusOK, []any{wire})
	t.Cleanup(setupTest(t, api))
	output := executeContractCommand(t, newAPIKeysListCmd())
	for _, want := range []string{"fixture_", "2026-", "2026-12-15", "2026-09-14T09:30:00Z"} {
		assert.Contains(t, output, want)
	}
	assert.NotContains(t, output, "never")
}

func TestDatabaseBackupCurrentResponsePreservesCLIJSON(t *testing.T) {
	var backup DBBackup
	require.NoError(t, json.Unmarshal([]byte(`{"id":"backup-1","databaseId":"db-1","name":"fixture","type":"manual","status":"completed","sizeBytes":2048,"completedAt":"2026-09-16T09:35:00Z","createdAt":"2026-09-16T09:30:00Z"}`), &backup))
	assert.Equal(t, DBBackup{
		ID: "backup-1", DatabaseID: "db-1", Name: "fixture", Type: "manual", Status: "completed",
		SizeBytes: 2048, CompletedAt: "2026-09-16T09:35:00Z", CreatedAt: "2026-09-16T09:30:00Z",
	}, backup)
	output, err := json.Marshal(backup)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"backup-1","database_id":"db-1","name":"fixture","type":"manual","status":"completed","size_bytes":2048,"completed_at":"2026-09-16T09:35:00Z","created_at":"2026-09-16T09:30:00Z"}`, string(output))
	var legacy DBBackup
	require.NoError(t, json.Unmarshal(output, &legacy))
	assert.Equal(t, backup, legacy)
}

func TestDatabaseBackupListDisplaysCurrentResponseFields(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathDatabases("srv-1"), http.StatusOK, []any{map[string]string{
		"id": "db-1", "name": "fixture", "type": "postgres16", "status": "active", "createdAt": "2026-09-16T09:30:00Z",
	}})
	api.on(http.MethodGet, client.PathDatabaseBackups("srv-1", "db-1"), http.StatusOK, []any{map[string]any{
		"id": "backup-1", "databaseId": "db-1", "name": "fixture", "type": "manual", "status": "completed",
		"sizeBytes": 2048, "completedAt": "2026-09-16T09:35:00Z", "createdAt": "2026-09-16T09:30:00Z",
	}})
	t.Cleanup(setupTest(t, api))
	cfg.DefaultServerID = "srv-1"
	output := executeContractCommand(t, newDBBackupsCmd(), "fixture")
	assert.Contains(t, output, formatBytes(2048))
	assert.Contains(t, output, "2026-09-16T09:35:00Z")
	assert.Contains(t, output, "2026-09-16T09:30:00Z")
}

func TestSiteCurrentFalseOverridesLegacyTrue(t *testing.T) {
	var site Site
	require.NoError(t, json.Unmarshal([]byte(`{"ssl_enabled":true,"sslEnabled":false}`), &site))
	assert.False(t, site.SSLEnabled)
}

func TestLogoutSaveFailureDoesNotReportSuccess(t *testing.T) {
	t.Cleanup(setupTest(t, newMockAPI()))
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("fixture"), 0600))
	t.Setenv("DCS_CONFIG_DIR", path)
	require.ErrorContains(t, clearStoredAuthentication(), "save logged-out config")
	assert.True(t, cfg.IsAuthenticated())
	assert.NotNil(t, apiClient)
}

func TestLogoutWithUnreadableKeychainStillRequiresConfirmation(t *testing.T) {
	t.Cleanup(setupTest(t, newMockAPI()))
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
	t.Setenv("DCS_CONFIG_DIR", t.TempDir())
	t.Setenv("DCS_API_KEY", "")
	cfg.UseKeychain = true
	require.NoError(t, cfg.Save())
	keyring.MockInitWithError(errors.New("fixture locked keychain"))
	cfg = config.Load()
	require.Empty(t, cfg.APIKey)
	require.True(t, cfg.UseKeychain)
	cmd := newLogoutCmd()
	var runErr error
	output := captureStdout(t, func() { runErr = cmd.RunE(cmd, nil) })
	require.ErrorIs(t, runErr, ui.ErrNonInteractive)
	assert.NotContains(t, output, "Not currently authenticated")
	assert.NotContains(t, output, "Logged out successfully")
}
