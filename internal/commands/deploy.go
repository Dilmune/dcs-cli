package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newDeployCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the current project",
		Long:  "Deploys the site linked in .dcs.json. Run 'dcs init' first to link a project.",
		Example: `  dcs deploy
  dcs deploy --branch staging`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			cwd, _ := os.Getwd()
			pc, err := config.LoadProjectConfig(cwd)
			if err != nil {
				return client.ErrNoProjectConfig
			}

			if pc.ServerID == "" || pc.SiteID == "" {
				return client.ErrNoProjectConfig
			}

			body := map[string]string{}
			if branch != "" {
				body["branch"] = branch
			}

			if !jsonOutput && !quietMode {
				fmt.Printf("  %s %s\n", ui.Muted.Render("Deploying"), ui.Bold.Render(pc.SiteDomain))
			}

			var deployment Deployment
			err = ui.RunWithSpinner("Deploying...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathSiteDeploy(pc.ServerID, pc.SiteID), body)
				if err != nil {
					return fmt.Errorf("trigger deploy: %w", err)
				}
				d, err := client.Decode[Deployment](resp)
				if err != nil {
					return fmt.Errorf("decode deploy: %w", err)
				}
				deployment = d
				return nil
			})
			if err != nil {
				return fmt.Errorf("deploy: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(deployment)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Deployment triggered for %s", ui.Bold.Render(pc.SiteDomain)))
			ui.PrintKeyValue("Deployment", deployment.ID)
			ui.PrintKeyValue("Status", deployment.Status)
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch to deploy")
	return cmd
}
