package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newSoftwareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "software",
		Aliases: []string{"sw"},
		Short:   "Manage server software",
	}

	cmd.AddCommand(newSoftwareInstallCmd())
	cmd.AddCommand(newSoftwareUninstallCmd())
	cmd.AddCommand(newSoftwareListCmd())

	return cmd
}

func newSoftwareInstallCmd() *cobra.Command {
	var (
		serverFlag string
		version    string
	)

	cmd := &cobra.Command{
		Use:     "install [software]",
		Short:   "Install software on a server",
		Example: "  dcs software install\n  dcs software install node --version 20\n  dcs software install redis --server web-1",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			var name string
			if len(args) > 0 {
				name = args[0]
			} else {
				softwareOptions := []huh.Option[string]{
					huh.NewOption("Node.js", "node"),
					huh.NewOption("PHP", "php"),
					huh.NewOption("Python", "python"),
					huh.NewOption("Go", "go"),
					huh.NewOption("Ruby", "ruby"),
					huh.NewOption("Java", "java"),
					huh.NewOption("Rust", "rust"),
					huh.NewOption("Deno", "deno"),
					huh.NewOption(".NET", "dotnet"),
					huh.NewOption("Bun", "bun"),
					huh.NewOption("MySQL", "mysql"),
					huh.NewOption("PostgreSQL", "postgres"),
					huh.NewOption("Redis", "redis"),
					huh.NewOption("MariaDB", "mariadb"),
					huh.NewOption("MongoDB", "mongodb"),
					huh.NewOption("SQLite", "sqlite"),
					huh.NewOption("Memcached", "memcached"),
					huh.NewOption("Docker", "docker"),
					huh.NewOption("Composer", "composer"),
					huh.NewOption("NPM", "npm"),
					huh.NewOption("Supervisor", "supervisor"),
					huh.NewOption("Certbot (Let's Encrypt)", "certbot"),
					huh.NewOption("Fail2Ban", "fail2ban"),
					huh.NewOption("PM2", "pm2"),
				}

				name, err = ui.SelectOption("Select software to install", softwareOptions)
				if err != nil {
					return fmt.Errorf("SelectOption: %w", ui.ExplainNonInteractive(err, "Name the software to install, for example 'dcs software install node'."))
				}
			}

			body := map[string]string{"software": name}
			if version != "" {
				body["version"] = version
			}

			err = ui.RunWithSpinner(fmt.Sprintf("Installing %s...", name), func() error {
				_, err := apiClient.Post(context.Background(), client.PathServerInstallSoftware(serverID), body)
				if err != nil {
					return fmt.Errorf("install: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Installation of %s enqueued. Run 'dcs servers events' to track progress.", name))
			return nil
		},
	}

	addServerFlag(cmd, &serverFlag)
	cmd.Flags().StringVar(&version, "version", "", "Software version to install")
	return cmd
}

func newSoftwareUninstallCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)

	cmd := &cobra.Command{
		Use:     "uninstall <software>",
		Aliases: []string{"remove"},
		Short:   "Uninstall software from a server",
		Example: "  dcs software uninstall redis\n  dcs software uninstall redis --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			name := args[0]

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if !force {
				confirmed, err := confirmForceable(fmt.Sprintf("Uninstall %s from this server?", ui.Bold.Render(name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner(fmt.Sprintf("Uninstalling %s...", name), func() error {
				_, err := apiClient.Post(context.Background(), client.PathServerUninstallSoftware(serverID), map[string]string{
					"software": name,
				})
				if err != nil {
					return fmt.Errorf("uninstall: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Uninstall of %s enqueued.", name))
			return nil
		},
	}

	addServerFlag(cmd, &serverFlag)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newSoftwareListCmd() *cobra.Command {
	var serverFlag string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed software on a server",
		Example: "  dcs software list\n  dcs software list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathServer(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			software, err := client.Decode[serverSoftware](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(software.names())
				return nil
			}

			packages := software.packages()
			if len(packages) == 0 {
				fmt.Println()
				ui.PrintInfo("No software installed. Run 'dcs software install' to add some.")
				fmt.Println()
				return nil
			}

			fmt.Println()
			for _, pkg := range packages {
				fmt.Printf("  %s %s\n", ui.Success.Render("*"), pkg.label())
			}
			fmt.Println()
			return nil
		},
	}

	addServerFlag(cmd, &serverFlag)
	return cmd
}

const (
	softwarePHP      = "php"
	softwareNode     = "node"
	softwareMySQL    = "mysql"
	softwarePostgres = "postgres"
	softwareRedis    = "redis"
)

// serverSoftware is the part of a server response that says what is
// installed. PHP, Node.js, MySQL, PostgreSQL and Redis predate the
// installedSoftware map and keep columns of their own; every other package is
// a map entry whose value is true or the installed version.
type serverSoftware struct {
	Installed       map[string]any `json:"installedSoftware"`
	PHPVersions     []string       `json:"phpVersions"`
	NodeVersion     string         `json:"nodeVersion"`
	InstallMySQL    bool           `json:"installMySQL"`
	InstallPostgres bool           `json:"installPostgres"`
	InstallRedis    bool           `json:"installRedis"`
}

type installedPackage struct {
	name    string
	version string
}

func (p installedPackage) label() string {
	return strings.TrimSpace(p.name + " " + p.version)
}

// packages lists what is installed under the names 'dcs software install'
// and 'uninstall' take, sorted, one entry per PHP version.
func (s serverSoftware) packages() []installedPackage {
	var packages []installedPackage
	for _, version := range s.PHPVersions {
		packages = append(packages, installedPackage{name: softwarePHP, version: version})
	}
	if s.NodeVersion != "" {
		packages = append(packages, installedPackage{name: softwareNode, version: s.NodeVersion})
	}
	for name, installed := range map[string]bool{softwareMySQL: s.InstallMySQL, softwarePostgres: s.InstallPostgres, softwareRedis: s.InstallRedis} {
		if installed {
			packages = append(packages, installedPackage{name: name})
		}
	}
	for name, value := range s.Installed {
		switch value := value.(type) {
		case bool:
			if value {
				packages = append(packages, installedPackage{name: name})
			}
		case string:
			if value != "" {
				packages = append(packages, installedPackage{name: name, version: value})
			}
		}
	}
	slices.SortFunc(packages, func(a, b installedPackage) int {
		return strings.Compare(a.label(), b.label())
	})
	return packages
}

// names is the JSON shape of 'dcs software list': the package names, each
// once, so a script can pass one straight to 'dcs software uninstall'.
func (s serverSoftware) names() []string {
	names := []string{}
	for _, pkg := range s.packages() {
		if !slices.Contains(names, pkg.name) {
			names = append(names, pkg.name)
		}
	}
	return names
}
