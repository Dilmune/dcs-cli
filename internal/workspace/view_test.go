package workspace

import (
	"bytes"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/ui"
)

func serverFixture() Item {
	return Item{ID: "srv-1", Title: "Fixture server", Status: "active", Description: "hetzner · hel1",
		Fields:  []Field{{"ID", "srv-1"}, {LabelStatus, "active"}, {"Provider", "hetzner"}, {"Region", "hel1"}, {"Size", "CX22 · 2 vCPU"}, {"IPv4", "203.0.113.10"}, {"Created", "2026-01-01T00:00:00Z"}},
		Command: "dcs servers info -- 'srv-1'", Body: "Inspect only. No server changes are performed here."}
}

func TestDetailPaneRightAlignsLabelsAndShowsStatusGlyph(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 24
	m.current, m.history = serverFixture(), []frameState{{item: m.root}}
	want := []string{
		"   Fixture server",
		"   ● active · hetzner · hel1",
		"   ",
		"             ID  srv-1",
		"         Status  ● active",
		"       Provider  hetzner",
		"         Region  hel1",
		"           Size  CX22 · 2 vCPU",
		"           IPv4  203.0.113.10",
		"        Created  2026-01-01T00:00:00Z",
		"   ",
		"   COMMAND REFERENCE · NOT EXECUTED",
		"   $ dcs servers info -- 'srv-1'",
		"   ",
		"   Inspect only. No server changes are performed here.",
	}
	view := m.render()
	assert.NotContains(t, view, "\x1b")
	assert.Contains(t, view, strings.Join(want, "\n"))
	for _, line := range want[3:10] {
		label := strings.TrimSpace(line[:len("   ")+12])
		assert.Equal(t, 12, ansi.StringWidth(strings.TrimPrefix(line, "   ")[:12]), "label %q sits in a 12-cell column", label)
		assert.Equal(t, "  ", line[15:17], "two spaces separate label %q from its value", label)
	}
}

func TestDetailPaneSSLAndBooleanStatusesUseTheGlyphTable(t *testing.T) {
	m := testModel()
	site := Item{Title: "fixture.example.com", Status: "active", Description: "node",
		Fields: []Field{{"Type", "node"}, {LabelStatus, "active"}, {LabelSSL, "off"}, {"Git branch", "main"}}}
	assert.Equal(t, []string{
		"        Type  node",
		"      Status  ● active",
		"         SSL  ○ off",
		"  Git branch  main",
		"",
	}, m.detailView(site, 80))
	assert.Equal(t, "● active · node", m.styles.subtitle(site))
	assert.Equal(t, "◐ deploying", m.styles.subtitle(Item{Status: "deploying"}))
	assert.Equal(t, "reference only", m.styles.subtitle(Item{Description: "reference only"}))
}

func TestDetailPaneHidesEmptyValuesAndClampsWideLabels(t *testing.T) {
	m := testModel()
	item := Item{Fields: []Field{{"IPv4", ""}, {"Default server", "srv-1"}, {"Twelve Chars", "x"}, {LabelStatus, ""}}}
	assert.Equal(t, []string{
		"Default server  srv-1",
		"Twelve Chars  x",
		"",
	}, m.detailView(item, 80))
	assert.Empty(t, m.detailView(Item{Fields: []Field{{"IPv4", ""}}}, 80), "all-hidden rows leave no stray blank line")
}

func TestListViewRightPaneUsesTheKeyValueBlock(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 30
	m.current, m.history = Item{ID: "live", Title: "Your servers", Children: []Item{serverFixture()}}, []frameState{{item: m.root}}
	view := m.render()
	assert.Contains(t, view, "│ ● active · hetzner · hel1")
	assert.Contains(t, view, "│       Status  ● active")
	assert.Contains(t, view, "│           ID  srv-1")
	assert.NotContains(t, view, "Status  active", "the bare word never appears without its glyph")

	m.width = 60
	assert.Contains(t, m.render(), "\n   ● active · hetzner · hel1", "the stacked layout keeps the glyph on the subtitle")
}

// Bubble Tea v2 takes the alternate screen from the view, so losing the field
// would silently move the workspace into the user's scrollback.
func TestViewDeclaresTheAlternateScreenAndHidesTheCursor(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 24
	view := m.View()
	assert.True(t, view.AltScreen)
	assert.Equal(t, m.render(), view.Content)
	assert.Nil(t, view.Cursor, "the search caret is drawn in the content; the terminal cursor stays hidden")
	assert.Equal(t, tea.MouseModeNone, view.MouseMode, "the workspace is keyboard only")
}

func TestSearchMatchesTheStatusWord(t *testing.T) {
	m := testModel()
	m.current, m.history = Item{ID: "live", Title: "Your servers", Children: []Item{serverFixture()}}, []frameState{{item: m.root}}
	m, _ = typeText(m, "/hetzner")
	require.Len(t, m.results(), 1)
	m, _ = press(m, ctrl('u'))
	m, _ = typeText(m, "active")
	require.Len(t, m.results(), 1, "the status word left the description and must still be searchable")
}

// seqFor is the escape a painter emits for a token, taken from the painter
// itself so the profile it holds decides the form.
func seqFor(p ui.Painter, c color.Color) string {
	rendered := p.Token(c).Render("x")
	return strings.TrimSuffix(strings.SplitN(rendered, "x", 2)[0], "m")
}

// The workspace renders in the mode it is given; a status must take that
// mode's token, not the process-wide ui styles that default to dark.
func TestStatusAndLabelsUseTheWorkspaceRendererTokens(t *testing.T) {
	for _, mode := range []ui.Mode{ui.ModeLight, ui.ModeDim, ui.ModeDark} {
		t.Run(string(mode), func(t *testing.T) {
			p := ui.NewPainter(colorprofile.TrueColor)
			s := workspaceStyles(p, mode)
			tokens := ui.Palette(mode)

			line := s.keyValueLines(Field{LabelStatus, "active"}, 80)[0]
			require.Contains(t, seqFor(p, tokens.Muted), "38;2;", "the painter must be in true color for this test to mean anything")
			assert.Contains(t, line, seqFor(p, tokens.Muted), "label uses the muted token")
			assert.Contains(t, line, seqFor(p, tokens.Success), "healthy status uses the success token")
			assert.Contains(t, s.status("provisioning"), seqFor(p, tokens.Warning))
			assert.Contains(t, s.status("failed"), seqFor(p, tokens.Danger))
			assert.Contains(t, s.status("off"), seqFor(p, tokens.Muted))
			if mode != ui.ModeDark {
				assert.NotContains(t, line, seqFor(p, ui.Palette(ui.ModeDark).Success), "the process-wide dark palette must not leak into a %s workspace", mode)
			}

			plain := s.keyValueLines(Field{"IPv4", "203.0.113.10"}, 80)[0]
			assert.True(t, strings.HasSuffix(plain, "  203.0.113.10"), "value follows two plain spaces with no escape: %q", plain)
			assert.Equal(t, "        IPv4  203.0.113.10", ansi.Strip(plain))
		})
	}
}

// The workspace draws in the mode ui.Init resolved, so building its styles
// must write nothing to the output. A background query would write.
func TestWorkspaceStylesNeverQueryTheTerminal(t *testing.T) {
	for _, mode := range []ui.Mode{ui.ModeLight, ui.ModeDim, ui.ModeDark} {
		for _, plain := range []bool{false, true} {
			var out bytes.Buffer
			newStyles(&out, mode, plain)
			assert.Empty(t, out.String(), "mode %s, plain %t", mode, plain)
		}
	}
}

func TestKeyValueContinuationLinesHangUnderTheValueColumn(t *testing.T) {
	m := testModel()
	const width = 60
	value := strings.Repeat("abcdefghi ", 12)[:120]
	item := Item{Fields: []Field{{"Size", value}, {"IPv4", "203.0.113.10"}}}
	lines := m.detailView(item, width)
	require.Greater(t, len(lines), 3, "a 120-cell value at width 60 must wrap")
	assert.True(t, strings.HasPrefix(lines[0], "        Size  abcdefghi"), "the first line keeps the label: %q", lines[0])
	var continuation []string
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "        IPv4") || line == "" {
			break
		}
		continuation = append(continuation, line)
	}
	require.NotEmpty(t, continuation)
	for _, line := range continuation {
		assert.True(t, strings.HasPrefix(line, strings.Repeat(" ", 14)), "continuation starts with 14 spaces: %q", line)
		assert.NotEqual(t, ' ', line[14], "exactly 14 spaces, then the value: %q", line)
	}
	for _, line := range lines {
		assert.LessOrEqual(t, ansi.StringWidth(line), width, "line exceeds the width: %q", line)
	}
	joined := strings.Join(append([]string{strings.TrimSpace(lines[0][14:])}, func() []string {
		var parts []string
		for _, line := range continuation {
			parts = append(parts, strings.TrimSpace(line))
		}
		return parts
	}()...), " ")
	assert.Equal(t, strings.TrimSpace(value), joined, "wrapping drops no text")
	assert.Equal(t, "        IPv4  203.0.113.10", lines[len(continuation)+1], "the next field follows without a hanging indent")

	m.width, m.height = 100, 30
	m.current, m.history = Item{ID: "live", Title: "Long", Children: []Item{{ID: "x", Title: "Wide", Fields: item.Fields}}}, []frameState{{item: m.root}}
	for _, line := range strings.Split(m.render(), "\n") {
		if strings.Contains(line, "│ "+strings.Repeat(" ", 14)) {
			return
		}
	}
	t.Fatal("the split right pane must hang continuation lines under the value column")
}

func TestListViewSitePickerShowsPurposeNotServerReference(t *testing.T) {
	m := testModel()
	m.width, m.height = 100, 30
	picker := Item{ID: "live-sites", Title: "Choose a server", Description: "Pick the server whose sites you want to browse.", Children: []Item{{ID: "srv-1", Title: "Fixture server", Status: "active", Description: "",
		Fields: []Field{{"Provider", "hetzner"}, {"Region", "hel1"}, {"IPv4", "203.0.113.10"}}, Request: &Request{Kind: Sites, ServerID: "srv-1"}}}}
	m.current, m.history = picker, []frameState{{item: m.root}}
	view := m.render()
	assert.Contains(t, view, "   Pick the server whose sites you want to browse.")
	assert.Contains(t, view, "│ ● active")
	assert.Contains(t, view, "│     Provider  hetzner")
	assert.NotContains(t, view, "$ dcs servers info")
	assert.NotContains(t, view, commandReferenceLabel)
	assert.NotContains(t, view, "Status  ●", "the picker carries no server inspection fields")
}

// goldenStyles pins the terminal's color support for one recording, so a
// golden is the same bytes on any machine.
func goldenStyles(mode ui.Mode, plain bool) styles {
	profile := colorprofile.TrueColor
	if plain {
		profile = colorprofile.NoTTY
	}
	return workspaceStyles(ui.NewPainter(profile), mode)
}
