package workspace

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/dilmune/dcs-cli/internal/ui"
)

const requestTimeout = 30 * time.Second

func Run(ctx context.Context, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	profile := ui.TerminalProfile(opts.Output, opts.NoColor)
	m := newModel(opts, profile)
	m.fetch = func(req Request, generation int) (tea.Cmd, context.CancelFunc) {
		requestCtx, stop := context.WithTimeout(ctx, requestTimeout)
		return func() tea.Msg {
			defer stop()
			item, err := opts.Source.Load(requestCtx, req)
			return loadedMsg{generation, cleanItem(item), err}
		}, stop
	}
	m.identify = func() tea.Msg {
		requestCtx, stop := context.WithTimeout(ctx, requestTimeout)
		defer stop()
		name, err := opts.Source.Identify(requestCtx)
		return identityMsg{Clean(name), err}
	}
	p := tea.NewProgram(m, programOptions(ctx, opts, profile)...)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run interactive workspace: %w", err)
	}
	return nil
}

// The renderer is given the profile the styles were built from. Left to detect
// its own it disagrees inside tmux, where it ignores COLORTERM and asks tmux,
// and quantizes every token to 256 colors.
func programOptions(ctx context.Context, opts Options, profile colorprofile.Profile) []tea.ProgramOption {
	return []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(opts.Input),
		tea.WithOutput(opts.Output),
		tea.WithColorProfile(profile),
	}
}
