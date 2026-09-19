package client

import "fmt"

// CLIError wraps errors with actionable suggestions.
type CLIError struct {
	Message    string
	Suggestion string
	Err        error
}

func (e *CLIError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s\n\n  %s", e.Message, e.Suggestion)
	}
	return e.Message
}

func (e *CLIError) Unwrap() error {
	return e.Err
}

var (
	ErrInsecureRedirect = &CLIError{
		Message:    "Refusing an HTTPS-to-HTTP redirect.",
		Suggestion: "Check the API endpoint or contact Dilmune support.",
	}
	ErrNotAuthenticated = &CLIError{
		Message:    "Not authenticated.",
		Suggestion: "Run 'dcs login' to get started.",
	}
	ErrNoProjectConfig = &CLIError{
		Message:    "No .dcs.json found in the current directory.",
		Suggestion: "Run 'dcs init' to link this directory to a project.",
	}
	ErrServerNotFound = &CLIError{
		Message:    "Server not found.",
		Suggestion: "Run 'dcs servers list' to see your servers.",
	}
	ErrSiteNotFound = &CLIError{
		Message:    "Site not found.",
		Suggestion: "Run 'dcs sites list' to see your sites.",
	}
	ErrForbidden = &CLIError{
		Message:    "Permission denied.",
		Suggestion: "Your API key may not have access to this resource.",
	}
	ErrRateLimited = &CLIError{
		Message:    "Too many requests.",
		Suggestion: "Slow down and try again in a moment.",
	}
)
