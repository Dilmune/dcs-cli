package workspace

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/ui"
)

// profileProbe quits on the color profile the program reports, which is the
// only way to read back what the renderer was configured with.
type profileProbe struct{ reported chan colorprofile.Profile }

func (p profileProbe) Init() tea.Cmd { return nil }

func (p profileProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if profile, ok := msg.(tea.ColorProfileMsg); ok {
		p.reported <- profile.Profile
		return p, tea.Quit
	}
	return p, nil
}

func (p profileProbe) View() tea.View { return tea.NewView("") }

// The styles and the renderer must agree on how much color the output takes.
// Bubble Tea detects its own profile when it is not told one, and inside tmux
// that answer is ANSI256 where ours is TrueColor, so every token would be
// quantized after the styles had already chosen it.
func TestWorkspaceRunsInTheProfileItsStylesWereBuiltFrom(t *testing.T) {
	for _, want := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI, colorprofile.NoTTY} {
		t.Run(want.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var out bytes.Buffer
			probe := profileProbe{reported: make(chan colorprofile.Profile, 1)}
			options := programOptions(ctx, Options{Input: strings.NewReader(""), Output: &out}, want)
			_, err := tea.NewProgram(probe, options...).Run()
			require.NoError(t, err)
			select {
			case got := <-probe.reported:
				assert.Equal(t, want, got)
			default:
				t.Fatal("the program never reported a color profile")
			}
		})
	}
}

// Nothing in the workspace may re-detect: the model paints with the profile it
// is handed, whatever the environment around it says.
func TestWorkspaceModelPaintsWithTheProfileItIsGiven(t *testing.T) {
	tests := []struct {
		profile colorprofile.Profile
		want    string
	}{
		{colorprofile.TrueColor, "\x1b[38;2;125;128;134mLoading...\x1b[m"},
		{colorprofile.ANSI256, "\x1b[38;5;244mLoading...\x1b[m"},
		{colorprofile.ANSI, "\x1b[37mLoading...\x1b[m"},
		{colorprofile.NoTTY, "Loading..."},
	}
	for _, tc := range tests {
		t.Run(tc.profile.String(), func(t *testing.T) {
			m := newModel(Options{Output: &bytes.Buffer{}, Mode: ui.ModeDark}, tc.profile)
			assert.Equal(t, tc.want, m.styles.muted.Render("Loading..."))
		})
	}
}
