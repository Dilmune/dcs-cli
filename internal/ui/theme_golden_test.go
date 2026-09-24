package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One call per style the CLI prints with, so a golden covers the bytes a
// terminal receives rather than the style object behind them.
var goldenSamples = []struct {
	name   string
	render func() string
}{
	{"title", func() string { return Title.Render("Servers") }},
	{"bold", func() string { return Bold.Render("COMMANDS") }},
	{"muted", func() string { return Muted.Render("region") }},
	{"accent", func() string { return Accent.Render("$") }},
	{"divider", func() string { return Divider.Render("────") }},
	{"success", func() string { return Success.Render("healthy") }},
	{"warning", func() string { return Warning.Render("!") }},
	{"danger", func() string { return Error.Render("✗") }},
	{"info", func() string { return Info.Render("i") }},
	{"status active", func() string { return Status(StatusActive) }},
	{"status provisioning", func() string { return Status(StatusProvisioning) }},
	{"status off", func() string { return Status(StatusOff) }},
	{"status failed", func() string { return Status(StatusFailed) }},
	{"status deleted", func() string { return Status(StatusDeleted) }},
	{"key value", func() string { return formatKeyValue("Name", "web-1") }},
	{"key value bare", func() string { return FormatKeyValue("Region", "hel1", Muted) }},
	{"key value status", func() string { return FormatKeyValue("Status", Status(StatusActive), Muted) }},
}

var goldenLight = map[string]string{
	"title":               "\x1b[1mServers\x1b[m",
	"bold":                "\x1b[1mCOMMANDS\x1b[m",
	"muted":               "\x1b[38;2;95;99;106mregion\x1b[m",
	"accent":              "\x1b[38;2;161;42;7m$\x1b[m",
	"divider":             "\x1b[38;2;225;221;216m────\x1b[m",
	"success":             "\x1b[38;2;5;137;62mhealthy\x1b[m",
	"warning":             "\x1b[38;2;171;116;0m!\x1b[m",
	"danger":              "\x1b[38;2;204;40;39m✗\x1b[m",
	"info":                "\x1b[38;2;95;99;106mi\x1b[m",
	"status active":       "\x1b[38;2;5;137;62m● active\x1b[m",
	"status provisioning": "\x1b[38;2;171;116;0m◐ provisioning\x1b[m",
	"status off":          "\x1b[38;2;95;99;106m○ off\x1b[m",
	"status failed":       "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"status deleted":      "\x1b[38;2;95;99;106m· deleted\x1b[m",
	"key value":           "  \x1b[38;2;95;99;106m        Name\x1b[m  web-1",
	"key value bare":      "\x1b[38;2;95;99;106m      Region\x1b[m  hel1",
	"key value status":    "\x1b[38;2;95;99;106m      Status\x1b[m  \x1b[38;2;5;137;62m● active\x1b[m",
}

var goldenDim = map[string]string{
	"title":               "\x1b[1mServers\x1b[m",
	"bold":                "\x1b[1mCOMMANDS\x1b[m",
	"muted":               "\x1b[38;2;90;80;75mregion\x1b[m",
	"accent":              "\x1b[38;2;153;15;5m$\x1b[m",
	"divider":             "\x1b[38;2;212;201;185m────\x1b[m",
	"success":             "\x1b[38;2;5;137;62mhealthy\x1b[m",
	"warning":             "\x1b[38;2;171;116;0m!\x1b[m",
	"danger":              "\x1b[38;2;204;40;39m✗\x1b[m",
	"info":                "\x1b[38;2;90;80;75mi\x1b[m",
	"status active":       "\x1b[38;2;5;137;62m● active\x1b[m",
	"status provisioning": "\x1b[38;2;171;116;0m◐ provisioning\x1b[m",
	"status off":          "\x1b[38;2;90;80;75m○ off\x1b[m",
	"status failed":       "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"status deleted":      "\x1b[38;2;90;80;75m· deleted\x1b[m",
	"key value":           "  \x1b[38;2;90;80;75m        Name\x1b[m  web-1",
	"key value bare":      "\x1b[38;2;90;80;75m      Region\x1b[m  hel1",
	"key value status":    "\x1b[38;2;90;80;75m      Status\x1b[m  \x1b[38;2;5;137;62m● active\x1b[m",
}

var goldenDark = map[string]string{
	"title":               "\x1b[1mServers\x1b[m",
	"bold":                "\x1b[1mCOMMANDS\x1b[m",
	"muted":               "\x1b[38;2;125;128;134mregion\x1b[m",
	"accent":              "\x1b[38;2;240;116;86m$\x1b[m",
	"divider":             "\x1b[38;2;54;56;60m────\x1b[m",
	"success":             "\x1b[38;2;85;201;117mhealthy\x1b[m",
	"warning":             "\x1b[38;2;234;181;50m!\x1b[m",
	"danger":              "\x1b[38;2;247;93;89m✗\x1b[m",
	"info":                "\x1b[38;2;125;128;134mi\x1b[m",
	"status active":       "\x1b[38;2;85;201;117m● active\x1b[m",
	"status provisioning": "\x1b[38;2;234;181;50m◐ provisioning\x1b[m",
	"status off":          "\x1b[38;2;125;128;134m○ off\x1b[m",
	"status failed":       "\x1b[38;2;247;93;89m✕ failed\x1b[m",
	"status deleted":      "\x1b[38;2;125;128;134m· deleted\x1b[m",
	"key value":           "  \x1b[38;2;125;128;134m        Name\x1b[m  web-1",
	"key value bare":      "\x1b[38;2;125;128;134m      Region\x1b[m  hel1",
	"key value status":    "\x1b[38;2;125;128;134m      Status\x1b[m  \x1b[38;2;85;201;117m● active\x1b[m",
}

var goldenPlain = map[string]string{
	"title":               "Servers",
	"bold":                "COMMANDS",
	"muted":               "region",
	"accent":              "$",
	"divider":             "────",
	"success":             "healthy",
	"warning":             "!",
	"danger":              "✗",
	"info":                "i",
	"status active":       "● active",
	"status provisioning": "◐ provisioning",
	"status off":          "○ off",
	"status failed":       "✕ failed",
	"status deleted":      "· deleted",
	"key value":           "          Name  web-1",
	"key value bare":      "      Region  hel1",
	"key value status":    "      Status  ● active",
}

// The bytes each style writes, per mode, with the terminal pinned to true
// color so the recording is the same everywhere. A diff here is a color
// change: read the token table in docs/DESIGN.md before accepting it.
func TestGoldenStyleBytes(t *testing.T) {
	tests := []struct {
		name  string
		mode  Mode
		plain bool
		want  map[string]string
	}{
		{"light", ModeLight, false, goldenLight},
		{"dim", ModeDim, false, goldenDim},
		{"dark", ModeDark, false, goldenDark},
		{"plain", ModeDark, true, goldenPlain},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withStyles(t, tc.mode, tc.plain)
			require.Len(t, tc.want, len(goldenSamples), "every sample needs a recorded golden")
			for _, sample := range goldenSamples {
				want, recorded := tc.want[sample.name]
				require.True(t, recorded, "no golden recorded for %q", sample.name)
				assert.Equal(t, want, sample.render(), sample.name)
			}
		})
	}
}

// ECMA-48 gives SGR a default parameter of 0, so CSI m and CSI 0 m are the
// same instruction and every golden above ends in the shorter one. The byte
// count is not the same, so the form is pinned here rather than left to drift
// across all of them.
func TestEveryStyleClosesWithTheDefaultSGRReset(t *testing.T) {
	withStyles(t, ModeDark, false)
	for _, sample := range goldenSamples {
		rendered := sample.render()
		assert.Contains(t, rendered, "\x1b[m", sample.name)
		assert.NotContains(t, rendered, "\x1b[0m", sample.name)
	}
}

// Plain output is the one mode with no escape byte at all, and the command
// list is the only surface that adds bold on top of a token style.
func TestPrintCommands_PlainWritesNoEscapes(t *testing.T) {
	withStyles(t, ModeDark, true)
	assert.NotContains(t, captureStdout(t, PrintCommands), "\x1b")
}

func TestPrintCommands_ColorModeWritesEscapes(t *testing.T) {
	withStyles(t, ModeDark, false)
	assert.Contains(t, captureStdout(t, PrintCommands), "\x1b")
}
