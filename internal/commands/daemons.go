package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type Daemon struct {
	ID          string   `json:"id"`
	ServerID    string   `json:"server_id"`
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Directory   string   `json:"directory"`
	RunAsUser   string   `json:"run_as_user"`
	NumProcs    int      `json:"num_procs"`
	Autostart   bool     `json:"autostart"`
	Autorestart string   `json:"autorestart"`
	EnvVars     []string `json:"env_vars"`
	Status      string   `json:"status"`
	CreatedAt   string   `json:"created_at"`
}

type DaemonLogs struct {
	Logs  string `json:"logs"`
	Lines int    `json:"lines"`
}

func newDaemonsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "daemon",
		Aliases: []string{"daemons", "process"},
		Short:   "Manage background processes",
	}

	cmd.AddCommand(newDaemonListCmd())
	cmd.AddCommand(newDaemonCreateCmd())
	cmd.AddCommand(newDaemonDeleteCmd())
	cmd.AddCommand(newDaemonStartCmd())
	cmd.AddCommand(newDaemonStopCmd())
	cmd.AddCommand(newDaemonRestartCmd())
	cmd.AddCommand(newDaemonLogsCmd())

	return cmd
}

const daemonStatusColumn = 2

// COMMAND is free text and the only column that shrinks.
var daemonListFixedColumns = []bool{true, false, true, true, true}

func newDaemonListCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List daemons",
		Example: "  dcs daemon list\n  dcs daemon list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDaemons(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			daemons, err := client.Decode[[]Daemon](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(daemons) == 0 {
				fmt.Println()
				ui.PrintInfo("No daemons yet. Run 'dcs daemon create' to add one.")
				fmt.Println()
				return nil
			}

			headers := []string{"Name", "Command", "Status", "Procs", "User"}
			rows := make([][]string, len(daemons))
			for i, d := range daemons {
				rows[i] = []string{
					d.Name,
					d.Command,
					d.Status,
					strconv.Itoa(d.NumProcs),
					d.RunAsUser,
				}
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, daemonStatusColumn), daemonListFixedColumns)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDaemonCreateCmd() *cobra.Command {
	var (
		serverFlag string
		name       string
		command    string
		directory  string
		user       string
		procs      int
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new daemon",
		Example: "  dcs daemon create\n  dcs daemon create --name my-worker --command 'node worker.js' --procs 2",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if name == "" {
				name, err = ui.Input("Daemon name", "my-worker")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if command == "" {
				command, err = ui.Input("Command", "node worker.js")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			var daemon Daemon
			err = ui.RunWithSpinner("Creating daemon...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathDaemons(serverID), map[string]any{
					"name":        name,
					"command":     command,
					"directory":   directory,
					"run_as_user": user,
					"num_procs":   procs,
				})
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				d, err := client.Decode[Daemon](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				daemon = d
				return nil
			})
			if err != nil {
				return fmt.Errorf("create daemon: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(daemon)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Daemon %s created!", ui.Bold.Render(daemon.Name)))
			ui.PrintKeyValue("Command", daemon.Command)
			ui.PrintKeyValue("Status", daemon.Status)
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&name, "name", "", "Daemon name")
	cmd.Flags().StringVar(&command, "command", "", "Command to run")
	cmd.Flags().StringVar(&directory, "directory", "", "Working directory")
	cmd.Flags().StringVar(&user, "user", "dilmune", "User to run as")
	cmd.Flags().IntVar(&procs, "procs", 1, "Number of processes")
	return cmd
}

func newDaemonDeleteCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:     "delete <name-or-id>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a daemon",
		Example: "  dcs daemon delete my-worker\n  dcs daemon delete my-worker --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			daemon, err := resolveDaemon(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDaemon: %w", err)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete daemon %s?", ui.Bold.Render(daemon.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting daemon...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathDaemon(serverID, daemon.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Daemon %s deleted.", daemon.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "start <name-or-id>",
		Short:   "Start a daemon",
		Example: "  dcs daemon start my-worker",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			daemon, err := resolveDaemon(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDaemon: %w", err)
			}

			err = ui.RunWithSpinner("Starting daemon...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathDaemonAction(serverID, daemon.ID, "start"), nil)
				if err != nil {
					return fmt.Errorf("start: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Daemon %s started.", daemon.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDaemonStopCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "stop <name-or-id>",
		Short:   "Stop a daemon",
		Example: "  dcs daemon stop my-worker",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			daemon, err := resolveDaemon(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDaemon: %w", err)
			}

			err = ui.RunWithSpinner("Stopping daemon...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathDaemonAction(serverID, daemon.ID, "stop"), nil)
				if err != nil {
					return fmt.Errorf("stop: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Daemon %s stopped.", daemon.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDaemonRestartCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "restart <name-or-id>",
		Short:   "Restart a daemon",
		Example: "  dcs daemon restart my-worker",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			daemon, err := resolveDaemon(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDaemon: %w", err)
			}

			err = ui.RunWithSpinner("Restarting daemon...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathDaemonAction(serverID, daemon.ID, "restart"), nil)
				if err != nil {
					return fmt.Errorf("restart: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Daemon %s restarted.", daemon.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDaemonLogsCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "logs <name-or-id>",
		Short:   "Show daemon logs",
		Example: "  dcs daemon logs my-worker",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			daemon, err := resolveDaemon(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDaemon: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDaemonLogs(serverID, daemon.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			logs, err := client.Decode[DaemonLogs](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			fmt.Println(logs.Logs)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func resolveDaemon(serverID, nameOrID string) (*Daemon, error) {
	resp, err := apiClient.Get(context.Background(), client.PathDaemons(serverID), nil)
	if err != nil {
		return nil, err
	}

	daemons, err := client.Decode[[]Daemon](resp)
	if err != nil {
		return nil, err
	}

	for _, d := range daemons {
		if strings.EqualFold(d.Name, nameOrID) {
			return &d, nil
		}
	}

	for _, d := range daemons {
		if d.ID == nameOrID || strings.HasPrefix(d.ID, nameOrID) {
			return &d, nil
		}
	}

	return nil, &client.CLIError{
		Message:    fmt.Sprintf("Daemon '%s' not found.", nameOrID),
		Suggestion: "Run 'dcs daemon list' to see your daemons.",
	}
}
