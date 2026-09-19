package workspace

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// API text must never become terminal control sequences or bidi overrides.
func Clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, ansi.Strip(text))
}

func Quote(value string) string { return "'" + strings.ReplaceAll(Clean(value), "'", "'\\''") + "'" }

func cleanItem(item Item) Item {
	item.Title, item.Description, item.Command = Clean(item.Title), Clean(item.Description), Clean(item.Command)
	lines := strings.Split(item.Body, "\n")
	for i := range lines {
		lines[i] = Clean(lines[i])
	}
	item.Body = strings.Join(lines, "\n")
	fields := make([]Field, len(item.Fields))
	for i, f := range item.Fields {
		fields[i] = Field{Clean(f.Label), Clean(f.Value)}
	}
	item.Fields = fields
	children := make([]Item, len(item.Children))
	for i, child := range item.Children {
		children[i] = cleanItem(child)
	}
	item.Children = children
	return item
}
