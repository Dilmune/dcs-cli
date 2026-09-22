package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
	"github.com/dilmune/dcs-cli/internal/workspace"
)

type dashboardStats struct {
	ServerCount   int `json:"totalServers"`
	SiteCount     int `json:"totalSites"`
	DatabaseCount int `json:"totalDatabases"`
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Dashboard overview",
		Example: "  dcs status\n  dcs status --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			ctx := context.Background()
			resp, err := apiClient.Get(ctx, client.PathDashboardStats, nil)
			if err != nil {
				return fmt.Errorf("fetch stats: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			stats, err := client.Decode[dashboardStats](resp)
			if err != nil {
				return fmt.Errorf("decode stats: %w", err)
			}

			if err := workspace.RenderOverview(cmd.OutOrStdout(), workspace.OverviewOptions{
				Width:   ui.TerminalWidth(),
				Version: client.Version,
				Account: statusAccount(ctx),
				Areas:   uiCatalog(cmd.Root()).Children,
				Counts: map[string]int{
					workspace.AreaServers:   stats.ServerCount,
					workspace.AreaSites:     stats.SiteCount,
					workspace.AreaDatabases: stats.DatabaseCount,
				},
				Mode:    ui.CurrentMode(),
				NoColor: ui.IsPlain(),
			}); err != nil {
				return fmt.Errorf("render overview: %w", err)
			}
			return nil
		},
	}
}

// An API-key login caches an empty user, so the account line reads the live
// identity in that case. It is decoration: a failed lookup hides the row
// rather than failing the overview.
func statusAccount(ctx context.Context) string {
	if cfg.User != nil && hasAuthenticatedIdentity(*cfg.User) {
		return cfg.User.DisplayName()
	}
	resp, err := apiClient.Get(ctx, client.PathAuthMe, nil)
	if err != nil {
		return ""
	}
	user, err := decodeAuthenticatedUser(resp)
	if err != nil {
		return ""
	}
	return user.DisplayName()
}
