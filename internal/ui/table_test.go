package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatKeyValue_LabelRightAlignedInTwelveCells(t *testing.T) {
	tests := []struct {
		name, label, value, want string
	}{
		{"short label pads to the column", "ID", "srv-1", "            ID  srv-1"},
		{"twelve-cell label needs no padding", "Twelve Chars", "x", "  Twelve Chars  x"},
		{"longer label is not truncated", "Default server", "srv-1", "  Default server  srv-1"},
		{"value keeps its own styling untouched", "SSL", "● active", "           SSL  ● active"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ansi.Strip(formatKeyValue(tc.label, tc.value)))
		})
	}
}

func TestFormatKeyValue_EmptyValueHidesTheRow(t *testing.T) {
	assert.Equal(t, "", formatKeyValue("IPv4", ""))
}

func TestFormatKeyValue_LabelUsesMutedAndValueIsUnstyled(t *testing.T) {
	withTrueColor(t)
	line := formatKeyValue("Name", "web-1")
	assert.Contains(t, line, rgbSeq(Palette(CurrentMode()).Muted))
	assert.True(t, strings.HasSuffix(line, "  web-1"), "value must follow two plain spaces with no escape: %q", line)
}

func plainLines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		out = append(out, ansi.Strip(l))
	}
	return out
}

func TestRenderTable_FixedColumnsKeepTheirWidth(t *testing.T) {
	headers := []string{"Name", "Status", "IPv4"}
	rows := [][]string{{"a-very-long-server-name-that-overflows", "◐ provisioning", "203.0.113.1"}}
	// 2 margin + (38+3) + (14+3) + (11+3) = 74 cells; ask for 60.
	const termWidth = 60

	out := renderTable(headers, rows, []bool{false, true, true}, termWidth)
	lines := plainLines(out)
	require.Len(t, lines, 3)
	for _, l := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(l), termWidth, "line overflows: %q", l)
	}
	assert.Contains(t, lines[2], "◐ provisioning", "status is fixed and must not shrink")
	assert.Contains(t, lines[2], "203.0.113.1", "IPv4 is fixed and must not shrink")
	assert.Contains(t, lines[2], "…", "the free-text column ends in a one-cell ellipsis")
	assert.NotContains(t, lines[2], "...", "never the three-dot ellipsis")
	assert.True(t, strings.HasPrefix(lines[0], leftMargin+"NAME"), "headers are uppercase behind the two-cell margin: %q", lines[0])
}

func TestRenderTable_WithoutFixedColumnsTheWidestShrinks(t *testing.T) {
	headers := []string{"Name", "Status"}
	rows := [][]string{{"web", "a-status-word-longer-than-any-name-column"}}
	out := renderTable(headers, rows, nil, 30)
	lines := plainLines(out)
	assert.Contains(t, lines[2], "…")
	assert.Contains(t, lines[2], "web")
}

func TestFitToTerminal_NeverShrinksFixedOrBelowFloor(t *testing.T) {
	widths := []int{40, 20, 15}
	fitToTerminal(widths, []bool{false, true, false}, 40)
	assert.Equal(t, 20, widths[1], "fixed column keeps its width")
	assert.GreaterOrEqual(t, widths[0], tableMinColWidth+tableCellPadding)
	assert.GreaterOrEqual(t, widths[2], tableMinColWidth+tableCellPadding)

	all := []int{20, 20}
	fitToTerminal(all, []bool{true, true}, 10)
	assert.Equal(t, []int{20, 20}, all, "all-fixed tables overflow rather than lose shape")
}

func TestRenderTable_HeaderRuleUsesDividerAndCellsAreUnstyled(t *testing.T) {
	withTrueColor(t)
	out := renderTable([]string{"Name"}, [][]string{{"web-1"}}, nil, 80)
	lines := strings.Split(out, "\n")
	tokens := Palette(CurrentMode())
	assert.Contains(t, lines[0], rgbSeq(tokens.Muted), "headers are muted")
	assert.NotContains(t, lines[0], "\x1b[1m", "headers are not bold")
	assert.Contains(t, lines[1], rgbSeq(tokens.Divider), "rule uses the divider token")
	assert.Equal(t, "  web-1   ", lines[2], "cells carry no escape sequences")
}
