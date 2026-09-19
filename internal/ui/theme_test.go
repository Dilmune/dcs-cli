package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusColor_Active(t *testing.T) {
	for _, status := range []string{StatusActive, StatusRunning, StatusDeployed, StatusCompleted, StatusSuccess} {
		assert.Equal(t, Success, StatusColor(status), "expected Success style for %q", status)
	}
}

func TestStatusColor_InProgress(t *testing.T) {
	for _, status := range []string{StatusProvisioning, StatusInstalling, StatusDeploying, StatusBuilding, StatusPending} {
		assert.Equal(t, Warning, StatusColor(status), "expected Warning style for %q", status)
	}
}

func TestStatusColor_Error(t *testing.T) {
	for _, status := range []string{StatusError, StatusFailed, StatusSuspended, StatusDelinquent} {
		assert.Equal(t, Error, StatusColor(status), "expected Error style for %q", status)
	}
}

func TestStatusColor_Muted(t *testing.T) {
	for _, status := range []string{StatusDeleting, StatusDeleted, "unknown", ""} {
		assert.Equal(t, Muted, StatusColor(status), "expected Muted style for %q", status)
	}
}

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "active", StatusActive)
	assert.Equal(t, "running", StatusRunning)
	assert.Equal(t, "error", StatusError)
	assert.Equal(t, "failed", StatusFailed)
	assert.Equal(t, "provisioning", StatusProvisioning)
	assert.Equal(t, "deploying", StatusDeploying)
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "error", EventError)
	assert.Equal(t, "success", EventSuccess)
	assert.Equal(t, "completed", EventCompleted)
	assert.Equal(t, "progress", EventProgress)
	assert.Equal(t, "deploy", EventDeploy)
	assert.Equal(t, "status_changed", EventStatusChanged)
}

func TestBillingConstants(t *testing.T) {
	assert.Equal(t, "grace_period", BillingGracePeriod)
	assert.Equal(t, StatusSuspended, BillingSuspended)
	assert.Equal(t, StatusDelinquent, BillingDelinquent)
}
