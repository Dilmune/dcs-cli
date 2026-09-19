package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const (
	tableLeftMargin  = 2
	tableCellPadding = 3
	// Columns narrower than this are never truncated.
	tableMinColWidth = 8
)

// PrintTable renders a formatted table with headers and rows, fitting the
// terminal width by truncating the widest text columns with an ellipsis.
func PrintTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Println(Muted.Render("  No results found."))
		return
	}

	widths := contentWidths(headers, rows)

	for i := range widths {
		widths[i] += tableCellPadding
	}

	fitToTerminal(widths)

	headerStyle := lipgloss.NewStyle().Foreground(TextMuted).Bold(true)
	cellStyle := lipgloss.NewStyle().Foreground(TextWhite)

	fmt.Print("  ")
	for i, h := range headers {
		fmt.Print(headerStyle.Width(widths[i]).Render(strings.ToUpper(h)))
	}
	fmt.Println()

	fmt.Print("  ")
	for _, w := range widths {
		fmt.Print(Dim.Render(strings.Repeat("─", w)))
	}
	fmt.Println()

	for _, row := range rows {
		fmt.Print("  ")
		for i, cell := range row {
			if i < len(widths) {
				maxChars := widths[i] - tableCellPadding
				cell = truncateRunes(cell, maxChars)
				fmt.Print(cellStyle.Width(widths[i]).Render(cell))
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

func contentWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				n := visibleLen(cell)
				if n > widths[i] {
					widths[i] = n
				}
			}
		}
	}
	return widths
}

func visibleLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

func fitToTerminal(widths []int) {
	tw := terminalWidth()
	total := tableLeftMargin
	for _, w := range widths {
		total += w
	}
	if total <= tw {
		return
	}

	excess := total - tw
	for excess > 0 {
		maxIdx, maxW := -1, 0
		for i, w := range widths {
			if w > maxW {
				maxW = w
				maxIdx = i
			}
		}
		if maxIdx < 0 || widths[maxIdx] <= tableMinColWidth+tableCellPadding {
			break
		}
		shrink := excess
		if shrink > widths[maxIdx]-(tableMinColWidth+tableCellPadding) {
			shrink = widths[maxIdx] - (tableMinColWidth + tableCellPadding)
		}
		widths[maxIdx] -= shrink
		excess -= shrink
	}
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	n := visibleLen(s)
	if n <= maxRunes {
		return s
	}
	// Cheap path for strings with no ANSI escapes.
	runes := []rune(s)
	if len(runes) == n {
		if maxRunes <= 3 {
			return string(runes[:maxRunes])
		}
		return string(runes[:maxRunes-3]) + "..."
	}
	// ANSI-aware truncation: count only visible characters.
	var b strings.Builder
	visible := 0
	limit := maxRunes - 3
	if limit < 0 {
		limit = 0
	}
	inEsc := false
	for _, r := range s {
		if inEsc {
			b.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			b.WriteRune(r)
			inEsc = true
			continue
		}
		if visible >= limit {
			break
		}
		b.WriteRune(r)
		visible++
	}
	b.WriteString("...")
	return b.String()
}

// PrintKeyValue renders a key-value pair.
func PrintKeyValue(key, value string) {
	fmt.Printf("  %s %s\n", KeyStyle.Render(key+":"), ValStyle.Render(value))
}

// PrintSection renders a section header.
func PrintSection(title string) {
	fmt.Println()
	fmt.Printf("  %s\n", Title.Render(title))
	fmt.Println()
}
