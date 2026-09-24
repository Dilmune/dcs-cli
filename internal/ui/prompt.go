package ui

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// ErrNonInteractive is returned when a prompt is attempted in a non-interactive environment.
var ErrNonInteractive = errors.New("this operation requires interactive input\n\n  Provide all required flags, or use --force to skip confirmations")

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
		return false, ErrNonInteractive
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
		return "", ErrNonInteractive
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
		return "", ErrNonInteractive
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
		return "", ErrNonInteractive
	}
	var value string
	err := runPrompt(huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		EchoMode(huh.EchoModePassword).
		Value(&value))
	return value, err
}
