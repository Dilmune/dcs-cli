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
