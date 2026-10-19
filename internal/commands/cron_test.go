package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dilmune/dcs-cli/internal/client"
)

// The bodies below are what the API sends: camelCase keys, and an exitCode of
// 0 on a run from the first cron runner, which never recorded one.
const cronJobsBody = `[{
	"id": "cron-1", "serverId": "srv-1", "name": "nightly", "command": "/usr/bin/backup",
	"schedule": "0 2 * * *", "runAsUser": "deploy", "enabled": true,
	"createdAt": "2026-10-01T00:00:00Z", "updatedAt": "2026-10-01T00:00:00Z"
}]`

const cronExecutionsBody = `[
	{"id": "e-fixed-ok", "cronJobId": "cron-1", "serverId": "srv-1", "startedAt": "2026-10-11T02:00:00Z",
	 "completedAt": "2026-10-11T02:00:04Z", "exitCode": 0, "exitCodeRecorded": true,
	 "output": "fixed-ok", "durationMs": 4000, "createdAt": "2026-10-11T02:00:04Z"},
	{"id": "e-fixed-failed", "cronJobId": "cron-1", "serverId": "srv-1", "startedAt": "2026-10-10T02:00:00Z",
	 "completedAt": "2026-10-10T02:00:01Z", "exitCode": 3, "exitCodeRecorded": true,
	 "output": "fixed-failed", "durationMs": 1000, "createdAt": "2026-10-10T02:00:01Z"},
	{"id": "e-old", "cronJobId": "cron-1", "serverId": "srv-1", "startedAt": "2026-10-01T02:00:00Z",
	 "exitCode": 0, "exitCodeRecorded": false,
	 "output": "old-runner", "durationMs": 2500, "createdAt": "2026-10-01T02:00:02Z"}
]`

func cronAPI(t *testing.T) {
	t.Helper()
	api := newMockAPI()
	api.on(http.MethodGet, client.PathCronJobs("srv-1"), http.StatusOK, json.RawMessage(cronJobsBody))
	api.on(http.MethodGet, client.PathCronJobExecutions("srv-1", "cron-1"), http.StatusOK, json.RawMessage(cronExecutionsBody))
	t.Cleanup(setupTest(t, api))
	cfg.DefaultServerID = "srv-1"
}

func lineContaining(t *testing.T, output, marker string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, marker) {
			return line
		}
	}
	require.Failf(t, "line not found", "no line contains %q in:\n%s", marker, output)
	return ""
}

func TestCronHistoryDoesNotShowAnUnrecordedRunAsExitZero(t *testing.T) {
	cronAPI(t)

	output := executeContractCommand(t, newCronHistoryCmd(), "nightly")

	old := lineContaining(t, output, "old-runner")
	assert.Contains(t, old, "not recorded")
	assert.Contains(t, old, "2500ms", "durationMs is read from the camelCase body")

	ok := lineContaining(t, output, "fixed-ok")
	assert.NotContains(t, ok, "not recorded")
	assert.Contains(t, ok, "4000ms")
	assert.Regexp(t, `\b0\b`, strings.Replace(ok, "4000ms", "", 1))

	failed := lineContaining(t, output, "fixed-failed")
	assert.NotContains(t, failed, "not recorded")
	assert.Regexp(t, `\b3\b`, failed)
}

func TestCronHistoryJSONKeepsTheServersFields(t *testing.T) {
	cronAPI(t)
	setJSONOutput()

	output := executeContractCommand(t, newCronHistoryCmd(), "nightly")

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &rows))
	require.Len(t, rows, 3)
	assert.Equal(t, false, rows[2]["exitCodeRecorded"])
	assert.Equal(t, true, rows[0]["exitCodeRecorded"])
}

func TestCronListReadsTheCamelCaseContract(t *testing.T) {
	cronAPI(t)

	output := executeContractCommand(t, newCronListCmd())

	line := lineContaining(t, output, "nightly")
	for _, want := range []string{"0 2 * * *", "/usr/bin/backup", "deploy", "2026-10-01"} {
		assert.Contains(t, line, want)
	}
}

func TestCronExecutionDecodesTheAPIBody(t *testing.T) {
	var rows []CronExecution
	require.NoError(t, json.Unmarshal([]byte(cronExecutionsBody), &rows))
	require.Len(t, rows, 3)

	assert.True(t, rows[0].ExitCodeRecorded)
	require.NotNil(t, rows[1].ExitCode)
	assert.Equal(t, 3, *rows[1].ExitCode)
	assert.Equal(t, "cron-1", rows[2].CronJobID)
	assert.Equal(t, 2500, rows[2].DurationMs)
	assert.False(t, rows[2].ExitCodeRecorded)
	assert.Contains(t, rows[2].exitCodeCell(), exitCodeNotRecorded)
}
