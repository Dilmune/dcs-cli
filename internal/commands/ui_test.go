package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dilmune/dcs-cli/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIRejectsScriptModesBeforeCredentialsOrWelcomeState(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ui"}, "interactive terminal"},
		{[]string{"ui", "--welcome"}, "interactive terminal"},
		{[]string{"ui", "--json"}, "--json cannot"},
		{[]string{"ui", "--output", "yaml"}, "--output cannot"},
		{[]string{"ui", "--quiet"}, "--quiet cannot"},
		{[]string{"ui", "--debug"}, "--debug cannot"},
		{[]string{"ui", "--theme", "unknown"}, "unknown UI theme"},
		{[]string{"ui", "extra"}, "unknown command"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("DCS_CONFIG_DIR", dir)
			// Invalid config would emit a parse warning if the parent hook ran.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte("invalid"), 0600))
			root := NewRootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetIn(strings.NewReader(""))
			root.SetArgs(tc.args)
			stderr := captureStderr(t, func() { err := root.Execute(); require.Error(t, err); assert.Contains(t, err.Error(), tc.want) })
			assert.Empty(t, stderr)
			assert.Empty(t, out.String())
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, "config.json", entries[0].Name())
		})
	}
}

func TestUIHelpIsAvailableWithoutTerminalOrAuthentication(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DCS_CONFIG_DIR", dir)
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"ui", "--help"})
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), "read-only")
	assert.Contains(t, out.String(), "--welcome")
	assert.NotContains(t, out.String(), "\x1b")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestUICatalogCoversThePlatformAndUsesActualCommands(t *testing.T) {
	root := NewRootCmd()
	catalog := uiCatalog(root)
	require.Len(t, catalog.Children, 6)
	assert.Equal(t, []string{"Servers", "Sites & deploys", "Databases", "Storage", "Access", "Operations"}, func() []string {
		var titles []string
		for _, item := range catalog.Children {
			titles = append(titles, item.Title)
		}
		return titles
	}())
	var paths []string
	var check func(workspace.Item)
	check = func(item workspace.Item) {
		if strings.HasPrefix(item.ID, "dcs ") {
			command, args, err := root.Find(strings.Fields(strings.TrimPrefix(item.ID, "dcs ")))
			require.NoError(t, err)
			require.Empty(t, args)
			assert.Equal(t, command.UseLine(), item.Command)
			assert.Nil(t, item.Request, "command references must not call the API")
			paths = append(paths, item.ID)
		}
		for _, child := range item.Children {
			check(child)
		}
	}
	check(catalog)
	for _, name := range []string{"dcs servers create", "dcs sites deploy", "dcs db schema", "dcs storage buckets", "dcs keys list", "dcs daemon restart", "dcs firewall list"} {
		assert.Contains(t, paths, name)
	}
}

func TestUICatalogListsSingleCommandAreasBySubcommand(t *testing.T) {
	root := NewRootCmd()
	catalog := uiCatalog(root)
	byTitle := map[string]workspace.Item{}
	for _, area := range catalog.Children {
		byTitle[area.Title] = area
	}
	titles := func(items []workspace.Item) []string {
		var out []string
		for _, item := range items {
			out = append(out, item.Title)
		}
		return out
	}
	for _, tc := range []struct{ area, parent string }{{"Databases", "db"}, {"Storage", "storage"}} {
		t.Run(tc.area, func(t *testing.T) {
			parent, _, err := root.Find([]string{tc.parent})
			require.NoError(t, err)
			var want []string
			for _, sub := range parent.Commands() {
				if sub.IsAvailableCommand() && !sub.Hidden {
					want = append(want, sub.CommandPath())
				}
			}
			require.NotEmpty(t, want, "the fixture needs a parent with subcommands")
			area := byTitle[tc.area]
			got := titles(area.Children)
			require.Equal(t, append([]string{parent.CommandPath()}, want...), got)
			assert.Equal(t, parent.UseLine(), area.Children[0].Command, "the parent guide stays first so its help is not lost")
			assert.Empty(t, area.Children[0].Children, "the parent entry does not repeat the subcommands a level down")
			for _, child := range area.Children[1:] {
				assert.NotEmpty(t, child.Command)
				assert.True(t, strings.HasPrefix(child.Title, parent.CommandPath()+" "), child.Title)
			}
		})
	}
	operations := byTitle["Operations"]
	assert.Equal(t, []string{"dcs logs", "dcs env", "dcs firewall", "dcs cron", "dcs daemon", "dcs software", "dcs status", "dcs config", "dcs open"}, titles(operations.Children))
	logs, _, err := root.Find([]string{"logs"})
	require.NoError(t, err)
	assert.Len(t, operations.Children[0].Children, len(logs.Commands()), "a multi-command area keeps subcommands one level down")
}
