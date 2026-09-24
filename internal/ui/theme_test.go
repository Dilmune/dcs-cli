package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	prev, prevMode := painter, CurrentMode()
	t.Cleanup(func() {
		painter = prev
		applyMode(prevMode)
	})
	painter = testPainter(false)
	applyMode(prevMode)
}

// testPainter pins how much color the styles carry, so a recorded byte is the
// same on any machine.
func testPainter(plainOutput bool) Painter {
	if plainOutput {
		return NewPainter(colorprofile.NoTTY)
	}
	return NewPainter(colorprofile.TrueColor)
}

func rgbSeq(c color.Color) string {
	v := toRGB(c)
	return fmt.Sprintf("38;2;%d;%d;%d", v[0], v[1], v[2])
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

// terminalRecorder stands in for a terminal that never answers: it keeps every
// byte written to it, so a background query cannot go unnoticed.
type terminalRecorder struct{ bytes.Buffer }

// recordingDetector is a background detection that leaves the trace a real one
// leaves: the query goes to the terminal before the answer comes back.
func recordingDetector(tty io.Writer, detected bool, calls *int) func() bool {
	return func() bool {
		*calls++
		_, _ = io.WriteString(tty, ansi.RequestBackgroundColor)
		return detected
	}
}

func TestRecordingDetectorWritesABackgroundQuery(t *testing.T) {
	tty := &terminalRecorder{}
	calls := 0
	recordingDetector(tty, true, &calls)()
	assert.Equal(t, 1, calls)
	assert.Contains(t, tty.String(), "\x1b]11;?", "without this control the empty-recorder checks below prove nothing")
}

func TestInitMode_OnlyAutoOnAColorTerminalConsultsTheDetector(t *testing.T) {
	tests := []struct {
		name       string
		theme      Mode
		plain      bool
		detected   bool
		wantMode   Mode
		wantDetect int
	}{
		{"plain ignores an explicit theme", ModeLight, true, false, ModeDark, 0},
		{"plain ignores auto", ModeAuto, true, false, ModeDark, 0},
		{"explicit light", ModeLight, false, true, ModeLight, 0},
		{"explicit dim", ModeDim, false, true, ModeDim, 0},
		{"explicit dark", ModeDark, false, false, ModeDark, 0},
		{"auto on a dark terminal", ModeAuto, false, true, ModeDark, 1},
		{"auto on a light terminal", ModeAuto, false, false, ModeLight, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withStyles(t, tc.theme, tc.plain)
			tty := &terminalRecorder{}
			detections := 0
			initMode(tc.theme, testPainter(tc.plain), recordingDetector(tty, tc.detected, &detections))

			assert.Equal(t, tc.wantDetect, detections)
			assert.Equal(t, tc.wantMode, CurrentMode())
			assert.Equal(t, tc.plain, IsPlain())
			assert.Equal(t,
				huh.ThemeCharm(tc.wantMode.HasDarkBackground()).Focused.Description.GetForeground(),
				promptTheme().Theme(!tc.wantMode.HasDarkBackground()).Focused.Description.GetForeground(),
				"prompts follow the resolved mode, not the background huh offers to detect")
			if tc.wantDetect == 0 {
				assert.Empty(t, tty.String(), "an explicit background leaves nothing to query")
			}
		})
	}
}

func TestInitMode_PlainNeverReachesTheDetector(t *testing.T) {
	withStyles(t, ModeAuto, true)
	tty := &terminalRecorder{}
	detections := 0
	initMode(ModeAuto, testPainter(true), recordingDetector(tty, false, &detections))
	assert.Zero(t, detections)
	assert.Empty(t, tty.String())
	assert.Equal(t, ModeDark, CurrentMode())
	assert.Equal(t, colorprofile.NoTTY, painter.profile, "prompts render through this profile, so plain output stays silent")
}

type rgb [3]int

func toRGB(c color.Color) rgb {
	r, g, b, _ := c.RGBA()
	return rgb{int(r >> 8), int(g >> 8), int(b >> 8)}
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
	withTrueColor(t)
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
	withStyles(t, ModeLight, true)
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
	withStyles(t, ModeDark, true)
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
		color    color.Color
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

// withStyles pins the palette and the terminal's color support for one test,
// so a recorded golden is the same bytes on any machine.
func withStyles(t *testing.T, m Mode, plainOutput bool) {
	t.Helper()
	prevPainter, prevPlain, prevMode := painter, plain, CurrentMode()
	t.Cleanup(func() {
		painter, plain = prevPainter, prevPlain
		applyMode(prevMode)
	})
	initMode(m, testPainter(plainOutput), nil)
}

// captureStdout collects what print writes to stdout. The reader runs while
// print does, so output larger than the pipe buffer cannot deadlock it.
func captureStdout(t *testing.T, print func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	prev := os.Stdout
	os.Stdout = w
	captured := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		captured <- buf.String()
	}()
	print()
	os.Stdout = prev
	require.NoError(t, w.Close())
	return <-captured
}
