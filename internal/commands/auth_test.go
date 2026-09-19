package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
	"github.com/dilmune/dcs-cli/internal/config"
)

func TestWhoami(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAuthMe, http.StatusOK, map[string]any{
		"user": map[string]string{"id": "user-1", "email": "test@example.com", "name": "Test User"},
		"orgs": []any{},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	cmd := newWhoamiCmd()
	cmd.SetContext(context.Background())
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	require.NoError(t, runErr)

	assert.Contains(t, out, "test@example.com")
	assert.Contains(t, out, "Test User")
	assert.Contains(t, out, "user-1")
}

func TestWhoamiJSON(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAuthMe, http.StatusOK, map[string]any{
		"user": map[string]string{"id": "user-1", "email": "test@example.com", "name": "Test User"},
		"orgs": []any{},
	})
	cleanup := setupTest(t, api)
	defer cleanup()

	setJSONOutput()

	cmd := newWhoamiCmd()
	cmd.SetContext(context.Background())
	var runErr error
	out := captureStdout(t, func() {
		runErr = cmd.RunE(cmd, nil)
	})
	require.NoError(t, runErr)

	assert.True(t, json.Valid([]byte(out)), "output should be valid JSON")
	var me struct {
		User config.UserInfo `json:"user"`
		Orgs []any           `json:"orgs"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &me))
	assert.Equal(t, "test@example.com", me.User.Email)
	assert.NotNil(t, me.Orgs)
}

func TestDecodeAuthenticatedUser(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want *config.UserInfo
	}{
		{"nested identity", `{"user":{"id":"user-1","email":"test@example.com","name":"Test User"},"orgs":[]}`, &config.UserInfo{ID: "user-1", Email: "test@example.com", Name: "Test User"}},
		{"optional name", `{"user":{"id":"user-1","email":"test@example.com"}}`, &config.UserInfo{ID: "user-1", Email: "test@example.com"}},
		{"old flat shape", `{"id":"user-1","email":"test@example.com"}`, nil},
		{"empty user", `{"user":{}}`, nil},
		{"null user", `{"user":null}`, nil},
		{"missing id", `{"user":{"email":"test@example.com"}}`, nil},
		{"missing email", `{"user":{"id":"user-1"}}`, nil},
		{"blank identity", `{"user":{"id":" ","email":" "}}`, nil},
		{"wrong field type", `{"user":{"id":42}}`, nil},
		{"invalid json", `{`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			user, err := decodeAuthenticatedUser(&client.APIResponse{Success: true, Data: json.RawMessage(tt.data)})
			if tt.want == nil {
				require.Error(t, err)
				assert.Nil(t, user)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, user)
		})
	}
}

func TestWhoamiRejectsMissingIdentity(t *testing.T) {
	api := newMockAPI()
	api.on(http.MethodGet, client.PathAuthMe, http.StatusOK, map[string]any{"user": map[string]string{}})
	defer setupTest(t, api)()
	cmd := newWhoamiCmd()
	cmd.SetContext(context.Background())
	var runErr error
	out := captureStdout(t, func() { runErr = cmd.RunE(cmd, nil) })
	require.ErrorContains(t, runErr, "missing user identity")
	assert.Empty(t, out)
}

func TestWhoamiUnauthenticated(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	apiClient = nil

	cmd := newWhoamiCmd()
	err := cmd.RunE(cmd, nil)
	assert.ErrorIs(t, err, client.ErrNotAuthenticated)
}

func TestLogoutNotAuthenticated(t *testing.T) {
	api := newMockAPI()
	cleanup := setupTest(t, api)
	defer cleanup()

	cfg.APIKey = ""
	apiClient = nil

	cmd := newLogoutCmd()
	out := captureStdout(t, func() {
		err := cmd.RunE(cmd, nil)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Not currently authenticated")
}
