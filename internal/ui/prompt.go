package ui

import (
	"errors"

	"github.com/charmbracelet/huh"
)

// ErrNonInteractive is returned when a prompt is attempted in a non-interactive environment.
var ErrNonInteractive = errors.New("this operation requires interactive input\n\n  Provide all required flags, or use --force to skip confirmations")

// NewTheme returns a Huh form theme on the accent token. The focused button
// keeps huh's own foreground so no color outside the palette is introduced.
func NewTheme() *huh.Theme {
	t := huh.ThemeCharm()
	accent := Palette(mode).Accent

	t.Focused.Title = t.Focused.Title.Foreground(accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(accent)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(accent)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(accent)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)

	return t
}

// Confirm shows a yes/no confirmation prompt.
func Confirm(message string) (bool, error) {
	if !IsInteractive() {
		return false, ErrNonInteractive
	}
	var confirmed bool
	err := huh.NewConfirm().
		Title(message).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		WithTheme(NewTheme()).
		Run()
	return confirmed, err
}

// SelectOption shows a selection prompt and returns the selected value.
func SelectOption(title string, options []huh.Option[string]) (string, error) {
	if !IsInteractive() {
		return "", ErrNonInteractive
	}
	var selected string
	err := huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Value(&selected).
		WithTheme(NewTheme()).
		Run()
	return selected, err
}

// Input shows a text input prompt and returns the value.
func Input(title, placeholder string) (string, error) {
	if !IsInteractive() {
		return "", ErrNonInteractive
	}
	var value string
	err := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value).
		WithTheme(NewTheme()).
		Run()
	return value, err
}

// InputPassword shows a masked text input.
func InputPassword(title, placeholder string) (string, error) {
	if !IsInteractive() {
		return "", ErrNonInteractive
	}
	var value string
	err := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		EchoMode(huh.EchoModePassword).
		Value(&value).
		WithTheme(NewTheme()).
		Run()
	return value, err
}
