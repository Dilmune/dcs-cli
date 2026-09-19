package main

import (
	"os"

	"github.com/dilmune/dcs-cli/internal/commands"
	"github.com/dilmune/dcs-cli/internal/ui"
)

func main() {
	cmd := commands.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		ui.PrintError(err)
		os.Exit(1)
	}
}
