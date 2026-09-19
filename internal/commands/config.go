package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(newConfigViewCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigPathCmd())
	cmd.AddCommand(newConfigKeychainCmd())

	return cmd
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "view",
		Aliases: []string{"show"},
		Short:   "Show current configuration",
		Example: "  dcs config view\n  dcs config view --json",
		Run: func(cmd *cobra.Command, args []string) {
			if jsonOutput {
				safe := map[string]any{
					"api_url":           cfg.APIURL,
					"authenticated":     cfg.IsAuthenticated(),
					"config_path":       config.FilePath(),
					"default_server_id": cfg.DefaultServerID,
					"use_keychain":      cfg.UseKeychain,
				}
				if cfg.User != nil {
					safe["user"] = cfg.User
				}
				ui.PrintJSON(safe)
				return
			}

			ui.PrintSection("Configuration")
			ui.PrintKeyValue("Config file", config.FilePath())
			ui.PrintKeyValue("API URL", cfg.APIURL)
			if cfg.IsAuthenticated() && len(cfg.APIKey) > 12 {
				ui.PrintKeyValue("API key", cfg.APIKey[:12]+"…")
			} else if cfg.IsAuthenticated() {
				ui.PrintKeyValue("API key", "***")
			} else {
				ui.PrintKeyValue("API key", ui.Muted.Render("not set"))
			}
			if cfg.User != nil {
				ui.PrintKeyValue("User", cfg.User.Email)
			}
			if cfg.DefaultServerID != "" {
				ui.PrintKeyValue("Default server", cfg.DefaultServerID)
			}
			if cfg.UseKeychain {
				ui.PrintKeyValue("Keychain", ui.Success.Render("enabled"))
			}
			fmt.Println()
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "set <key> <value>",
		Short:   "Set a configuration value",
		Long:    "Available keys: api_url, default_server_id",
		Example: "  dcs config set default_server_id srv_abc123\n  dcs config set api_url http://localhost:8080",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			switch key {
			case "api_url":
				cfg.APIURL = value
			case "default_server_id":
				cfg.DefaultServerID = value
			default:
				return fmt.Errorf("unknown config key: %s\n\n  Available: api_url, default_server_id", key)
			}

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("Save: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Set %s = %s", key, value))
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "path",
		Short:   "Show config file path",
		Example: "  dcs config path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.FilePath())
		},
	}
}

func newConfigKeychainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keychain",
		Short: "Manage OS keychain integration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:     "enable",
		Short:   "Store API key in OS keychain instead of config file",
		Example: "  dcs config keychain enable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cfg.IsAuthenticated() {
				return fmt.Errorf("not authenticated, run 'dcs login' first")
			}

			cfg.UseKeychain = true
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("Save: %w", err)
			}

			ui.PrintSuccess("Keychain enabled. API key moved to OS keychain.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "disable",
		Short:   "Store API key in config file instead of OS keychain",
		Example: "  dcs config keychain disable",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiKey := cfg.APIKey
			cfg.UseKeychain = false
			cfg.APIKey = apiKey

			if err := cfg.Save(); err != nil {
				return fmt.Errorf("Save: %w", err)
			}

			_ = config.DeleteFromKeychain()

			ui.PrintSuccess("Keychain disabled. API key stored in config file.")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "status",
		Short:   "Show keychain status",
		Example: "  dcs config keychain status",
		Run: func(cmd *cobra.Command, args []string) {
			if cfg.UseKeychain {
				ui.PrintKeyValue("Keychain", ui.Success.Render("enabled"))
				if _, err := config.LoadFromKeychain(); err == nil {
					ui.PrintKeyValue("Key stored", ui.Success.Render("yes"))
				} else {
					ui.PrintKeyValue("Key stored", ui.Error.Render("no"))
				}
			} else {
				ui.PrintKeyValue("Keychain", ui.Muted.Render("disabled"))
			}
			fmt.Println()
		},
	})

	return cmd
}
