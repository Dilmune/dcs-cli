package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWSTypeConstants(t *testing.T) {
	assert.Equal(t, "subscribe", WSTypeSubscribe)
	assert.Equal(t, "unsubscribe", WSTypeUnsubscribe)
	assert.Equal(t, "ping", WSTypePing)
	assert.Equal(t, "event", WSTypeEvent)
	assert.Equal(t, "error", WSTypeError)
	assert.Equal(t, "pong", WSTypePong)
}

func TestWSTopicConstants(t *testing.T) {
	assert.Equal(t, "server:", WSTopicServer)
	assert.Equal(t, "site:", WSTopicSite)
	assert.Equal(t, "deployment:", WSTopicDeployment)
	assert.Equal(t, ":status", WSTopicStatus)
}

func TestWSTopicBuilding(t *testing.T) {
	topic := WSTopicServer + "srv-123" + WSTopicStatus
	assert.Equal(t, "server:srv-123:status", topic)

	topic = WSTopicDeployment + "site-456" + WSTopicStatus
	assert.Equal(t, "deployment:site-456:status", topic)
}
