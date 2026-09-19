package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c := New(ts.URL, "test-api-key")
	return ts, c
}

func TestClient_Get_Success(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Contains(t, r.Header.Get("User-Agent"), "dcs-cli/")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{
			Success: true,
			Data:    json.RawMessage(`{"name":"test"}`),
		})
	})

	resp, err := c.Get(context.Background(), "/test", nil)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, string(resp.Data), "test")
}

func TestClient_Get_WithQueryParams(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bar", r.URL.Query().Get("foo"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	})

	query := url.Values{"foo": {"bar"}}
	resp, err := c.Get(context.Background(), "/test", query)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestClient_Post_WithBody(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "hello", body["key"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true})
	})

	resp, err := c.Post(context.Background(), "/test", map[string]string{"key": "hello"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestClient_Delete(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse{Success: true, Message: "deleted"})
	})

	resp, err := c.Delete(context.Background(), "/test")
	require.NoError(t, err)
	assert.Equal(t, "deleted", resp.Message)
}

func TestClient_Unauthorized(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	})

	_, err := c.Get(context.Background(), "/test", nil)
	assert.ErrorIs(t, err, ErrNotAuthenticated)
}

func TestClient_APIError(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIErrorResponse{
			Error: APIError{Code: "bad_request", Message: "invalid input"},
		})
	})

	_, err := c.Get(context.Background(), "/test", nil)
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "bad_request", apiErr.Code)
	assert.Equal(t, "invalid input", apiErr.Message)
}

func TestClient_GenericError(t *testing.T) {
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})

	_, err := c.Get(context.Background(), "/test", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDecode_Success(t *testing.T) {
	type item struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	resp := &APIResponse{
		Data: json.RawMessage(`{"name":"Alice","age":30}`),
	}

	result, err := Decode[item](resp)
	require.NoError(t, err)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, 30, result.Age)
}

func TestDecode_Slice(t *testing.T) {
	resp := &APIResponse{
		Data: json.RawMessage(`[{"name":"a"},{"name":"b"}]`),
	}

	type item struct {
		Name string `json:"name"`
	}

	result, err := Decode[[]item](resp)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Name)
	assert.Equal(t, "b", result[1].Name)
}

func TestDecode_InvalidJSON(t *testing.T) {
	resp := &APIResponse{
		Data: json.RawMessage(`not json`),
	}

	type item struct{}
	_, err := Decode[item](resp)
	assert.Error(t, err)
}
