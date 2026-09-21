package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	tableLeftMargin  = 2
	tableCellPadding = 3
	// Columns narrower than this are never truncated.
	tableMinColWidth = 8
	tableEllipsis    = "…"
	leftMargin       = "  "

	keyValueLabelWidth = 12
	keyValueGap        = "  "
)

// PrintTable renders a table where every column may shrink to fit the terminal.
func PrintTable(headers []string, rows [][]string) {
	PrintTableFixed(headers, rows, nil)
}

// PrintTableFixed renders a table whose fixed columns keep their content width;
// only the remaining columns shrink, widest first, with a one-cell ellipsis.
func PrintTableFixed(headers []string, rows [][]string, fixed []bool) {
	if len(rows) == 0 {
		fmt.Println(Muted.Render(leftMargin + "No results found."))
		return
	}
	fmt.Print(renderTable(headers, rows, fixed, TerminalWidth()))
}

func renderTable(headers []string, rows [][]string, fixed []bool, termWidth int) string {
	widths := contentWidths(headers, rows)
	for i := range widths {
		widths[i] += tableCellPadding
	}
	fitToTerminal(widths, fixed, termWidth)

	var b strings.Builder
	b.WriteString(leftMargin)
	for i, h := range headers {
		b.WriteString(Muted.Render(padCell(strings.ToUpper(h), widths[i])))
	}
	b.WriteString("\n")

	b.WriteString(leftMargin)
	for _, w := range widths {
		b.WriteString(Divider.Render(strings.Repeat("─", w)))
	}
	b.WriteString("\n")

	for _, row := range rows {
		b.WriteString(leftMargin)
		for i, cell := range row {
			if i < len(widths) {
				cell = ansi.Truncate(cell, widths[i]-tableCellPadding, tableEllipsis)
				b.WriteString(padCell(cell, widths[i]))
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func padCell(s string, width int) string {
	if gap := width - ansi.StringWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func contentWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = ansi.StringWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if n := ansi.StringWidth(cell); n > widths[i] {
					widths[i] = n
				}
			}
		}
	}
	return widths
}

func fitToTerminal(widths []int, fixed []bool, termWidth int) {
	total := tableLeftMargin
	for _, w := range widths {
		total += w
	}
	excess := total - termWidth
	floor := tableMinColWidth + tableCellPadding
	for excess > 0 {
		maxIdx, maxW := -1, 0
		for i, w := range widths {
			if isFixed(fixed, i) {
				continue
			}
			if w > maxW {
				maxW = w
				maxIdx = i
			}
		}
		if maxIdx < 0 || widths[maxIdx] <= floor {
			break
		}
		shrink := min(excess, widths[maxIdx]-floor)
		widths[maxIdx] -= shrink
		excess -= shrink
	}
}

func isFixed(fixed []bool, i int) bool {
	return i < len(fixed) && fixed[i]
}

// PrintKeyValue renders a muted right-aligned label, two spaces, and the value
// in the terminal foreground. An empty value prints nothing.
func PrintKeyValue(label, value string) {
	if line := formatKeyValue(label, value); line != "" {
		fmt.Println(line)
	}
}

// PrintKeyValues renders a block of key-value rows, skipping empty values.
func PrintKeyValues(pairs [][2]string) {
	for _, p := range pairs {
		PrintKeyValue(p[0], p[1])
	}
}

func formatKeyValue(label, value string) string {
	if line := FormatKeyValue(label, value, Muted); line != "" {
		return leftMargin + line
	}
	return ""
}

// FormatKeyValue lays out one key-value row without a margin: the label in
// labelStyle, right-aligned in a 12-cell column, two spaces, the value as
// given. Labels wider than the column are not truncated and take no padding.
// An empty value returns an empty string so callers can hide the row.
func FormatKeyValue(label, value string, labelStyle lipgloss.Style) string {
	if value == "" {
		return ""
	}
	pad := keyValueLabelWidth - ansi.StringWidth(label)
	if pad < 0 {
		pad = 0
	}
	return labelStyle.Render(strings.Repeat(" ", pad)+label) + keyValueGap + value
}

// PrintSection renders a bold title with one blank line on each side.
func PrintSection(title string) {
	fmt.Println()
	fmt.Printf("%s%s\n", leftMargin, Title.Render(title))
	fmt.Println()
}
