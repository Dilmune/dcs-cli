package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// WebSocket message types (client → server).
const (
	WSTypeSubscribe   = "subscribe"
	WSTypeUnsubscribe = "unsubscribe"
	WSTypePing        = "ping"
)

// WebSocket message types (server → client).
const (
	WSTypeEvent = "event"
	WSTypeError = "error"
	WSTypePong  = "pong"
)

// WebSocket topic prefixes for building subscription topics.
const (
	WSTopicServer     = "server:"
	WSTopicSite       = "site:"
	WSTopicDeployment = "deployment:"
	WSTopicDatabase   = "database:"
	WSTopicStatus     = ":status"
	WSTopicEvents     = ":events"
	WSTopicMetrics    = ":metrics"
	WSTopicDatabases  = ":databases"
)

// WSMessage represents a message from the server's WebSocket.
type WSMessage struct {
	Type    string          `json:"type"`
	Topic   string          `json:"topic,omitempty"`
	Event   string          `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WSConn wraps a gorilla/websocket connection with DCS auth.
type WSConn struct {
	conn *websocket.Conn
}

// DialWS connects to the DCS WebSocket endpoint with Bearer auth.
func (c *Client) DialWS() (*WSConn, error) {
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += PathWS

	header := http.Header{}
	header.Set("Authorization", bearerPrefix+c.apiKey)
	header.Set("User-Agent", userAgentPrefix+Version)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrNotAuthenticated
		}
		return nil, fmt.Errorf("websocket connect: %w", err)
	}

	return &WSConn{conn: conn}, nil
}

// Subscribe sends a subscription request for a topic.
func (ws *WSConn) Subscribe(topic string) error {
	msg := map[string]string{"type": WSTypeSubscribe, "topic": topic}
	if err := ws.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}

// Unsubscribe sends an unsubscription request for a topic.
func (ws *WSConn) Unsubscribe(topic string) error {
	msg := map[string]string{"type": WSTypeUnsubscribe, "topic": topic}
	if err := ws.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("unsubscribe %s: %w", topic, err)
	}
	return nil
}

// ReadMessage reads the next message from the WebSocket.
func (ws *WSConn) ReadMessage() (*WSMessage, error) {
	var msg WSMessage
	if err := ws.conn.ReadJSON(&msg); err != nil {
		return nil, fmt.Errorf("read ws message: %w", err)
	}
	return &msg, nil
}

// Ping sends a ping message to keep the connection alive.
func (ws *WSConn) Ping() error {
	msg := map[string]string{"type": WSTypePing}
	if err := ws.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("ws ping: %w", err)
	}
	return nil
}

// Close gracefully closes the WebSocket connection.
func (ws *WSConn) Close() error {
	return ws.conn.Close()
}
