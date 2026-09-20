package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type Server struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Provider    string   `json:"provider"`
	Region      string   `json:"region"`
	Size        string   `json:"size"`
	Tier        string   `json:"tier"`
	SizeLabel   string   `json:"sizeLabel"`
	IPv4        string   `json:"ipv4"`
	InstalledSW []string `json:"installed_software"`
	CreatedAt   string   `json:"created_at"`
}

func (s *Server) UnmarshalJSON(data []byte) error {
	type legacy Server
	var decoded legacy
	wire := struct {
		*legacy
		CreatedAt *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode server: %w", err)
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*s = Server(decoded)
	return nil
}

// displaySize prefers the catalog-resolved label, falling back to the raw size
// slug (or legacy tier) when the size can't be resolved.
func (s Server) displaySize() string {
	if s.SizeLabel != "" {
		return s.SizeLabel
	}
	if s.Size != "" {
		return s.Size
	}
	return s.Tier
}

type Region struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	City     string `json:"city"`
	Country  string `json:"country"`
}

type CatalogSKU struct {
	SKU                string `json:"sku"`
	Provider           string `json:"provider"`
	Region             string `json:"region"`
	CPU                int    `json:"cpu"`
	MemoryMB           int    `json:"memoryMb"`
	DiskGB             int    `json:"diskGb"`
	PricePerMonthCents int    `json:"pricePerMonthCents"`
}

type catalogResponse struct {
	SKUs []CatalogSKU `json:"skus"`
}

// sizeLabel renders a catalog SKU as a clean one-line picker option, e.g.
// "2 GB RAM · 1 vCPU · 50 GB SSD · $15.00/mo". The RAM/vCPU lead mirrors the
// server's domain.SizeLabel.
func sizeLabel(s CatalogSKU) string {
	mem := fmt.Sprintf("%d MB RAM", s.MemoryMB)
	if s.MemoryMB >= 1024 {
		mem = fmt.Sprintf("%d GB RAM", s.MemoryMB/1024)
	}
	return fmt.Sprintf("%s · %d vCPU · %d GB SSD · $%.2f/mo",
		mem, s.CPU, s.DiskGB, float64(s.PricePerMonthCents)/100)
}

func newServersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "servers",
		Aliases: []string{"server", "srv"},
		Short:   "Manage cloud servers",
	}

	cmd.AddCommand(newServersListCmd())
	cmd.AddCommand(newServersCreateCmd())
	cmd.AddCommand(newServersInfoCmd())
	cmd.AddCommand(newServersDeleteCmd())
	cmd.AddCommand(newServersRebootCmd())
	cmd.AddCommand(newServersSSHCmd())
	cmd.AddCommand(newServersCredentialsCmd())
	cmd.AddCommand(newServersEventsCmd())

	return cmd
}

// Only NAME and SIZE may shrink; the other columns have a known shape.
var serverListFixedColumns = []bool{false, true, true, true, false, true}

const serverStatusColumn = 1

func newServersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List your servers",
		Example: `  dcs servers list
  dcs servers list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathServers, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			servers, err := client.Decode[[]Server](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			headers := []string{"Name", "Status", "Provider", "Region", "Size", "IPv4"}
			rows := make([][]string, len(servers))
			for i, s := range servers {
				rows[i] = []string{s.Name, s.Status, s.Provider, s.Region, s.displaySize(), s.IPv4}
			}

			if ui.PrintFormatted(resp.Data, headers, rows) {
				return nil
			}

			if len(servers) == 0 {
				fmt.Println()
				ui.PrintInfo("No servers yet. Run 'dcs servers create' to provision one.")
				fmt.Println()
				return nil
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, serverStatusColumn), serverListFixedColumns)
			return nil
		},
	}
}

func newServersCreateCmd() *cobra.Command {
	var (
		name     string
		provider string
		region   string
		sku      string
		tier     string
		wait     bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Provision a new server",
		Example: `  dcs servers create
  dcs servers create --name web-1 --provider hetzner --region fsn1 --sku cx22
  dcs servers create --name api --provider digitalocean --region nyc1 --sku s-1vcpu-1gb --wait`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			var server *Server
			var err error
			if name != "" && provider != "" && region != "" && (sku != "" || tier != "") {
				server, err = provisionServer(name, provider, region, sku, tier)
			} else {
				server, err = createServerInteractive()
			}
			if err != nil {
				return fmt.Errorf("createServerInteractive: %w", err)
			}

			if wait && server != nil {
				return waitForCondition("Waiting for server to be ready...", func() (bool, error) {
					resp, err := apiClient.Get(context.Background(), client.PathServer(server.ID), nil)
					if err != nil {
						return false, err
					}
					s, err := client.Decode[Server](resp)
					if err != nil {
						return false, err
					}
					switch s.Status {
					case "active", "running":
						return true, nil
					case "error", "failed":
						return false, fmt.Errorf("server provisioning failed")
					}
					return false, nil
				})
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Server name")
	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider (hetzner, digitalocean, vultr)")
	cmd.Flags().StringVar(&region, "region", "", "Region slug")
	cmd.Flags().StringVar(&sku, "sku", "", "Catalog size SKU")
	cmd.Flags().StringVar(&tier, "tier", "", "Legacy tier slug (deprecated; use --sku)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for server to be ready")

	return cmd
}

func createServerInteractive() (*Server, error) {
	ctx := context.Background()

	name, err := ui.Input("Server name", "my-server")
	if err != nil {
		return nil, err
	}

	regResp, err := apiClient.Get(ctx, client.PathProvisionerRegions, nil)
	if err != nil {
		return nil, err
	}
	regions, err := client.Decode[[]Region](regResp)
	if err != nil {
		return nil, err
	}

	providers := make(map[string]bool)
	for _, r := range regions {
		providers[r.Provider] = true
	}
	providerOpts := make([]huh.Option[string], 0)
	for p := range providers {
		label := strings.Title(p)
		providerOpts = append(providerOpts, huh.NewOption(label, p))
	}

	provider, err := ui.SelectOption("Cloud provider", providerOpts)
	if err != nil {
		return nil, err
	}

	regionOpts := make([]huh.Option[string], 0)
	for _, r := range regions {
		if r.Provider == provider {
			label := fmt.Sprintf("%s (%s, %s)", r.Name, r.City, r.Country)
			regionOpts = append(regionOpts, huh.NewOption(label, r.Slug))
		}
	}

	region, err := ui.SelectOption("Region", regionOpts)
	if err != nil {
		return nil, err
	}

	sizeResp, err := apiClient.Get(ctx, client.PathCatalog, url.Values{
		"provider": {provider},
		"region":   {region},
	})
	if err != nil {
		return nil, err
	}
	catalog, err := client.Decode[catalogResponse](sizeResp)
	if err != nil {
		return nil, err
	}
	if len(catalog.SKUs) == 0 {
		return nil, fmt.Errorf("no server sizes available for %s in %s", provider, region)
	}

	sizeOpts := make([]huh.Option[string], 0, len(catalog.SKUs))
	sizeLabels := make(map[string]string, len(catalog.SKUs))
	for _, s := range catalog.SKUs {
		label := sizeLabel(s)
		sizeLabels[s.SKU] = label
		sizeOpts = append(sizeOpts, huh.NewOption(label, s.SKU))
	}

	sku, err := ui.SelectOption("Server size", sizeOpts)
	if err != nil {
		return nil, err
	}

	fmt.Println()
	ui.PrintKeyValue("Name", name)
	ui.PrintKeyValue("Provider", provider)
	ui.PrintKeyValue("Region", region)
	ui.PrintKeyValue("Size", sizeLabels[sku])
	fmt.Println()

	confirmed, err := ui.Confirm("Create this server?")
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, nil
	}

	return provisionServer(name, provider, region, sku, "")
}

func provisionServer(name, provider, region, sku, tier string) (*Server, error) {
	var server Server
	err := ui.RunWithSpinner("Provisioning server...", func() error {
		body := map[string]string{
			"name":     name,
			"provider": provider,
			"region":   region,
		}
		if sku != "" {
			body["sku"] = sku
		} else if tier != "" {
			body["tier"] = tier
		}
		resp, err := apiClient.Post(context.Background(), client.PathServers, body)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		s, err := client.Decode[Server](resp)
		if err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		server = s
		return nil
	})
	if err != nil {
		return nil, err
	}

	if jsonOutput {
		ui.PrintJSON(server)
		return &server, nil
	}

	ui.PrintSuccess(fmt.Sprintf("Server %s created!", ui.Bold.Render(server.Name)))
	ui.PrintKeyValue("ID", server.ID)
	ui.PrintKeyValue("Provider", server.Provider)
	ui.PrintKeyValue("Region", server.Region)
	if server.IPv4 != "" {
		ui.PrintKeyValue("IPv4", server.IPv4)
	}
	fmt.Printf("\n  %s\n\n", ui.Muted.Render("Run 'dcs ssh "+server.Name+"' to connect once ready."))
	return &server, nil
}

func newServersInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "info <name-or-id>",
		Short:             "Show server details",
		Example:           "  dcs servers info web-1",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := resolveServer(args[0])
			if err != nil {
				return fmt.Errorf("resolveServer: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathServer(server.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			detail, err := client.Decode[Server](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			ui.PrintSection(detail.Name)
			ui.PrintKeyValue("ID", detail.ID)
			ui.PrintKeyValue("Status", ui.Status(detail.Status))
			ui.PrintKeyValue("Provider", detail.Provider)
			ui.PrintKeyValue("Region", detail.Region)
			ui.PrintKeyValue("Size", detail.displaySize())
			ui.PrintKeyValue("IPv4", detail.IPv4)
			if len(detail.InstalledSW) > 0 {
				ui.PrintKeyValue("Software", strings.Join(detail.InstalledSW, ", "))
			}
			ui.PrintKeyValue("Created", detail.CreatedAt)
			fmt.Println()
			return nil
		},
	}
}

func newServersDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "delete <name-or-id>",
		Short:             "Delete a server",
		Example:           "  dcs servers delete web-1\n  dcs servers delete web-1 --force",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := resolveServer(args[0])
			if err != nil {
				return fmt.Errorf("resolveServer: %w", err)
			}

			if jsonOutput {
				return deleteJSON(cmd.Context(), client.PathServer(server.ID), server.ID, force)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete server %s? This cannot be undone.", ui.Bold.Render(server.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting server...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathServer(server.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Server %s deleted.", server.Name))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newServersRebootCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "reboot <name-or-id>",
		Short:             "Reboot a server",
		Example:           "  dcs servers reboot web-1",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := resolveServer(args[0])
			if err != nil {
				return fmt.Errorf("resolveServer: %w", err)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Reboot server %s?", ui.Bold.Render(server.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Rebooting server...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathServerReboot(server.ID), nil)
				if err != nil {
					return fmt.Errorf("reboot: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Server %s is rebooting.", server.Name))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newServersSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "ssh <name-or-id>",
		Short:             "SSH into a server",
		Example:           "  dcs servers ssh web-1",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}
			return sshIntoServer(args[0])
		},
	}
}

func newServersCredentialsCmd() *cobra.Command {
	var show bool
	cmd := &cobra.Command{
		Use:               "credentials <name-or-id>",
		Short:             "Show server credentials",
		Example:           "  dcs servers credentials web-1 --show",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := resolveServer(args[0])
			if err != nil {
				return fmt.Errorf("resolveServer: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathServerCredentials(server.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			creds, err := client.Decode[map[string]string](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			ui.PrintSection("Credentials for " + server.Name)
			for k, v := range creds {
				if show {
					ui.PrintKeyValue(k, v)
				} else {
					ui.PrintKeyValue(k, "••••••••")
				}
			}
			if !show {
				fmt.Printf("\n  %s\n", ui.Muted.Render("Use --show to reveal values."))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().BoolVar(&show, "show", false, "Reveal credential values")
	return cmd
}

func newServersEventsCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "events <name-or-id>",
		Short:             "Show server events",
		Example:           "  dcs servers events web-1",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			server, err := resolveServer(args[0])
			if err != nil {
				return fmt.Errorf("resolveServer: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathServerEvents(server.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			events, err := client.Decode[[]struct {
				Type      string `json:"type"`
				Message   string `json:"message"`
				CreatedAt string `json:"created_at"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(events) == 0 {
				ui.PrintInfo("No events found.")
				return nil
			}

			fmt.Println()
			for _, e := range events {
				icon := ui.Muted.Render("●")
				switch e.Type {
				case "error":
					icon = ui.Error.Render("●")
				case "success", "completed":
					icon = ui.Success.Render("●")
				case "progress":
					icon = ui.Warning.Render("●")
				}
				fmt.Printf("  %s %s  %s\n", icon, e.CreatedAt, e.Message)
			}
			fmt.Println()
			return nil
		},
	}
}

// resolveServer finds a server by name or ID.
func resolveServer(nameOrID string) (*Server, error) {
	resp, err := apiClient.Get(context.Background(), client.PathServers, nil)
	if err != nil {
		return nil, err
	}

	servers, err := client.Decode[[]Server](resp)
	if err != nil {
		return nil, err
	}

	for _, s := range servers {
		if strings.EqualFold(s.Name, nameOrID) {
			return &s, nil
		}
	}

	for _, s := range servers {
		if s.ID == nameOrID || strings.HasPrefix(s.ID, nameOrID) {
			return &s, nil
		}
	}

	return nil, client.ErrServerNotFound
}

// sshIntoServer resolves a server and starts an SSH session.
func sshIntoServer(nameOrID string) error {
	server, err := resolveServer(nameOrID)
	if err != nil {
		return fmt.Errorf("resolveServer: %w", err)
	}

	if server.IPv4 == "" {
		return fmt.Errorf("server %s has no IPv4 address yet", server.Name)
	}

	fmt.Printf("  %s %s@%s\n\n", ui.Muted.Render("Connecting to"), client.SSHUser, server.IPv4)

	sshCmd := exec.Command("ssh", client.SSHUser+"@"+server.IPv4)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	return sshCmd.Run()
}

// selectServer prompts the user to pick a server if multiple exist, or uses the only one.
func selectServer() (*Server, error) {
	resp, err := apiClient.Get(context.Background(), client.PathServers, nil)
	if err != nil {
		return nil, err
	}

	servers, err := client.Decode[[]Server](resp)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, &client.CLIError{
			Message:    "No servers found.",
			Suggestion: "Run 'dcs servers create' to provision one.",
		}
	}

	if len(servers) == 1 {
		return &servers[0], nil
	}

	opts := make([]huh.Option[string], len(servers))
	serverMap := make(map[string]*Server)
	for i, s := range servers {
		label := fmt.Sprintf("%s (%s, %s)", s.Name, s.Provider, s.Region)
		opts[i] = huh.NewOption(label, s.ID)
		srv := s
		serverMap[s.ID] = &srv
	}

	selected, err := ui.SelectOption("Select a server", opts)
	if err != nil {
		return nil, err
	}

	return serverMap[selected], nil
}

// getServerID returns a server ID from --server flag, .dcs.json, or interactive selection.
func getServerID(serverFlag string) (string, error) {
	if serverFlag != "" {
		server, err := resolveServer(serverFlag)
		if err != nil {
			return "", err
		}
		return server.ID, nil
	}

	// Try .dcs.json
	cwd, _ := os.Getwd()
	if pc, err := loadProjectConfigQuiet(cwd); err == nil && pc.ServerID != "" {
		return pc.ServerID, nil
	}

	// Try default
	if cfg.DefaultServerID != "" {
		return cfg.DefaultServerID, nil
	}

	// Interactive selection
	server, err := selectServer()
	if err != nil {
		return "", err
	}
	return server.ID, nil
}

func loadProjectConfigQuiet(dir string) (*struct {
	ServerID string `json:"server_id"`
	SiteID   string `json:"site_id"`
}, error) {
	data, err := os.ReadFile(dir + "/.dcs.json")
	if err != nil {
		return nil, err
	}
	var pc struct {
		ServerID string `json:"server_id"`
		SiteID   string `json:"site_id"`
	}
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, err
	}
	return &pc, nil
}
