package client

import "encoding/json"

// APIResponse matches the server's httputil.Response format.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
	Meta    *Meta           `json:"meta,omitempty"`
}

type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type APIErrorResponse struct {
	Success bool     `json:"success"`
	Error   APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}
