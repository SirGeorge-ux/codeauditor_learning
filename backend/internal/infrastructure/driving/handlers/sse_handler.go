package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// SSEWriter implements the SSEStreamer port over an http.ResponseWriter.
// It is a per-request adapter: each SSE connection gets its own SSEWriter.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
}

// Verify SSEWriter satisfies the SSEStreamer interface.
var _ ports.SSEStreamer = (*SSEWriter)(nil)

// NewSSEWriter creates a new SSEWriter from a response writer and request.
func NewSSEWriter(w http.ResponseWriter, r *http.Request) *SSEWriter {
	flusher, _ := w.(http.Flusher)
	return &SSEWriter{w: w, flusher: flusher, ctx: r.Context()}
}

// StreamEvent sends a JSON event to the SSE client.
func (s *SSEWriter) StreamEvent(ctx context.Context, clientID string, eventType string, payload interface{}) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}

	data, _ := json.Marshal(payload)
	event := models.AuditEvent{
		Type:      eventType,
		Data:      string(data),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	eventData, _ := json.Marshal(event)
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", eventData)
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return err
}

// BroadcastLLMTokens streams a single LLM token delta to the SSE client.
func (s *SSEWriter) BroadcastLLMTokens(ctx context.Context, clientID string, tokenDelta string) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	default:
	}

	event := models.AuditEvent{
		Type:      "llm_token",
		Data:      tokenDelta,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	eventData, _ := json.Marshal(event)
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", eventData)
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return err
}

// RegisterClient is a no-op for SSEWriter (not used in audit flow).
func (s *SSEWriter) RegisterClient(ctx context.Context, clientID string) <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// UnregisterClient is a no-op for SSEWriter (not used in audit flow).
func (s *SSEWriter) UnregisterClient(clientID string) {}
