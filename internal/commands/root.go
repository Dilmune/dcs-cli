package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

var (
	jsonOutput   bool
	outputFormat string
	debugMode    bool
	quietMode    bool
	noColor      bool
	cfg          *config.Config
	apiClient    *client.Client
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dcs",
		Short: "Dilmune Cloud Services CLI",
		Long:  "Manage your cloud infrastructure from the command line.",
		Run: func(cmd *cobra.Command, args []string) {
			ui.PrintBanner(client.Version)
			ui.PrintCommands()
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor || os.Getenv("NO_COLOR") != "" {
				os.Setenv("NO_COLOR", "1")
			}
			if quietMode {
				ui.SetQuiet(true)
			}
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
				apiClient = client.New(cfg.APIURL, cfg.APIKey)
				if debugMode {
					apiClient.SetDebug(true)
				}
			}
			checkVersionInBackground()
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

	root.AddCommand(newLoginCmd())
	root.AddCommand(newLogoutCmd())
	root.AddCommand(newWhoamiCmd())

	root.AddCommand(newServersCmd())
	root.AddCommand(newSitesCmd())
	root.AddCommand(newDatabasesCmd())

	root.AddCommand(newDeployCmd())
	root.AddCommand(newSSHCmd())
	root.AddCommand(newInitCmd())

	root.AddCommand(newEnvCmd())
	root.AddCommand(newLogsCmd())
	root.AddCommand(newStorageCmd())
	root.AddCommand(newKeysCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newOpenCmd())

	root.AddCommand(newFirewallCmd())
	root.AddCommand(newCronCmd())
	root.AddCommand(newDaemonsCmd())
	root.AddCommand(newSoftwareCmd())
	root.AddCommand(newAPIKeysCmd())
	root.AddCommand(newConfigCmd())

	root.AddCommand(newDocsCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newCompletionCmd())
	root.AddCommand(newUICmd())

	return root
}

func requireAuth() error {
	if apiClient == nil {
		return client.ErrNotAuthenticated
	}
	return nil
}
