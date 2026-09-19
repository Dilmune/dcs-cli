package commands

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

// Set via ldflags at build time:
//
//	go build -ldflags "-X github.com/dilmune/dcs-cli/internal/commands.Commit=abc123
//	                    -X github.com/dilmune/dcs-cli/internal/commands.BuildDate=2026-03-29"
var (
	Commit    = "dev"
	BuildDate = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show CLI version",
		Example: "  dcs version\n  dcs version --json",
		Run: func(cmd *cobra.Command, args []string) {
			if jsonOutput {
				info := map[string]string{
					"version": client.Version,
					"commit":  Commit,
					"built":   BuildDate,
					"os":      runtime.GOOS,
					"arch":    runtime.GOARCH,
					"go":      runtime.Version(),
				}
				ui.PrintJSON(info)
				return
			}
			fmt.Printf("  %s %s\n", ui.Title.Render("dcs"), "v"+client.Version)
			fmt.Printf("  %s/%s %s\n", runtime.GOOS, runtime.GOARCH, runtime.Version())
			if Commit != "dev" {
				commit := Commit
				if len(commit) > 8 {
					commit = commit[:8]
				}
				fmt.Printf("  %s %s\n", ui.Muted.Render("commit"), commit)
			}
			if BuildDate != "unknown" {
				fmt.Printf("  %s %s\n", ui.Muted.Render("built"), BuildDate)
			}
			fmt.Println()
		},
	}
}
