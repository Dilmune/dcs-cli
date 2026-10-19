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
	// ErrEnvAPIKeyRejected is a 401 for a key from DCS_API_KEY. It is still
	// ErrNotAuthenticated to errors.Is, but 'dcs login' cannot help: the
	// variable keeps overriding whatever login stores.
	ErrEnvAPIKeyRejected = &CLIError{
		Message:    "The API key in DCS_API_KEY was rejected.",
		Suggestion: "Check its value, or create a new key in the portal under Settings, API Keys, and set DCS_API_KEY to it.",
		Err:        ErrNotAuthenticated,
	}
	ErrNoProjectConfig = &CLIError{
		Message:    "No .dcs.json found in the current directory.",
		Suggestion: "Run 'dcs init' to link this directory to a project.",
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
