package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// TutorHandler serves the tutor chat endpoints at POST /api/v1/tutor/chat.
// PR 2 wires the SSE shape; PR 3 will extend the same handler with
// the public methods the in-shell panel calls (level, hint, etc.).
//
// The SSE wire format mirrors design.md §5:
//
//	event: <type>\ndata: <payload>\n\n
//
// where <type> is one of "status" | "token" | "done" | "error" and
// <payload> is a JSON string (object for status/error, raw text for
// token). The frontend's EventSource parses each frame and rebuilds
// the assistant message incrementally.
type TutorHandler struct {
	svc ports.TutorServicePort
}

// tutorChatRequest is the JSON body shape the frontend POSTs. The
// fields mirror ports.StreamChatRequest minus the UserID (which the
// handler stamps from the JWT context).
type tutorChatRequest struct {
	Message           string `json:"message"`
	Route             string `json:"route"`
	ChallengeID       string `json:"challengeId"`
	CodeSnippet       string `json:"codeSnippet"`
	Output            string `json:"output"`
	PreferredProvider string `json:"preferredProvider"`
}

// NewTutorHandler wires the SSE handler against a service that
// implements ports.TutorServicePort. Production wires this with
// services.TutorService; tests use a stub.
func NewTutorHandler(svc ports.TutorServicePort) *TutorHandler {
	if svc == nil {
		panic("tutor handler: TutorServicePort must not be nil")
	}
	return &TutorHandler{svc: svc}
}

// compile-time guard so the handler tracks the TutorService shape
// drift at PR review.
var _ ports.TutorServicePort = (*services.TutorService)(nil)

// StreamChat handles POST /api/v1/tutor/chat with the SSE wire format.
// The handler claims the response writer, sets SSE headers, opens an
// event-stream, and drains the TutorService.StreamChat channel onto
// the wire. Each event is flushed individually so the frontend sees
// incremental tokens, not buffered batches.
func (h *TutorHandler) StreamChat(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"message":"missing user id"}`, http.StatusBadRequest)
		return
	}

	// Decode the body. A malformed payload is a 400.
	var body tutorChatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, fmt.Sprintf(`{"message":"invalid body: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Establish the SSE envelope. The Content-Type is mandatory and
	// the cache/connection headers disable intermediate buffering so
	// the EventSource on the frontend doesn't see first-event lag.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	// Drive the service. A sync error here means we never get to
	// stream — wrap it in an SSE error frame so the frontend can
	// surface the message without a 500.
	evCh, err := h.svc.StreamChat(r.Context(), ports.StreamChatRequest{
		UserID:            userID,
		Message:           body.Message,
		Route:             body.Route,
		ChallengeID:       body.ChallengeID,
		Code:              body.CodeSnippet,
		Output:            body.Output,
		PreferredProvider: body.PreferredProvider,
	})
	if err != nil {
		writeSSEError(w, flusher, err)
		return
	}

	for ev := range evCh {
		if err := writeSSEEvent(w, flusher, ev); err != nil {
			// Client disconnected or transport died — stop draining.
			return
		}
	}
}

// writeSSEEvent serializes a single StreamChatEvent into the
// `event: ...\ndata: ...\n\n` wire shape and flushes so the next
// event is buffered-only on the next iteration.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, ev ports.StreamChatEvent) error {
	payload := serializePayload(ev)
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload)
	if flusher != nil {
		flusher.Flush()
	}
	if err != nil {
		return err
	}
	return nil
}

// writeSSEError emits a final `event: error\ndata: {"message":"..."}`
// frame and returns. The HTTP status was already 200 OK (the stream
// is open); the error is the terminal event.
func writeSSEError(w http.ResponseWriter, flusher http.Flusher, err error) {
	msg := err.Error()
	if errors.Is(err, services.ErrTutorCommandUnknown) {
		msg = "Comando no reconocido. Usá /help para listar los disponibles."
	}
	_, _ = fmt.Fprintf(w, "event: error\ndata: {\"message\":\"%s\"}\n\n", escapeSSE(msg))
	if flusher != nil {
		flusher.Flush()
	}
}

// serializePayload shapes the event body. For "status" we emit
// {"level":<int>,"status":"<text>"}; for "done" we emit {}; for
// "token" we emit {"text":"<chunk>"}; for "error" we emit
// {"message":"<err>"}. The frontend's TS contract is documented in
// PR 3's tutor-chat-panel.
func serializePayload(ev ports.StreamChatEvent) string {
	switch ev.Type {
	case "status":
		body := map[string]any{
			"status": ev.Content,
		}
		if ev.Level != nil {
			body["level"] = *ev.Level
		}
		out, _ := json.Marshal(body)
		return string(out)
	case "done":
		return "{}"
	case "token":
		out, _ := json.Marshal(map[string]any{"text": ev.Content})
		return string(out)
	case "error":
		out, _ := json.Marshal(map[string]any{"message": ev.Content})
		return string(out)
	}
	out, _ := json.Marshal(map[string]any{"content": ev.Content})
	return string(out)
}

// escapeSSE escapes the characters that would terminate an SSE
// frame: newline, carriage return, and backslash-quoted quotes.
// The frontend's EventSource is strict about these.
func escapeSSE(s string) string {
	out := ""
	for _, r := range s {
		switch r {
		case '\n':
			out += "\\n"
		case '\r':
			out += "\\r"
		case '"':
			out += "\\\""
		default:
			out += string(r)
		}
	}
	return out
}
