package workspace

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
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
	view := m.View()
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
	view := m.View()
	assert.Contains(t, view, "│ ● active · hetzner · hel1")
	assert.Contains(t, view, "│       Status  ● active")
	assert.Contains(t, view, "│           ID  srv-1")
	assert.NotContains(t, view, "Status  active", "the bare word never appears without its glyph")

	m.width = 60
	assert.Contains(t, m.View(), "\n   ● active · hetzner · hel1", "the stacked layout keeps the glyph on the subtitle")
}

func TestSearchMatchesTheStatusWord(t *testing.T) {
	m := testModel()
	m.current, m.history = Item{ID: "live", Title: "Your servers", Children: []Item{serverFixture()}}, []frameState{{item: m.root}}
	m, _ = press(m, tea.KeyRunes, []rune("/hetzner")...)
	require.Len(t, m.results(), 1)
	m, _ = press(m, tea.KeyCtrlU)
	m, _ = press(m, tea.KeyRunes, []rune("active")...)
	require.Len(t, m.results(), 1, "the status word left the description and must still be searchable")
}

// seqFor is the escape the renderer emits for a token, taken from the renderer
// itself because termenv rounds hex to RGB its own way.
func seqFor(r *lipgloss.Renderer, c lipgloss.Color) string {
	rendered := r.NewStyle().Foreground(c).Render("x")
	return strings.TrimSuffix(strings.SplitN(rendered, "x", 2)[0], "m")
}

// The workspace resolves its mode once in newStyles; a status must take that
// mode's token, not the process-wide ui styles that default to dark.
func TestStatusAndLabelsUseTheWorkspaceRendererTokens(t *testing.T) {
	for _, tc := range []struct {
		theme string
		mode  ui.Mode
	}{{"light", ui.ModeLight}, {"dim", ui.ModeDim}, {"dark", ui.ModeDark}} {
		t.Run(tc.theme, func(t *testing.T) {
			r := lipgloss.NewRenderer(io.Discard)
			r.SetColorProfile(termenv.TrueColor)
			r.SetHasDarkBackground(tc.theme == "dark")
			s := workspaceStyles(r, tc.theme, tc.theme == "dark")
			tokens := ui.Palette(tc.mode)

			line := s.keyValue(Field{LabelStatus, "active"})
			require.Contains(t, seqFor(r, tokens.Muted), "38;2;", "the renderer must be in true color for this test to mean anything")
			assert.Contains(t, line, seqFor(r, tokens.Muted), "label uses the muted token")
			assert.Contains(t, line, seqFor(r, tokens.Success), "healthy status uses the success token")
			assert.Contains(t, s.status("provisioning"), seqFor(r, tokens.Warning))
			assert.Contains(t, s.status("failed"), seqFor(r, tokens.Danger))
			assert.Contains(t, s.status("off"), seqFor(r, tokens.Muted))
			if tc.mode != ui.ModeDark {
				assert.NotContains(t, line, seqFor(r, ui.Palette(ui.ModeDark).Success), "the process-wide dark palette must not leak into a %s workspace", tc.theme)
			}

			plain := s.keyValue(Field{"IPv4", "203.0.113.10"})
			assert.True(t, strings.HasSuffix(plain, "  203.0.113.10"), "value follows two plain spaces with no escape: %q", plain)
			assert.Equal(t, "        IPv4  203.0.113.10", ansi.Strip(plain))
		})
	}
}
