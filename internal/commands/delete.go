package commands

import (
	"context"
	"fmt"

	"github.com/dilmune/dcs-cli/internal/ui"
)

func deleteJSON(ctx context.Context, path, id string, force bool) error {
	if !force {
		return fmt.Errorf("deletion with JSON output requires --force to confirm")
	}
	response, err := apiClient.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	if !response.Success {
		return fmt.Errorf("API did not confirm the deletion request")
	}
	ui.PrintJSON(map[string]any{"id": id, "success": true})
	return nil
}

// forceHint answers a confirmation that the command's --force flag skips.
const forceHint = "Pass --force to skip the confirmation."

// confirmForceable is ui.Confirm for a command with --force, so a script that
// hits the prompt is told the flag rather than to find a terminal.
func confirmForceable(message string) (bool, error) {
	confirmed, err := ui.Confirm(message)
	return confirmed, ui.ExplainNonInteractive(err, forceHint)
}
