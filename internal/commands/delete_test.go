package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func TestDeleteCommandOutput(t *testing.T) {
	resources := []struct {
		name, id, listPath, deletePath, success string
		command                                 func() *cobra.Command
		args                                    []string
		listing                                 any
	}{
		{"server", "srv-1", client.PathServers, client.PathServer("srv-1"), "Server prod-web deleted.", newServersDeleteCmd, []string{"prod-web"}, sampleServers()},
		{"site", "site-1", client.PathSites("srv-1"), client.PathSite("srv-1", "site-1"), "Site app.example.com deleted.", newSitesDeleteCmd, []string{"app.example.com", "--server", "srv-1"}, sampleSites()},
		{"database", "db-1", client.PathDatabases("srv-1"), client.PathDatabase("srv-1", "db-1"), "Database myapp_prod deleted.", newDBDeleteCmd, []string{"myapp_prod", "--server", "srv-1"}, sampleDatabases()},
		{"SSH key", "key-1", client.PathSSHKeys, client.PathSSHKey("key-1"), "SSH key fixture removed.", newKeysRmCmd, []string{"fixture"}, []map[string]string{{"id": "key-other", "name": "other"}, {"id": "key-1", "name": "fixture"}}},
		{"API key", "key-1", client.PathAPIKeys, client.PathAPIKey("key-1"), "API key fixture deleted.", newAPIKeysDeleteCmd, []string{"fixture"}, []APIKey{{ID: "key-other", Name: "other"}, {ID: "key-1", Name: "fixture"}}},
	}
	modes := []struct {
		name                                        string
		json, quiet, force, lookupFailure, canceled bool
		unconfirmedResponse                         bool
		status                                      int
		wantDeletes                                 int32
		wantError                                   string
	}{
		{name: "JSON success", json: true, force: true, status: http.StatusOK, wantDeletes: 1},
		{name: "JSON accepted", json: true, force: true, status: http.StatusAccepted, wantDeletes: 1},
		{name: "JSON missing envelope", json: true, force: true, status: http.StatusNoContent, wantDeletes: 1, wantError: "parse response"},
		{name: "JSON API did not confirm", json: true, force: true, unconfirmedResponse: true, status: http.StatusOK, wantDeletes: 1, wantError: "API did not confirm"},
		{name: "JSON quiet", json: true, quiet: true, force: true, status: http.StatusOK, wantDeletes: 1},
		{name: "JSON rejected", json: true, force: true, status: http.StatusForbidden, wantDeletes: 1, wantError: "Permission denied"},
		{name: "JSON requires force", json: true, status: http.StatusOK, wantError: "requires --force"},
		{name: "JSON lookup failed", json: true, force: true, lookupFailure: true, status: http.StatusOK, wantError: "Permission denied"},
		{name: "JSON canceled", json: true, force: true, canceled: true, status: http.StatusOK, wantError: "context canceled"},
		{name: "human success", force: true, status: http.StatusOK, wantDeletes: 1},
		{name: "human quiet", quiet: true, force: true, status: http.StatusOK, wantDeletes: 1},
		{name: "human rejected", force: true, status: http.StatusForbidden, wantDeletes: 1, wantError: "Permission denied"},
		{name: "human requires confirmation", status: http.StatusOK, wantError: "interactive input"},
	}
	for _, resource := range resources {
		t.Run(resource.name, func(t *testing.T) {
			for _, mode := range modes {
				t.Run(mode.name, func(t *testing.T) {
					var deletes atomic.Int32
					setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
						assert.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
						switch {
						case r.Method == http.MethodGet && (r.URL.Path == resource.listPath || r.URL.Path == client.PathServers):
							if mode.lookupFailure {
								w.WriteHeader(http.StatusForbidden)
								_, _ = w.Write([]byte(`{"success":false,"error":{"message":"fixture denied","code":"forbidden"}}`))
								return
							}
							if r.URL.Path == resource.listPath {
								writeContractResponse(w, resource.listing)
							} else {
								writeContractResponse(w, sampleServers())
							}
						case r.Method == http.MethodDelete && r.URL.Path == resource.deletePath:
							deletes.Add(1)
							assert.Empty(t, r.URL.RawQuery)
							w.WriteHeader(mode.status)
							if mode.status == http.StatusForbidden {
								_, _ = w.Write([]byte(`{"success":false,"error":{"message":"fixture denied","code":"forbidden"}}`))
							} else if mode.unconfirmedResponse {
								_, _ = w.Write([]byte(`{"success":false}`))
							} else if mode.status != http.StatusNoContent {
								writeContractResponse(w, nil)
							}
						default:
							t.Errorf("unexpected request: %s %s", r.Method, r.URL)
							w.WriteHeader(http.StatusNotFound)
						}
					})
					if mode.json {
						setJSONOutput()
					}
					ui.SetQuiet(mode.quiet)
					t.Cleanup(func() { ui.SetQuiet(false) })
					cmd := resource.command()
					cmd.SilenceUsage, cmd.SilenceErrors = true, true
					args := append([]string{}, resource.args...)
					if mode.force {
						args = append(args, "--force")
					}
					cmd.SetArgs(args)
					if mode.canceled {
						ctx, cancel := context.WithCancel(context.Background())
						cancel()
						cmd.SetContext(ctx)
					}
					var runErr error
					var stdout string
					stderr := captureStderr(t, func() {
						stdout = captureStdout(t, func() { runErr = cmd.Execute() })
					})
					assert.Equal(t, mode.wantDeletes, deletes.Load())
					if mode.wantError != "" {
						require.ErrorContains(t, runErr, mode.wantError)
						assert.Empty(t, stdout)
					} else {
						require.NoError(t, runErr)
						if mode.json {
							want, err := json.Marshal(map[string]any{"id": resource.id, "success": true})
							require.NoError(t, err)
							assert.JSONEq(t, string(want), stdout)
						} else if mode.quiet {
							assert.Empty(t, stdout)
						} else {
							assert.Contains(t, stdout, resource.success)
							assert.False(t, json.Valid([]byte(stdout)))
						}
					}
					if mode.json {
						assert.Empty(t, stderr, "JSON mode must not run the spinner")
					}
				})
			}
		})
	}
}
