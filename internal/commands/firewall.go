package commands

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type FirewallStatus struct {
	Enabled bool           `json:"enabled"`
	Rules   []FirewallRule `json:"rules"`
}

type FirewallRule struct {
	Number    int    `json:"number"`
	Action    string `json:"action"`
	Direction string `json:"direction"`
	To        string `json:"to"`
	From      string `json:"from"`
	Comment   string `json:"comment"`
}

func newFirewallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "firewall",
		Aliases: []string{"fw"},
		Short:   "Manage server firewall",
	}

	cmd.AddCommand(newFirewallListCmd())
	cmd.AddCommand(newFirewallAddCmd())
	cmd.AddCommand(newFirewallRemoveCmd())
	cmd.AddCommand(newFirewallToggleCmd())

	return cmd
}

func newFirewallListCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List firewall rules",
		Example: "  dcs firewall list\n  dcs firewall list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathFirewall(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			status, err := client.Decode[FirewallStatus](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			fmt.Println()
			firewall := ui.StatusDisabled
			if status.Enabled {
				firewall = ui.StatusEnabled
			}
			ui.PrintKeyValue("Firewall", ui.Status(firewall))

			if len(status.Rules) == 0 {
				fmt.Println()
				ui.PrintInfo("No firewall rules configured.")
				fmt.Println()
				return nil
			}

			headers := []string{"Number", "Action", "To", "From", "Comment"}
			rows := make([][]string, len(status.Rules))
			for i, r := range status.Rules {
				rows[i] = []string{
					strconv.Itoa(r.Number),
					r.Action,
					r.To,
					r.From,
					r.Comment,
				}
			}

			fmt.Println()
			ui.PrintTable(headers, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newFirewallAddCmd() *cobra.Command {
	var (
		serverFlag string
		port       string
		protocol   string
		action     string
		from       string
		comment    string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"create"},
		Short:   "Add a firewall rule",
		Example: "  dcs firewall add --port 443\n  dcs firewall add --port 3306 --from 10.0.0.0/8 --comment 'DB access'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			body := map[string]string{
				"action":   action,
				"port":     port,
				"protocol": protocol,
			}
			if from != "" {
				body["from_ip"] = from
			}
			if comment != "" {
				body["comment"] = comment
			}

			err = ui.RunWithSpinner("Adding firewall rule...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathFirewallRules(serverID), body)
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Firewall rule added: %s %s/%s", action, port, protocol))
			return nil
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&port, "port", "", "Port number or range (e.g. 80, 8080:8090)")
	cmd.Flags().StringVar(&protocol, "protocol", "tcp", "Protocol (tcp, udp)")
	cmd.Flags().StringVar(&action, "action", "allow", "Action (allow, deny)")
	cmd.Flags().StringVar(&from, "from", "", "Source IP or CIDR")
	cmd.Flags().StringVar(&comment, "comment", "", "Rule comment")
	_ = cmd.MarkFlagRequired("port")
	return cmd
}

func newFirewallRemoveCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)

	cmd := &cobra.Command{
		Use:     "rm <rule-number>",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove a firewall rule",
		Example: "  dcs firewall rm 3\n  dcs firewall rm 3 --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			ruleNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid rule number: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete firewall rule %d?", ruleNumber))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Removing firewall rule...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathFirewallRule(serverID, ruleNumber))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Firewall rule %d removed.", ruleNumber))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newFirewallToggleCmd() *cobra.Command {
	var (
		serverFlag string
		enable     bool
		disable    bool
	)

	cmd := &cobra.Command{
		Use:     "toggle",
		Short:   "Enable or disable the firewall",
		Example: "  dcs firewall toggle\n  dcs firewall toggle --enable\n  dcs firewall toggle --disable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			var enabled bool
			if enable {
				enabled = true
			} else if disable {
				enabled = false
			} else {
				resp, err := apiClient.Get(context.Background(), client.PathFirewall(serverID), nil)
				if err != nil {
					return fmt.Errorf("fetch: %w", err)
				}
				status, err := client.Decode[FirewallStatus](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}

				action := "enable"
				if status.Enabled {
					action = "disable"
				}

				confirmed, err := ui.Confirm(fmt.Sprintf("Firewall is currently %s. %s it?",
					formatFirewallState(status.Enabled), action))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
				enabled = !status.Enabled
			}

			err = ui.RunWithSpinner("Updating firewall...", func() error {
				_, err := apiClient.Put(context.Background(), client.PathFirewallToggle(serverID), map[string]bool{
					"enabled": enabled,
				})
				if err != nil {
					return fmt.Errorf("update: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			if enabled {
				ui.PrintSuccess("Firewall enabled.")
			} else {
				ui.PrintSuccess("Firewall disabled.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the firewall")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the firewall")
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
	return cmd
}

func formatFirewallState(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
