package workspace

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const requestTimeout = 30 * time.Second

func Run(ctx context.Context, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := newModel(opts)
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
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(opts.Input), tea.WithOutput(opts.Output), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run interactive workspace: %w", err)
	}
	return nil
}
