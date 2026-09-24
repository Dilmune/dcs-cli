package ui

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

const spinnerFrameInterval = 80 * time.Millisecond

type spinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
}

type spinnerDoneMsg struct{}

func newSpinnerModel(message string) spinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Spinner{Frames: spinner.Dot.Frames, FPS: spinnerFrameInterval}
	s.Style = Accent
	return spinnerModel{
		spinner: s,
		message: message,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case spinnerDoneMsg:
		m.done = true
		return m, tea.Quit
	case tea.KeyPressMsg:
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("  %s %s", m.spinner.View(), Muted.Render(m.message)))
}

// RunWithSpinner shows a spinner while executing the given function.
// In non-interactive environments, falls back to a simple status message on stderr.
func RunWithSpinner(message string, fn func() error) error {
	if !IsInteractive() {
		fmt.Fprintf(os.Stderr, "  %s\n", message)
		return fn()
	}

	errCh := make(chan error, 1)
	p := tea.NewProgram(newSpinnerModel(message), tea.WithColorProfile(painter.profile))

	go func() {
		err := fn()
		errCh <- err
		p.Send(spinnerDoneMsg{})
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return <-errCh
}
