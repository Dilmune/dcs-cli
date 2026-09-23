package workspace

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dilmune/dcs-cli/internal/ui"
)

const (
	AreaServers   = "servers"
	AreaSites     = "sites"
	AreaDatabases = "databases"

	plainMargin        = "  "
	stackedDescription = "     "
	inlineMinimumWidth = 64
	areaTitleWidth     = 21
	brandSuffix        = "  /  dcs"
)

// The overview counts live resources for the areas that have them; the
// rest keep their catalog description.
var areaNouns = map[string]string{AreaServers: "server", AreaSites: "site", AreaDatabases: "database"}

type OverviewOptions struct {
	Width   int
	Version string
	Account string
	Areas   []Item
	Counts  map[string]int
	Mode    ui.Mode
	NoColor bool
}

// RenderOverview prints the dcs ui home screen as static text: no cube, no
// footer, no selection marker, so the layout survives pipes and pagers.
func RenderOverview(w io.Writer, opts OverviewOptions) error {
	s := newStyles(w, opts.Mode, opts.NoColor)
	width := max(1, opts.Width-2*len(plainMargin))
	brand := s.title.Render("Dilmune Cloud") + s.muted.Render(brandSuffix)
	lines := []string{"", pair(brand, s.muted.Render("v"+Clean(opts.Version)), width)}
	if account := Clean(opts.Account); account != "" {
		lines = append(lines, s.muted.Render(ansi.Truncate(account, width, "…")))
	}
	lines = append(lines, "")
	lines = append(lines, overviewAreas(s, opts, width)...)
	var b strings.Builder
	for _, line := range lines {
		if line != "" {
			b.WriteString(plainMargin + ansi.Truncate(line, width, "…"))
		}
		b.WriteByte('\n')
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write overview: %w", err)
	}
	return nil
}

func overviewAreas(s styles, opts OverviewOptions, width int) []string {
	inline := width >= inlineMinimumWidth
	var lines []string
	for i, area := range opts.Areas {
		area = cleanItem(area)
		line := "  " + s.muted.Render(fmt.Sprintf("%d", i+1)) + "  " + area.Title
		detail := s.muted.Render(areaDetail(area, opts.Counts))
		if inline {
			lines = append(lines, pad(line, areaTitleWidth)+"  "+detail, "")
			continue
		}
		lines = append(lines, line, stackedDescription+detail, "")
	}
	return lines
}

func areaDetail(area Item, counts map[string]int) string {
	count, counted := counts[area.ID]
	noun, known := areaNouns[area.ID]
	if !counted || !known {
		return area.Description
	}
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
