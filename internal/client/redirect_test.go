package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RejectsHTTPSDowngradesWithoutRetry(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			t.Run(fmt.Sprintf("%s/%d", method, status), func(t *testing.T) {
				var originRequests, targetRequests atomic.Int32
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					targetRequests.Add(1)
					assert.NoError(t, json.NewEncoder(w).Encode(APIResponse{Success: true}))
				}))
				defer target.Close()
				origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					originRequests.Add(1)
					assert.Equal(t, method, r.Method)
					assert.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
					http.Redirect(w, r, target.URL+"/insecure?token=redirect-fixture-secret", status)
				}))
				defer origin.Close()
				c := New(origin.URL, "fixture-key")
				c.http.Transport = origin.Client().Transport
				response, err := c.do(context.Background(), method, "/test", map[string]string{"value": "fixture-body"}, nil)
				assert.Nil(t, response)
				assert.Zero(t, targetRequests.Load(), "no request or credentials may reach the insecure target")
				assert.EqualValues(t, 1, originRequests.Load(), "a rejected redirect must not replay the API action")
				require.ErrorIs(t, err, ErrInsecureRedirect)
				assert.NotContains(t, err.Error(), "redirect-fixture-secret")
				assert.NotContains(t, err.Error(), target.URL)
			})
		}
	}
}

func TestClient_AllowsSafeRedirects(t *testing.T) {
	for _, tc := range []struct {
		name, authorization             string
		originTLS, targetTLS, crossHost bool
	}{
		{name: "https to https", originTLS: true, targetTLS: true, authorization: "Bearer fixture-key"},
		{name: "local http to http", authorization: "Bearer fixture-key"},
		{name: "http to https", targetTLS: true, authorization: "Bearer fixture-key"},
		{name: "different hostname strips credentials", crossHost: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var targetRequests atomic.Int32
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetRequests.Add(1)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, tc.authorization, r.Header.Get("Authorization"))
				assert.NoError(t, json.NewEncoder(w).Encode(APIResponse{Success: true}))
			}))
			if tc.targetTLS {
				target.StartTLS()
			} else {
				target.Start()
			}
			defer target.Close()
			location := target.URL
			if tc.crossHost {
				location = strings.Replace(location, "127.0.0.1", "localhost", 1)
			}
			origin := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer fixture-key", r.Header.Get("Authorization"))
				http.Redirect(w, r, location, http.StatusFound)
			}))
			if tc.originTLS {
				origin.StartTLS()
			} else {
				origin.Start()
			}
			defer origin.Close()
			c := New(origin.URL, "fixture-key")
			if tc.targetTLS {
				c.http.Transport = target.Client().Transport
			}
			response, err := c.Get(context.Background(), "/test", nil)
			require.NoError(t, err)
			require.NotNil(t, response)
			assert.True(t, response.Success)
			assert.EqualValues(t, 1, targetRequests.Load())
		})
	}
}

func TestClient_RejectsDowngradeAfterHTTPUpgrade(t *testing.T) {
	var originRequests, secureRequests, insecureRequests atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		insecureRequests.Add(1)
		assert.NoError(t, json.NewEncoder(w).Encode(APIResponse{Success: true}))
	}))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secureRequests.Add(1)
		http.Redirect(w, r, insecure.URL, http.StatusFound)
	}))
	defer secure.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originRequests.Add(1)
		http.Redirect(w, r, secure.URL, http.StatusFound)
	}))
	defer origin.Close()
	c := New(origin.URL, "fixture-key")
	c.http.Transport = secure.Client().Transport
	_, err := c.Get(context.Background(), "/test", nil)
	assert.Zero(t, insecureRequests.Load())
	assert.EqualValues(t, 1, originRequests.Load())
	assert.EqualValues(t, 1, secureRequests.Load())
	require.ErrorIs(t, err, ErrInsecureRedirect)
}

func TestClient_PreservesDefaultRedirectLimit(t *testing.T) {
	var requests atomic.Int32
	_, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	_, _, err := c.execute(context.Background(), http.MethodGet, c.baseURL+"/loop", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped after 10 redirects")
	assert.EqualValues(t, 10, requests.Load())
}
