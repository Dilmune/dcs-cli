package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

// mockAPI provides a mock DCS API server for command tests.
type mockAPI struct {
	routes map[string]mockRoute
}

type mockRoute struct {
	status int
	data   any
}

func newMockAPI() *mockAPI {
	return &mockAPI{routes: make(map[string]mockRoute)}
}

// on registers a handler that returns a JSON API response.
func (m *mockAPI) on(method, path string, status int, data any) {
	m.routes[method+" "+path] = mockRoute{status: status, data: data}
}

func (m *mockAPI) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		route, ok := m.routes[key]
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   map[string]string{"message": "not found", "code": "not_found"},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(route.status)
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    route.data,
		})
	})
}

// setupTest creates a mock API server and configures the package globals.
// Returns a cleanup function that must be deferred.
func setupTest(t *testing.T, api *mockAPI) func() {
	t.Helper()

	server := httptest.NewServer(api.handler())

	origClient := apiClient
	origCfg := cfg
	origJSON := jsonOutput
	origFormat := outputFormat
	origDebug := debugMode
	origQuiet := quietMode

	cfg = &config.Config{
		APIURL: server.URL,
		APIKey: "test-key-123",
		User:   &config.UserInfo{ID: "user-1", Email: "test@example.com", Name: "Test User"},
	}
	apiClient = client.New(server.URL, "test-key-123")
	jsonOutput = false
	outputFormat = ""
	debugMode = false
	quietMode = false
	ui.SetOutputFormat("")

	return func() {
		server.Close()
		apiClient = origClient
		cfg = origCfg
		jsonOutput = origJSON
		outputFormat = origFormat
		debugMode = origDebug
		quietMode = origQuiet
		ui.SetOutputFormat("")
	}
}

// captureStdout captures stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String()
}

// captureStderr captures stderr during fn execution.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stderr = old
	return buf.String()
}

// setJSONOutput enables JSON output mode for tests.
func setJSONOutput() {
	jsonOutput = true
	outputFormat = ui.FormatJSON
	ui.SetOutputFormat(ui.FormatJSON)
}

// sampleServers returns test server data.
func sampleServers() []Server {
	return []Server{
		{ID: "srv-1", Name: "prod-web", Status: "active", Provider: "hetzner", Region: "nbg1", Tier: "cx21", IPv4: "203.0.113.1", CreatedAt: "2026-03-01"},
		{ID: "srv-2", Name: "staging", Status: "provisioning", Provider: "hetzner", Region: "fsn1", Tier: "cx11", IPv4: "", CreatedAt: "2026-03-15"},
	}
}

// sampleSites returns test site data.
func sampleSites() []Site {
	return []Site{
		{ID: "site-1", Domain: "app.example.com", ProjectType: "node", Status: "deployed", SSLEnabled: true, GitRepo: "git@github.com:user/app.git", GitBranch: "main", CreatedAt: "2026-03-01"},
		{ID: "site-2", Domain: "docs.example.com", ProjectType: "html", Status: "active", SSLEnabled: false, GitBranch: "", CreatedAt: "2026-03-10"},
	}
}

// sampleDatabases returns test database data.
func sampleDatabases() []Database {
	return []Database{
		{ID: "db-1", Name: "myapp_prod", Type: "postgres", Status: "active", CreatedAt: "2026-03-01"},
		{ID: "db-2", Name: "cache_db", Type: "redis", Status: "active", CreatedAt: "2026-03-05"},
	}
}
