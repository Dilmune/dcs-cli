package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIWelcomeMarkerIsPrivateAndDoesNotRewriteCredentials(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := []byte(`{"api_key":"test-fixture-not-a-real-key"}`)
	require.NoError(t, os.WriteFile(configPath, content, 0600))
	seen, err := UIWelcomeSeen(dir)
	require.NoError(t, err)
	assert.False(t, seen)
	require.NoError(t, RememberUIWelcome(dir))
	require.NoError(t, RememberUIWelcome(dir))
	seen, err = UIWelcomeSeen(dir)
	require.NoError(t, err)
	assert.True(t, seen)
	info, err := os.Stat(filepath.Join(dir, uiWelcomeFile))
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular())
	assertUIWelcomePermissions(t, info, 0600)
	assert.Zero(t, info.Size())
	unchanged, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, content, unchanged)
}

func TestUIWelcomeRejectsSymlinksAndDirectories(t *testing.T) {
	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, uiWelcomeFile)
			if kind == "symlink" {
				require.NoError(t, os.Symlink(filepath.Join(dir, "untouched"), marker))
			} else {
				require.NoError(t, os.Mkdir(marker, 0700))
			}
			seen, err := UIWelcomeSeen(dir)
			assert.Error(t, err)
			assert.False(t, seen)
			assert.Error(t, RememberUIWelcome(dir))
			_, err = os.Stat(filepath.Join(dir, "untouched"))
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestUIWelcomeCreatesOwnDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "preferences")
	require.NoError(t, RememberUIWelcome(dir))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assertUIWelcomePermissions(t, info, 0700)
}

func assertUIWelcomePermissions(t *testing.T, info os.FileInfo, want os.FileMode) {
	t.Helper()
	// Windows synthesizes mode bits, so they cannot verify POSIX permissions.
	if runtime.GOOS != "windows" {
		assert.Equal(t, want, info.Mode().Perm())
	}
}
