package ports

import (
	"context"
	"encoding/json"
)

// SSEStreamer defines the contract for streaming Server-Sent Events to clients.
// It is a driven (secondary) port — the application use cases call it to push
// real-time updates (LLM tokens, audit progress, etc.) to the frontend.
type SSEStreamer interface {
	// StreamEvent sends a JSON-serializable event to the client identified by clientID.
	// It is safe to call concurrently from multiple goroutines.
	StreamEvent(ctx context.Context, clientID string, eventType string, payload interface{}) error

	// BroadcastLLMTokens streams LLM token deltas to a client.
	// Each call sends one token; the client accumulates them as a complete response.
	BroadcastLLMTokens(ctx context.Context, clientID string, tokenDelta string) error

	// RegisterClient adds a new SSE connection. Returns a channel that closes on disconnect.
	RegisterClient(ctx context.Context, clientID string) <-chan struct{}

	// UnregisterClient removes a client and cleans up resources.
	UnregisterClient(clientID string)
}

// SSEClientMessage represents a message received from an SSE client.
type SSEClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}