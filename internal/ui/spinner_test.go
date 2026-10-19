package ui

import (
	"errors"
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

func TestRunAlongsideAnswersOnlyAfterTheOperation(t *testing.T) {
	displayBroke := errors.New("terminal gone")
	requestFailed := errors.New("request failed")
	tests := []struct {
		name      string
		showErr   error
		fnErr     error
		wantIs    []error
		wantText  string
		wantExact error
	}{
		{name: "both succeed"},
		{name: "operation error is returned untouched", fnErr: requestFailed, wantExact: requestFailed},
		{name: "display error is wrapped", showErr: displayBroke, wantIs: []error{displayBroke}, wantText: "show progress spinner: terminal gone"},
		{name: "both errors survive", showErr: displayBroke, fnErr: requestFailed, wantIs: []error{displayBroke, requestFailed}, wantText: "request failed\nshow progress spinner: terminal gone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			release := make(chan struct{})
			var doneCalled bool
			result := make(chan error, 1)
			go func() {
				result <- runAlongside(
					func() error { return tt.showErr },
					func() error { <-release; return tt.fnErr },
					func() { doneCalled = true },
				)
			}()

			select {
			case <-result:
				t.Fatal("answered while the operation was still running")
			case <-time.After(50 * time.Millisecond):
			}
			close(release)

			err := <-result
			assert.True(t, doneCalled)
			switch {
			case tt.wantExact != nil:
				assert.Same(t, tt.wantExact, err)
			case len(tt.wantIs) == 0:
				assert.NoError(t, err)
			default:
				for _, want := range tt.wantIs {
					assert.ErrorIs(t, err, want)
				}
				assert.Equal(t, tt.wantText, err.Error())
			}
		})
	}
}
