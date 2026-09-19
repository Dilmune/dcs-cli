package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderGradientLine_SingleRuneDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		out := renderGradientLine("█", false)
		assert.NotEmpty(t, out)
	})
}

func TestRenderGradientLine_EmptyLine(t *testing.T) {
	assert.Equal(t, "", renderGradientLine("", false))
}

func TestRenderGradientLine_NoColorReturnsPlainText(t *testing.T) {
	line := "██████╗  ██████╗███████╗"
	assert.Equal(t, line, renderGradientLine(line, true))
}

func TestIsNoColor(t *testing.T) {
	original, wasSet := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		if wasSet {
			os.Setenv("NO_COLOR", original)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})

	os.Unsetenv("NO_COLOR")
	assert.False(t, isNoColor())

	os.Setenv("NO_COLOR", "1")
	assert.True(t, isNoColor())
}

func TestStyled_NoColorSkipsRendering(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(BrandPrimary).Bold(true)
	assert.Equal(t, "DCS", styled(style, "DCS", true))
}

func TestInterpolateGradient_Boundaries(t *testing.T) {
	stops := []lipgloss.Color{lipgloss.Color("#000000"), lipgloss.Color("#ffffff")}

	assert.Equal(t, stops[0], interpolateGradient(stops, 0))
	assert.Equal(t, stops[0], interpolateGradient(stops, -1))
	assert.Equal(t, stops[len(stops)-1], interpolateGradient(stops, 1))
	assert.Equal(t, stops[len(stops)-1], interpolateGradient(stops, 2))
	assert.Equal(t, stops[0], interpolateGradient([]lipgloss.Color{stops[0]}, 0.5))
}
