package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Windows opens a device or an alternate data stream for these, whatever
// directory they are joined to.
func TestLocalDownloadNameRefusesWindowsDevicesAndStreams(t *testing.T) {
	for _, remote := range []string{"CON", "nul", "COM1", "LPT1", "CONIN$", "CONOUT$", `dir\NUL`, "report.txt:stream"} {
		t.Run(remote, func(t *testing.T) {
			_, err := localDownloadName(remote)
			assert.ErrorContains(t, err, "not a safe local file name")
		})
	}
}

// filepath.Base drops the volume on Windows, so a drive or UNC prefix leaves
// only the last element, and a bare volume leaves nothing usable.
func TestLocalDownloadNameKeepsOnlyTheLastWindowsElement(t *testing.T) {
	kept := map[string]string{
		"C:evil.bat":               `evil.bat`,
		`C:\Users\victim\evil.bat`: "evil.bat",
		`\\host\share\x`:           "x",
	}
	for remote, want := range kept {
		t.Run(remote, func(t *testing.T) {
			got, err := localDownloadName(remote)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
	for _, remote := range []string{"C:", `C:\`, `\\host\share`} {
		t.Run(remote, func(t *testing.T) {
			_, err := localDownloadName(remote)
			assert.ErrorContains(t, err, "not a safe local file name")
		})
	}
}

// CreateHardLink, not link(2), is what refuses here, so it gets its own check.
func TestSaveStorageDownloadOnWindows(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "existing.txt")
	require.NoError(t, os.WriteFile(file, []byte("mine"), 0o600))
	folder := filepath.Join(dir, "folder")
	require.NoError(t, os.Mkdir(folder, 0o755))

	for _, dest := range []string{file, folder} {
		_, err := saveStorageDownload(dest, strings.NewReader("remote"), false, os.Link)
		assert.ErrorContains(t, err, "already exists", dest)
	}
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "mine", string(got))

	_, err = saveStorageDownload(file, strings.NewReader("remote"), true, os.Link)
	require.NoError(t, err)
	got, err = os.ReadFile(file)
	require.NoError(t, err)
	assert.Equal(t, "remote", string(got))
}
