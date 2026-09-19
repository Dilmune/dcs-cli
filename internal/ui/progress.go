package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

const progressBarWidth = 30

// ProgressWriter wraps an io.Writer and tracks bytes written for progress display.
type ProgressWriter struct {
	total   int64
	written int64
	mu      sync.Mutex
	label   string
}

// NewProgressWriter creates a writer that displays upload/download progress.
func NewProgressWriter(total int64, label string) *ProgressWriter {
	return &ProgressWriter{total: total, label: label}
}

// WrapReader returns a reader that updates progress as data is read.
func (pw *ProgressWriter) WrapReader(r io.Reader) io.Reader {
	return &progressReader{r: r, pw: pw}
}

// Finish prints the final completed state.
func (pw *ProgressWriter) Finish() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	bar := lipgloss.NewStyle().Foreground(SuccessGreen).Render(strings.Repeat("█", progressBarWidth))
	fmt.Printf("\r  %s [%s] 100%%\n", pw.label, bar)
}

func (pw *ProgressWriter) render() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	var pct float64
	if pw.total > 0 {
		pct = float64(pw.written) / float64(pw.total)
	}
	filled := int(pct * float64(progressBarWidth))
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	empty := progressBarWidth - filled

	barFilled := lipgloss.NewStyle().Foreground(BrandPrimary).Render(strings.Repeat("█", filled))
	barEmpty := lipgloss.NewStyle().Foreground(TextDim).Render(strings.Repeat("░", empty))
	pctStr := fmt.Sprintf("%3.0f%%", pct*100)

	fmt.Printf("\r  %s [%s%s] %s", pw.label, barFilled, barEmpty, pctStr)
}

type progressReader struct {
	r  io.Reader
	pw *ProgressWriter
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.pw.mu.Lock()
		pr.pw.written += int64(n)
		pr.pw.mu.Unlock()
		pr.pw.render()
	}
	return n, err
}
