package workspace

import (
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dilmune/dcs-cli/internal/ui"
)

type styles struct {
	title, muted, accent, rule, success, warning, danger lipgloss.Style
	logoFace, logoSide, logoAccent                       lipgloss.Style
	selected                                             lipgloss.Style
	asciiLogo                                            bool
}

// Colors come from the ui token table; the workspace owns nothing but the
// logo inks. The user's background and body color stay intact.
func newStyles(out io.Writer, mode ui.Mode, plain bool) styles {
	return workspaceStyles(ui.NewPainter(ui.TerminalProfile(out, plain)), mode)
}

// workspaceStyles takes the mode ui.Init resolved and a painter for its own
// output, so the workspace never queries the terminal a second time.
func workspaceStyles(p ui.Painter, mode ui.Mode) styles {
	t := ui.Palette(mode)
	face, side, terracotta := logoInks(p)
	return styles{title: p.Bold(), muted: p.Token(t.Muted), accent: p.Token(t.Accent), rule: p.Token(t.Divider),
		success: p.Token(t.Success), warning: p.Token(t.Warning), danger: p.Token(t.Danger),
		logoFace: face, logoSide: side, logoAccent: terracotta,
		selected: p.Emphasize(p.Token(t.Accent)), asciiLogo: p.Bare()}
}

// status renders glyph plus word in the workspace's own styles so its profile
// and mode apply; ui.Status would use the process-wide ones.
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
