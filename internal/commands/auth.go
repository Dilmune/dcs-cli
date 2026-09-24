package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "login",
		Short:   "Authenticate with DCS",
		Long:    "Log in to Dilmune Cloud Services using an API key.",
		Example: "  dcs login",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.PrintBanner(client.Version)

			fmt.Println(ui.Muted.Render("  To authenticate, you need a DCS API key."))
			fmt.Println(ui.Muted.Render("  Create one in your portal under Settings → API Keys."))
			fmt.Println()

			method, err := ui.SelectOption("How would you like to authenticate?", []huh.Option[string]{
				huh.NewOption("Open portal to create a key", "browser"),
				huh.NewOption("Paste an existing API key", "paste"),
			})
			if err != nil {
				return fmt.Errorf("select an authentication method: %w", err)
			}

			if method == "browser" {
				portalURL := client.PortalBaseURL + "/settings/api-keys"
				if v := os.Getenv("DCS_PORTAL_URL"); v != "" {
					portalURL = v + "/settings/api-keys"
				}
				fmt.Printf("  %s\n\n", ui.Info.Render("Opening "+portalURL+" in your browser..."))
				_ = browser.OpenURL(portalURL)
			}

			apiKey, err := ui.InputPassword("Paste your API key", "dcs_xxxxxxxxxxxx")
			if err != nil {
				return fmt.Errorf("input: %w", err)
			}

			if apiKey == "" {
				return fmt.Errorf("API key cannot be empty")
			}

			tempClient := client.New(cfg.APIURL, apiKey)
			var user *config.UserInfo

			err = ui.RunWithSpinner("Validating API key...", func() error {
				resp, err := tempClient.Get(cmd.Context(), client.PathAuthMe, nil)
				if err != nil {
					return fmt.Errorf("operation: %w", err)
				}
				u, err := decodeAuthenticatedUser(resp)
				if err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				user = u
				return nil
			})
			if err != nil {
				ui.PrintError(fmt.Errorf("Invalid API key: %w", err))
				return fmt.Errorf("invalid API key: %w", err)
			}

			cfg.APIKey = apiKey
			cfg.User = user
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			ui.PrintSuccess("Authenticated as " + authenticatedAs(*user))
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Log out of DCS",
		Example: "  dcs logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cfg.IsAuthenticated() && !cfg.UseKeychain {
				ui.PrintInfo("Not currently authenticated.")
				return nil
			}

			confirmed, err := ui.Confirm("Are you sure you want to log out?")
			if err != nil {
				return fmt.Errorf("confirm: %w", err)
			}
			if !confirmed {
				return nil
			}

			if err := clearStoredAuthentication(); err != nil {
				return fmt.Errorf("log out: %w", err)
			}

			ui.PrintSuccess("Logged out successfully.")
			if os.Getenv("DCS_API_KEY") != "" {
				ui.PrintInfo("DCS_API_KEY is still set; unset it to stop using that environment key.")
			}
			return nil
		},
	}
}

func clearStoredAuthentication() error {
	if cfg.UseKeychain {
		if err := config.DeleteFromKeychain(); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("remove keychain credentials: %w", err)
		}
	}
	cleared := *cfg
	cleared.APIKey = ""
	cleared.User = nil
	if err := cleared.Save(); err != nil {
		return fmt.Errorf("save logged-out config: %w", err)
	}
	*cfg = cleared
	apiClient = nil
	return nil
}

func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Short:   "Show current user info",
		Example: "  dcs whoami",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAuth(); err != nil {
				return fmt.Errorf("auth: %w", err)
			}

			resp, err := apiClient.Get(cmd.Context(), client.PathAuthMe, nil)
			if err != nil {
				return fmt.Errorf("fetch: %w", err)
			}

			user, err := decodeAuthenticatedUser(resp)
			if err != nil {
				return fmt.Errorf("decode response: %w", err)
			}

			if jsonOutput {
				ui.PrintJSONRaw(resp.Data)
				return nil
			}

			ui.PrintSection("Account")
			ui.PrintKeyValue("Name", user.DisplayName())
			if user.HasDisplayName() {
				ui.PrintKeyValue("Email", user.Email)
			}
			ui.PrintKeyValue("ID", user.ID)
			ui.PrintKeyValue("API", cfg.APIURL)
			fmt.Println()
			return nil
		},
	}
}

func decodeAuthenticatedUser(resp *client.APIResponse) (*config.UserInfo, error) {
	me, err := client.Decode[struct {
		User config.UserInfo `json:"user"`
	}](resp)
	if err != nil {
		return nil, fmt.Errorf("decode authenticated user: %w", err)
	}
	if !hasAuthenticatedIdentity(me.User) {
		return nil, fmt.Errorf("authentication response is missing user identity")
	}
	return &me.User, nil
}

func authenticatedAs(user config.UserInfo) string {
	if user.HasDisplayName() {
		return fmt.Sprintf("%s (%s)", ui.Bold.Render(user.Name), user.Email)
	}
	return ui.Bold.Render(user.Email)
}

func hasAuthenticatedIdentity(user config.UserInfo) bool {
	return strings.TrimSpace(user.ID) != "" && strings.TrimSpace(user.Email) != ""
}
