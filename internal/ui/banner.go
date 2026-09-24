package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

var dcsLogo = []string{
	"██████╗  ██████╗███████╗",
	"██╔══██╗██╔════╝██╔════╝",
	"██║  ██║██║     ███████╗",
	"██║  ██║██║     ╚════██║",
	"██████╔╝╚██████╗███████║",
	"╚═════╝  ╚═════╝╚══════╝",
}

const (
	bannerFullMinWidth    = 60
	bannerCompactMinWidth = 40
	dividerMaxWidth       = 48
	commandColumnWidth    = 16
)

func PrintBanner(version string) {
	if quiet {
		return
	}
	if isCINoColor() {
		return
	}

	width := TerminalWidth()
	noColor := plain || isNoColor()

	switch {
	case width < bannerCompactMinWidth:
		fmt.Printf("DCS v%s\n", version)
	case width < bannerFullMinWidth:
		printCompactBanner(version, noColor)
	default:
		printFullBanner(version, noColor)
	}
}

func printCompactBanner(version string, noColor bool) {
	dcsStyle := painter.Emphasize(Accent)
	nameStyle := Muted
	verStyle := Muted
	fmt.Printf("  %s  %s  %s\n",
		styled(dcsStyle, "DCS", noColor),
		styled(nameStyle, "Dilmune Cloud Services", noColor),
		styled(verStyle, "v"+version, noColor))
}

func printFullBanner(version string, noColor bool) {
	nameStyle := Bold
	taglineStyle := Muted.Italic(true)
	versionStyle := Muted
	lineStyle := Divider

	fmt.Println()

	for _, line := range dcsLogo {
		fmt.Println("  " + renderGradientLine(line, noColor))
	}

	fmt.Println()
	fmt.Println(styled(lineStyle, "  "+strings.Repeat("─", dividerMaxWidth), noColor))
	fmt.Println()
	fmt.Printf("  %s\n", styled(nameStyle, "Dilmune Cloud Services", noColor))
	fmt.Printf("  %s   %s\n",
		styled(taglineStyle, "Infrastructure for Builders", noColor),
		styled(versionStyle, "v"+version, noColor))
	fmt.Println()
}

// styled applies style only when color output is allowed, so every banner
// line has a single call site to keep in sync with --no-color / NO_COLOR.
func styled(style lipgloss.Style, s string, noColor bool) string {
	if noColor {
		return s
	}
	return style.Render(s)
}

func renderGradientLine(line string, noColor bool) string {
	if noColor {
		return line
	}

	runes := []rune(line)
	count := utf8.RuneCountInString(line)
	if count == 0 {
		return ""
	}

	divisor := max(count-1, 1)

	var b strings.Builder
	for i, r := range runes {
		stop := interpolateGradient(bannerGradient, float64(i)/float64(divisor))
		b.WriteString(painter.Emphasize(painter.Token(stop)).Render(string(r)))
	}
	return b.String()
}

func interpolateGradient(stops []color.RGBA, t float64) color.RGBA {
	if t <= 0 || len(stops) < 2 {
		return stops[0]
	}
	if t >= 1 {
		return stops[len(stops)-1]
	}

	segment := t * float64(len(stops)-1)
	idx := int(segment)
	if idx >= len(stops)-1 {
		idx = len(stops) - 2
	}
	frac := segment - float64(idx)

	from, to := stops[idx], stops[idx+1]
	return color.RGBA{R: lerp(from.R, to.R, frac), G: lerp(from.G, to.G, frac), B: lerp(from.B, to.B, frac), A: 0xff}
}

func lerp(from, to uint8, frac float64) uint8 {
	return uint8(int(from) + int(float64(int(to)-int(from))*frac))
}

// TerminalWidth is the stdout column count for layout, 80 when stdout is
// not a terminal.
func TerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// isCINoColor skips the banner entirely in CI; isNoColor (theme.go) covers the
// interactive case where NO_COLOR must still suppress color per no-color.org.
func isCINoColor() bool {
	return isNoColor() && !IsTerminal()
}

func PrintCommands() {
	if quiet {
		return
	}

	headerStyle := Bold
	cmdStyle := painter.Emphasize(Accent).Width(commandColumnWidth)
	descStyle := Muted
	sectionStyle := Bold
	hintStyle := Muted

	fmt.Println(headerStyle.Render("  COMMANDS"))
	fmt.Println()

	sections := []struct {
		label    string
		commands []struct{ cmd, desc string }
	}{
		{
			label: "  Getting Started",
			commands: []struct{ cmd, desc string }{
				{"login", "Authenticate with DCS"},
				{"init", "Link directory to a project"},
				{"status", "Dashboard overview"},
			},
		},
		{
			label: "  Infrastructure",
			commands: []struct{ cmd, desc string }{
				{"servers", "Manage cloud servers"},
				{"sites", "Manage sites & deployments"},
				{"db", "Manage databases"},
				{"keys", "Manage SSH keys"},
				{"storage", "Manage object storage"},
			},
		},
		{
			label: "  Workflow",
			commands: []struct{ cmd, desc string }{
				{"deploy", "Deploy current project"},
				{"ssh", "SSH into a server"},
				{"env", "Manage environment variables"},
				{"logs", "View deployment logs"},
			},
		},
	}

	for _, s := range sections {
		fmt.Println(sectionStyle.Render(s.label))
		for _, c := range s.commands {
			fmt.Printf("    %s %s\n", cmdStyle.Render(c.cmd), descStyle.Render(c.desc))
		}
		fmt.Println()
	}

	fmt.Printf("  %s\n", hintStyle.Render("Run 'dcs <command> --help' for usage details."))
	fmt.Printf("  %s\n", hintStyle.Render("Use --json flag for machine-readable output."))
	fmt.Println()
}
