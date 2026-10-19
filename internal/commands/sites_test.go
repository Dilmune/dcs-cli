package commands

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func registerServersAndSites(api *mockAPI) {
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathSites("srv-1"), http.StatusOK, sampleSites())
}

func TestSitesList(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "app.example.com")
	assert.Contains(t, output, "docs.example.com")
}

func TestSitesListJSON(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newSitesListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "app.example.com")
	assert.Contains(t, output, "site-1")
	assert.Contains(t, output, "site-2")
}

func TestSitesListEmpty(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathSites("srv-1"), http.StatusOK, []Site{})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesListCmd()
	cmd.SetArgs([]string{"--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No sites yet")
}

func TestSitesInfo(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)

	site := sampleSites()[0]
	api.on(http.MethodGet, client.PathSite("srv-1", "site-1"), http.StatusOK, site)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesInfoCmd()
	cmd.SetArgs([]string{"app.example.com", "--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "app.example.com")
	assert.Contains(t, output, "node")
	assert.Contains(t, output, "deployed")
}

func TestSitesDeleteForce(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)
	api.on(http.MethodDelete, client.PathSite("srv-1", "site-1"), http.StatusOK, nil)
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesDeleteCmd()
	cmd.SetArgs([]string{"app.example.com", "--server", "prod-web", "--force"})

	err := cmd.Execute()
	require.NoError(t, err)
}

func TestSitesDeploy(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)
	api.on(http.MethodPost, client.PathSiteDeploy("srv-1", "site-1"), http.StatusOK, Deployment{
		ID:      "dep-1",
		Status:  "building",
		Trigger: "cli",
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesDeployCmd()
	cmd.SetArgs([]string{"app.example.com", "--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "Deployment triggered")
}

func TestSitesSSLAlreadyEnabled(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathServers, http.StatusOK, sampleServers())
	api.on(http.MethodGet, client.PathSites("srv-1"), http.StatusOK, sampleSites())
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newSitesSSLCmd()
	cmd.SetArgs([]string{"app.example.com", "--server", "prod-web"})

	output := captureStdout(t, func() {
		err := cmd.Execute()
		require.NoError(t, err)
	})

	assert.Contains(t, output, "already enabled")
}

// Two deployments share their first 8 characters, the short form the
// deployments table prints; the third is unique on its short form.
func sampleDeployments() []map[string]string {
	return []map[string]string{
		{"id": "a1b2c3d4-0000-4000-8000-000000000001", "status": "completed"},
		{"id": "a1b2c3d4-0000-4000-8000-000000000002", "status": "completed"},
		{"id": "5e1f0000-aaaa-4000-8000-000000000003", "status": "completed"},
	}
}

func TestSitesRollback(t *testing.T) {
	const unique = "5e1f0000-aaaa-4000-8000-000000000003"
	tests := []struct {
		name         string
		deploymentID string
		force        bool
		wantTarget   string
		wantErr      error
		wantMessage  string
		wantListed   bool
	}{
		{name: "the short ID from the table reaches the full ID", deploymentID: "5e1f0000", force: true, wantTarget: unique, wantListed: true},
		{name: "a full ID", deploymentID: "a1b2c3d4-0000-4000-8000-000000000002", force: true, wantTarget: "a1b2c3d4-0000-4000-8000-000000000002", wantListed: true},
		{
			name: "a short ID two deployments share is ambiguous", deploymentID: "a1b2c3d4", force: true, wantErr: errLookupAmbiguous, wantListed: true,
			wantMessage: `Deployment "a1b2c3d4" is ambiguous: it matches a1b2c3d4-0000-4000-8000-000000000001 and a1b2c3d4-0000-4000-8000-000000000002.`,
		},
		{name: "fewer than 8 characters is not a lookup", deploymentID: "5e1f000", force: true, wantErr: errLookupNotFound, wantMessage: "at least 8 characters", wantListed: true},
		{name: "an unknown ID", deploymentID: "ffffffff", force: true, wantErr: errLookupNotFound, wantMessage: "dcs sites deployments app.example.com", wantListed: true},
		{name: "a blank ID is refused before any lookup", deploymentID: " ", force: true, wantErr: errLookupInputRequired},
		{name: "without --force a script is told the flag", deploymentID: "5e1f0000", wantMessage: "Pass --force to skip the confirmation.", wantListed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rollbacks []string
			var listed atomic.Int32
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == client.PathServers:
					writeContractResponse(w, sampleServers())
				case r.Method == http.MethodGet && r.URL.Path == client.PathSites("srv-1"):
					writeContractResponse(w, sampleSites())
				case r.Method == http.MethodGet && r.URL.Path == client.PathSiteDeployments("srv-1", "site-1"):
					listed.Add(1)
					writeContractResponse(w, sampleDeployments())
				case r.Method == http.MethodPost:
					rollbacks = append(rollbacks, r.URL.Path)
					writeContractResponse(w, map[string]string{"id": "dep-new", "status": "queued"})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			cmd := newSitesRollbackCmd()
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			args := []string{"app.example.com", tt.deploymentID, "--server", "srv-1"}
			if tt.force {
				args = append(args, "--force")
			}
			cmd.SetArgs(args)
			var err error
			captureStderr(t, func() {
				captureStdout(t, func() { err = cmd.Execute() })
			})
			switch {
			case tt.wantTarget != "":
				require.NoError(t, err)
				assert.Equal(t, []string{client.PathDeploymentRollback("srv-1", "site-1", tt.wantTarget)}, rollbacks)
			default:
				require.Error(t, err)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				}
				assert.Empty(t, rollbacks, "no rollback may be requested")
			}
			if tt.wantMessage != "" {
				assert.Contains(t, err.Error(), tt.wantMessage)
			}
			assert.Equal(t, tt.wantListed, listed.Load() > 0)
		})
	}
}

// The deployment lookup reads every page, so a short ID for an older
// deployment, past the first hundred, still resolves.
func TestSitesLogsResolvesAShortIDOnALaterPage(t *testing.T) {
	deployments := make([]Deployment, 2*lookupPageSize+1)
	for i := range deployments {
		deployments[i] = Deployment{ID: fmt.Sprintf("%08x-0000-4000-8000-000000000000", i)}
	}
	target := deployments[lookupPageSize+7].ID
	var queries, logReads []string
	pages := servePages(t, deployments, perPageQuery, &queries)
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case client.PathServers:
			writeContractResponse(w, sampleServers())
		case client.PathSites("srv-1"):
			writeContractResponse(w, sampleSites())
		case client.PathSiteDeployments("srv-1", "site-1"):
			pages(w, r)
		default:
			logReads = append(logReads, r.URL.Path)
			writeContractResponse(w, map[string]any{"lines": []map[string]string{{"timestamp": "t", "line": "built"}}})
		}
	})

	out := executeContractCommand(t, newSitesLogsCmd(), "app.example.com", target[:shortIDLength], "--server", "srv-1")
	assert.Contains(t, out, "built")
	assert.Equal(t, []string{client.PathDeploymentLogs("srv-1", "site-1", target)}, logReads)
	assert.Len(t, queries, 3, "every page is read")
}

func TestSitesDeploymentsShortIDsDoNotPanic(t *testing.T) {
	api := newMockAPI()
	registerServersAndSites(api)
	api.on(http.MethodGet, client.PathSiteDeployments("srv-1", "site-1"), http.StatusOK, []map[string]string{
		{"id": "d1", "status": "completed", "commitHash": "abc", "trigger": "manual"},
		{"id": "0123456789abcdef", "status": "failed", "commitHash": "0123456789", "trigger": "push"},
	})
	t.Cleanup(setupTest(t, api))

	out := executeContractCommand(t, newSitesDeploymentsCmd(), "app.example.com", "--server", "srv-1")
	assert.Contains(t, out, "d1 ")
	assert.Contains(t, out, "01234567 ")
	assert.NotContains(t, out, "0123456789abcdef")
}
