package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// OutputFormat constants.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
	FormatCSV   = "csv"
)

var outputFormat string

// SetOutputFormat sets the global output format.
func SetOutputFormat(f string) { outputFormat = f }

// GetOutputFormat returns the current output format.
func GetOutputFormat() string { return outputFormat }

// HasFormatOverride returns true if a non-table output format is set.
func HasFormatOverride() bool {
	return outputFormat != "" && outputFormat != FormatTable
}

// PrintFormatted handles output in the requested format.
// For json/yaml it uses the raw API response data.
// For csv it uses headers + rows.
// Returns true if output was handled.
func PrintFormatted(data json.RawMessage, headers []string, rows [][]string) bool {
	switch outputFormat {
	case FormatJSON:
		PrintJSONRaw(data)
		return true
	case FormatYAML:
		printYAML(data)
		return true
	case FormatCSV:
		printCSV(headers, rows)
		return true
	default:
		return false
	}
}

func printYAML(data json.RawMessage) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Fprintln(os.Stderr, "failed to parse data for YAML output")
		return
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to marshal YAML")
		return
	}
	fmt.Print(string(out))
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func printCSV(headers []string, rows [][]string) {
	w := csv.NewWriter(os.Stdout)
	_ = w.Write(headers)
	for _, row := range rows {
		clean := make([]string, len(row))
		for i, cell := range row {
			clean[i] = stripAnsi(cell)
		}
		_ = w.Write(clean)
	}
	w.Flush()
}
