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

type Database struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (d *Database) UnmarshalJSON(data []byte) error {
	type legacy Database
	var decoded legacy
	wire := struct {
		*legacy
		CreatedAt *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode database: %w", err)
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*d = Database(decoded)
	return nil
}

type DBUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Database string `json:"database"`
}

type DBBackup struct {
	ID          string `json:"id"`
	DatabaseID  string `json:"database_id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	SizeBytes   int64  `json:"size_bytes"`
	CompletedAt string `json:"completed_at"`
	CreatedAt   string `json:"created_at"`
}

func (b *DBBackup) UnmarshalJSON(data []byte) error {
	type legacy DBBackup
	var decoded legacy
	wire := struct {
		*legacy
		DatabaseID  *string `json:"databaseId"`
		SizeBytes   *int64  `json:"sizeBytes"`
		CompletedAt *string `json:"completedAt"`
		CreatedAt   *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode database backup: %w", err)
	}
	if wire.DatabaseID != nil {
		decoded.DatabaseID = *wire.DatabaseID
	}
	if wire.SizeBytes != nil {
		decoded.SizeBytes = *wire.SizeBytes
	}
	if wire.CompletedAt != nil {
		decoded.CompletedAt = *wire.CompletedAt
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*b = DBBackup(decoded)
	return nil
}

type QueryResult struct {
	Columns      []string         `json:"columns"`
	Rows         []map[string]any `json:"rows"`
	RowsAffected int              `json:"rows_affected"`
	Truncated    bool             `json:"truncated"`
	ExecutionMs  int              `json:"execution_ms"`
	Error        string           `json:"error"`
}

func newDatabasesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "db",
		Aliases: []string{"databases", "database"},
		Short:   "Manage databases",
	}

	cmd.AddCommand(newDBListCmd())
	cmd.AddCommand(newDBCreateCmd())
	cmd.AddCommand(newDBDeleteCmd())
	cmd.AddCommand(newDBUsersCmd())
	cmd.AddCommand(newDBSchemaCmd())
	cmd.AddCommand(newDBBackupCmd())
	cmd.AddCommand(newDBBackupsCmd())
	cmd.AddCommand(newDBRestoreCmd())
	cmd.AddCommand(newDBQueryCmd())

	return cmd
}

const (
	databaseStatusColumn = 2
	backupStatusColumn   = 1
)

// Only NAME may shrink in the database list; every backup column has a known shape.
var (
	databaseListFixedColumns = []bool{false, true, true, true}
	backupListFixedColumns   = []bool{true, true, true, true, true}
)

func newDBListCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List databases",
		Example: "  dcs db list\n  dcs db list --server web-1",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDatabases(serverID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			dbs, err := client.Decode[[]Database](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			headers := []string{"Name", "Type", "Status", "Created"}
			rows := make([][]string, len(dbs))
			for i, d := range dbs {
				rows[i] = []string{
					d.Name,
					d.Type,
					d.Status,
					d.CreatedAt,
				}
			}

			if ui.PrintFormatted(resp.Data, headers, rows) {
				return nil
			}

			if len(dbs) == 0 {
				fmt.Println()
				ui.PrintInfo("No databases yet. Run 'dcs db create' to add one.")
				fmt.Println()
				return nil
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, databaseStatusColumn), databaseListFixedColumns)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDBCreateCmd() *cobra.Command {
	var (
		serverFlag string
		name       string
		dbType     string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new database",
		Example: `  dcs db create --server web-1 --name mydb --type postgres16
  dcs db create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			if name == "" {
				name, err = ui.Input("Database name", "myapp_db")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			if dbType == "" {
				typeOpts := []huh.Option[string]{
					huh.NewOption("MySQL 8", "mysql8"),
					huh.NewOption("PostgreSQL 16", "postgres16"),
					huh.NewOption("MariaDB", "mariadb"),
				}
				dbType, err = ui.SelectOption("Database type", typeOpts)
				if err != nil {
					return fmt.Errorf("SelectOption: %w", err)
				}
			}

			switch dbType {
			case "mysql":
				dbType = "mysql8"
			case "postgres":
				dbType = "postgres16"
			}

			var db Database
			err = ui.RunWithSpinner("Creating database...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathDatabases(serverID), map[string]string{
					"name": name,
					"type": dbType,
				})
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				d, err := client.Decode[Database](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				db = d
				return nil
			})
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(db)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Database %s created!", ui.Bold.Render(db.Name)))
			ui.PrintKeyValue("Type", db.Type)
			ui.PrintKeyValue("Status", db.Status)
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&name, "name", "", "Database name")
	cmd.Flags().StringVar(&dbType, "type", "", "Database type (mysql8, mariadb, postgres16, postgres15, mongodb7, redis7; aliases: mysql, postgres)")
	return cmd
}

func newDBDeleteCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			if jsonOutput {
				return deleteJSON(cmd.Context(), client.PathDatabase(serverID, db.ID), db.ID, force)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete database %s? This cannot be undone.", ui.Bold.Render(db.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting database...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathDatabase(serverID, db.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Database %s deleted.", db.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newDBUsersCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:   "users <name>",
		Short: "List database users",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDatabaseUsers(serverID, db.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			users, err := client.Decode[[]DBUser](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(users) == 0 {
				ui.PrintInfo("No database users found.")
				return nil
			}

			headers := []string{"Username", "Database"}
			rows := make([][]string, len(users))
			for i, u := range users {
				rows[i] = []string{u.Username, u.Database}
			}

			fmt.Println()
			ui.PrintTable(headers, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDBSchemaCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:   "schema <name>",
		Short: "Show database schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDatabaseSchema(serverID, db.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			schema, err := client.Decode[map[string]any](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			ui.PrintSection("Schema: " + db.Name)
			ui.PrintJSON(schema)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDBBackupCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "backup <name>",
		Short:   "Create a database backup",
		Example: "  dcs db backup mydb",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			var backup DBBackup
			err = ui.RunWithSpinner("Creating backup for "+db.Name+"...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathDatabaseBackups(serverID, db.ID), nil)
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				b, err := client.Decode[DBBackup](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				backup = b
				return nil
			})
			if err != nil {
				return fmt.Errorf("create backup: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(backup)
				return nil
			}

			ui.PrintSuccess(fmt.Sprintf("Backup created for %s", ui.Bold.Render(db.Name)))
			ui.PrintKeyValue("Backup ID", backup.ID)
			ui.PrintKeyValue("Status", backup.Status)
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDBBackupsCmd() *cobra.Command {
	var serverFlag string
	cmd := &cobra.Command{
		Use:     "backups <name>",
		Aliases: []string{"backup-list"},
		Short:   "List database backups",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathDatabaseBackups(serverID, db.ID), nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			backups, err := client.Decode[[]DBBackup](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(backups) == 0 {
				fmt.Println()
				ui.PrintInfo("No backups yet. Run 'dcs db backup " + db.Name + "' to create one.")
				fmt.Println()
				return nil
			}

			headers := []string{"ID", "Status", "Size", "Completed", "Created"}
			rows := make([][]string, len(backups))
			for i, b := range backups {
				id := b.ID
				if len(id) > 8 {
					id = id[:8]
				}
				rows[i] = []string{
					id,
					b.Status,
					formatBytes(b.SizeBytes),
					b.CompletedAt,
					b.CreatedAt,
				}
			}

			fmt.Println()
			ui.PrintTableFixed(headers, ui.WithStatusColumns(rows, backupStatusColumn), backupListFixedColumns)
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	return cmd
}

func newDBRestoreCmd() *cobra.Command {
	var (
		serverFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:   "restore <name> <backup-id>",
		Short: "Restore a database from backup",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			backupID := args[1]

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Restore %s from backup %s? Current data will be overwritten.", ui.Bold.Render(db.Name), backupID[:8]))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Restoring "+db.Name+"...", func() error {
				_, err := apiClient.Post(context.Background(), client.PathDatabaseBackupRestore(serverID, db.ID, backupID), nil)
				if err != nil {
					return fmt.Errorf("restore: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Restore started for %s", ui.Bold.Render(db.Name)))
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}

func newDBQueryCmd() *cobra.Command {
	var (
		serverFlag string
		sql        string
	)
	cmd := &cobra.Command{
		Use:     "query <name>",
		Short:   "Execute a SQL query",
		Example: `  dcs db query mydb --sql "SELECT * FROM users LIMIT 10"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			serverID, err := getServerID(serverFlag)
			if err != nil {
				return fmt.Errorf("getServerID: %w", err)
			}

			db, err := resolveDB(serverID, args[0])
			if err != nil {
				return fmt.Errorf("resolveDB: %w", err)
			}

			if sql == "" {
				sql, err = ui.Input("SQL query", "SELECT 1")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			resp, err := apiClient.Post(context.Background(), client.PathDatabaseQuery(serverID, db.ID), map[string]string{
				"sql": sql,
			})
			if err != nil {
				return fmt.Errorf("Post: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			result, err := client.Decode[QueryResult](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if result.Error != "" {
				return fmt.Errorf("query error: %s", result.Error)
			}

			if len(result.Columns) == 0 || len(result.Rows) == 0 {
				fmt.Printf("\n  %s (%d rows affected, %dms)\n\n",
					ui.Success.Render("Query executed"),
					result.RowsAffected,
					result.ExecutionMs,
				)
				return nil
			}

			rows := make([][]string, len(result.Rows))
			for i, row := range result.Rows {
				cells := make([]string, len(result.Columns))
				for j, col := range result.Columns {
					cells[j] = fmt.Sprintf("%v", row[col])
				}
				rows[i] = cells
			}

			fmt.Println()
			ui.PrintTable(result.Columns, rows)
			fmt.Printf("  %s\n\n", ui.Muted.Render(fmt.Sprintf("%d rows, %dms", len(result.Rows), result.ExecutionMs)))
			if result.Truncated {
				ui.PrintWarning("Results truncated. Use the portal for full output.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serverFlag, "server", "", "Server name or ID")
	cmd.Flags().StringVar(&sql, "sql", "", "SQL query to execute")
	return cmd
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func resolveDB(serverID, nameOrID string) (*Database, error) {
	resp, err := apiClient.Get(context.Background(), client.PathDatabases(serverID), nil)
	if err != nil {
		return nil, err
	}

	dbs, err := client.Decode[[]Database](resp)
	if err != nil {
		return nil, err
	}

	for _, d := range dbs {
		if strings.EqualFold(d.Name, nameOrID) || d.ID == nameOrID || strings.HasPrefix(d.ID, nameOrID) {
			return &d, nil
		}
	}

	return nil, &client.CLIError{
		Message:    fmt.Sprintf("Database '%s' not found.", nameOrID),
		Suggestion: "Run 'dcs db list' to see your databases.",
	}
}
