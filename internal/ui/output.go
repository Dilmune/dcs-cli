package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/term"
)

var quiet bool

// SetQuiet enables or disables quiet mode, suppressing non-essential output.
func SetQuiet(q bool) { quiet = q }

// IsTerminal returns true if stdout is a terminal.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsInteractive returns true if both stdin and stdout are terminals.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && IsTerminal()
}

// PrintJSON outputs data as formatted JSON.
func PrintJSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

// PrintJSONRaw outputs raw JSON bytes.
func PrintJSONRaw(data json.RawMessage) {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		fmt.Println(buf.String())
	} else {
		fmt.Println(string(data))
	}
}

// PrintSuccess prints a green success message.
func PrintSuccess(message string) {
	if quiet {
		return
	}
	fmt.Printf("\n  %s %s\n\n", Success.Render("✓"), message)
}

// PrintError prints a red error message to stderr. Never suppressed.
func PrintError(err error) {
	fmt.Fprintf(os.Stderr, "\n  %s %s\n\n", Error.Render("✗"), err.Error())
}

// PrintWarning prints a yellow warning message.
func PrintWarning(message string) {
	if quiet {
		return
	}
	fmt.Printf("  %s %s\n", Warning.Render("!"), message)
}

// PrintInfo prints a blue info message.
func PrintInfo(message string) {
	if quiet {
		return
	}
	fmt.Printf("  %s %s\n", Info.Render("i"), message)
}
