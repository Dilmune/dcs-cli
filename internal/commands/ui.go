package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/workspace"
)

func newUICmd() *cobra.Command {
	var welcome bool
	var theme string
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the interactive cloud workspace",
		Long:  "Browse servers and sites, and explore the CLI's command reference.\nThis optional workspace is read-only; it never runs the displayed commands.\nRequires an interactive terminal. Existing commands and JSON output are unchanged.",
		Args:  cobra.NoArgs,
		// Do not inherit credential loading or asynchronous version output before
		// validating the terminal. Those hooks belong to the ordinary CLI.
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateUIFlags(cmd, theme); err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			if !hasUITerminal(cmd) {
				return fmt.Errorf("dcs ui requires an interactive terminal on both stdin and stdout; use ordinary dcs commands for scripts")
			}
			return nil
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {},
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings := config.Load()
			var api *client.Client
			if settings.IsAuthenticated() {
				api = client.New(settings.APIURL, settings.APIKey)
			}
			dir := config.Dir()
			seen, preferenceErr := config.UIWelcomeSeen(dir)
			// A bad preference must not block account access. Opening an item
			// retries the marker write without rewriting the saved credentials.
			if preferenceErr != nil {
				seen = false
			}
			plain, err := cmd.Flags().GetBool("no-color")
			if err != nil {
				return fmt.Errorf("read color preference: %w", err)
			}
			if err := workspace.Run(cmd.Context(), workspace.Options{
				Input: cmd.InOrStdin(), Output: cmd.OutOrStdout(), Catalog: uiCatalog(cmd.Root()),
				Source: &uiSource{api: api}, ShowWelcome: welcome || !seen,
				RememberWelcome: func() error { return config.RememberUIWelcome(dir) },
				NoColor:         plain || os.Getenv("NO_COLOR") != "", Theme: theme, Version: client.Version,
			}); err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&welcome, "welcome", false, "Show the first-run cube on the workspace overview")
	cmd.Flags().StringVar(&theme, "theme", "auto", "Terminal accents: auto, light, dim, dark")
	return cmd
}

func validateUIFlags(cmd *cobra.Command, theme string) error {
	for _, name := range []string{"json", "quiet", "debug"} {
		enabled, err := cmd.Flags().GetBool(name)
		if err != nil {
			return fmt.Errorf("read --%s: %w", name, err)
		}
		if enabled {
			return fmt.Errorf("--%s cannot be combined with dcs ui; use an ordinary command instead", name)
		}
	}
	format, err := cmd.Flags().GetString("output")
	if err != nil {
		return fmt.Errorf("read --output: %w", err)
	}
	if format != "" {
		return fmt.Errorf("--output cannot be combined with dcs ui; use an ordinary command instead")
	}
	switch theme {
	case "auto", "light", "dim", "dark":
		return nil
	default:
		return fmt.Errorf("unknown UI theme %q; use auto, light, dim, or dark", theme)
	}
}

func hasUITerminal(cmd *cobra.Command) bool {
	in, inputOK := cmd.InOrStdin().(*os.File)
	out, outputOK := cmd.OutOrStdout().(*os.File)
	return inputOK && outputOK && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}
