package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

// In the first three, two IDs start with "a", which
// is how `dcs servers info a` used to answer with the first of them.
func lookupFixtureServers() []Server {
	return []Server{
		{ID: "a1c2e3f4-1111-4111-8111-111111111111", Name: "api-core"},
		{ID: "a7b8c9d0-2222-4222-8222-222222222222", Name: "worker-two"},
		{ID: "3c4d5e6f-3333-4333-8333-333333333333", Name: "db-main"},
		{ID: "5e1f0000-0000-4000-8000-000000000001", Name: "web"},
		{ID: "5e1f0000-0000-4000-8000-000000000002", Name: "WEB"},
		{ID: "9a9a9a9a-0000-4000-8000-000000000003", Name: "3c4d5e6f"},
		{ID: "srv-9", Name: "legacy"},
	}
}

func TestLookupPick(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantID       string
		wantErr      error
		wantMessage  string
		wantContains []string
		wantMissing  []string
	}{
		{name: "empty input is refused", input: "", wantErr: errLookupInputRequired, wantMessage: "A server name or ID is required."},
		{name: "whitespace input is refused", input: " \t\n", wantErr: errLookupInputRequired, wantMessage: "A server name or ID is required."},
		{name: "exact name", input: "api-core", wantID: "a1c2e3f4-1111-4111-8111-111111111111"},
		{name: "name ignores case", input: "Worker-Two", wantID: "a7b8c9d0-2222-4222-8222-222222222222"},
		{name: "name wins over an ID prefix", input: "3c4d5e6f", wantID: "9a9a9a9a-0000-4000-8000-000000000003"},
		{name: "exact ID", input: "3c4d5e6f-3333-4333-8333-333333333333", wantID: "3c4d5e6f-3333-4333-8333-333333333333"},
		{name: "exact ID shorter than a usable prefix", input: "srv-9", wantID: "srv-9"},
		{name: "eight character prefix", input: "a7b8c9d0", wantID: "a7b8c9d0-2222-4222-8222-222222222222"},
		{name: "longer prefix", input: "a1c2e3f4-1111", wantID: "a1c2e3f4-1111-4111-8111-111111111111"},
		{
			name: "two names that differ only in case are ambiguous", input: "web", wantErr: errLookupAmbiguous,
			wantContains: []string{`Server "web" is ambiguous`, "web (5e1f0000-0000-4000-8000-000000000001)", "WEB (5e1f0000-0000-4000-8000-000000000002)", "full ID"},
		},
		{
			name: "prefix shared by two IDs is ambiguous", input: "5e1f0000", wantErr: errLookupAmbiguous,
			wantContains: []string{"5e1f0000-0000-4000-8000-000000000001", "5e1f0000-0000-4000-8000-000000000002"},
		},
		{
			name: "one letter matching two IDs is not a lookup", input: "a", wantErr: errLookupNotFound,
			wantMessage: `Server "a" not found.`, wantContains: []string{"at least 8 characters"},
		},
		{
			name: "seven character prefix of one ID is not a lookup", input: "a7b8c9d", wantErr: errLookupNotFound,
			wantContains: []string{"at least 8 characters"},
		},
		{
			name: "unknown input", input: "nope", wantErr: errLookupNotFound,
			wantContains: []string{`Server "nope" not found.`, "dcs servers list"}, wantMissing: []string{"8 characters"},
		},
		{name: "text inside an ID is not a prefix", input: "b51a-67b4", wantErr: errLookupNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := serverLookup.pick(tt.input, lookupFixtureServers())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, found)
				var cliErr *client.CLIError
				require.ErrorAs(t, err, &cliErr)
				if tt.wantMessage != "" {
					assert.Equal(t, tt.wantMessage, cliErr.Message)
				}
				for _, want := range tt.wantContains {
					assert.Contains(t, err.Error(), want)
				}
				for _, missing := range tt.wantMissing {
					assert.NotContains(t, err.Error(), missing)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, found.ID)
		})
	}
}

func TestLookupAmbiguityNamesAtMostFiveCandidates(t *testing.T) {
	servers := make([]Server, 7)
	for i := range servers {
		servers[i] = Server{ID: fmt.Sprintf("0000000%d-0000-4000-8000-000000000000", i), Name: "twin"}
	}
	_, err := serverLookup.pick("twin", servers)
	require.ErrorIs(t, err, errLookupAmbiguous)
	assert.Contains(t, err.Error(), "twin (00000004-0000-4000-8000-000000000000) and 2 more.")
	assert.NotContains(t, err.Error(), "00000005")
}

func TestLookupFindRefusesBlankInputBeforeListing(t *testing.T) {
	listed := false
	_, err := serverLookup.find(t.Context(), "  ", func(ctx context.Context) ([]Server, error) {
		listed = true
		return lookupFixtureServers(), nil
	})
	require.ErrorIs(t, err, errLookupInputRequired)
	assert.False(t, listed)
}

// servePages answers a list endpoint in pages of the requested size, the way
// the API's pagination does, and records every query it was sent.
func servePages[T any](t *testing.T, items []T, perPageKey string, queries *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*queries = append(*queries, r.URL.RawQuery)
		page, err := strconv.Atoi(r.URL.Query().Get(pageQuery))
		require.NoError(t, err)
		perPage, err := strconv.Atoi(r.URL.Query().Get(perPageKey))
		require.NoError(t, err)
		start := min((page-1)*perPage, len(items))
		end := min(start+perPage, len(items))
		totalPages := (len(items) + perPage - 1) / perPage
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    items[start:end],
			"meta":    map[string]int{"page": page, "perPage": perPage, "total": len(items), "totalPages": totalPages},
		}))
	}
}

func TestResolveServerReadsEveryPage(t *testing.T) {
	servers := make([]Server, 2*lookupPageSize+1)
	for i := range servers {
		servers[i] = Server{ID: fmt.Sprintf("%08d-0000-4000-8000-000000000000", i), Name: fmt.Sprintf("server-%d", i)}
	}
	servers[len(servers)-1].Name = "server-0"
	var queries []string
	setupContractAPI(t, servePages(t, servers, perPageQuery, &queries))

	found, err := resolveServer("server-200")
	require.Error(t, err, "a name on the last page must still be seen")
	require.ErrorIs(t, err, errLookupNotFound)
	assert.Nil(t, found)

	found, err = resolveServer(servers[150].Name)
	require.NoError(t, err)
	assert.Equal(t, servers[150].ID, found.ID)

	_, err = resolveServer("server-0")
	require.ErrorIs(t, err, errLookupAmbiguous, "a duplicate on a later page must make the name ambiguous")
	assert.Equal(t, []string{"page=1&perPage=100", "page=2&perPage=100", "page=3&perPage=100"}, queries[len(queries)-3:])
}

func TestResolveStorageFileFollowsPageCountPastAnEmptyPage(t *testing.T) {
	var queries []string
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		files := []storageFile{}
		if r.URL.Query().Get(pageQuery) == "2" {
			files = []storageFile{{ID: "f11e0000-0000-4000-8000-000000000001", Name: "logo.png"}}
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    storageListing{Files: files},
			"meta":    map[string]int{"page": 1, "perPage": lookupPageSize, "total": 101, "totalPages": 2},
		}))
	})

	found, err := resolveStorageFile("logo.png")
	require.NoError(t, err)
	assert.Equal(t, "f11e0000-0000-4000-8000-000000000001", found.ID)
	assert.Equal(t, []string{"page=1&per_page=100", "page=2&per_page=100"}, queries)
}

// Every command in the real tree that takes --server is checked, so a command
// added later with a plain string flag fails here instead of shipping.
func TestServerFlagRefusesABlankValue(t *testing.T) {
	var paths [][]string
	var walk func(cmd *cobra.Command, path []string)
	walk = func(cmd *cobra.Command, path []string) {
		if cmd.Flags().Lookup(serverFlagName) != nil {
			paths = append(paths, path)
		}
		for _, child := range cmd.Commands() {
			walk(child, append(append([]string{}, path...), child.Name()))
		}
	}
	walk(NewRootCmd(), nil)
	require.GreaterOrEqual(t, len(paths), 38, "the walk must reach every --server command")
	assert.Contains(t, paths, []string{"sites", "delete"})

	for _, path := range paths {
		for _, value := range []string{"", "   "} {
			t.Run(fmt.Sprintf("%v %q", path, value), func(t *testing.T) {
				// Executing through the root runs its hooks the moment the
				// guard stops firing. TestMain gives them an empty
				// configuration, which leaves the fake client below in place.
				var requests atomic.Int32
				setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					writeContractResponse(w, nil)
				})
				root := NewRootCmd()
				root.SetArgs(append(append([]string{}, path...), "--server", value))
				var err error
				captureStderr(t, func() {
					captureStdout(t, func() { err = root.Execute() })
				})
				require.ErrorIs(t, err, errLookupInputRequired)
				assert.Equal(t, fmt.Sprintf(`invalid argument %q for "--server" flag: server name or ID must not be empty`, value), err.Error())
				assert.Zero(t, requests.Load(), "a blank --server must not fall back to the linked or default server")
			})
		}
	}
}

func TestServerFlagHelpIsUnchanged(t *testing.T) {
	cmd := newSitesDeleteCmd()
	assert.Contains(t, cmd.Flags().FlagUsages(), "--server string   Server name or ID\n")
}
