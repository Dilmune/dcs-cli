package ui

import (
	"fmt"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"
)

// bubbles v2 takes lipgloss v2 styles, so the accent reaches the spinner
// through the same painter as every other surface instead of being converted
// for a second renderer. Plain output therefore loses the escape sequences
// here for the same reason it loses them everywhere else.
func TestSpinnerDrawsTheAccentTokenOfTheResolvedMode(t *testing.T) {
	for _, mode := range []Mode{ModeLight, ModeDim, ModeDark} {
		for _, plainOutput := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/plain=%t", mode, plainOutput), func(t *testing.T) {
				withStyles(t, mode, plainOutput)
				m := newSpinnerModel("Creating server")
				assert.Equal(t, Accent.Render("⣾ "), m.spinner.View())
				assert.Equal(t, "  "+Accent.Render("⣾ ")+" "+Muted.Render("Creating server"), m.View().Content)
				if plainOutput {
					assert.NotContains(t, m.View().Content, "\x1b")
				}
			})
		}
	}
}

// docs/DESIGN.md "Motion and timing": spinner frame interval 80ms.
func TestSpinnerKeepsTheDesignFrameInterval(t *testing.T) {
	m := newSpinnerModel("Creating server")
	assert.Equal(t, 80*time.Millisecond, m.spinner.Spinner.FPS)
	assert.Equal(t, spinner.Dot.Frames, m.spinner.Spinner.Frames)
}

func TestSpinnerDrawsNothingOnceItIsDone(t *testing.T) {
	m := newSpinnerModel("Creating server")
	m.done = true
	assert.Empty(t, m.View().Content)
}
