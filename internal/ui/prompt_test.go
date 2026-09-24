package ui

import (
	"io"
	"testing"

	"charm.land/huh/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
)

func TestPromptTheme_CarriesTheAccentOfTheResolvedMode(t *testing.T) {
	for _, mode := range []Mode{ModeLight, ModeDim, ModeDark} {
		t.Run(string(mode), func(t *testing.T) {
			withStyles(t, mode, false)
			accent := Palette(mode).Accent
			styles := promptTheme().Theme(mode.HasDarkBackground())

			assert.Equal(t, accent, styles.Focused.Title.GetForeground())
			assert.Equal(t, accent, styles.Focused.SelectedOption.GetForeground())
			assert.Equal(t, accent, styles.Focused.SelectSelector.GetForeground())
			assert.Equal(t, accent, styles.Focused.FocusedButton.GetBackground())
			assert.Equal(t, accent, styles.Focused.TextInput.Cursor.GetForeground())
		})
	}
}

func TestPromptTheme_IgnoresTheBackgroundHuhOffers(t *testing.T) {
	withStyles(t, ModeLight, false)
	theme := promptTheme()

	assert.Equal(t, theme.Theme(true), theme.Theme(false), "Init settles the background once; a form may not overrule it mid-render")
	assert.Equal(t,
		huh.ThemeCharm(false).Focused.Description.GetForeground(),
		theme.Theme(true).Focused.Description.GetForeground(),
		"light keeps huh's light styles even when the form reports a dark terminal")
}

func TestPromptTheme_KeepsHuhsOwnButtonForeground(t *testing.T) {
	withStyles(t, ModeDark, false)
	styles := promptTheme().Theme(true)

	assert.Equal(t,
		huh.ThemeCharm(true).Focused.FocusedButton.GetForeground(),
		styles.Focused.FocusedButton.GetForeground(),
		"only the button background takes the accent, so no color outside the palette is introduced")
}

func TestTmuxCarriesTrueColor(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want bool
	}{
		{"tmux announcing truecolor", []string{"TMUX=/tmp/tmux-501/default,1,0", "COLORTERM=truecolor"}, true},
		{"tmux announcing 24bit", []string{"TMUX=/tmp/s,1,0", "COLORTERM=24bit"}, true},
		{"tmux announcing 24bit in caps", []string{"TMUX=/tmp/s,1,0", "COLORTERM=TrueColor"}, true},
		{"tmux with no announcement", []string{"TMUX=/tmp/s,1,0"}, false},
		{"tmux announcing plain color", []string{"TMUX=/tmp/s,1,0", "COLORTERM=yes"}, false},
		{"an emptied TMUX is not tmux", []string{"TMUX=", "COLORTERM=truecolor"}, false},
		{"screen announcing truecolor sets no TMUX", []string{"TERM=screen-256color", "COLORTERM=truecolor"}, false},
		{"no tmux at all", []string{"TERM=xterm-ghostty", "COLORTERM=truecolor"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tmuxCarriesTrueColor(tc.env))
		})
	}
}

// TTY_FORCE is colorprofile's own way to say "treat this writer as a terminal",
// so these run the real detection rather than a stand-in for it.
func TestTerminalProfile(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		plainOutput bool
		want        colorprofile.Profile
	}{
		{
			name:        "plain output shows nothing whatever the terminal can do",
			env:         map[string]string{"TERM": "xterm-ghostty", "COLORTERM": "truecolor"},
			plainOutput: true,
			want:        colorprofile.NoTTY,
		},
		{
			name: "a truecolor terminal keeps its 16 million colors",
			env:  map[string]string{"TERM": "xterm-ghostty", "COLORTERM": "truecolor"},
			want: colorprofile.TrueColor,
		},
		{
			name: "tmux alone quantizes to 256 colors",
			env:  map[string]string{"TERM": "tmux-256color"},
			want: colorprofile.ANSI256,
		},
		{
			name: "tmux announcing truecolor keeps it",
			env:  map[string]string{"TERM": "tmux-256color", "COLORTERM": "truecolor", "TMUX": "/tmp/tmux-501/default,1,0"},
			want: colorprofile.TrueColor,
		},
		{
			name: "NO_COLOR outranks the announcement",
			env:  map[string]string{"TERM": "tmux-256color", "COLORTERM": "truecolor", "TMUX": "/tmp/tmux-501/default,1,0", "NO_COLOR": "1"},
			want: colorprofile.ASCII,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []string{"TERM", "COLORTERM", "TMUX", "NO_COLOR"} {
				t.Setenv(name, tc.env[name])
			}
			t.Setenv("TTY_FORCE", "1")

			assert.Equal(t, tc.want, TerminalProfile(io.Discard, tc.plainOutput))
		})
	}
}
