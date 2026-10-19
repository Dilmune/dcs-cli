package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

const helpCommandName = "help"

const (
	groupServers    = "servers"
	groupSites      = "sites"
	groupDatabases  = "databases"
	groupStorage    = "storage"
	groupAccess     = "access"
	groupOperations = "operations"
)

var (
	jsonOutput   bool
	outputFormat string
	debugMode    bool
	quietMode    bool
	noColor      bool
	themeFlag    string
	cfg          *config.Config
	apiClient    *client.Client
)

func NewRootCmd() *cobra.Command {
	// Help lists each group in registration order so completion, docs, help
	// and version close the Operations group as DESIGN.md requires.
	cobra.EnableCommandSorting = false
	root := &cobra.Command{
		Use:   "dcs",
		Short: "Dilmune Cloud Services CLI",
		Long:  "Manage your cloud infrastructure from the command line.",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintBanner(client.Version)
			ui.PrintCommands()
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateTheme(themeFlag); err != nil {
				return err
			}
			if quietMode {
				ui.SetQuiet(true)
			}
			ui.Init(themeFlag, noColor)
			if os.Getenv("DCS_DEBUG") != "" {
				debugMode = true
			}

			if outputFormat != "" {
				ui.SetOutputFormat(outputFormat)
				if outputFormat == ui.FormatJSON {
					jsonOutput = true
				}
			} else if jsonOutput {
				ui.SetOutputFormat(ui.FormatJSON)
			}

			cfg = config.Load()
			if cfg.IsAuthenticated() {
				apiClient = newAPIClient(cfg)
				if debugMode {
					apiClient.SetDebug(true)
				}
			}
			checkVersionInBackground()
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			printVersionWarning()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON (shorthand for --output json)")
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format: json, yaml, csv")
	root.PersistentFlags().BoolVar(&debugMode, "debug", false, "Show debug output (requests, timing)")
	root.PersistentFlags().BoolVar(&quietMode, "quiet", false, "Suppress non-essential output")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	root.PersistentFlags().StringVar(&themeFlag, "theme", string(ui.ModeAuto), "Terminal accents: auto, light, dim, dark")

	root.AddGroup(
		&cobra.Group{ID: groupServers, Title: "Servers"},
		&cobra.Group{ID: groupSites, Title: "Sites & deploys"},
		&cobra.Group{ID: groupDatabases, Title: "Databases"},
		&cobra.Group{ID: groupStorage, Title: "Storage"},
		&cobra.Group{ID: groupAccess, Title: "Access"},
		&cobra.Group{ID: groupOperations, Title: "Operations"},
	)
	root.SetHelpCommandGroupID(groupOperations)
	root.SetCompletionCommandGroupID(groupOperations)

	addToGroup(root, groupServers, newServersCmd(), newSSHCmd(), newKeysCmd(), newFirewallCmd(), newSoftwareCmd())
	addToGroup(root, groupSites, newSitesCmd(), newDeployCmd(), newInitCmd(), newEnvCmd(), newLogsCmd())
	addToGroup(root, groupDatabases, newDatabasesCmd())
	addToGroup(root, groupStorage, newStorageCmd())
	addToGroup(root, groupAccess, newLoginCmd(), newLogoutCmd(), newWhoamiCmd(), newAPIKeysCmd(), newConfigCmd())
	addToGroup(root, groupOperations, newCronCmd(), newDaemonsCmd(), newStatusCmd(), newOpenCmd(), newUICmd(), newVersionCmd(), newCompletionCmd(), newDocsCmd())
	rejectUnknownSubcommands(root)

	return root
}

// rejectUnknownSubcommands makes every command group fail on a word it does
// not know. Cobra only checks for unknown commands at the root, so
// 'dcs servers show x' printed help and exited 0, and a script read a wrong
// guess as success. The check runs as argument validation, before any hook
// loads the configuration or reaches the network.
func rejectUnknownSubcommands(cmd *cobra.Command) {
	for _, child := range cmd.Commands() {
		if isCommandGroup(child) {
			child.Args = noUnknownSubcommand
			child.RunE = showGroupHelp
		}
		rejectUnknownSubcommands(child)
	}
}

func isCommandGroup(cmd *cobra.Command) bool {
	return cmd.HasSubCommands() && !cmd.Runnable()
}

func noUnknownSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 || args[0] == helpCommandName {
		return nil
	}
	return &client.CLIError{
		Message:    fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath()),
		Suggestion: fmt.Sprintf("Run '%s --help' to see its commands.", cmd.CommandPath()),
	}
}

// showGroupHelp keeps 'dcs servers' and 'dcs servers help' printing the
// group's help and exiting 0, as they did before the group became runnable.
func showGroupHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func addToGroup(root *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = groupID
		root.AddCommand(cmd)
	}
}

// newAPIClient builds the client for the loaded credentials, flagging a key
// from DCS_API_KEY so a rejection does not send a script to 'dcs login'.
func newAPIClient(settings *config.Config) *client.Client {
	api := client.New(settings.APIURL, settings.APIKey)
	api.SetKeyFromEnvironment(settings.APIKeyFromEnv)
	return api
}

func requireAuth() error {
	if apiClient == nil {
		return client.ErrNotAuthenticated
	}
	return nil
}
