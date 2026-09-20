package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "active", StatusActive)
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "error", StatusError)
	assert.Equal(t, "failed", StatusFailed)
	assert.Equal(t, "provisioning", StatusProvisioning)
	assert.Equal(t, "deploying", StatusDeploying)
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "error", EventError)
	assert.Equal(t, "success", EventSuccess)
	assert.Equal(t, "completed", EventCompleted)
	assert.Equal(t, "progress", EventProgress)
	assert.Equal(t, "deploy", EventDeploy)
	assert.Equal(t, "status_changed", EventStatusChanged)
}

func TestBillingConstants(t *testing.T) {
	assert.Equal(t, "grace_period", BillingGracePeriod)
	assert.Equal(t, StatusSuspended, BillingSuspended)
	assert.Equal(t, StatusDelinquent, BillingDelinquent)
}

func withTrueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func rgbSeq(c lipgloss.Color) string {
	r, g, b := hexToRGB(string(c))
	return fmt.Sprintf("38;2;%d;%d;%d", r, g, b)
}

func TestResolveMode(t *testing.T) {
	dark := func() bool { return true }
	light := func() bool { return false }
	tests := []struct {
		name   string
		theme  Mode
		detect func() bool
		want   Mode
	}{
		{"explicit light ignores detection", ModeLight, dark, ModeLight},
		{"explicit dim ignores detection", ModeDim, dark, ModeDim},
		{"explicit dark ignores detection", ModeDark, light, ModeDark},
		{"auto follows a dark background", ModeAuto, dark, ModeDark},
		{"auto follows a light background", ModeAuto, light, ModeLight},
		{"empty flag with no detector falls back to dark", "", nil, ModeDark},
		{"unknown flag behaves like auto", "solarized", light, ModeLight},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveMode(tc.theme, tc.detect))
		})
	}
}

type rgb [3]int

func toRGB(c lipgloss.Color) rgb {
	r, g, b := hexToRGB(string(c))
	return rgb{r, g, b}
}

// Golden sRGB values for the OKLCH table in docs/DESIGN.md, kept as decimal
// triples so the hex grep stays clean. A change here is a change to the design
// standard, not a refactor.
func TestPalette_MatchesDesignTable(t *testing.T) {
	tests := map[Mode][6]rgb{
		ModeLight: {{161, 42, 7}, {95, 99, 106}, {225, 221, 216}, {5, 137, 62}, {171, 116, 0}, {204, 40, 39}},
		ModeDim:   {{153, 15, 5}, {90, 80, 75}, {212, 201, 185}, {5, 137, 62}, {171, 116, 0}, {204, 40, 39}},
		ModeDark:  {{240, 116, 86}, {125, 128, 134}, {54, 56, 60}, {85, 201, 117}, {234, 181, 50}, {247, 93, 89}},
	}
	for mode, want := range tests {
		p := Palette(mode)
		got := [6]rgb{toRGB(p.Accent), toRGB(p.Muted), toRGB(p.Divider), toRGB(p.Success), toRGB(p.Warning), toRGB(p.Danger)}
		assert.Equal(t, want, got, "mode %s", mode)
	}
	assert.Equal(t, Palette(ModeDark), Palette(ModeAuto), "unresolved auto must render as dark")
}

func TestApplyMode_RebuildsEveryStyleFromTheTokens(t *testing.T) {
	t.Cleanup(func() { applyMode(ModeDark) })
	for _, mode := range []Mode{ModeLight, ModeDim, ModeDark} {
		applyMode(mode)
		want := Palette(mode)
		assert.Equal(t, mode, CurrentMode())
		assert.Equal(t, want.Accent, Accent.GetForeground())
		assert.Equal(t, want.Muted, Muted.GetForeground())
		assert.Equal(t, want.Muted, Info.GetForeground(), "info borrows the muted token")
		assert.Equal(t, want.Divider, Divider.GetForeground())
		assert.Equal(t, want.Success, Success.GetForeground())
		assert.Equal(t, want.Warning, Warning.GetForeground())
		assert.Equal(t, want.Danger, Error.GetForeground())
		assert.Equal(t, lipgloss.NoColor{}, Title.GetForeground(), "titles use the terminal foreground")
		assert.Equal(t, lipgloss.NoColor{}, Bold.GetForeground())
	}
}

func TestInit_PlainOutputHasNoEscapes(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	prev := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		applyMode(ModeDark)
	})
	Init(string(ModeLight), true)
	assert.True(t, IsPlain())
	assert.Equal(t, ModeDark, CurrentMode(), "plain output never queries the terminal")
	assert.Equal(t, "x", Muted.Render("x"))
	assert.Equal(t, "x", Bold.Render("x"))
	assert.Equal(t, "● active", Status(StatusActive))
	assert.Equal(t, "  "+strings.Repeat(" ", 8)+"Name  web-1", formatKeyValue("Name", "web-1"))
}

func TestInit_NoColorEnvIsPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	prev := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(prev)
		applyMode(ModeDark)
	})
	Init(string(ModeDark), false)
	assert.True(t, IsPlain())
	assert.Equal(t, "✓", Success.Render("✓"))
}

func TestStatus_GlyphWordAndToken(t *testing.T) {
	withTrueColor(t)
	tokens := Palette(CurrentMode())
	tests := []struct {
		name     string
		statuses []string
		glyph    string
		color    lipgloss.Color
	}{
		{"healthy", []string{StatusActive, StatusRunning, StatusDeployed, StatusCompleted, StatusSuccess, StatusEnabled}, GlyphHealthy, tokens.Success},
		{"in progress", []string{StatusProvisioning, StatusInstalling, StatusDeploying, StatusBuilding, StatusPending}, GlyphInProgress, tokens.Warning},
		{"off", []string{StatusOff, StatusStopped, StatusInactive, StatusDisabled}, GlyphOff, tokens.Muted},
		{"failed", []string{StatusError, StatusFailed, StatusSuspended, StatusDelinquent}, GlyphFailed, tokens.Danger},
		{"gone", []string{StatusDeleting, StatusDeleted}, GlyphGone, tokens.Muted},
		{"unknown", []string{"mystery"}, GlyphGone, tokens.Muted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, status := range tc.statuses {
				got := Status(status)
				assert.Equal(t, tc.glyph+" "+status, ansi.Strip(got), "glyph and word for %q", status)
				assert.Contains(t, got, rgbSeq(tc.color), "token for %q", status)
			}
		})
	}
}

func TestStatus_EmptyRendersNothing(t *testing.T) {
	assert.Equal(t, "", Status(""))
}

func TestWithStatusColumns_CopiesAndLeavesInputPlain(t *testing.T) {
	rows := [][]string{{"web-1", StatusActive, StatusOff}, {"web-2", StatusFailed, StatusActive}}
	got := WithStatusColumns(rows, 1, 2)
	assert.Equal(t, [][]string{{"web-1", StatusActive, StatusOff}, {"web-2", StatusFailed, StatusActive}}, rows, "CSV rows must keep the bare word")
	assert.Equal(t, "● active", ansi.Strip(got[0][1]))
	assert.Equal(t, "○ off", ansi.Strip(got[0][2]))
	assert.Equal(t, "✕ failed", ansi.Strip(got[1][1]))
	assert.Equal(t, "web-2", got[1][0])
}
