package commands

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Dashboard overview",
		Example: "  dcs status\n  dcs status --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDashboardStats, nil)
			if err != nil {
				return fmt.Errorf("fetch stats: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			stats, err := client.Decode[struct {
				ServerCount   int `json:"totalServers"`
				SiteCount     int `json:"totalSites"`
				DatabaseCount int `json:"totalDatabases"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode stats: %w", err)
			}

			boxStyle := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.TextDim).
				Padding(0, 2)

			numStyle := lipgloss.NewStyle().Foreground(ui.BrandPrimary).Bold(true)
			labelStyle := lipgloss.NewStyle().Foreground(ui.TextMuted)

			fmt.Println()
			fmt.Println(ui.Title.Render("  Dashboard"))
			fmt.Println()

			servers := boxStyle.Render(
				numStyle.Render(fmt.Sprintf("%d", stats.ServerCount)) + " " + labelStyle.Render("Servers"),
			)
			sites := boxStyle.Render(
				numStyle.Render(fmt.Sprintf("%d", stats.SiteCount)) + " " + labelStyle.Render("Sites"),
			)
			databases := boxStyle.Render(
				numStyle.Render(fmt.Sprintf("%d", stats.DatabaseCount)) + " " + labelStyle.Render("Databases"),
			)

			row := lipgloss.JoinHorizontal(lipgloss.Top, "  ", servers, "  ", sites, "  ", databases)
			fmt.Println(row)
			fmt.Println()

			if cfg.User != nil {
				ui.PrintKeyValue("Account", cfg.User.Email)
			}
			fmt.Println()
			return nil
		},
	}
}
