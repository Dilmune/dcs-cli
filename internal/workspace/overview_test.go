package workspace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func overviewAreasFixture() []Item {
	return []Item{
		{ID: AreaServers, Title: "Servers", Description: "Live views and SSH commands"},
		{ID: AreaSites, Title: "Sites & deploys", Description: "Live views and deployment commands"},
		{ID: AreaDatabases, Title: "Databases", Description: "Reference: schemas, queries, backups"},
		{ID: "storage", Title: "Storage", Description: "Reference: buckets and files"},
		{ID: "access", Title: "Access", Description: "Reference: keys and account"},
		{ID: "operations", Title: "Operations", Description: "Reference: logs, environment, processes"},
	}
}

func renderOverview(t *testing.T, opts OverviewOptions) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, RenderOverview(&out, opts))
	return out.String()
}

func TestRenderOverviewInlineAt100(t *testing.T) {
	out := renderOverview(t, OverviewOptions{Width: 100, Version: "3.8.1", Account: "Test User", Areas: overviewAreasFixture(),
		Counts: map[string]int{AreaServers: 3, AreaSites: 1, AreaDatabases: 0}, Theme: "light", NoColor: true})
	want := strings.Join([]string{
		"",
		"  Dilmune Cloud  /  dcs" + strings.Repeat(" ", 69) + "v3.8.1",
		"  Test User",
		"",
		"    1  Servers           3 servers",
		"",
		"    2  Sites & deploys   1 site",
		"",
		"    3  Databases         0 databases",
		"",
		"    4  Storage           Reference: buckets and files",
		"",
		"    5  Access            Reference: keys and account",
		"",
		"    6  Operations        Reference: logs, environment, processes",
		"",
		"",
	}, "\n")
	assert.Equal(t, want, out)
	assert.NotContains(t, out, "\x1b")
}

func TestRenderOverviewStackedAt60(t *testing.T) {
	out := renderOverview(t, OverviewOptions{Width: 60, Version: "3.8.1", Account: "", Areas: overviewAreasFixture(),
		Counts: map[string]int{AreaServers: 1, AreaSites: 12, AreaDatabases: 2}, Theme: "dark", NoColor: true})
	want := strings.Join([]string{
		"",
		"  Dilmune Cloud  /  dcs" + strings.Repeat(" ", 29) + "v3.8.1",
		"",
		"    1  Servers",
		"       1 server",
		"",
		"    2  Sites & deploys",
		"       12 sites",
		"",
		"    3  Databases",
		"       2 databases",
		"",
		"    4  Storage",
		"       Reference: buckets and files",
		"",
		"    5  Access",
		"       Reference: keys and account",
		"",
		"    6  Operations",
		"       Reference: logs, environment, processes",
		"",
		"",
	}, "\n")
	assert.Equal(t, want, out)
	assert.NotContains(t, out, "\x1b")
}

func TestRenderOverviewFitsAndSanitizes(t *testing.T) {
	for _, width := range []int{20, 48, 63, 64, 80, 120} {
		out := renderOverview(t, OverviewOptions{Width: width, Version: "3.8.1\x1b[31m", Account: "\x1b]0;evil\x07" + strings.Repeat("account ", 30),
			Areas: overviewAreasFixture(), Counts: map[string]int{AreaServers: 3}, Theme: "dim", NoColor: true})
		assert.NotContains(t, out, "\x1b", "width %d", width)
		assert.NotContains(t, out, "\a", "width %d", width)
		for _, line := range strings.Split(out, "\n") {
			assert.LessOrEqual(t, ansi.StringWidth(line), width, "width %d", width)
		}
		assert.Contains(t, out, "3 servers")
		if width >= inlineMinimumWidth {
			assert.Contains(t, out, "Live views and deployment commands", "an area without a count keeps its description")
		}
	}
}

// A pipe is not a terminal: lipgloss must downgrade to ASCII on its own, so
// scripts get the same text whether or not --no-color was passed.
func TestRenderOverviewOnPipeHasNoEscapes(t *testing.T) {
	out := renderOverview(t, OverviewOptions{Width: 100, Version: "3.8.1", Account: "Test User", Areas: overviewAreasFixture(), Theme: ThemeAuto, NoColor: false})
	assert.NotContains(t, out, "\x1b")
	assert.Contains(t, out, "    1  Servers           Live views and SSH commands")
}
