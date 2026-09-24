package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/ui"
)

// One call per style the workspace draws with, so a golden covers the bytes a
// terminal receives rather than the style object behind them.
var goldenSamples = []struct {
	name   string
	render func(styles) string
}{
	{"title", func(s styles) string { return s.title.Render("Your servers") }},
	{"muted", func(s styles) string { return s.muted.Render("Loading...") }},
	{"accent", func(s styles) string { return s.accent.Render("›") }},
	{"pane separator", func(s styles) string { return s.rule.Render("│") }},
	{"footer rule", func(s styles) string { return s.rule.Render("────") }},
	{"success", func(s styles) string { return s.success.Render("healthy") }},
	{"warning", func(s styles) string { return s.warning.Render("!") }},
	{"danger", func(s styles) string { return s.danger.Render("✗") }},
	{"status active", func(s styles) string { return s.status("active") }},
	{"status provisioning", func(s styles) string { return s.status("provisioning") }},
	{"status off", func(s styles) string { return s.status("off") }},
	{"status failed", func(s styles) string { return s.status("failed") }},
	{"subtitle status and description", func(s styles) string {
		return s.subtitle(Item{Status: "active", Description: "hetzner · hel1"})
	}},
	{"subtitle status only", func(s styles) string { return s.subtitle(Item{Status: "failed"}) }},
	{"subtitle description only", func(s styles) string { return s.subtitle(Item{Description: "hetzner · hel1"}) }},
	{"key value", func(s styles) string { return s.keyValueLines(Field{"IPv4", "203.0.113.10"}, 80)[0] }},
	{"key value status", func(s styles) string { return s.keyValueLines(Field{LabelStatus, "active"}, 80)[0] }},
	{"logo face", func(s styles) string { return s.logoFace.Render("▀") }},
	{"logo side", func(s styles) string { return s.logoSide.Render("▀") }},
	{"logo accent", func(s styles) string { return s.logoAccent.Render("▀") }},
}

var goldenLight = map[string]string{
	"title":                           "\x1b[1mYour servers\x1b[m",
	"muted":                           "\x1b[38;2;95;99;106mLoading...\x1b[m",
	"accent":                          "\x1b[38;2;161;42;7m›\x1b[m",
	"pane separator":                  "\x1b[38;2;225;221;216m│\x1b[m",
	"footer rule":                     "\x1b[38;2;225;221;216m────\x1b[m",
	"success":                         "\x1b[38;2;5;137;62mhealthy\x1b[m",
	"warning":                         "\x1b[38;2;171;116;0m!\x1b[m",
	"danger":                          "\x1b[38;2;204;40;39m✗\x1b[m",
	"status active":                   "\x1b[38;2;5;137;62m● active\x1b[m",
	"status provisioning":             "\x1b[38;2;171;116;0m◐ provisioning\x1b[m",
	"status off":                      "\x1b[38;2;95;99;106m○ off\x1b[m",
	"status failed":                   "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"subtitle status and description": "\x1b[38;2;5;137;62m● active\x1b[m\x1b[38;2;95;99;106m · hetzner · hel1\x1b[m",
	"subtitle status only":            "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"subtitle description only":       "\x1b[38;2;95;99;106mhetzner · hel1\x1b[m",
	"key value":                       "\x1b[38;2;95;99;106m        IPv4\x1b[m  203.0.113.10",
	"key value status":                "\x1b[38;2;95;99;106m      Status\x1b[m  \x1b[38;2;5;137;62m● active\x1b[m",
	"logo face":                       "\x1b[38;2;255;255;215m▀\x1b[m",
	"logo side":                       "\x1b[38;2;255;215;175m▀\x1b[m",
	"logo accent":                     "\x1b[38;2;215;135;95m▀\x1b[m",
}

var goldenDim = map[string]string{
	"title":                           "\x1b[1mYour servers\x1b[m",
	"muted":                           "\x1b[38;2;90;80;75mLoading...\x1b[m",
	"accent":                          "\x1b[38;2;153;15;5m›\x1b[m",
	"pane separator":                  "\x1b[38;2;212;201;185m│\x1b[m",
	"footer rule":                     "\x1b[38;2;212;201;185m────\x1b[m",
	"success":                         "\x1b[38;2;5;137;62mhealthy\x1b[m",
	"warning":                         "\x1b[38;2;171;116;0m!\x1b[m",
	"danger":                          "\x1b[38;2;204;40;39m✗\x1b[m",
	"status active":                   "\x1b[38;2;5;137;62m● active\x1b[m",
	"status provisioning":             "\x1b[38;2;171;116;0m◐ provisioning\x1b[m",
	"status off":                      "\x1b[38;2;90;80;75m○ off\x1b[m",
	"status failed":                   "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"subtitle status and description": "\x1b[38;2;5;137;62m● active\x1b[m\x1b[38;2;90;80;75m · hetzner · hel1\x1b[m",
	"subtitle status only":            "\x1b[38;2;204;40;39m✕ failed\x1b[m",
	"subtitle description only":       "\x1b[38;2;90;80;75mhetzner · hel1\x1b[m",
	"key value":                       "\x1b[38;2;90;80;75m        IPv4\x1b[m  203.0.113.10",
	"key value status":                "\x1b[38;2;90;80;75m      Status\x1b[m  \x1b[38;2;5;137;62m● active\x1b[m",
	"logo face":                       "\x1b[38;2;255;255;215m▀\x1b[m",
	"logo side":                       "\x1b[38;2;255;215;175m▀\x1b[m",
	"logo accent":                     "\x1b[38;2;215;135;95m▀\x1b[m",
}

var goldenDark = map[string]string{
	"title":                           "\x1b[1mYour servers\x1b[m",
	"muted":                           "\x1b[38;2;125;128;134mLoading...\x1b[m",
	"accent":                          "\x1b[38;2;240;116;86m›\x1b[m",
	"pane separator":                  "\x1b[38;2;54;56;60m│\x1b[m",
	"footer rule":                     "\x1b[38;2;54;56;60m────\x1b[m",
	"success":                         "\x1b[38;2;85;201;117mhealthy\x1b[m",
	"warning":                         "\x1b[38;2;234;181;50m!\x1b[m",
	"danger":                          "\x1b[38;2;247;93;89m✗\x1b[m",
	"status active":                   "\x1b[38;2;85;201;117m● active\x1b[m",
	"status provisioning":             "\x1b[38;2;234;181;50m◐ provisioning\x1b[m",
	"status off":                      "\x1b[38;2;125;128;134m○ off\x1b[m",
	"status failed":                   "\x1b[38;2;247;93;89m✕ failed\x1b[m",
	"subtitle status and description": "\x1b[38;2;85;201;117m● active\x1b[m\x1b[38;2;125;128;134m · hetzner · hel1\x1b[m",
	"subtitle status only":            "\x1b[38;2;247;93;89m✕ failed\x1b[m",
	"subtitle description only":       "\x1b[38;2;125;128;134mhetzner · hel1\x1b[m",
	"key value":                       "\x1b[38;2;125;128;134m        IPv4\x1b[m  203.0.113.10",
	"key value status":                "\x1b[38;2;125;128;134m      Status\x1b[m  \x1b[38;2;85;201;117m● active\x1b[m",
	"logo face":                       "\x1b[38;2;255;255;215m▀\x1b[m",
	"logo side":                       "\x1b[38;2;255;215;175m▀\x1b[m",
	"logo accent":                     "\x1b[38;2;215;135;95m▀\x1b[m",
}

var goldenPlain = map[string]string{
	"title":                           "Your servers",
	"muted":                           "Loading...",
	"accent":                          "›",
	"pane separator":                  "│",
	"footer rule":                     "────",
	"success":                         "healthy",
	"warning":                         "!",
	"danger":                          "✗",
	"status active":                   "● active",
	"status provisioning":             "◐ provisioning",
	"status off":                      "○ off",
	"status failed":                   "✕ failed",
	"subtitle status and description": "● active · hetzner · hel1",
	"subtitle status only":            "✕ failed",
	"subtitle description only":       "hetzner · hel1",
	"key value":                       "        IPv4  203.0.113.10",
	"key value status":                "      Status  ● active",
	"logo face":                       "▀",
	"logo side":                       "▀",
	"logo accent":                     "▀",
}

// The bytes each workspace style writes, per mode, with the terminal pinned to
// true color so the recording is the same everywhere. A diff here is a color
// change: read the token table in docs/DESIGN.md before accepting it.
func TestGoldenWorkspaceStyleBytes(t *testing.T) {
	tests := []struct {
		name  string
		mode  ui.Mode
		plain bool
		want  map[string]string
	}{
		{"light", ui.ModeLight, false, goldenLight},
		{"dim", ui.ModeDim, false, goldenDim},
		{"dark", ui.ModeDark, false, goldenDark},
		{"plain", ui.ModeDark, true, goldenPlain},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := goldenStyles(tc.mode, tc.plain)
			assert.Equal(t, tc.plain, s.asciiLogo, "the plain logo follows the same switch as the styles")
			require.Len(t, tc.want, len(goldenSamples), "every sample needs a recorded golden")
			for _, sample := range goldenSamples {
				want, recorded := tc.want[sample.name]
				require.True(t, recorded, "no golden recorded for %q", sample.name)
				assert.Equal(t, want, sample.render(s), sample.name)
			}
		})
	}
}
