package ui

import (
	"fmt"
	"math"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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
	Accent, Muted, Divider, Success, Warning, Danger lipgloss.Color
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

// Styles default to dark so output before Init (tests, early errors) still
// resolves to a token; Init rebuilds them for the resolved mode.
var (
	mode  = ModeDark
	plain bool

	Bold    = lipgloss.NewStyle().Bold(true)
	Title   = lipgloss.NewStyle().Bold(true)
	Accent  = lipgloss.NewStyle().Foreground(Palette(ModeDark).Accent)
	Muted   = lipgloss.NewStyle().Foreground(Palette(ModeDark).Muted)
	Divider = lipgloss.NewStyle().Foreground(Palette(ModeDark).Divider)
	Success = lipgloss.NewStyle().Foreground(Palette(ModeDark).Success)
	Warning = lipgloss.NewStyle().Foreground(Palette(ModeDark).Warning)
	Error   = lipgloss.NewStyle().Foreground(Palette(ModeDark).Danger)
	Info    = Muted

	bannerGradient = accentGradient(bannerGradientSteps, bannerGradientMinL, bannerGradientMaxL)
)

// Init resolves the color mode once for the process. Explicit theme wins,
// then the terminal background, then dark. --no-color, NO_COLOR, a non-TTY
// stdout and --quiet all disable escape sequences without changing layout.
func Init(theme string, noColor bool) {
	plain = noColor || isNoColor() || !IsTerminal() || quiet
	if plain {
		lipgloss.SetColorProfile(termenv.Ascii)
		applyMode(ModeDark)
		return
	}
	applyMode(ResolveMode(Mode(theme), lipgloss.HasDarkBackground))
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
	Accent = lipgloss.NewStyle().Foreground(t.Accent)
	Muted = lipgloss.NewStyle().Foreground(t.Muted)
	Divider = lipgloss.NewStyle().Foreground(t.Divider)
	Success = lipgloss.NewStyle().Foreground(t.Success)
	Warning = lipgloss.NewStyle().Foreground(t.Warning)
	Error = lipgloss.NewStyle().Foreground(t.Danger)
	Info = Muted
	bannerGradient = accentGradient(bannerGradientSteps, bannerGradientMinL, bannerGradientMaxL)
}

const (
	bannerGradientSteps = 7
	bannerGradientMinL  = .40
	bannerGradientMaxL  = .78
)

func accentGradient(steps int, minL, maxL float64) []lipgloss.Color {
	stops := make([]lipgloss.Color, steps)
	for i := range stops {
		l := minL + (maxL-minL)*float64(i)/float64(steps-1)
		stops[i] = oklch(l, accentChroma, accentHue)
	}
	return stops
}

func isNoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

func oklch(l, c, h float64) lipgloss.Color {
	h *= math.Pi / 180
	a, b := c*math.Cos(h), c*math.Sin(h)
	x := math.Pow(l+.3963377774*a+.2158037573*b, 3)
	y := math.Pow(l-.1055613458*a-.0638541728*b, 3)
	z := math.Pow(l-.0894841775*a-1.291485548*b, 3)
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", srgb(4.0767416621*x-3.3077115913*y+.2309699292*z),
		srgb(-1.2684380046*x+2.6097574011*y-.3413193965*z), srgb(-.0041960863*x-.7034186147*y+1.707614701*z)))
}

func srgb(v float64) int {
	if v <= .0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - .055
	}
	return int(math.Round(max(0, min(1, v)) * 255))
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
