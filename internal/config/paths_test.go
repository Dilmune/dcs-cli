package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDir_DCSConfigDir(t *testing.T) {
	t.Setenv("DCS_CONFIG_DIR", "/custom/dcs")
	assert.Equal(t, "/custom/dcs", Dir())
}

func TestDir_XDGConfigHome(t *testing.T) {
	t.Setenv("DCS_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	assert.Equal(t, filepath.Join("/xdg/config", "dcs"), Dir())
}

func TestDir_DefaultHome(t *testing.T) {
	t.Setenv("DCS_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	dir := Dir()
	assert.Contains(t, dir, ".dcs")
}

func TestFilePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "CLI config")
	t.Setenv("DCS_CONFIG_DIR", dir)
	assert.Equal(t, filepath.Join(dir, "config.json"), FilePath())
}

func TestProjectFilePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My project")
	assert.Equal(t, filepath.Join(dir, ".dcs.json"), ProjectFilePath(dir))
}
