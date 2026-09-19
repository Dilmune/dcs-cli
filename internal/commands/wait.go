package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dilmune/dcs-cli/internal/ui"
)

const (
	waitPollInterval = 3 * time.Second
	waitTimeout      = 10 * time.Minute
)

// waitForCondition polls check at regular intervals until it returns true or an error.
// In interactive mode, shows a spinner. In non-interactive mode, prints status to stderr.
func waitForCondition(message string, check func() (done bool, err error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	if ui.IsInteractive() {
		return ui.RunWithSpinner(message, func() error {
			return pollUntilDone(ctx, check)
		})
	}

	fmt.Fprintf(os.Stderr, "  %s\n", message)
	return pollUntilDone(ctx, check)
}

func pollUntilDone(ctx context.Context, check func() (bool, error)) error {
	for {
		done, err := check()
		if err != nil {
			return fmt.Errorf("poll check: %w", err)
		}
		if done {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for operation to complete")
		case <-time.After(waitPollInterval):
		}
	}
}
