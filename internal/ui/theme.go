package ui

import (
	"image/color"
	"io"
	"math"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Mode is the resolved color mode. Tokens are picked once per process from
// the table in docs/DESIGN.md; body text always keeps the terminal foreground.
type Mode string

const (
	ModeAuto  Mode = "auto"
	ModeLight Mode = "light"
	ModeDim   Mode = "dim"
	ModeDark  Mode = "dark"
)

// Tokens holds the six semantic colors for one mode.
type Tokens struct {
	Accent, Muted, Divider, Success, Warning, Danger color.Color
}

const (
	accentHue    = 35
	accentChroma = .16
)

// Palette returns the OKLCH tokens for a mode, converted to sRGB at this
// boundary and nowhere else.
func Palette(mode Mode) Tokens {
	switch mode {
	case ModeLight:
		return Tokens{
			Accent:  oklch(.47, accentChroma, accentHue),
			Muted:   oklch(.50, .012, 260),
			Divider: oklch(.90, .008, 80),
			Success: oklch(.55, .15, 150),
			Warning: oklch(.60, .14, 80),
			Danger:  oklch(.55, .20, 27),
		}
	case ModeDim:
		return Tokens{
			Accent:  oklch(.435, .17, 30),
			Muted:   oklch(.44, .015, 50),
			Divider: oklch(.84, .025, 75),
			Success: oklch(.55, .15, 150),
			Warning: oklch(.60, .14, 80),
			Danger:  oklch(.55, .20, 27),
		}
	default:
		return Tokens{
			Accent:  oklch(.70, accentChroma, accentHue),
			Muted:   oklch(.60, .01, 260),
			Divider: oklch(.34, .008, 260),
			Success: oklch(.75, .16, 150),
			Warning: oklch(.80, .15, 85),
			Danger:  oklch(.68, .19, 25),
		}
	}
}

// HasDarkBackground reports whether the mode's tokens are drawn for a dark
// background. Like Palette, anything but light or dim is dark.
func (m Mode) HasDarkBackground() bool {
	return m != ModeLight && m != ModeDim
}

// Painter builds styles for one output. It carries how much color that output
// can show, resolved once from the environment and from whether the output is
// a terminal, never from a query, so plain output and an explicit --theme stay
// silent.
type Painter struct{ profile colorprofile.Profile }

// NewPainter builds a painter for a profile the caller already resolved.
func NewPainter(p colorprofile.Profile) Painter { return Painter{profile: p} }

// TerminalProfile reports how much color an output can show. Plain output
// shows nothing at all, which is how --no-color and NO_COLOR drop every escape
// sequence without touching the layout.
func TerminalProfile(out io.Writer, plainOutput bool) colorprofile.Profile {
	if plainOutput {
		return colorprofile.NoTTY
	}
	env := os.Environ()
	profile := colorprofile.Detect(out, env)
	if profile > colorprofile.ASCII && profile < colorprofile.TrueColor && tmuxCarriesTrueColor(env) {
		return colorprofile.TrueColor
	}
	return profile
}

// tmuxCarriesTrueColor reports whether COLORTERM's 24-bit announcement still
// holds inside tmux. colorprofile discards COLORTERM for a tmux or screen
// TERM, which costs every token 16 million colors down to 256. Measured on
// tmux 3.7b a pane stores the exact 24-bit triple and forwards it to its
// client, so the announcement holds; GNU Screen, which sets no TMUX, keeps the
// downgrade it needs.
func tmuxCarriesTrueColor(env []string) bool {
	var inTmux, announced bool
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		switch name {
		case "TMUX":
			inTmux = value != ""
		case "COLORTERM":
			switch strings.ToLower(value) {
			case "truecolor", "24bit":
				announced = true
			}
		}
	}
	return inTmux && announced
}

// Token renders text in one palette token, downgraded to what the output can
// show. An output with no color renders bare text.
func (p Painter) Token(c color.Color) lipgloss.Style {
	return styleFor(p.profile.Convert(c))
}

// Fixed picks the cell of a fixed terminal palette that matches what the
// output can show. The workspace logo is the only artwork specified that way.
func (p Painter) Fixed(ansi, ansi256, truecolor color.Color) lipgloss.Style {
	return styleFor(lipgloss.Complete(p.profile)(ansi, ansi256, truecolor))
}

// Bold is the only emphasis docs/DESIGN.md allows, and it leaves with the
// color when the output carries no escape sequences.
func (p Painter) Bold() lipgloss.Style { return p.Emphasize(lipgloss.NewStyle()) }

// Emphasize adds bold to a style already carrying a token. Plain output has to
// keep every escape sequence out, an attribute as much as a color, so there it
// returns the style untouched.
func (p Painter) Emphasize(s lipgloss.Style) lipgloss.Style {
	if p.Bare() {
		return s
	}
	return s.Bold(true)
}

// Bare reports whether this output renders text with no escape sequence at all.
func (p Painter) Bare() bool { return p.profile <= colorprofile.ASCII }

func styleFor(c color.Color) lipgloss.Style {
	if c == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}

// Styles default to dark so output before Init (tests, early errors) still
// resolves to a token; Init rebuilds them for the resolved mode.
var (
	mode    = ModeDark
	plain   bool
	painter = NewPainter(TerminalProfile(os.Stdout, false))

	Bold    = painter.Bold()
	Title   = painter.Bold()
	Accent  = painter.Token(Palette(ModeDark).Accent)
	Muted   = painter.Token(Palette(ModeDark).Muted)
	Divider = painter.Token(Palette(ModeDark).Divider)
	Success = painter.Token(Palette(ModeDark).Success)
	Warning = painter.Token(Palette(ModeDark).Warning)
	Error   = painter.Token(Palette(ModeDark).Danger)
	Info    = Muted

	bannerGradient = accentGradient(bannerGradientSteps, bannerGradientMinL, bannerGradientMaxL)
)

// Init resolves the color mode once for the process. Explicit theme wins,
// then the terminal background, then dark. --no-color, NO_COLOR, a non-TTY
// stdout and --quiet all disable escape sequences without changing layout.
func Init(theme string, noColor bool) {
	plainOutput := noColor || isNoColor() || !IsTerminal() || quiet
	initMode(Mode(theme), NewPainter(TerminalProfile(os.Stdout, plainOutput)), detectDarkBackground)
}

// detectDarkBackground asks the terminal what it is drawing on. Only auto on a
// real terminal reaches it; a terminal that never answers costs the query its
// two-second timeout and then reads as dark.
func detectDarkBackground() bool {
	return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}

func initMode(theme Mode, p Painter, detectDark func() bool) {
	painter, plain = p, p.Bare()
	if plain {
		applyMode(ModeDark)
		return
	}
	applyMode(ResolveMode(theme, detectDark))
}

// ResolveMode maps a theme flag to a concrete mode. detectDark is only
// consulted for auto so tests and plain output never query the terminal.
func ResolveMode(theme Mode, detectDark func() bool) Mode {
	switch theme {
	case ModeLight, ModeDim, ModeDark:
		return theme
	}
	if detectDark != nil && !detectDark() {
		return ModeLight
	}
	return ModeDark
}

// CurrentMode reports the mode chosen by Init.
func CurrentMode() Mode { return mode }

// IsPlain reports whether Init disabled escape sequences.
func IsPlain() bool { return plain }

func applyMode(m Mode) {
	mode = m
	t := Palette(m)
	Bold = painter.Bold()
	Title = painter.Bold()
	Accent = painter.Token(t.Accent)
	Muted = painter.Token(t.Muted)
	Divider = painter.Token(t.Divider)
	Success = painter.Token(t.Success)
	Warning = painter.Token(t.Warning)
	Error = painter.Token(t.Danger)
	Info = Muted
	bannerGradient = accentGradient(bannerGradientSteps, bannerGradientMinL, bannerGradientMaxL)
}

const (
	bannerGradientSteps = 7
	bannerGradientMinL  = .40
	bannerGradientMaxL  = .78
)

func accentGradient(steps int, minL, maxL float64) []color.RGBA {
	stops := make([]color.RGBA, steps)
	for i := range stops {
		l := minL + (maxL-minL)*float64(i)/float64(steps-1)
		stops[i] = oklch(l, accentChroma, accentHue)
	}
	return stops
}

func isNoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

func oklch(l, c, h float64) color.RGBA {
	h *= math.Pi / 180
	a, b := c*math.Cos(h), c*math.Sin(h)
	x := math.Pow(l+.3963377774*a+.2158037573*b, 3)
	y := math.Pow(l-.1055613458*a-.0638541728*b, 3)
	z := math.Pow(l-.0894841775*a-1.291485548*b, 3)
	return color.RGBA{
		R: srgb(4.0767416621*x - 3.3077115913*y + .2309699292*z),
		G: srgb(-1.2684380046*x + 2.6097574011*y - .3413193965*z),
		B: srgb(-.0041960863*x - .7034186147*y + 1.707614701*z),
		A: 0xff,
	}
}

func srgb(v float64) uint8 {
	if v <= .0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - .055
	}
	return uint8(math.Round(max(0, min(1, v)) * 255))
}

// Resource statuses.
const (
	StatusActive       = "active"
	StatusRunning      = "running"
	StatusDeployed     = "deployed"
	StatusCompleted    = "completed"
	StatusSuccess      = "success"
	StatusEnabled      = "enabled"
	StatusProvisioning = "provisioning"
	StatusInstalling   = "installing"
	StatusDeploying    = "deploying"
	StatusBuilding     = "building"
	StatusPending      = "pending"
	StatusOff          = "off"
	StatusStopped      = "stopped"
	StatusInactive     = "inactive"
	StatusDisabled     = "disabled"
	StatusError        = "error"
	StatusFailed       = "failed"
	StatusSuspended    = "suspended"
	StatusDelinquent   = "delinquent"
	StatusDeleting     = "deleting"
	StatusDeleted      = "deleted"
)

// Billing statuses.
const (
	BillingGracePeriod = "grace_period"
	BillingSuspended   = StatusSuspended
	BillingDelinquent  = StatusDelinquent
)

// Event types used in log/event display.
const (
	EventError         = "error"
	EventSuccess       = "success"
	EventCompleted     = "completed"
	EventProgress      = "progress"
	EventDeploy        = "deploy"
	EventStatusChanged = "status_changed"
)

// Status glyphs. The glyph carries the meaning so --no-color loses nothing.
const (
	GlyphHealthy    = "●"
	GlyphInProgress = "◐"
	GlyphOff        = "○"
	GlyphFailed     = "✕"
	GlyphGone       = "·"
)

// Tone names the token a status renders in. Callers with their own renderer
// (the workspace) map it to their styles so mode resolution stays in one place.
type Tone uint8

const (
	ToneSuccess Tone = iota
	ToneWarning
	ToneMuted
	ToneDanger
)

// Status renders a status as glyph plus word in the matching token. An empty
// status renders nothing so key-value rows can hide it.
func Status(status string) string {
	if status == "" {
		return ""
	}
	glyph, tone := StatusGlyph(status)
	return toneStyle(tone).Render(glyph + " " + status)
}

// StatusGlyph maps a status word to its glyph and tone. Unknown words read as
// gone rather than healthy so a new API state never looks fine by accident.
func StatusGlyph(status string) (string, Tone) {
	switch status {
	case StatusActive, StatusRunning, StatusDeployed, StatusCompleted, StatusSuccess, StatusEnabled:
		return GlyphHealthy, ToneSuccess
	case StatusProvisioning, StatusInstalling, StatusDeploying, StatusBuilding, StatusPending:
		return GlyphInProgress, ToneWarning
	case StatusOff, StatusStopped, StatusInactive, StatusDisabled:
		return GlyphOff, ToneMuted
	case StatusError, StatusFailed, StatusSuspended, StatusDelinquent:
		return GlyphFailed, ToneDanger
	case StatusDeleting, StatusDeleted:
		return GlyphGone, ToneMuted
	default:
		return GlyphGone, ToneMuted
	}
}

func toneStyle(tone Tone) lipgloss.Style {
	switch tone {
	case ToneSuccess:
		return Success
	case ToneWarning:
		return Warning
	case ToneDanger:
		return Error
	default:
		return Muted
	}
}

// WithStatusColumns returns a copy of rows with the given columns rendered
// through Status. Callers hand the plain rows to PrintFormatted first so CSV
// keeps the bare word.
func WithStatusColumns(rows [][]string, cols ...int) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = make([]string, len(row))
		copy(out[i], row)
		for _, c := range cols {
			if c < len(row) {
				out[i][c] = Status(row[c])
			}
		}
	}
	return out
}
