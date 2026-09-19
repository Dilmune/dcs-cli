package commands

import (
	"net/http"
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
