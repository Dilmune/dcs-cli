package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keys",
		Aliases: []string{"ssh-keys"},
		Short:   "Manage SSH keys",
	}

	cmd.AddCommand(newKeysListCmd())
	cmd.AddCommand(newKeysAddCmd())
	cmd.AddCommand(newKeysRmCmd())

	return cmd
}

func newKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List SSH keys",
		Example: "  dcs keys list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSSHKeys, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			keys, err := client.Decode[[]struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Fingerprint string `json:"fingerprint"`
				CreatedAt   string `json:"created_at"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(keys) == 0 {
				fmt.Println()
				ui.PrintInfo("No SSH keys yet. Run 'dcs keys add' to add one.")
				fmt.Println()
				return nil
			}

			headers := []string{"Name", "Fingerprint", "Created"}
			rows := make([][]string, len(keys))
			for i, k := range keys {
				rows[i] = []string{k.Name, k.Fingerprint, k.CreatedAt}
			}

			fmt.Println()
			ui.PrintTable(headers, rows)
			return nil
		},
	}
}

func newKeysAddCmd() *cobra.Command {
	var (
		name    string
		keyFile string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add an SSH key",
		Example: "  dcs keys add\n  dcs keys add --name my-laptop --file ~/.ssh/id_ed25519.pub",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			var err error
			if name == "" {
				name, err = ui.Input("Key name", "my-laptop")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			var publicKey string
			if keyFile != "" {
				data, err := os.ReadFile(keyFile)
				if err != nil {
					return fmt.Errorf("read key file: %w", err)
				}
				publicKey = strings.TrimSpace(string(data))
			} else {
				defaultPath := os.Getenv("HOME") + "/.ssh/id_ed25519.pub"
				if _, err := os.Stat(defaultPath); err != nil {
					defaultPath = os.Getenv("HOME") + "/.ssh/id_rsa.pub"
				}

				publicKey, err = ui.Input("Public key (paste or path)", defaultPath)
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}

				if !strings.HasPrefix(publicKey, "ssh-") {
					data, err := os.ReadFile(publicKey)
					if err != nil {
						return fmt.Errorf("read key file: %w", err)
					}
					publicKey = strings.TrimSpace(string(data))
				}
			}

			var key struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			err = ui.RunWithSpinner("Adding SSH key...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathSSHKeys, map[string]string{
					"name":      name,
					"publicKey": publicKey,
				})
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				k, err := client.Decode[struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				key = k
				return nil
			})
			if err != nil {
				return fmt.Errorf("add SSH key: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(key)
				return nil
			}
			ui.PrintSuccess(fmt.Sprintf("SSH key %s added.", ui.Bold.Render(key.Name)))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Key name")
	cmd.Flags().StringVar(&keyFile, "file", "", "Path to public key file")
	return cmd
}

func newKeysRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "rm <name-or-id>",
		Aliases: []string{"remove", "delete"},
		Short:   "Remove an SSH key",
		Example: "  dcs keys rm my-laptop\n  dcs keys rm my-laptop --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathSSHKeys, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			keys, err := client.Decode[[]struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			var keyID, keyName string
			for _, k := range keys {
				if strings.EqualFold(k.Name, args[0]) || k.ID == args[0] || strings.HasPrefix(k.ID, args[0]) {
					keyID = k.ID
					keyName = k.Name
					break
				}
			}

			if keyID == "" {
				return fmt.Errorf("SSH key '%s' not found. Run 'dcs keys list' to see your keys.", args[0])
			}

			if jsonOutput {
				return deleteJSON(cmd.Context(), client.PathSSHKey(keyID), keyID, force)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Remove SSH key %s?", ui.Bold.Render(keyName)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Removing SSH key...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathSSHKey(keyID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("SSH key %s removed.", keyName))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}
