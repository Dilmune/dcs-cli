package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
