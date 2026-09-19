package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables",
	}

	cmd.AddCommand(newEnvListCmd())
	cmd.AddCommand(newEnvGetCmd())
	cmd.AddCommand(newEnvSetCmd())
	cmd.AddCommand(newEnvRmCmd())

	return cmd
}

func getProjectContext() (serverID, siteID string, err error) {
	cwd, _ := os.Getwd()
	pc, err := config.LoadProjectConfig(cwd)
	if err != nil {
		return "", "", client.ErrNoProjectConfig
	}
	return pc.ServerID, pc.SiteID, nil
}

func decodeEnvironment(resp *client.APIResponse) (map[string]string, error) {
	result, err := client.Decode[struct {
		Vars map[string]*string `json:"vars"`
	}](resp)
	if err != nil {
		return nil, fmt.Errorf("decode environment: %w", err)
	}
	if result.Vars == nil {
		return nil, fmt.Errorf("environment response is missing vars")
	}
	vars := make(map[string]string, len(result.Vars))
	for key, value := range result.Vars {
		if value == nil {
			return nil, fmt.Errorf("environment response contains a null value")
		}
		vars[key] = *value
	}
	return vars, nil
}

func fetchEnvironment(ctx context.Context, serverID, siteID string) (map[string]string, error) {
	resp, err := apiClient.Get(ctx, client.PathSiteEnv(serverID, siteID), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch environment: %w", err)
	}
	return decodeEnvironment(resp)
}

func newEnvListCmd() *cobra.Command {
	var showValues bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List environment variables",
		Example: "  dcs env list\n  dcs env list --show-values",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, siteID, err := getProjectContext()
			if err != nil {
				return fmt.Errorf("getProjectContext: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSiteEnv(serverID, siteID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			envVars, err := decodeEnvironment(resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(envVars) == 0 {
				ui.PrintInfo("No environment variables set.")
				return nil
			}

			fmt.Println()
			for k, v := range envVars {
				if showValues {
					fmt.Printf("  %s=%s\n", ui.Bold.Render(k), v)
				} else {
					truncated := v
					if len(truncated) > 40 {
						truncated = truncated[:40] + "..."
					}
					fmt.Printf("  %s=%s\n", ui.Bold.Render(k), ui.Muted.Render(truncated))
				}
			}
			if !showValues {
				fmt.Printf("\n  %s\n", ui.Dim.Render("Use --show-values to reveal full values."))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().BoolVar(&showValues, "show-values", false, "Show full values")
	return cmd
}

func newEnvGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <KEY>",
		Short:   "Get a single environment variable",
		Example: "  dcs env get DATABASE_URL",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, siteID, err := getProjectContext()
			if err != nil {
				return fmt.Errorf("getProjectContext: %w", err)
			}

			envVars, err := fetchEnvironment(cmd.Context(), serverID, siteID)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			key := args[0]
			if val, ok := envVars[key]; ok {
				if jsonOutput {
					ui.PrintJSON(map[string]string{key: val})
					return nil
				}
				fmt.Println(val)
			} else {
				return fmt.Errorf("environment variable %s not found", key)
			}
			return nil
		},
	}
}

func newEnvSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set KEY=VALUE [KEY2=VALUE2...]",
		Short: "Set environment variables",
		Example: `  dcs env set DATABASE_URL=postgres://localhost/mydb
  dcs env set API_KEY=secret NODE_ENV=production`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, siteID, err := getProjectContext()
			if err != nil {
				return fmt.Errorf("getProjectContext: %w", err)
			}

			updates := make(map[string]string)
			for _, arg := range args {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid environment assignment: expected KEY=VALUE")
				}
				updates[parts[0]] = parts[1]
			}

			err = ui.RunWithSpinner("Updating environment variables...", func() error {
				vars, err := fetchEnvironment(cmd.Context(), serverID, siteID)
				if err != nil {
					return fmt.Errorf("read existing variables: %w", err)
				}
				for key, value := range updates {
					vars[key] = value
				}
				// The API replaces the entire map, as in the portal's environment editor.
				_, err = apiClient.Put(cmd.Context(), client.PathSiteEnv(serverID, siteID), map[string]map[string]string{"vars": vars})
				if err != nil {
					return fmt.Errorf("update: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(map[string]int{"updated": len(updates)})
				return nil
			}
			ui.PrintSuccess(fmt.Sprintf("Set %d environment variable(s).", len(updates)))
			return nil
		},
	}
}

func newEnvRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <KEY>",
		Short:   "Remove an environment variable",
		Example: "  dcs env rm DATABASE_URL",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, siteID, err := getProjectContext()
			if err != nil {
				return fmt.Errorf("getProjectContext: %w", err)
			}

			err = ui.RunWithSpinner("Removing environment variable...", func() error {
				vars, err := fetchEnvironment(cmd.Context(), serverID, siteID)
				if err != nil {
					return fmt.Errorf("read existing variables: %w", err)
				}
				if _, ok := vars[args[0]]; !ok {
					return fmt.Errorf("environment variable %s not found", args[0])
				}
				delete(vars, args[0])
				_, err = apiClient.Put(cmd.Context(), client.PathSiteEnv(serverID, siteID), map[string]map[string]string{"vars": vars})
				if err != nil {
					return fmt.Errorf("update: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(map[string]string{"removed": args[0]})
				return nil
			}
			ui.PrintSuccess(fmt.Sprintf("Removed %s.", args[0]))
			return nil
		},
	}
}
