package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIError_WithSuggestion(t *testing.T) {
	err := &CLIError{
		Message:    "Something went wrong.",
		Suggestion: "Try running 'dcs fix'.",
	}

	assert.Contains(t, err.Error(), "Something went wrong.")
	assert.Contains(t, err.Error(), "Try running 'dcs fix'.")
}

func TestCLIError_WithoutSuggestion(t *testing.T) {
	err := &CLIError{Message: "Plain error."}
	assert.Equal(t, "Plain error.", err.Error())
}

func TestCLIError_Unwrap(t *testing.T) {
	inner := assert.AnError
	err := &CLIError{
		Message: "Wrapped.",
		Err:     inner,
	}

	assert.ErrorIs(t, err, inner)
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		Code:    "not_found",
		Message: "Resource not found",
	}

	assert.Equal(t, "Resource not found", err.Error())
}

func TestSentinelErrors(t *testing.T) {
	assert.Contains(t, ErrNotAuthenticated.Error(), "Not authenticated")
	assert.Contains(t, ErrNotAuthenticated.Error(), "dcs login")

	assert.Contains(t, ErrNoProjectConfig.Error(), ".dcs.json")
	assert.Contains(t, ErrNoProjectConfig.Error(), "dcs init")

	assert.Contains(t, ErrServerNotFound.Error(), "Server not found")
	assert.Contains(t, ErrSiteNotFound.Error(), "Site not found")
}
