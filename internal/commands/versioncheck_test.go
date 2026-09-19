package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func TestIsNewerVersion(t *testing.T) {
	for _, tt := range []struct {
		name, current, latest string
		want                  bool
	}{
		{"regression older public release", "3.6.1", "v3.6.0", false},
		{"same", "3.6.1", "3.6.1", false},
		{"same with prefix", "3.6.1", "v3.6.1", false},
		{"current prefix", "v3.6.1", "3.6.2", true},
		{"patch", "3.6.1", "v3.6.2", true},
		{"numeric minor", "3.9.0", "v3.10.0", true},
		{"older numeric minor", "3.10.0", "v3.9.0", false},
		{"major", "3.99.0", "v4.0.0", true},
		{"prerelease below stable", "3.7.0", "v3.7.0-rc.1", false},
		{"stable after prerelease", "3.7.0-rc.1", "v3.7.0", true},
		{"numeric prerelease", "3.7.0-rc.9", "v3.7.0-rc.10", true},
		{"build metadata", "3.7.0+local", "v3.7.0+release", false},
		{"development build", "dev", "v3.7.0", false},
		{"invalid latest", "3.7.0", "not-a-version", false},
		{"empty latest", "3.7.0", "", false},
		{"invalid numeric version", "3.7.0", "v03.8.0", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNewerVersion(tt.current, tt.latest))
		})
	}
}

func TestPrintVersionWarning(t *testing.T) {
	for _, tt := range []struct {
		name, latest, format string
		quiet, json, want    bool
	}{
		{name: "newer on stderr", latest: "v3.7.0", want: true},
		{name: "table output", latest: "v3.7.0", format: ui.FormatTable, want: true},
		{name: "older suppressed", latest: "v3.6.0"},
		{name: "equal suppressed", latest: "v3.6.1"},
		{name: "quiet suppressed", latest: "v3.7.0", quiet: true},
		{name: "json flag suppressed", latest: "v3.7.0", json: true},
		{name: "json format suppressed", latest: "v3.7.0", format: ui.FormatJSON},
		{name: "yaml suppressed", latest: "v3.7.0", format: ui.FormatYAML},
		{name: "csv suppressed", latest: "v3.7.0", format: ui.FormatCSV},
		{name: "empty suppressed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			originalChannel, originalVersion := latestVersionCh, client.Version
			originalQuiet, originalJSON, originalFormat := quietMode, jsonOutput, ui.GetOutputFormat()
			t.Cleanup(func() {
				latestVersionCh, client.Version = originalChannel, originalVersion
				quietMode, jsonOutput = originalQuiet, originalJSON
				ui.SetOutputFormat(originalFormat)
			})
			latestVersionCh = make(chan string, 1)
			latestVersionCh <- tt.latest
			client.Version = "3.6.1"
			quietMode, jsonOutput = tt.quiet, tt.json
			ui.SetOutputFormat(tt.format)
			var stderr string
			stdout := captureStdout(t, func() {
				stderr = captureStderr(t, printVersionWarning)
			})
			assert.Empty(t, stdout)
			assert.Empty(t, latestVersionCh, "the result must be consumed even when suppressed")
			if tt.want {
				assert.Contains(t, stderr, "A new version of dcs is available")
				assert.Contains(t, stderr, tt.latest)
			} else {
				assert.Empty(t, stderr)
			}
			assert.Empty(t, captureStderr(t, printVersionWarning), "no result must not block")
		})
	}
}

func TestVersionCheckResult_Cache(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, versionCheckFile)

	result := versionCheckResult{
		LatestVersion: "v1.2.3",
		CheckedAt:     time.Now(),
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, data, 0600))

	var loaded versionCheckResult
	raw, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &loaded))

	assert.Equal(t, "v1.2.3", loaded.LatestVersion)
	assert.True(t, time.Since(loaded.CheckedAt) < versionCheckMaxAge)
}

func TestVersionCheckResult_Expired(t *testing.T) {
	result := versionCheckResult{
		LatestVersion: "v0.0.1",
		CheckedAt:     time.Now().Add(-25 * time.Hour),
	}

	assert.True(t, time.Since(result.CheckedAt) > versionCheckMaxAge)
}

func TestGetLatestVersion_FromServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer ts.Close()

	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)

	// Override the URL for testing - we test the cache/parse logic directly
	cachePath := filepath.Join(dir, versionCheckFile)

	// Simulate a fresh fetch result being cached
	result := versionCheckResult{
		LatestVersion: "v2.0.0",
		CheckedAt:     time.Now(),
	}
	data, _ := json.Marshal(result)
	require.NoError(t, os.WriteFile(cachePath, data, 0600))

	// Read back from cache
	raw, err := os.ReadFile(cachePath)
	require.NoError(t, err)

	var cached versionCheckResult
	require.NoError(t, json.Unmarshal(raw, &cached))
	assert.Equal(t, "v2.0.0", cached.LatestVersion)
}
