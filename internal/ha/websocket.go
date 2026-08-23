// Package ha provides Home Assistant integration for TapeDeck.
package ha

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Chuntttttt/tapedeck/internal/constants"
	"github.com/Chuntttttt/tapedeck/internal/logger"
	"github.com/gorilla/websocket"
)

// HAClient manages a WebSocket connection to Home Assistant
type HAClient struct { //nolint:revive // HAClient name is intentional for clarity
	url            string
	token          string
	conn           *websocket.Conn
	done           chan struct{}
	mu             sync.Mutex
	reconnectMu    sync.Mutex // Ensures only one Reconnect operation at a time
	tagCallback    func(tagID string)
	reconnectDelay time.Duration
}

// NewHAClient creates a new Home Assistant WebSocket client
func NewHAClient(haURL, token string) *HAClient {
	// Convert HTTP(S) URL to WS(S) URL
	wsURL := strings.Replace(haURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/websocket"

	return &HAClient{
		url:            wsURL,
		token:          token,
		done:           make(chan struct{}),
		reconnectDelay: constants.HAWebSocketReconnectDelay,
	}
}

// OnTagScanned registers a callback for tag_scanned events
func (c *HAClient) OnTagScanned(callback func(tagID string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tagCallback = callback
}

// Connect establishes a WebSocket connection to Home Assistant
func (c *HAClient) Connect() error {
	// Dial WebSocket
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = constants.HAWebSocketHandshakeTimeout

	conn, _, err := dialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial Home Assistant: %w", err)
	}

	// Read auth_required message
	var authRequired map[string]interface{}
	if err := conn.ReadJSON(&authRequired); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("failed to read auth_required: %w", err)
	}

	if msgType, ok := authRequired["type"].(string); !ok || msgType != "auth_required" {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("expected auth_required, got %v", authRequired["type"])
	}

	// Send auth message
	authMsg := map[string]string{
		"type":         "auth",
		"access_token": c.token,
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("failed to send auth: %w", err)
	}

	// Read auth response
	var authResponse map[string]interface{}
	if err := conn.ReadJSON(&authResponse); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	msgType, ok := authResponse["type"].(string)
	if !ok {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("auth response missing type field")
	}

	if msgType == "auth_invalid" {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("authentication failed: invalid token")
	}

	if msgType != "auth_ok" {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("unexpected auth response: %s", msgType)
	}

	// Subscribe to tag_scanned events
	subscribeMsg := map[string]interface{}{
		"id":         1,
		"type":       "subscribe_events",
		"event_type": "tag_scanned",
	}
	if err := conn.WriteJSON(subscribeMsg); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Read subscription result
	var result map[string]interface{}
	if err := conn.ReadJSON(&result); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("failed to read subscription result: %w", err)
	}

	if resultType, ok := result["type"].(string); !ok || resultType != "result" {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("expected result, got %v", result["type"])
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Warn("Failed to close connection", "error", closeErr)
		}
		return fmt.Errorf("subscription failed")
	}

	// Only set c.conn after successful auth and subscription
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	logger.Info("Connected to Home Assistant WebSocket")

	// Start message handler
	go c.handleMessages(conn)

	return nil
}

// handleMessages reads and processes messages from Home Assistant
func (c *HAClient) handleMessages(conn *websocket.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Warn("Failed to close WebSocket", "error", err)
		}
		// Only clear the shared connection if it is still ours - a manual Reconnect
		// may already have swapped in a newer one.
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket error", "error", err)
			}
			break
		}

		// Handle event messages
		if msgType, ok := msg["type"].(string); ok && msgType == "event" {
			c.handleEvent(msg)
		}
	}

	// The read loop only ends when the connection died. Home Assistant restarts
	// often enough - updates, add-on installs, a reboot - that without this
	// TapeDeck stays silently deaf to tag_scanned events until someone restarts it.
	select {
	case <-c.done:
		return
	default:
		go c.retryConnect(conn)
	}
}

// RetryConnect starts reconnect attempts in the background. Use it when the initial
// Connect fails at startup, so a Home Assistant that is merely slow to boot does not
// leave the WebSocket down until the next restart.
func (c *HAClient) RetryConnect() {
	go c.retryConnect(nil)
}

// retryConnect re-dials Home Assistant until it succeeds, the client is closed, or
// another connection takes over. lost is the connection being replaced, or nil when
// there was never one.
func (c *HAClient) retryConnect(lost *websocket.Conn) {
	delay := c.reconnectDelay
	if delay <= 0 {
		// Clients built as struct literals (tests) leave this zero; without a floor
		// the loop below would spin.
		delay = constants.HAWebSocketReconnectDelay
	}

	for attempt := 1; ; attempt++ {
		select {
		case <-c.done:
			return
		case <-time.After(delay):
		}

		// Take reconnectMu per attempt rather than for the whole loop, so a manual
		// Reconnect waits at most one dial instead of the entire outage.
		c.reconnectMu.Lock()
		c.mu.Lock()
		superseded := c.conn != nil && c.conn != lost
		c.mu.Unlock()
		if superseded {
			c.reconnectMu.Unlock()
			return
		}
		err := c.Connect()
		c.reconnectMu.Unlock()

		if err == nil {
			logger.Info("Reconnected to Home Assistant WebSocket", "attempts", attempt)
			return
		}

		logger.Warn("Home Assistant reconnect failed, retrying",
			"attempt", attempt,
			"retry_in", delay,
			"error", err)

		if delay < constants.HAWebSocketMaxReconnectDelay {
			delay *= 2
			if delay > constants.HAWebSocketMaxReconnectDelay {
				delay = constants.HAWebSocketMaxReconnectDelay
			}
		}
	}
}

// handleEvent processes event messages from Home Assistant
func (c *HAClient) handleEvent(msg map[string]interface{}) {
	event, ok := msg["event"].(map[string]interface{})
	if !ok {
		return
	}

	eventType, ok := event["event_type"].(string)
	if !ok || eventType != "tag_scanned" {
		return
	}

	data, ok := event["data"].(map[string]interface{})
	if !ok {
		return
	}

	tagID, ok := data["tag_id"].(string)
	if !ok {
		return
	}

	logger.Info("Tag scanned", "tag_id", tagID)

	// Call callback
	c.mu.Lock()
	callback := c.tagCallback
	c.mu.Unlock()

	if callback != nil {
		callback(tagID)
	}
}

// IsConnected returns true if the WebSocket connection is active
func (c *HAClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if connection exists and done channel is not closed
	if c.conn == nil {
		return false
	}

	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// Reconnect attempts to reconnect to Home Assistant with a new token
func (c *HAClient) Reconnect(_ context.Context, newToken string) error {
	// Ensure only one reconnect operation at a time to prevent:
	// - Multiple concurrent Connect() calls creating orphaned connections
	// - Multiple handleMessages() goroutines leaking resources
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	// Close existing connection if any
	c.mu.Lock()
	oldTokenPreview := "empty"
	if len(c.token) > 8 {
		oldTokenPreview = c.token[:8] + "..."
	} else if len(c.token) > 0 {
		oldTokenPreview = c.token + "..."
	}

	newTokenPreview := "empty"
	if len(newToken) > 8 {
		newTokenPreview = newToken[:8] + "..."
	} else if len(newToken) > 0 {
		newTokenPreview = newToken + "..."
	}

	logger.Info("Reconnecting to Home Assistant", "old_token", oldTokenPreview, "old_token_len", len(c.token), "new_token", newTokenPreview, "new_token_len", len(newToken), "same", c.token == newToken)

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			logger.Warn("Failed to close existing connection", "error", err)
		}
		c.conn = nil
	}
	// Update token
	c.token = newToken
	c.mu.Unlock()

	// Attempt new connection
	return c.Connect()
}

// Close closes the WebSocket connection
func (c *HAClient) Close() {
	select {
	case <-c.done:
		// Already closed
		return
	default:
		close(c.done)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		if err := c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(constants.HAWebSocketCloseTimeout)); err != nil {
			logger.Warn("Failed to write close message", "error", err)
		}
		if err := c.conn.Close(); err != nil {
			logger.Warn("Failed to close WebSocket", "error", err)
		}
		c.conn = nil
	}

	logger.Info("Closed Home Assistant WebSocket connection")
}
