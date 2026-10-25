// Package constants defines configuration constants used throughout the application.
package constants

import "time"

const (
	// WebSocketSendBufferSize is the buffer size for WebSocket send channels.
	WebSocketSendBufferSize = 256
	// WebSocketPingInterval is the interval between WebSocket ping messages.
	WebSocketPingInterval = 54 * time.Second
	// WebSocketWriteTimeout is the deadline for WebSocket write operations.
	WebSocketWriteTimeout = 10 * time.Second
	// WebSocketReadTimeout is the deadline for WebSocket read operations.
	WebSocketReadTimeout = 60 * time.Second

	// HAWebSocketHandshakeTimeout is the timeout for Home Assistant WebSocket handshake.
	HAWebSocketHandshakeTimeout = 10 * time.Second
	// HAWebSocketReconnectDelay is the delay before reconnecting to Home Assistant.
	HAWebSocketReconnectDelay = 5 * time.Second

	// HAWebSocketMaxReconnectDelay caps the backoff between reconnect attempts, so
	// an overnight Home Assistant outage does not fill the log with retry warnings.
	HAWebSocketMaxReconnectDelay = 1 * time.Minute
	// HAWebSocketCloseTimeout is the timeout for sending close message to Home Assistant.
	HAWebSocketCloseTimeout = 1 * time.Second

	// AppleTVWakeTime is the wait time for Apple TV to wake up after power on.
	AppleTVWakeTime = 5 * time.Second

	// PlexDeepLinkTimeout is how long plex-infuse-fallback waits for Plex to start playing a
	// plex:// link before it sends the Infuse link. It starts after the wake wait, and every
	// tap pays it while Plex ignores deep links, so it's kept short. Raise it if Infuse takes
	// over from a Plex launch that would have succeeded.
	PlexDeepLinkTimeout = 5 * time.Second

	// SessionMaxAge is the maximum age of session cookies (10 years = effectively forever).
	// For self-hosted single-user apps, forcing re-authentication is unnecessary.
	SessionMaxAge = 10 * 365 * 24 * time.Hour

	// ServerReadHeaderTimeout is the timeout for reading HTTP request headers.
	ServerReadHeaderTimeout = 10 * time.Second
	// ServerShutdownTimeout is the timeout for graceful server shutdown.
	ServerShutdownTimeout = 10 * time.Second

	// PlexAuthTimeout is the HTTP client timeout for Plex authentication requests.
	PlexAuthTimeout = 10 * time.Second
	// PlexClientTimeout is the HTTP client timeout for Plex API requests.
	PlexClientTimeout = 30 * time.Second
	// HARestTimeout is the HTTP client timeout for Home Assistant REST API requests.
	HARestTimeout = 30 * time.Second

	// PlexAPITimeout is the context timeout for Plex API operations.
	PlexAPITimeout = 30 * time.Second
	// HAAPITimeout is the context timeout for Home Assistant API operations.
	HAAPITimeout = 30 * time.Second
)
