package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type Site struct {
	ID          string `json:"id"`
	Domain      string `json:"domain"`
	ProjectType string `json:"project_type"`
	Status      string `json:"status"`
	SSLEnabled  bool   `json:"ssl_enabled"`
	GitRepo     string `json:"git_repo"`
	GitBranch   string `json:"git_branch"`
	CreatedAt   string `json:"created_at"`
}

// The API uses camel case; keep existing CLI JSON keys for scripts.
func (s *Site) UnmarshalJSON(data []byte) error {
	type legacy Site
	var decoded legacy
	wire := struct {
		*legacy
		ProjectType *string `json:"projectType"`
		SSLEnabled  *bool   `json:"sslEnabled"`
		GitRepo     *string `json:"gitRepoUrl"`
		GitBranch   *string `json:"gitBranch"`
		CreatedAt   *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode site: %w", err)
	}
	if wire.ProjectType != nil {
		decoded.ProjectType = *wire.ProjectType
	}
	if wire.SSLEnabled != nil {
		decoded.SSLEnabled = *wire.SSLEnabled
	}
	if wire.GitRepo != nil {
		decoded.GitRepo = *wire.GitRepo
	}
	if wire.GitBranch != nil {
		decoded.GitBranch = *wire.GitBranch
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*s = Site(decoded)
	return nil
}

type Deployment struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CommitSHA string `json:"commit_sha"`
	Trigger   string `json:"trigger"`
	CreatedAt string `json:"created_at"`
}

func (d *Deployment) UnmarshalJSON(data []byte) error {
	type legacy Deployment
	var decoded legacy
	wire := struct {
		*legacy
		CommitHash *string `json:"commitHash"`
		CreatedAt  *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode deployment: %w", err)
	}
	if wire.CommitHash != nil {
		decoded.CommitSHA = *wire.CommitHash
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*d = Deployment(decoded)
	return nil
}

func newSitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sites",
		Aliases: []string{"site"},
		Short:   "Manage sites & deployments",
	}

	cmd.AddCommand(newSitesListCmd())
	cmd.AddCommand(newSitesCreateCmd())
	cmd.AddCommand(newSitesInfoCmd())
	cmd.AddCommand(newSitesDeleteCmd())
	cmd.AddCommand(newSitesDeployCmd())
	cmd.AddCommand(newSitesDeploymentsCmd())
	cmd.AddCommand(newSitesSSLCmd())
	cmd.AddCommand(newSitesRollbackCmd())
	cmd.AddCommand(newSitesLogsCmd())

	return cmd
}

// Only DOMAIN may shrink; TYPE, STATUS, SSL and GIT BRANCH have a known shape.
var siteListFixedColumns = []bool{false, true, true, true, true}

const (
	siteStatusColumn = 2
	siteSSLColumn    = 3
)

func newSitesListCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sites",
		Example: "  dcs sites list\n  dcs sites list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSites(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			sites, err := client.Decode[[]Site](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			headers := []string{"Domain", "Type", "Status", "SSL", "Git Branch"}
			rows := make([][]string, len(sites))
			for i, s := range sites {
				ssl := ui.StatusOff
				if s.SSLEnabled {
					ssl = ui.StatusActive
				}
				rows[i] = []string{s.Domain, s.ProjectType, s.Status, ssl, s.GitBranch}
			}

			if ui.PrintFormatted(resp.Data, headers, rows) {
				return nil
			}

			if len(sites) == 0 {
				fmt.Println()
				ui.PrintInfo("No sites yet. Run 'dcs sites create' to add one.")
				fmt.Println()
				return nil
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, siteStatusColumn, siteSSLColumn), siteListFixedColumns)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newSitesCreateCmd() *cobra.Command {
	var (
		serverFlag  string
		domain      string
		projectType string
		gitRepo     string
		gitBranch   string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new site",
		Example: "  dcs sites create --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if domain != "" && projectType != "" {
				return createSite(serverID, domain, projectType, gitRepo, gitBranch)
			}

			// Interactive mode
			if domain == "" {
				domain, err = ui.Input("Domain name", "app.example.com")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if projectType == "" {
				typeOpts := []huh.Option[string]{
					huh.NewOption("PHP", "php"),
					huh.NewOption("HTML / Static", "html"),
					huh.NewOption("Node.js", "node"),
					huh.NewOption("Laravel", "laravel"),
					huh.NewOption("WordPress", "wordpress"),
					huh.NewOption("Next.js", "nextjs"),
					huh.NewOption("Nuxt.js", "nuxtjs"),
					huh.NewOption("React", "react"),
					huh.NewOption("Python", "python"),
					huh.NewOption("Symfony", "symfony"),
					huh.NewOption("GitHub Repo", "github"),
				}
				projectType, err = ui.SelectOption("Project type", typeOpts)
				if err != nil {
					return fmt.Errorf("SelectOption: %w", err)
				}
			}

			if gitRepo == "" {
				gitRepo, err = ui.Input("Git repository URL (optional)", "git@github.com:user/repo.git")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if gitBranch == "" && gitRepo != "" {
				gitBranch, err = ui.Input("Git branch", "main")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			return createSite(serverID, domain, projectType, gitRepo, gitBranch)
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&domain, "domain", "", "Domain name")
	cmd.Flags().StringVar(&projectType, "type", "", "Project type")
	cmd.Flags().StringVar(&gitRepo, "repo", "", "Git repository URL")
	cmd.Flags().StringVar(&gitBranch, "branch", "", "Git branch")
	return cmd
}

func createSite(serverID, domain, projectType, gitRepo, gitBranch string) error {
	body := map[string]string{
		"domain":      domain,
		"projectType": projectType,
	}
	if gitRepo != "" {
		body["gitRepoUrl"] = gitRepo
	}
	if gitBranch != "" {
		body["gitBranch"] = gitBranch
	}

	var site Site
	err := ui.RunWithSpinner("Creating site...", func() error {
		resp, err := apiClient.Post(context.Background(), client.PathSites(serverID), body)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		s, err := client.Decode[Site](resp)
		if err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		site = s
		return nil
	})
	if err != nil {
		return fmt.Errorf("create site: %w", err)
	}

	if jsonOutput {
		ui.PrintJSON(site)
		return nil
	}

	ui.PrintSuccess(fmt.Sprintf("Site %s created!", ui.Bold.Render(site.Domain)))
	ui.PrintKeyValue("Type", site.ProjectType)
	ui.PrintKeyValue("Status", site.Status)
	fmt.Printf("\n  %s\n\n", ui.Muted.Render("Run 'dcs sites deploy "+site.Domain+"' to deploy."))
	return nil
}

func newSitesInfoCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:               "info <domain>",
		Short:             "Show site details",
		Example:           "  dcs sites info example.com",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSite(serverID, site.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			detail, err := client.Decode[Site](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			ssl := ui.StatusOff
			if detail.SSLEnabled {
				ssl = ui.StatusActive
			}

			ui.PrintSection(detail.Domain)
			ui.PrintKeyValue("ID", detail.ID)
			ui.PrintKeyValue("Type", detail.ProjectType)
			ui.PrintKeyValue("Status", ui.Status(detail.Status))
			ui.PrintKeyValue("SSL", ui.Status(ssl))
			if detail.GitRepo != "" {
				ui.PrintKeyValue("Repository", detail.GitRepo)
				ui.PrintKeyValue("Branch", detail.GitBranch)
			}
			ui.PrintKeyValue("Created", detail.CreatedAt)
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newSitesDeleteCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:               "delete <domain>",
		Short:             "Delete a site",
		Example:           "  dcs sites delete example.com\n  dcs sites delete example.com --force",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			if jsonOutput {
				return deleteJSON(cmd.Context(), client.PathSite(serverID, site.ID), site.ID, force)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete site %s?", ui.Bold.Render(site.Domain)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting site...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathSite(serverID, site.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Site %s deleted.", site.Domain))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newSitesDeployCmd() *cobra.Command {
	var (
		serverFlag string
		branch     string
		wait       bool
	)
	cmd := &cobra.Command{
		Use:               "deploy <domain>",
		Short:             "Deploy a site",
		Example:           "  dcs sites deploy example.com\n  dcs sites deploy example.com --branch staging",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			body := map[string]string{}
			if branch != "" {
				body["branch"] = branch
			}

			var deployment Deployment
			err = ui.RunWithSpinner("Deploying "+site.Domain+"...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathSiteDeploy(serverID, site.ID), body)
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				d, err := client.Decode[Deployment](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				deployment = d
				return nil
			})
			if err != nil {
				return fmt.Errorf("deploy site: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(deployment)
			} else {
				ui.PrintSuccess(fmt.Sprintf("Deployment triggered for %s", ui.Bold.Render(site.Domain)))
				ui.PrintKeyValue("Deployment", deployment.ID)
				ui.PrintKeyValue("Status", deployment.Status)
			}

			if wait {
				err = waitForCondition("Waiting for deployment to complete...", func() (bool, error) {
					resp, err := apiClient.Get(context.Background(), client.PathDeployment(serverID, site.ID, deployment.ID), nil)
					if err != nil {
						return false, err
					}
					d, err := client.Decode[Deployment](resp)
					if err != nil {
						return false, err
					}
					switch d.Status {
					case "deployed", "success", "completed":
						return true, nil
					case "failed", "error":
						return false, fmt.Errorf("deployment failed")
					}
					return false, nil
				})
				if err != nil {
					return fmt.Errorf("wait for deployment: %w", err)
				}
				ui.PrintSuccess("Deployment complete!")
			} else if !jsonOutput {
				fmt.Printf("\n  %s\n\n", ui.Muted.Render("Run 'dcs logs --follow' to watch progress, or use --wait."))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&branch, "branch", "", "Git branch to deploy")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for deployment to complete")
	return cmd
}

const deploymentStatusColumn = 1

// Every deployment column has a known shape; none shrinks.
var deploymentListFixedColumns = []bool{true, true, true, true, true}

func newSitesDeploymentsCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:               "deployments <domain>",
		Aliases:           []string{"deploys"},
		Short:             "List deployment history",
		Example:           "  dcs sites deployments example.com",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSiteDeployments(serverID, site.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			deployments, err := client.Decode[[]Deployment](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(deployments) == 0 {
				ui.PrintInfo("No deployments yet.")
				return nil
			}

			headers := []string{"ID", "Status", "Commit", "Trigger", "Created"}
			rows := make([][]string, len(deployments))
			for i, d := range deployments {
				sha := d.CommitSHA
				if len(sha) > 7 {
					sha = sha[:7]
				}
				rows[i] = []string{
					d.ID[:8],
					d.Status,
					sha,
					d.Trigger,
					d.CreatedAt,
				}
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, deploymentStatusColumn), deploymentListFixedColumns)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newSitesSSLCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:               "ssl <domain>",
		Short:             "Enable SSL/TLS for a site",
		Example:           "  dcs sites ssl example.com",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			if site.SSLEnabled {
				ui.PrintInfo(fmt.Sprintf("SSL is already enabled for %s.", site.Domain))
				return nil
			}

			err = ui.RunWithSpinner("Enabling SSL for "+site.Domain+"...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathSiteSSL(serverID, site.ID), map[string]bool{
					"auto_renew": true,
				})
				if err != nil {
					return fmt.Errorf("enable ssl: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("SSL enabled for %s", ui.Bold.Render(site.Domain)))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newSitesRollbackCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:               "rollback <domain> <deployment-id>",
		Short:             "Rollback to a previous deployment",
		Example:           "  dcs sites rollback example.com abc12345",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			deploymentID := args[1]

			confirmed, err := ui.Confirm(fmt.Sprintf("Rollback %s to deployment %s?", ui.Bold.Render(site.Domain), deploymentID[:8]))
			if err != nil {
				return fmt.Errorf("confirm: %w", err)
			}
			if !confirmed {
				return nil
			}

			var deployment Deployment
			err = ui.RunWithSpinner("Rolling back...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathDeploymentRollback(serverID, site.ID, deploymentID), nil)
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				d, err := client.Decode[Deployment](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				deployment = d
				return nil
			})
			if err != nil {
				return fmt.Errorf("rollback deployment: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(deployment)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Rollback triggered for %s", ui.Bold.Render(site.Domain)))
			ui.PrintKeyValue("Deployment", deployment.ID)
			ui.PrintKeyValue("Status", deployment.Status)
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newSitesLogsCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:               "logs <domain> <deployment-id>",
		Short:             "Show deployment build logs",
		Example:           "  dcs sites logs example.com abc12345",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeSiteNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			site, err := resolveSite(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveSite: %w", err)
			}

			deploymentID := args[1]

			resp, err := apiClient.Get(context.Background(), client.PathDeploymentLogs(serverID, site.ID, deploymentID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			logs, err := client.Decode[struct {
				Lines []struct {
					Timestamp string `json:"timestamp"`
					Line      string `json:"line"`
				} `json:"lines"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(logs.Lines) == 0 {
				ui.PrintInfo("No build logs available yet.")
				return nil
			}

			fmt.Println()
			for _, l := range logs.Lines {
				fmt.Printf("  %s  %s\n", l.Timestamp, l.Line)
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

// resolveSite finds a site by domain name or ID.
func resolveSite(serverID, domainOrID string) (*Site, error) {
	resp, err := apiClient.Get(context.Background(), client.PathSites(serverID), nil)
	if err != nil {
		return nil, err
	}

	sites, err := client.Decode[[]Site](resp)
	if err != nil {
		return nil, err
	}

	for _, s := range sites {
		if strings.EqualFold(s.Domain, domainOrID) || s.ID == domainOrID || strings.HasPrefix(s.ID, domainOrID) {
			return &s, nil
		}
	}

	return nil, client.ErrSiteNotFound
}
