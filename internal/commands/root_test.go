package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func TestRequireAuth_NoClient(t *testing.T) {
	apiClient = nil
	err := requireAuth()
	assert.ErrorIs(t, err, client.ErrNotAuthenticated)
}

func TestRequireAuth_WithClient(t *testing.T) {
	apiClient = client.New("http://localhost", "test-key")
	defer func() { apiClient = nil }()

	err := requireAuth()
	assert.NoError(t, err)
}

func TestNewRootCmd_Structure(t *testing.T) {
	root := NewRootCmd()

	assert.Equal(t, "dcs", root.Use)
	assert.True(t, root.SilenceUsage)
	assert.True(t, root.SilenceErrors)

	// Verify key subcommands are registered
	subcommands := make(map[string]bool)
	for _, cmd := range root.Commands() {
		subcommands[cmd.Name()] = true
	}

	expected := []string{
		"login", "logout", "whoami",
		"servers", "sites", "db",
		"deploy", "ssh", "init",
		"env", "logs", "storage", "keys", "status", "open",
		"version", "completion",
	}

	for _, name := range expected {
		assert.True(t, subcommands[name], "missing subcommand: %s", name)
	}
}

func TestNewRootCmd_JSONFlag(t *testing.T) {
	root := NewRootCmd()
	f := root.PersistentFlags().Lookup("json")
	assert.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

func TestNewRootCmd_EveryCommandHasAGroup(t *testing.T) {
	root := NewRootCmd()
	root.InitDefaultHelpCmd()

	for _, cmd := range root.Commands() {
		assert.NotEmpty(t, cmd.GroupID, "ungrouped command: %s", cmd.Name())
		assert.True(t, root.ContainsGroup(cmd.GroupID), "%s points at unknown group %q", cmd.Name(), cmd.GroupID)
	}
}

func TestNewRootCmd_HelpGroupsInDesignOrder(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())

	help := out.String()
	assert.NotContains(t, help, "Additional Commands")
	assert.NotContains(t, help, "Available Commands")

	titles := []string{"Servers", "Sites & deploys", "Databases", "Storage", "Access", "Operations"}
	last := -1
	for _, title := range titles {
		at := strings.Index(help, "\n"+title+"\n")
		require.GreaterOrEqual(t, at, 0, "group title missing: %s", title)
		assert.Greater(t, at, last, "group out of order: %s", title)
		last = at
	}

	operations := help[last:strings.Index(help, "\nFlags:")]
	tail := []string{"version", "completion", "docs", "help"}
	previous := -1
	for _, name := range tail {
		at := strings.Index(operations, "\n  "+name+" ")
		require.GreaterOrEqual(t, at, 0, "%s missing from Operations", name)
		assert.Greater(t, at, previous, "%s must close the Operations group", name)
		previous = at
	}
	assert.Greater(t, strings.Index(operations, "\n  version "), strings.Index(operations, "\n  ui "))
}
