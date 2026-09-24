package commands

import (
	"context"
	"fmt"
	"os"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh [server-name]",
		Short: "SSH into a server",
		Long:  "Opens an SSH session. Uses .dcs.json context if no server specified.",
		Example: `  dcs ssh web-1
  dcs ssh`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			if len(args) > 0 {
				return sshIntoServer(args[0])
			}

			cwd, _ := os.Getwd()
			pc, err := config.LoadProjectConfig(cwd)
			if err == nil && pc.ServerID != "" {
				return sshIntoServer(pc.ServerID)
			}

			if cfg.DefaultServerID != "" {
				return sshIntoServer(cfg.DefaultServerID)
			}

			server, err := selectServer()
			if err != nil {
				return fmt.Errorf("selectServer: %w", err)
			}

			return sshIntoServer(server.ID)
		},
		ValidArgsFunction: completeServerNames,
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "init",
		Short:   "Link this directory to a DCS project",
		Long:    "Creates a .dcs.json file linking this directory to a server and site.",
		Example: "  dcs init",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := selectServer()
			if err != nil {
				return fmt.Errorf("selectServer: %w", err)
			}

			site, err := selectSite(server.ID)
			if err != nil {
				return fmt.Errorf("selectSite: %w", err)
			}

			cwd, _ := os.Getwd()
			pc := &config.ProjectConfig{
				ServerID:   server.ID,
				ServerName: server.Name,
				SiteID:     site.ID,
				SiteDomain: site.Domain,
			}

			if err := config.SaveProjectConfig(cwd, pc); err != nil {
				return fmt.Errorf("SaveProjectConfig: %w", err)
			}

			ui.PrintSuccess("Project linked!")
			ui.PrintKeyValue("Server", server.Name)
			ui.PrintKeyValue("Site", site.Domain)
			ui.PrintInfo("Created .dcs.json, you can now use 'dcs deploy' from this directory.")

			return nil
		},
	}
}

func selectSite(serverID string) (*Site, error) {
	resp, err := apiClient.Get(context.Background(), client.PathSites(serverID), nil)
	if err != nil {
		return nil, err
	}

	sites, err := client.Decode[[]Site](resp)
	if err != nil {
		return nil, err
	}

	if len(sites) == 0 {
		return nil, &client.CLIError{
			Message:    "No sites on this server.",
			Suggestion: "Run 'dcs sites create' to add one first.",
		}
	}

	if len(sites) == 1 {
		return &sites[0], nil
	}

	opts := make([]huh.Option[string], len(sites))
	siteMap := make(map[string]*Site)
	for i, s := range sites {
		opts[i] = huh.NewOption(s.Domain+" ("+s.ProjectType+")", s.ID)
		site := s
		siteMap[s.ID] = &site
	}

	selected, err := ui.SelectOption("Select a site", opts)
	if err != nil {
		return nil, err
	}

	return siteMap[selected], nil
}
