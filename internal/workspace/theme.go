package workspace

import (
	"fmt"
	"io"
	"math"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type styles struct {
	title, muted, accent, rule, logoFace, logoSide, logoAccent lipgloss.Style
	asciiLogo                                                  bool
}

const (
	logoCreamANSI256      = "230"
	logoCreamANSI         = "15"
	logoShadeANSI256      = "223"
	logoShadeANSI         = "7"
	logoTerracottaANSI256 = "173"
	logoTerracottaANSI    = "9"
)

// Semantic OKLCH tokens shared with Dilmune's design system. Conversion lives
// at the terminal boundary; the user's background and body color stay intact.
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
	primaryInk, muted, border := oklch(.47, .16, 35), oklch(.5, .012, 260), oklch(.9, .008, 80)
	if dark {
		primaryInk, muted, border = oklch(.705, .16, 35), oklch(.6, .01, 260), oklch(.26, .008, 260)
	} else if theme == "dim" {
		primaryInk, muted, border = oklch(.435, .17, 30), oklch(.44, .015, 50), oklch(.84, .025, 75)
	}
	b := r.NewStyle()
	// The approved asset uses this fixed terminal palette, independent of menu
	// theme. RGB values match its 256-color cells exactly, avoiding quantization.
	face := lipgloss.CompleteColor{TrueColor: "#ffffd7", ANSI256: logoCreamANSI256, ANSI: logoCreamANSI}
	side := lipgloss.CompleteColor{TrueColor: "#ffd7af", ANSI256: logoShadeANSI256, ANSI: logoShadeANSI}
	terracotta := lipgloss.CompleteColor{TrueColor: "#d7875f", ANSI256: logoTerracottaANSI256, ANSI: logoTerracottaANSI}
	return styles{title: b.Bold(true), muted: b.Foreground(muted), accent: b.Foreground(primaryInk), rule: b.Foreground(border),
		logoFace: b.Foreground(face), logoSide: b.Foreground(side), logoAccent: b.Foreground(terracotta), asciiLogo: r.ColorProfile() == termenv.Ascii}
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
