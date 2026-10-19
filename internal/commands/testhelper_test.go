package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

// TestMain keeps every test in this package away from the developer's own
// CLI state and from the internet, including tests written later that never
// think about either. A test that reaches the root command's hooks loads the
// configuration, which can hold an API key, and starts the background
// version check against GitHub.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, "DCS_") {
			_ = os.Unsetenv(name)
		}
	}
	dir, err := os.MkdirTemp("", "dcs-cli-commands-test-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create test config dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	if err := os.Setenv("DCS_CONFIG_DIR", dir); err != nil {
		fmt.Fprintf(os.Stderr, "set test config dir: %v\n", err)
		return 1
	}

	keyring.MockInit()

	// An error answer writes no cache file, so a check that outlives its test
	// cannot race that test's temporary directory cleanup.
	releases := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer releases.Close()
	versionCheckURL = releases.URL

	return m.Run()
}

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
	return captureFile(t, &os.Stdout, fn)
}

// captureStderr captures stderr during fn execution.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureFile(t, &os.Stderr, fn)
}

// captureFile reads the pipe while fn writes to it. A pipe holds 64 KiB on
// Linux and less on Windows; reading only after fn returns blocks fn forever
// once its output passes that.
func captureFile(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	original := *target
	*target = w
	defer func() { *target = original }()

	captured := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		captured <- buf.String()
	}()

	fn()
	require.NoError(t, w.Close())
	output := <-captured
	require.NoError(t, r.Close())
	return output
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

// The helper is driven through a variable of its own rather than os.Stdout:
// if the drain ever regresses, only this pipe blocks, and the test runner can
// still print the failure.
func TestCaptureDrainsPastThePipeBuffer(t *testing.T) {
	const size = 1 << 20
	var stream *os.File
	result := make(chan string, 1)
	go func() {
		result <- captureFile(t, &stream, func() {
			_, _ = stream.WriteString(strings.Repeat("x", size))
		})
	}()
	select {
	case out := <-result:
		assert.Len(t, out, size)
		assert.Nil(t, stream, "the original file is put back")
	case <-time.After(10 * time.Second):
		t.Fatal("capture blocked once the output passed the pipe buffer")
	}
}
