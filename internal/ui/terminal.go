package ui

// TerminalWidth is the stdout column count for command layout, 80 when
// stdout is not a terminal.
func TerminalWidth() int { return terminalWidth() }
