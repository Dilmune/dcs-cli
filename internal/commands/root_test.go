package commands

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
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

// Every group in the real tree is checked, so a group added later cannot go
// back to printing help and exiting 0 on a word it does not know.
func TestCommandGroupsRejectUnknownSubcommands(t *testing.T) {
	var groups [][]string
	var walk func(cmd *cobra.Command, path []string)
	walk = func(cmd *cobra.Command, path []string) {
		if cmd.HasParent() && cmd.HasSubCommands() {
			groups = append(groups, path)
		}
		for _, child := range cmd.Commands() {
			walk(child, append(append([]string{}, path...), child.Name()))
		}
	}
	walk(NewRootCmd(), nil)
	require.GreaterOrEqual(t, len(groups), 14, "the walk must reach every command group")
	assert.Contains(t, groups, []string{"servers"})
	assert.Contains(t, groups, []string{"config", "keychain"})

	const unknown = "nosuchcommand"
	for _, path := range groups {
		name := "dcs " + strings.Join(path, " ")
		t.Run(name, func(t *testing.T) {
			// If the check ever stops firing, the root hooks run against the
			// empty configuration TestMain provides.
			t.Cleanup(setupTest(t, newMockAPI()))
			root := NewRootCmd()
			found, _, err := root.Find(append(append([]string{}, path...), unknown))
			require.NoError(t, err)
			require.Equal(t, name, found.CommandPath(), "the probe word must not be a real subcommand or alias")
			root.SetArgs(append(append([]string{}, path...), unknown, "x"))
			stdout := captureStdout(t, func() {
				captureStderr(t, func() { err = root.Execute() })
			})
			require.EqualError(t, err, fmt.Sprintf("unknown command %q for %q\n\n  Run '%s --help' to see its commands.", unknown, name, name))
			assert.Empty(t, stdout, "a wrong guess must not print help as if it succeeded")

			group, _, err := root.Find(path)
			require.NoError(t, err)
			require.NoError(t, group.ValidateArgs(nil))
			require.NoError(t, group.ValidateArgs([]string{"help"}))
			help := captureStdout(t, func() { require.NoError(t, group.RunE(group, nil)) })
			assert.Contains(t, help, "Available Commands:")
		})
	}
}

func TestNonInteractiveErrorsNameWhatAnswersThePrompt(t *testing.T) {
	tests := []struct {
		name    string
		command func() *cobra.Command
		args    []string
		want    string
	}{
		{"confirmation that --force skips", newServersRebootCmd, []string{"prod-web"}, "Pass --force to skip the confirmation."},
		{"server picker behind --server", newSitesListCmd, nil, "Pass --server with a server name or ID"},
		{"server picker behind the ssh argument", newSSHCmd, nil, "Name the server: dcs ssh <server>."},
		{"login", newLoginCmd, nil, "Set the DCS_API_KEY environment variable"},
		{"firewall toggle", newFirewallToggleCmd, []string{"--server", "srv-1"}, "Pass --enable or --disable."},
		{"software picker", newSoftwareInstallCmd, []string{"--server", "srv-1"}, "dcs software install node"},
		{"server create", newServersCreateCmd, nil, "Pass --name, --provider, --region and --sku"},
		{"logout has no flag to name", newLogoutCmd, nil, "Run it in a terminal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case client.PathServers:
					writeContractResponse(w, sampleServers())
				case client.PathFirewall("srv-1"):
					writeContractResponse(w, FirewallStatus{Enabled: true})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			cfg.DefaultServerID = ""
			t.Chdir(t.TempDir())

			cmd := tt.command()
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			cmd.SetArgs(tt.args)
			var err error
			captureStderr(t, func() {
				captureStdout(t, func() { err = cmd.Execute() })
			})
			require.ErrorIs(t, err, ui.ErrNonInteractive)
			assert.Contains(t, err.Error(), tt.want)
			if !strings.Contains(tt.want, "--force") {
				assert.NotContains(t, err.Error(), "--force", "--force does not answer this prompt")
			}
		})
	}
}
