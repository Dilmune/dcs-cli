package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func newDocsCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Generate CLI documentation",
		Long:  "Generate man pages or markdown documentation for all CLI commands.",
	}

	cmd.AddCommand(newDocsManCmd(&outputDir))
	cmd.AddCommand(newDocsMarkdownCmd(&outputDir))

	cmd.PersistentFlags().StringVarP(&outputDir, "dir", "d", "./docs", "Output directory")

	return cmd
}

func newDocsManCmd(outputDir *string) *cobra.Command {
	return &cobra.Command{
		Use:     "man",
		Short:   "Generate man pages",
		Example: "  dcs docs man\n  dcs docs man -d ./output",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(*outputDir, "man")
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}

			root := NewRootCmd()
			header := &doc.GenManHeader{
				Title:   "DCS",
				Section: "1",
				Source:  "DCS CLI " + client.Version,
				Manual:  "Dilmune Cloud Services",
			}

			if err := doc.GenManTree(root, header, dir); err != nil {
				return fmt.Errorf("generate man pages: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Man pages written to %s", dir))
			return nil
		},
	}
}

func newDocsMarkdownCmd(outputDir *string) *cobra.Command {
	return &cobra.Command{
		Use:     "markdown",
		Short:   "Generate markdown documentation",
		Aliases: []string{"md"},
		Example: "  dcs docs markdown\n  dcs docs markdown -d ./output",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := filepath.Join(*outputDir, "md")
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}

			root := NewRootCmd()
			if err := doc.GenMarkdownTree(root, dir); err != nil {
				return fmt.Errorf("generate markdown docs: %w", err)
			}

			ui.PrintSuccess(fmt.Sprintf("Markdown docs written to %s", dir))
			return nil
		},
	}
}
