package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/workspace"
)

func uiCatalog(root *cobra.Command) workspace.Item {
	area := func(id, title, description string, names ...string) workspace.Item {
		item := workspace.Item{ID: id, Title: title, Description: description}
		for _, name := range names {
			for _, command := range root.Commands() {
				if command.Name() == name {
					item.Children = append(item.Children, uiCommandGuide(command))
				}
			}
		}
		return item
	}
	servers := area("servers", "Servers", "Live views and SSH commands", "servers", "ssh")
	servers.Children = append([]workspace.Item{{ID: "live-servers", Title: "Your servers", Description: "Read the current server list from Dilmune Cloud.", Request: &workspace.Request{Kind: workspace.Servers}}}, servers.Children...)
	sites := area("sites", "Sites & deploys", "Live views and deployment commands", "sites", "deploy", "init")
	sites.Children = append([]workspace.Item{{ID: "live-sites", Title: "Your sites", Description: "Choose a server to browse its sites.", Request: &workspace.Request{Kind: workspace.SiteServers}}}, sites.Children...)
	return workspace.Item{ID: "home", Title: "Your workspace", Description: "Live server and site views. A reference for the rest of your CLI.", Children: []workspace.Item{
		servers, sites,
		area("databases", "Databases", "Reference: schemas, queries, backups", "db"),
		area("storage", "Storage", "Reference: buckets and files", "storage"),
		area("access", "Access", "Reference: keys and account", "keys", "api-keys", "login", "whoami", "logout"),
		area("operations", "Operations", "Reference: logs, environment, processes", "logs", "env", "firewall", "cron", "daemon", "software", "status", "config", "open"),
	}}
}

// Derive references from Cobra so this workspace cannot invent a second CLI.
func uiCommandGuide(cmd *cobra.Command) workspace.Item {
	cmd.InheritedFlags()
	item := workspace.Item{ID: cmd.CommandPath(), Title: cmd.CommandPath(), Description: cmd.Short, Command: cmd.UseLine()}
	var body []string
	if cmd.Long != "" {
		body = append(body, cmd.Long)
	}
	if cmd.Example != "" {
		body = append(body, "EXAMPLES\n"+cmd.Example)
	}
	if cmd.HasAvailableLocalFlags() {
		body = append(body, "FLAGS\n"+cmd.LocalFlags().FlagUsages())
	}
	body = append(body, "Reference only. Exit the workspace to run this command.\nArguments in <angle brackets> are placeholders.")
	item.Body = strings.Join(body, "\n\n")
	for _, child := range cmd.Commands() {
		if child.IsAvailableCommand() && !child.Hidden {
			item.Children = append(item.Children, uiCommandGuide(child))
		}
	}
	return item
}
