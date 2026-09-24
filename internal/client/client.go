package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Version is set via ldflags at build time.
var Version = "3.9.0"

const (
	bearerPrefix    = "Bearer "
	userAgentPrefix = "dcs-cli/"
	defaultTimeout  = 30 * time.Second
	maxRetries      = 2
	maxRedirects    = 10
	httpScheme      = "http"
	httpsScheme     = "https"
)

// Client is the DCS API HTTP client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	debug   bool
}

// New creates a new API client.
func New(baseURL, apiKey string) *Client {
	timeout := defaultTimeout
	if v := os.Getenv("DCS_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout:       timeout,
			CheckRedirect: checkAPIRedirect,
		},
	}
}

func checkAPIRedirect(req *http.Request, via []*http.Request) error {
	if isHTTPSDowngrade(via[len(via)-1].URL, req.URL) {
		return fmt.Errorf("follow API redirect: %w", ErrInsecureRedirect)
	}
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	return nil
}

func isHTTPSDowngrade(previous, next *url.URL) bool {
	return previous.Scheme == httpsScheme && next.Scheme == httpScheme
}

// SetDebug enables request/response logging to stderr.
func (c *Client) SetDebug(debug bool) {
	c.debug = debug
}

func (c *Client) do(ctx context.Context, method, path string, body any, query url.Values) (*APIResponse, error) {
	u := c.baseURL + path
	if query != nil {
		u += "?" + query.Encode()
	}

	var bodyData []byte
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyData = data
	}

	if c.debug {
		fmt.Fprintf(os.Stderr, "  [debug] %s %s\n", method, path)
	}

	var lastErr error
	for attempt := range maxRetries + 1 {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			if c.debug {
				fmt.Fprintf(os.Stderr, "  [debug] retry #%d\n", attempt)
			}
		}

		result, retry, err := c.execute(ctx, method, u, bodyData)
		if err != nil {
			lastErr = err
			if retry && attempt < maxRetries {
				continue
			}
			return nil, err
		}
		return result, nil
	}
	return nil, lastErr
}

func (c *Client) execute(ctx context.Context, method, rawURL string, bodyData []byte) (*APIResponse, bool, error) {
	var bodyReader io.Reader
	if bodyData != nil {
		bodyReader = bytes.NewReader(bodyData)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", bearerPrefix+c.apiKey)
	req.Header.Set("User-Agent", userAgentPrefix+Version)
	if bodyData != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrInsecureRedirect) {
			// Redirect errors wrap a destination URL, which may contain credentials.
			return nil, false, fmt.Errorf("send API request: %w", ErrInsecureRedirect)
		}
		return nil, true, &CLIError{
			Message:    fmt.Sprintf("Request failed: %v", err),
			Suggestion: "Check your internet connection or API status at status.dilmune.com",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	if c.debug {
		fmt.Fprintf(os.Stderr, "  [debug] %d %s (%s)\n", resp.StatusCode, req.URL.Path, time.Since(start).Round(time.Millisecond))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, false, ErrNotAuthenticated

	case resp.StatusCode == http.StatusForbidden:
		return nil, false, ErrForbidden

	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, ErrRateLimited

	case resp.StatusCode >= 500:
		var errResp APIErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, true, &CLIError{
				Message:    errResp.Error.Message,
				Suggestion: "This is a server-side error. Try again or check status.dilmune.com",
			}
		}
		return nil, true, &CLIError{
			Message:    fmt.Sprintf("Server error (%d)", resp.StatusCode),
			Suggestion: "Try again or check status.dilmune.com",
		}

	case resp.StatusCode >= 400:
		var errResp APIErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			return nil, false, &APIError{
				Code:    errResp.Error.Code,
				Message: errResp.Error.Message,
				Details: errResp.Error.Details,
			}
		}
		return nil, false, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, false, fmt.Errorf("parse response: %w", err)
	}
	return &apiResp, false, nil
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (*APIResponse, error) {
	return c.do(ctx, http.MethodGet, path, nil, query)
}

func (c *Client) Post(ctx context.Context, path string, body any) (*APIResponse, error) {
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) Put(ctx context.Context, path string, body any) (*APIResponse, error) {
	return c.do(ctx, http.MethodPut, path, body, nil)
}

func (c *Client) Patch(ctx context.Context, path string, body any) (*APIResponse, error) {
	return c.do(ctx, http.MethodPatch, path, body, nil)
}

func (c *Client) Delete(ctx context.Context, path string) (*APIResponse, error) {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// Decode unmarshals the API response data into the given type.
func Decode[T any](resp *APIResponse) (T, error) {
	var result T
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return result, fmt.Errorf("decode response data: %w", err)
	}
	return result, nil
}
