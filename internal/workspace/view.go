package workspace

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const commandReferenceLabel = "COMMAND REFERENCE · NOT EXECUTED"

// Bubble Tea v2 has no alt-screen program option; it is a view field, so every
// frame declares it.
func (m model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m model) render() string {
	w, h := max(1, m.width), max(1, m.height)
	if m.isTooSmall() {
		return canvas([]string{"Dilmune Cloud", "Resize to at least 48 x 20.", "Ctrl+C exits."}, w, h)
	}
	width := w - 6
	const footerHeight = 3
	logoCapacity := h - footerHeight - 1
	if m.notice != "" {
		logoCapacity--
	}
	layout := m.responsiveWelcome(width, logoCapacity)
	lines := m.headerView(width)
	if layout.hasLogo() && !layout.headerAbove {
		lines = []string{""}
	}
	capacity := h - len(lines) - footerHeight
	if m.notice != "" {
		capacity--
	}
	var body []string
	switch {
	case layout.hasLogo():
		body = m.welcomeView(width, capacity, layout)
	case m.help:
		body = m.helpView()
	case m.searching:
		body = m.listView(width, capacity)
	case m.isHome() && !m.loading && !m.hasError():
		body = m.homeView(width, capacity, true)
	default:
		body = []string{m.styles.title.Render(m.current.Title)}
		if subtitle := m.styles.subtitle(m.current); subtitle != "" {
			body = append(body, wrap(subtitle, width)...)
		}
		body = append(body, "")
		switch {
		case m.loading:
			body = append(body, m.styles.muted.Render("Loading..."))
		case m.hasError():
			body = append(body, "Could not load this view.")
			body = append(body, wrap(Clean(m.err.Error()), width)...)
			body = append(body, "", "Press r to retry, or Esc to go back.")
		case len(m.current.Children) > 0:
			body = append(body, m.listView(width, capacity-len(body))...)
		default:
			body = append(body, m.detailView(m.current, width)...)
		}
	}
	body = viewport(body, capacity, m.scroll)
	lines = append(lines, body...)
	for len(lines) < h-footerHeight {
		lines = append(lines, "")
	}
	if m.notice != "" {
		lines[h-footerHeight-1] = m.styles.muted.Render(m.notice)
	}
	lines = append(lines, m.styles.rule.Render(strings.Repeat("─", width)), m.styles.muted.Render(m.footer()), "")
	for i := range lines {
		lines[i] = "   " + ansi.Truncate(lines[i], width, "…")
	}
	return canvas(lines, w, h)
}

func (m model) headerView(width int) []string {
	brand := m.styles.title.Render("Dilmune Cloud") + m.styles.muted.Render("  /  dcs ui")
	accountWidth := width - len("Read-only") - 3
	return []string{"", pair(brand, m.styles.muted.Render("v"+m.version), width),
		pair(m.styles.muted.Render(ansi.Truncate(m.account, accountWidth, "…")), m.styles.muted.Render("Read-only"), width), ""}
}

func (m model) homeView(width, capacity int, showDescription bool) []string {
	results := m.results()
	if len(results) == 0 {
		return []string{"No areas available. Press / to find a command."}
	}
	const titleWidth = 21
	inline := width >= 64
	step := 1
	descriptionHeight := 2
	if !showDescription {
		descriptionHeight = 0
	}
	if capacity >= len(results)*2+descriptionHeight {
		step = 2
	}
	var lines []string
	for i, result := range results {
		prefix := "  " + m.styles.muted.Render(fmt.Sprintf("%d", i+1)) + "  "
		label := result.item.Title
		if i == m.cursor {
			prefix = m.styles.accent.Render("› ") + m.styles.muted.Render(fmt.Sprintf("%d", i+1)) + "  "
			label = m.styles.selected.Render(label)
		}
		line := prefix + label
		if inline {
			line = pad(line, titleWidth) + "  " + m.styles.muted.Render(result.item.Description)
		}
		lines = append(lines, line)
		if step == 2 {
			lines = append(lines, "")
		}
	}
	if showDescription && !inline && capacity-len(lines) >= 2 {
		selected := results[min(m.cursor, len(results)-1)].item
		lines = append(lines, "", "     "+m.styles.muted.Render(ansi.Truncate(selected.Description, width-5, "…")))
	}
	return lines
}

func (m model) listView(width, capacity int) []string {
	var lines []string
	if m.searching {
		lines = []string{m.styles.title.Render("Find a command or resource"), "", m.styles.accent.Render("/ ") + Clean(string(m.query)) + "▏", ""}
		capacity -= 4
	}
	results := m.results()
	if len(results) == 0 {
		return append(lines, "No matches. Press Ctrl+U to clear the search.")
	}
	m.cursor = min(m.cursor, len(results)-1)
	leftWidth := min(34, width/2-2)
	split := width >= 82 && !m.searching
	if !split {
		leftWidth = width
	}
	step := 1
	if capacity >= 16 {
		step = 2
	}
	limit := max(1, (capacity-2)/step)
	start := max(0, m.cursor-limit+1)
	end := min(len(results), start+limit)
	var left []string
	for i := start; i < end; i++ {
		label := results[i].item.Title
		if m.isHome() && !m.searching {
			label = fmt.Sprintf("%d  %s", i+1, label)
		}
		marker := "  "
		label = ansi.Truncate(label, leftWidth-2, "…")
		if i == m.cursor {
			marker = m.styles.accent.Render("› ")
			label = m.styles.selected.Render(label)
		}
		left = append(left, marker+label)
		if step == 2 {
			left = append(left, "")
		}
	}
	if end < len(results) {
		left = append(left, m.styles.muted.Render("  ↓ more"))
	}
	selected := results[m.cursor].item
	if !split {
		lines = append(lines, left...)
		if !m.searching {
			lines = append(lines, "", m.styles.subtitle(selected))
		}
		return lines
	}
	rightWidth := width - leftWidth - 4
	right := []string{m.styles.title.Render(selected.Title)}
	right = append(right, wrap(m.styles.subtitle(selected), rightWidth)...)
	right = append(right, "", m.styles.muted.Render("Enter to explore"))
	fields := len(right)
	for _, f := range selected.Fields {
		right = append(right, m.styles.keyValueLines(f, rightWidth)...)
	}
	if len(right) > fields {
		right = append(right[:fields], append([]string{""}, right[fields:]...)...)
	}
	if selected.Command != "" {
		right = append(right, "", m.styles.muted.Render(commandReferenceLabel))
		right = append(right, wrap("$ "+selected.Command, rightWidth)...)
	}
	for i := 0; i < max(len(left), len(right)); i++ {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, pad(l, leftWidth)+"  "+m.styles.rule.Render("│")+" "+r)
	}
	return lines
}

func (m model) detailView(item Item, width int) []string {
	var lines []string
	for _, f := range item.Fields {
		lines = append(lines, m.styles.keyValueLines(f, width)...)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	if item.Command != "" {
		lines = append(lines, m.styles.muted.Render(commandReferenceLabel))
		lines = append(lines, wrap("$ "+item.Command, width)...)
		lines = append(lines, "")
	}
	if item.Body != "" {
		lines = append(lines, wrap(item.Body, width)...)
	}
	return lines
}

func (m model) helpView() []string {
	return []string{m.styles.title.Render("Keyboard shortcuts"), "", "↑↓ / j k      Choose an item", "Enter         Open the selected view", "Esc           Back; cancel a pending read", "0             Overview", "1 through 6   Jump to an area", "/             Search commands and current resources", "r             Refresh the current live view", "PgUp / PgDn   Scroll long details", "q / Ctrl+C    Exit", "", "Only account, server, and site views read live data.", "Command references never execute. Use the normal CLI", "for deployments, SSH, storage, and other changes.", "", "dcs ui --welcome    Show the cube on the overview"}
}

func (m model) footer() string {
	if m.help {
		return "esc back   PgUp/PgDn scroll   q quit"
	}
	if m.searching {
		return "↑↓ move  ↵ open  esc back  ctrl+u clear"
	}
	if m.loading {
		return "esc cancel request   0 home   q quit"
	}
	if m.width < 70 {
		if !m.isHome() {
			return "↑↓ move  ↵ open  esc back  ? help  q quit"
		}
		return "↑↓ move  ↵ open  / find  ? help  q quit"
	}
	if m.isHome() {
		return "↑↓ move   enter open   / find   ? help   q quit"
	}
	if m.canRefresh() {
		return "↑↓ move   enter open   esc back   r refresh   ? help   q quit"
	}
	return "↑↓ move   enter open   esc back   / find   ? help   q quit"
}

func wrap(text string, width int) []string {
	return strings.Split(ansi.Wrap(text, max(1, width), ""), "\n")
}
func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(0, width-lipgloss.Width(text)))
}
func pair(left, right string, width int) string {
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-lipgloss.Width(right))) + right
}
func viewport(lines []string, height, offset int) []string {
	height = max(1, height)
	offset = min(offset, max(0, len(lines)-height))
	end := min(len(lines), offset+height)
	visible := append([]string(nil), lines[offset:end]...)
	if end < len(lines) && len(visible) > 0 {
		visible[len(visible)-1] = "↓ PgDn for more"
	}
	return visible
}
func canvas(lines []string, width, height int) string {
	result := make([]string, height)
	for i := range result {
		if i < len(lines) {
			result[i] = ansi.Truncate(lines[i], width, "…")
		}
	}
	return strings.Join(result, "\n")
}
