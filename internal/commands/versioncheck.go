package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

const (
	versionCheckFile   = "version-check.json"
	versionCheckMaxAge = 24 * time.Hour
	versionCheckURL    = "https://api.github.com/repos/dilmune/dcs-cli/releases/latest"
)

type versionCheckResult struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// checkVersionInBackground starts a non-blocking version check.
// The result is printed after command execution via printVersionWarning.
var latestVersionCh = make(chan string, 1)

func checkVersionInBackground() {
	go func() {
		version, err := getLatestVersion()
		if err != nil || version == "" {
			latestVersionCh <- ""
			return
		}
		latestVersionCh <- version
	}()
}

func printVersionWarning() {
	select {
	case latest := <-latestVersionCh:
		if shouldShowVersionWarning(client.Version, latest) {
			fmt.Fprintf(os.Stderr, "\n  %s A new version of dcs is available: %s → %s\n",
				ui.Info.Render("i"),
				ui.Dim.Render(client.Version),
				ui.Success.Render(latest),
			)
			fmt.Fprintf(os.Stderr, "  %s\n\n", ui.Muted.Render("Run 'brew upgrade dcs' or download from github.com/dilmune/dcs-cli"))
		}
	default:
	}
}

func shouldShowVersionWarning(current, latest string) bool {
	return !quietMode && !jsonOutput && !ui.HasFormatOverride() && isNewerVersion(current, latest)
}

func isNewerVersion(current, latest string) bool {
	current = "v" + strings.TrimPrefix(current, "v")
	latest = "v" + strings.TrimPrefix(latest, "v")
	return semver.IsValid(current) && semver.IsValid(latest) && semver.Compare(latest, current) > 0
}

func getLatestVersion() (string, error) {
	cachePath := filepath.Join(config.Dir(), versionCheckFile)

	// Check cache first
	if data, err := os.ReadFile(cachePath); err == nil {
		var cached versionCheckResult
		if json.Unmarshal(data, &cached) == nil && time.Since(cached.CheckedAt) < versionCheckMaxAge {
			return cached.LatestVersion, nil
		}
	}

	// Fetch from GitHub
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get(versionCheckURL) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("version check: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse version response: %w", err)
	}

	// Cache result
	result := versionCheckResult{
		LatestVersion: release.TagName,
		CheckedAt:     time.Now(),
	}
	if data, err := json.Marshal(result); err == nil {
		_ = os.MkdirAll(config.Dir(), 0700)
		_ = os.WriteFile(cachePath, data, 0600)
	}

	return release.TagName, nil
}
