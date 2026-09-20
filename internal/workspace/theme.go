package workspace

import (
	"io"

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
func newStyles(out io.Writer, theme string, plain bool) styles {
	r := lipgloss.NewRenderer(out)
	if plain {
		r.SetColorProfile(termenv.Ascii)
	}
	dark := theme == "dark"
	if theme == "auto" && !plain {
		dark = r.HasDarkBackground()
	}
	r.SetHasDarkBackground(dark)
	return workspaceStyles(r, theme, dark)
}

func workspaceStyles(r *lipgloss.Renderer, theme string, dark bool) styles {
	mode := ui.ModeLight
	if dark {
		mode = ui.ModeDark
	} else if theme == string(ui.ModeDim) {
		mode = ui.ModeDim
	}
	t := ui.Palette(mode)
	b := r.NewStyle()
	face, side, terracotta := logoPalette()
	return styles{title: b.Bold(true), muted: b.Foreground(t.Muted), accent: b.Foreground(t.Accent), rule: b.Foreground(t.Divider),
		success: b.Foreground(t.Success), warning: b.Foreground(t.Warning), danger: b.Foreground(t.Danger),
		logoFace: b.Foreground(face), logoSide: b.Foreground(side), logoAccent: b.Foreground(terracotta), asciiLogo: r.ColorProfile() == termenv.Ascii}
}

// status renders glyph plus word in the workspace's own renderer so the mode
// chosen in newStyles applies; ui.Status would use the process-wide styles.
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

// keyValue follows the plain-output rule: muted label right-aligned in the
// 12-cell column, two spaces, unstyled value. Empty values return "".
func (s styles) keyValue(f Field) string {
	value := f.Value
	if f.IsStatus() {
		value = s.status(value)
	}
	return ui.FormatKeyValue(f.Label, value, s.muted)
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
