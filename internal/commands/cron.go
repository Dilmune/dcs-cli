package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type CronJob struct {
	ID        string `json:"id"`
	ServerID  string `json:"server_id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	Schedule  string `json:"schedule"`
	RunAsUser string `json:"run_as_user"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type CronExecution struct {
	ID          string `json:"id"`
	CronJobID   string `json:"cron_job_id"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	ExitCode    int    `json:"exit_code"`
	Output      string `json:"output"`
	DurationMs  int    `json:"duration_ms"`
}

func newCronCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cron",
		Aliases: []string{"cron-jobs"},
		Short:   "Manage cron jobs",
	}

	cmd.AddCommand(newCronListCmd())
	cmd.AddCommand(newCronCreateCmd())
	cmd.AddCommand(newCronDeleteCmd())
	cmd.AddCommand(newCronRunCmd())
	cmd.AddCommand(newCronHistoryCmd())

	return cmd
}

func newCronListCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List cron jobs",
		Example: "  dcs cron list\n  dcs cron list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathCronJobs(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			jobs, err := client.Decode[[]CronJob](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(jobs) == 0 {
				fmt.Println()
				ui.PrintInfo("No cron jobs configured. Run 'dcs cron create' to add one.")
				fmt.Println()
				return nil
			}

			headers := []string{"Name", "Schedule", "Command", "User", "Enabled", "Created"}
			rows := make([][]string, len(jobs))
			for i, j := range jobs {
				enabled := ui.Muted.Render("no")
				if j.Enabled {
					enabled = ui.Success.Render("yes")
				}
				rows[i] = []string{
					j.Name,
					j.Schedule,
					j.Command,
					j.RunAsUser,
					enabled,
					j.CreatedAt,
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

func newCronCreateCmd() *cobra.Command {
	var (
		serverFlag string
		name       string
		command    string
		schedule   string
		user       string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new cron job",
		Example: "  dcs cron create\n  dcs cron create --name nightly-backup --command '/usr/bin/pg_dump mydb' --schedule '0 2 * * *'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if name == "" {
				name, err = ui.Input("Cron job name", "nightly-backup")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if command == "" {
				command, err = ui.Input("Command", "/usr/bin/php /home/dilmune/artisan schedule:run")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if schedule == "" {
				schedule, err = ui.Input("Schedule (cron expression)", "0 * * * *")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			var job CronJob
			err = ui.RunWithSpinner("Creating cron job...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathCronJobs(serverID), map[string]string{
					"name":        name,
					"command":     command,
					"schedule":    schedule,
					"run_as_user": user,
				})
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				j, err := client.Decode[CronJob](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				job = j
				return nil
			})
			if err != nil {
				return fmt.Errorf("create cron job: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(job)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Cron job %s created!", ui.Bold.Render(job.Name)))
			ui.PrintKeyValue("Schedule", job.Schedule)
			ui.PrintKeyValue("Command", job.Command)
			ui.PrintKeyValue("User", job.RunAsUser)
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&name, "name", "", "Cron job name")
	cmd.Flags().StringVar(&command, "command", "", "Command to execute")
	cmd.Flags().StringVar(&schedule, "schedule", "", "Cron schedule expression")
	cmd.Flags().StringVar(&user, "user", "dilmune", "User to run the command as")
	return cmd
}

func newCronDeleteCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:     "delete <name-or-id>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a cron job",
		Example: "  dcs cron delete nightly-backup\n  dcs cron delete nightly-backup --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			cronJob, err := resolveCronJob(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveCronJob: %w", err)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete cron job %s?", ui.Bold.Render(cronJob.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting cron job...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathCronJob(serverID, cronJob.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Cron job %s deleted.", cronJob.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newCronRunCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "run <name-or-id>",
		Short:   "Trigger a cron job manually",
		Example: "  dcs cron run nightly-backup",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			cronJob, err := resolveCronJob(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveCronJob: %w", err)
			}

			err = ui.RunWithSpinner("Triggering cron job...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathCronJobRun(serverID, cronJob.ID), nil)
				if err != nil {
					return fmt.Errorf("trigger: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Cron job %s triggered.", ui.Bold.Render(cronJob.Name)))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newCronHistoryCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "history <name-or-id>",
		Aliases: []string{"executions"},
		Short:   "Show cron job execution history",
		Example: "  dcs cron history nightly-backup",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			cronJob, err := resolveCronJob(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveCronJob: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathCronJobExecutions(serverID, cronJob.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			executions, err := client.Decode[[]CronExecution](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(executions) == 0 {
				ui.PrintInfo("No executions yet.")
				return nil
			}

			headers := []string{"Started", "Duration", "Exit Code", "Output"}
			rows := make([][]string, len(executions))
			for i, e := range executions {
				output := e.Output
				if len(output) > 50 {
					output = output[:50] + "..."
				}
				duration := fmt.Sprintf("%dms", e.DurationMs)
				exitCode := fmt.Sprintf("%d", e.ExitCode)
				rows[i] = []string{
					e.StartedAt,
					duration,
					exitCode,
					output,
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

func resolveCronJob(serverID, nameOrID string) (*CronJob, error) {
	resp, err := apiClient.Get(context.Background(), client.PathCronJobs(serverID), nil)
	if err != nil {
		return nil, err
	}

	jobs, err := client.Decode[[]CronJob](resp)
	if err != nil {
		return nil, err
	}

	for _, j := range jobs {
		if strings.EqualFold(j.Name, nameOrID) {
			return &j, nil
		}
	}

	for _, j := range jobs {
		if j.ID == nameOrID || strings.HasPrefix(j.ID, nameOrID) {
			return &j, nil
		}
	}

	return nil, &client.CLIError{
		Message:    fmt.Sprintf("Cron job '%s' not found.", nameOrID),
		Suggestion: "Run 'dcs cron list' to see your cron jobs.",
	}
}
