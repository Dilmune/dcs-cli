package commands

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "open [resource]",
		Short:   "Open DCS portal in browser",
		Long:    "Opens the DCS web portal. Optionally specify a resource: billing, servers, sites.",
		Example: "  dcs open\n  dcs open billing\n  dcs open servers",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseURL := client.PortalBaseURL

			url := baseURL
			if len(args) > 0 {
				switch args[0] {
				case "billing":
					url = baseURL + "/billing"
				case "servers":
					url = baseURL + "/servers"
				case "sites":
					url = baseURL + "/sites"
				case "settings":
					url = baseURL + "/settings"
				case "storage":
					url = baseURL + "/storage"
				case "keys", "api-keys":
					url = baseURL + "/settings/api-keys"
				default:
					url = baseURL + "/" + args[0]
				}
			}

			fmt.Printf("  %s %s\n", ui.Muted.Render("Opening"), url)
			return browser.OpenURL(url)
		},
	}
	return cmd
}
