package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUISourceReadsVerifiedAPIShapesAndPaginates(t *testing.T) {
	serverData := map[string]any{"id": "srv-1", "name": "Fixture server", "status": "active", "provider": "test-provider", "region": "test-region", "sizeLabel": "Fixture size", "ipv4": "192.0.2.1", "createdAt": "2026-01-01T00:00:00Z"}
	siteData := map[string]any{"id": "site-1", "domain": "fixture.example.com", "projectType": "node", "status": "active", "sslEnabled": true, "gitBranch": "main", "createdAt": "2026-01-01T00:00:00Z", "deployHookToken": "DO-NOT-DISPLAY"}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "the workspace must only issue GETs")
		response := map[string]any{"success": true}
		switch r.URL.Path {
		case client.PathAuthMe:
			response["data"] = map[string]any{"user": map[string]string{"id": "user-1", "email": "fixture@example.com", "name": "Fixture"}}
		case client.PathServers:
			assert.Equal(t, "25", r.URL.Query().Get("perPage"))
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			response["data"] = []any{serverData}
			response["meta"] = map[string]int{"page": 1, "perPage": 25, "total": 26, "totalPages": 2}
		case client.PathServer("srv-1"):
			response["data"] = serverData
		case client.PathSites("srv-1"):
			response["data"] = []any{siteData}
			response["meta"] = map[string]int{"page": 1, "perPage": 25, "total": 1, "totalPages": 1}
		case client.PathSite("srv-1", "site-1"):
			response["data"] = siteData
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	source := &uiSource{api: client.New(api.URL, "fixture-key")}
	identity, err := source.Identify(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Fixture · fixture@example.com", identity)
	for _, kind := range []workspace.Kind{workspace.Servers, workspace.SiteServers, workspace.Sites, workspace.ServerDetail, workspace.SiteDetail} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			req := workspace.Request{Kind: kind, ServerID: "srv-1", SiteID: "site-1"}
			item, err := source.Load(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, &req, item.Request)
			assert.NotContains(t, fmt.Sprint(item), "DO-NOT-DISPLAY")
			switch kind {
			case workspace.Servers, workspace.SiteServers:
				require.Len(t, item.Children, 2)
				assert.Equal(t, "Next page", item.Children[1].Title)
				assert.Equal(t, 2, item.Children[1].Request.Page)
				assert.Equal(t, "Fixture size", fieldValue(item.Children[0].Fields, "Size"))
				if kind == workspace.SiteServers {
					assert.Equal(t, workspace.Sites, item.Children[0].Request.Kind)
				}
			case workspace.Sites:
				require.Len(t, item.Children, 1)
				assert.Equal(t, "node", fieldValue(item.Children[0].Fields, "Type"))
				assert.Equal(t, "true", fieldValue(item.Children[0].Fields, "SSL enabled"))
				assert.Equal(t, "main", fieldValue(item.Children[0].Fields, "Git branch"))
			case workspace.ServerDetail:
				assert.Equal(t, "2026-01-01T00:00:00Z", fieldValue(item.Fields, "Created"))
			case workspace.SiteDetail:
				assert.Equal(t, "dcs sites info --server 'srv-1' -- 'fixture.example.com'", item.Command)
			}
		})
	}
}

func fieldValue(fields []workspace.Field, label string) string {
	for _, field := range fields {
		if field.Label == label {
			return field.Value
		}
	}
	return ""
}

func TestUISourceAuthenticationAndSafeErrors(t *testing.T) {
	source := &uiSource{}
	_, err := source.Load(context.Background(), workspace.Request{Kind: workspace.Servers})
	assert.ErrorIs(t, err, client.ErrNotAuthenticated)
	assert.Contains(t, err.Error(), "dcs login")
	for _, cause := range []error{client.ErrForbidden, client.ErrRateLimited, context.DeadlineExceeded, context.Canceled, errors.New("secret-url?key=DO-NOT-DISPLAY")} {
		err := safeUIError(fmt.Errorf("read: %w", cause))
		assert.ErrorIs(t, err, cause)
		assert.NotContains(t, err.Error(), "DO-NOT-DISPLAY")
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusBadRequest} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"success":false,"error":{"message":"DO-NOT-DISPLAY"}}`)
			}))
			defer api.Close()
			source := &uiSource{api: client.New(api.URL, "fixture-key")}
			_, err := source.Load(context.Background(), workspace.Request{Kind: workspace.Servers})
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "DO-NOT-DISPLAY")
		})
	}
}

func TestUISourceEmptyAndMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		data     string
		identity bool
		wantErr  bool
	}{
		{`[]`, false, false}, {`null`, false, false}, {`{"unexpected":"shape"}`, false, true},
		{`[{"id":"../escape"}]`, false, true}, {`[{"name":"missing ID"}]`, false, true},
		{`{"user":{"id":"u1","email":""}}`, true, true},
		{`{"user":{"id":"u1","email":"fixture@example.com"}}`, true, false},
	} {
		t.Run(tc.data, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, `{"success":true,"data":%s}`, tc.data) }))
			defer api.Close()
			source := &uiSource{api: client.New(api.URL, "fixture-key")}
			var err error
			if tc.identity {
				_, err = source.Identify(context.Background())
			} else {
				var item workspace.Item
				item, err = source.Load(context.Background(), workspace.Request{Kind: workspace.Servers})
				if !tc.wantErr {
					assert.Contains(t, item.Body, "No servers")
				}
			}
			assert.Equal(t, tc.wantErr, err != nil)
		})
	}
}

func TestUIResourceIDsAndPagination(t *testing.T) {
	for _, id := range []string{"", ".", "..", "a/b", "a\\b", "a\x1b[31m", "a\u202e"} {
		assert.False(t, validUIResourceID(id), id)
	}
	assert.True(t, validUIResourceID("srv-1"))
	assert.Equal(t, url.Values{"page": {"1"}, "perPage": {"25"}}, uiPageQuery(workspace.Request{}))
	item := uiPaginate(workspace.Item{}, &client.Meta{Page: 2, TotalPages: 3}, workspace.Request{Kind: workspace.Servers})
	require.Len(t, item.Children, 2)
	assert.Equal(t, 1, item.Children[0].Request.Page)
	assert.Equal(t, 3, item.Children[1].Request.Page)
	assert.Empty(t, fieldValue(uiSiteFields(uiSite{}), "SSL enabled"), "omitted SSL must not become false")
}
