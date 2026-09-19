package commands

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

func TestEnvironmentReadUsesVarsEnvelope(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command func() *cobra.Command
		args    []string
		json    bool
		want    string
	}{
		{"list", newEnvListCmd, []string{"--show-values"}, false, "KEEP=fixture-value"},
		{"get", newEnvGetCmd, []string{"KEEP"}, false, "fixture-value\n"},
		{"json list unchanged", newEnvListCmd, nil, true, `{"vars":{"KEEP":"fixture-value"}}`},
		{"json get", newEnvGetCmd, []string{"KEEP"}, true, `{"KEEP":"fixture-value"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, client.PathSiteEnv("srv-1", "site-1"), r.URL.Path)
				writeContractResponse(w, map[string]any{"vars": map[string]string{"KEEP": "fixture-value"}})
			})
			linkContractProject(t)
			if tt.json {
				setJSONOutput()
			}
			output := executeContractCommand(t, tt.command(), tt.args...)
			if tt.json {
				assert.JSONEq(t, tt.want, output)
			} else {
				assert.Contains(t, output, tt.want)
			}
		})
	}
}

func TestEnvironmentMutationsPreserveOtherValues(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command func() *cobra.Command
		args    []string
		initial map[string]string
		want    map[string]string
		output  string
	}{
		{"set", newEnvSetCmd, []string{"CHANGE=new=value", "EMPTY="}, map[string]string{"KEEP": "keep-me", "CHANGE": "old"}, map[string]string{"KEEP": "keep-me", "CHANGE": "new=value", "EMPTY": ""}, `{"updated":2}`},
		{"set empty", newEnvSetCmd, []string{"NEW=value"}, map[string]string{}, map[string]string{"NEW": "value"}, `{"updated":1}`},
		{"remove", newEnvRmCmd, []string{"REMOVE"}, map[string]string{"KEEP": "keep-me", "REMOVE": "old"}, map[string]string{"KEEP": "keep-me"}, `{"removed":"REMOVE"}`},
		{"remove last", newEnvRmCmd, []string{"REMOVE"}, map[string]string{"REMOVE": "old"}, map[string]string{}, `{"removed":"REMOVE"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var gets, puts atomic.Int64
			setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, client.PathSiteEnv("srv-1", "site-1"), r.URL.Path)
				switch r.Method {
				case http.MethodGet:
					gets.Add(1)
					writeContractResponse(w, map[string]any{"vars": tt.initial})
				case http.MethodPut:
					puts.Add(1)
					assert.EqualValues(t, 1, gets.Load())
					var body map[string]map[string]string
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, map[string]map[string]string{"vars": tt.want}, body)
					writeContractResponse(w, nil)
				default:
					t.Errorf("unexpected request method %s", r.Method)
					http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
				}
			})
			linkContractProject(t)
			setJSONOutput()
			output := executeContractCommand(t, tt.command(), tt.args...)
			assert.JSONEq(t, tt.output, output)
			assert.EqualValues(t, 1, gets.Load())
			assert.EqualValues(t, 1, puts.Load())
		})
	}
}

func TestEnvironmentMutationRejectsUnusableSnapshot(t *testing.T) {
	for _, tt := range []struct {
		name     string
		response any
		status   int
	}{
		{"missing vars", map[string]any{}, http.StatusOK},
		{"null vars", map[string]any{"vars": nil}, http.StatusOK},
		{"null unrelated value", map[string]any{"vars": map[string]any{"KEEP": nil, "REMOVE": "old"}}, http.StatusOK},
		{"null target value", map[string]any{"vars": map[string]any{"KEEP": "keep-me", "REMOVE": nil, "CHANGE": nil}}, http.StatusOK},
		{"non-string value", map[string]any{"vars": map[string]any{"KEEP": 42, "REMOVE": "old"}}, http.StatusOK},
		{"wrong shape", map[string]any{"vars": []any{}}, http.StatusOK},
		{"read denied", nil, http.StatusForbidden},
	} {
		for _, operation := range []string{"set", "rm"} {
			t.Run(tt.name+"_"+operation, func(t *testing.T) {
				var puts atomic.Int64
				setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPut {
						puts.Add(1)
					}
					w.WriteHeader(tt.status)
					writeContractResponse(w, tt.response)
				})
				linkContractProject(t)
				cmd := newEnvSetCmd()
				cmd.SetArgs([]string{"CHANGE=new"})
				if operation == "rm" {
					cmd = newEnvRmCmd()
					cmd.SetArgs([]string{"REMOVE"})
				}
				require.Error(t, cmd.Execute())
				assert.Zero(t, puts.Load())
			})
		}
	}
}

func TestEnvironmentRemoveMissingKeyDoesNotWrite(t *testing.T) {
	var puts atomic.Int64
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts.Add(1)
		}
		writeContractResponse(w, map[string]any{"vars": map[string]string{"KEEP": "keep-me"}})
	})
	linkContractProject(t)
	cmd := newEnvRmCmd()
	cmd.SetArgs([]string{"MISSING"})
	require.ErrorContains(t, cmd.Execute(), "not found")
	assert.Zero(t, puts.Load())
}

func TestEnvironmentInvalidArgumentDoesNotLeakOrCallAPI(t *testing.T) {
	var requests atomic.Int64
	setupContractAPI(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeContractResponse(w, nil)
	})
	linkContractProject(t)
	cmd := newEnvSetCmd()
	cmd.SetArgs([]string{"fixture-secret-without-equals"})
	err := cmd.Execute()
	require.ErrorContains(t, err, "expected KEY=VALUE")
	assert.NotContains(t, err.Error(), "fixture-secret")
	assert.Zero(t, requests.Load())
}
