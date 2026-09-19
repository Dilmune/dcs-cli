package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

type APIKey struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	KeyPrefix  string `json:"key_prefix"`
	LastUsedAt string `json:"last_used_at"`
	ExpiresAt  string `json:"expires_at"`
	CreatedAt  string `json:"created_at"`
}

func (k *APIKey) UnmarshalJSON(data []byte) error {
	type legacy APIKey
	var decoded legacy
	wire := struct {
		*legacy
		KeyPrefix  *string `json:"keyPrefix"`
		LastUsedAt *string `json:"lastUsedAt"`
		ExpiresAt  *string `json:"expiresAt"`
		CreatedAt  *string `json:"createdAt"`
	}{legacy: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("decode API key: %w", err)
	}
	if wire.KeyPrefix != nil {
		decoded.KeyPrefix = *wire.KeyPrefix
	}
	if wire.LastUsedAt != nil {
		decoded.LastUsedAt = *wire.LastUsedAt
	}
	if wire.ExpiresAt != nil {
		decoded.ExpiresAt = *wire.ExpiresAt
	}
	if wire.CreatedAt != nil {
		decoded.CreatedAt = *wire.CreatedAt
	}
	*k = APIKey(decoded)
	return nil
}

type APIKeyCreated struct {
	APIKey
	Key string `json:"key"`
}

func (k *APIKeyCreated) UnmarshalJSON(data []byte) error {
	var key APIKey
	if err := json.Unmarshal(data, &key); err != nil {
		return fmt.Errorf("decode created API key: %w", err)
	}
	var secret struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &secret); err != nil {
		return fmt.Errorf("decode API key secret: %w", err)
	}
	*k = APIKeyCreated{APIKey: key, Key: secret.Key}
	return nil
}

func apiKeyExpiry(value string, now time.Time) (string, error) {
	const day = 24 * time.Hour
	const maxDays = int64((1<<63 - 1) / day)
	if !strings.HasSuffix(value, "d") {
		return "", fmt.Errorf("expiry must be a positive number of days, such as 30d or 90d")
	}
	days, err := strconv.ParseInt(strings.TrimSuffix(value, "d"), 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse expiry days: %w", err)
	}
	if days <= 0 || days > maxDays {
		return "", fmt.Errorf("expiry days must be between 1 and %d", maxDays)
	}
	return now.Add(time.Duration(days) * day).UTC().Format(time.RFC3339), nil
}

func newAPIKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "api-keys",
		Aliases: []string{"apikeys"},
		Short:   "Manage API keys",
	}

	cmd.AddCommand(newAPIKeysListCmd())
	cmd.AddCommand(newAPIKeysCreateCmd())
	cmd.AddCommand(newAPIKeysDeleteCmd())

	return cmd
}

func newAPIKeysListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List API keys",
		Example: "  dcs api-keys list\n  dcs api-keys list --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathAPIKeys, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			keys, err := client.Decode[[]APIKey](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if len(keys) == 0 {
				fmt.Println()
				ui.PrintInfo("No API keys found. Run 'dcs api-keys create' to add one.")
				fmt.Println()
				return nil
			}

			headers := []string{"Name", "Key Prefix", "Last Used", "Expires", "Created"}
			rows := make([][]string, len(keys))
			for i, k := range keys {
				lastUsed := k.LastUsedAt
				if lastUsed == "" {
					lastUsed = ui.Muted.Render("never")
				}
				expires := k.ExpiresAt
				if expires == "" {
					expires = ui.Muted.Render("never")
				}
				rows[i] = []string{k.Name, k.KeyPrefix, lastUsed, expires, k.CreatedAt}
			}

			fmt.Println()
			ui.PrintTable(headers, rows)
			return nil
		},
	}
}

func newAPIKeysCreateCmd() *cobra.Command {
	var (
		name    string
		expires string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new API key",
		Example: "  dcs api-keys create\n  dcs api-keys create --name ci-deploy --expires 90d",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			var err error
			var expiresAt string
			if cmd.Flags().Changed("expires") {
				expiresAt, err = apiKeyExpiry(expires, time.Now())
				if err != nil {
					return fmt.Errorf("invalid --expires: %w", err)
				}
			}
			if name == "" {
				name, err = ui.Input("Key name", "my-api-key")
				if err != nil {
					return fmt.Errorf("input: %w", err)
				}
			}

			body := map[string]string{"name": name}
			if expiresAt != "" {
				body["expiresAt"] = expiresAt
			}

			var created APIKeyCreated
			err = ui.RunWithSpinner("Creating API key...", func() error {
				resp, err := apiClient.Post(context.Background(), client.PathAPIKeys, body)
				if err != nil {
					return fmt.Errorf("create: %w", err)
				}
				c, err := client.Decode[APIKeyCreated](resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				created = c
				return nil
			})
			if err != nil {
				return fmt.Errorf("create API key: %w", err)
			}

			if jsonOutput {
				ui.PrintJSON(created)
				return nil
			}

			ui.PrintSuccess("API key created!")
			ui.PrintKeyValue("Name", created.Name)
			ui.PrintKeyValue("Key", created.Key)
			fmt.Printf("\n  %s\n\n", ui.Warning.Render("Save this key, it won't be shown again."))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Key name")
	cmd.Flags().StringVar(&expires, "expires", "", "Expiration in days (e.g. 30d, 90d)")
	return cmd
}

func newAPIKeysDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:     "delete <name-or-id>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete an API key",
		Example: "  dcs api-keys delete ci-deploy\n  dcs api-keys delete ci-deploy --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(context.Background(), client.PathAPIKeys, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			keys, err := client.Decode[[]APIKey](resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			var key *APIKey
			for _, k := range keys {
				if strings.EqualFold(k.Name, args[0]) || k.ID == args[0] || strings.HasPrefix(k.ID, args[0]) {
					found := k
					key = &found
					break
				}
			}

			if key == nil {
				return &client.CLIError{
					Message:    fmt.Sprintf("API key '%s' not found.", args[0]),
					Suggestion: "Run 'dcs api-keys list' to see your keys.",
				}
			}

			if jsonOutput {
				return deleteJSON(cmd.Context(), client.PathAPIKey(key.ID), key.ID, force)
			}

			if !force {
				confirmed, err := ui.Confirm(fmt.Sprintf("Delete API key %s?", ui.Bold.Render(key.Name)))
				if err != nil {
					return fmt.Errorf("confirm: %w", err)
				}
				if !confirmed {
					return nil
				}
			}

			err = ui.RunWithSpinner("Deleting API key...", func() error {
				_, err := apiClient.Delete(context.Background(), client.PathAPIKey(key.ID))
				if err != nil {
					return fmt.Errorf("delete: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("Errorf: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("API key %s deleted.", key.Name))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	return cmd
}
