package workspace

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dilmune/dcs-cli/internal/ui"
)

type styles struct {
	title, muted, accent, rule, success, warning, danger lipgloss.Style
	logoFace, logoSide, logoAccent                       lipgloss.Style
	asciiLogo                                            bool
}

// Colors come from the ui token table; the workspace owns nothing but the
// logo palette. The user's background and body color stay intact.
func newStyles(out io.Writer, mode ui.Mode, plain bool) styles {
	return workspaceStyles(newRenderer(out, mode, plain), mode)
}

// newRenderer takes the mode ui.Init resolved instead of detecting its own, so
// the workspace never queries the terminal a second time.
func newRenderer(out io.Writer, mode ui.Mode, plain bool) *lipgloss.Renderer {
	r := lipgloss.NewRenderer(out)
	if plain {
		r.SetColorProfile(termenv.Ascii)
	}
	r.SetHasDarkBackground(mode.HasDarkBackground())
	return r
}

func workspaceStyles(r *lipgloss.Renderer, mode ui.Mode) styles {
	t := ui.Palette(mode)
	b := r.NewStyle()
	face, side, terracotta := logoPalette()
	return styles{title: b.Bold(true), muted: b.Foreground(t.Muted), accent: b.Foreground(t.Accent), rule: b.Foreground(t.Divider),
		success: b.Foreground(t.Success), warning: b.Foreground(t.Warning), danger: b.Foreground(t.Danger),
		logoFace: b.Foreground(face), logoSide: b.Foreground(side), logoAccent: b.Foreground(terracotta), asciiLogo: r.ColorProfile() == termenv.Ascii}
}

// status renders glyph plus word in the workspace's own renderer so its
// profile and mode apply; ui.Status would use the process-wide styles.
func (s styles) status(word string) string {
	if word == "" {
		return ""
	}
	glyph, tone := ui.StatusGlyph(word)
	return s.tone(tone).Render(glyph + " " + word)
}

func (s styles) tone(tone ui.Tone) lipgloss.Style {
	switch tone {
	case ui.ToneSuccess:
		return s.success
	case ui.ToneWarning:
		return s.warning
	case ui.ToneDanger:
		return s.danger
	default:
		return s.muted
	}
}

// keyValueIndent is the value column: the 12-cell label plus the two-space gap.
const keyValueIndent = 12 + 2

// keyValueLines follows the plain-output rule: muted label right-aligned in
// the 12-cell column, two spaces, unstyled value. A value longer than the
// remaining width wraps under its own column instead of falling back to the
// label column. Empty values return nil.
func (s styles) keyValueLines(f Field, width int) []string {
	if f.Value == "" {
		return nil
	}
	value := f.Value
	if f.IsStatus() {
		value = s.status(value)
	}
	lines := wrap(value, width-keyValueIndent)
	lines[0] = ui.FormatKeyValue(f.Label, lines[0], s.muted)
	indent := strings.Repeat(" ", keyValueIndent)
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return lines
}

// subtitle is the muted line under a title: "● active · hetzner · hel1".
func (s styles) subtitle(item Item) string {
	if item.Status == "" {
		return s.muted.Render(item.Description)
	}
	if item.Description == "" {
		return s.status(item.Status)
	}
	return s.status(item.Status) + s.muted.Render(" · "+item.Description)
}
