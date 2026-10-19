package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// ErrNonInteractive is returned when a prompt is attempted in a non-interactive environment.
var ErrNonInteractive = errors.New("this operation requires interactive input")

// nonInteractiveAdvice is all a prompt can say on its own. It does not know
// which flag would have answered it, and --force only answers a confirmation,
// so the caller that knows names it through ExplainNonInteractive.
const nonInteractiveAdvice = "Run it in a terminal, or check --help for flags that supply the answer."

// NonInteractiveError is ErrNonInteractive plus what would have answered the
// prompt, so a script is sent to a flag rather than to a terminal.
type NonInteractiveError struct {
	Hint string
}

func (e *NonInteractiveError) Error() string {
	return ErrNonInteractive.Error() + "\n\n  " + e.Hint
}

func (e *NonInteractiveError) Unwrap() error {
	return ErrNonInteractive
}

// ExplainNonInteractive replaces the advice on a prompt that could not run
// with hint, and returns any other error, or nil, unchanged.
func ExplainNonInteractive(err error, hint string) error {
	if errors.Is(err, ErrNonInteractive) {
		return &NonInteractiveError{Hint: hint}
	}
	return err
}

func nonInteractive() error {
	return &NonInteractiveError{Hint: nonInteractiveAdvice}
}

// promptTheme returns huh's own styles with the accent token on the five
// elements a form highlights. The focused button keeps huh's foreground so no
// color outside the palette is introduced.
//
// huh offers to pick light or dark from what the terminal answers mid-form.
// Init has already settled that for the process, so the offer is declined and
// the resolved mode answers instead.
func promptTheme() huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		s := huh.ThemeCharm(mode.HasDarkBackground())
		accent := Palette(mode).Accent

		s.Focused.Title = s.Focused.Title.Foreground(accent)
		s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(accent)
		s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(accent)
		s.Focused.FocusedButton = s.Focused.FocusedButton.Background(accent)
		s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(accent)

		return s
	})
}

// runPrompt runs one field as its own form, the way huh.Run does, with the
// color support of this output attached. huh's styles carry their own colors
// and there is no renderer to set globally, so the program is the only place
// that can keep plain output free of escape sequences.
func runPrompt(field huh.Field) error {
	return huh.NewForm(huh.NewGroup(field)).
		WithShowHelp(false).
		WithTheme(promptTheme()).
		WithProgramOptions(tea.WithColorProfile(painter.profile)).
		Run()
}

// Confirm shows a yes/no confirmation prompt.
func Confirm(message string) (bool, error) {
	if !IsInteractive() {
		return false, nonInteractive()
	}
	var confirmed bool
	err := runPrompt(huh.NewConfirm().
		Title(message).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed))
	return confirmed, err
}

// SelectOption shows a selection prompt and returns the selected value.
func SelectOption(title string, options []huh.Option[string]) (string, error) {
	if !IsInteractive() {
		return "", nonInteractive()
	}
	var selected string
	err := runPrompt(huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(&selected))
	return selected, err
}

// Input shows a text input prompt and returns the value.
func Input(title, placeholder string) (string, error) {
	if !IsInteractive() {
		return "", nonInteractive()
	}
	var value string
	err := runPrompt(huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value))
	return value, err
}

// InputPassword shows a masked text input.
func InputPassword(title, placeholder string) (string, error) {
	if !IsInteractive() {
		return "", nonInteractive()
	}
	var value string
	err := runPrompt(huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		EchoMode(huh.EchoModePassword).
		Value(&value))
	return value, err
}
