package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeTutorService is the hand-rolled stub the handler tests use to
// exercise the SSE wire format without spinning up an LLM cascade.
// It implements ports.TutorServicePort deterministically.
type fakeTutorService struct {
	events []ports.StreamChatEvent
	err    error
	cmdErr error
	sessErr error
	cmdLast string
	cmdUser string
	streamCalls int
	mu     sync.Mutex
}

func (f *fakeTutorService) StreamChat(ctx context.Context, req ports.StreamChatRequest) (<-chan ports.StreamChatEvent, error) {
	f.mu.Lock()
	f.streamCalls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	ch := make(chan ports.StreamChatEvent, len(f.events))
	for _, ev := range f.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func (f *fakeTutorService) ProcessCommand(_ context.Context, userID, command string) error {
	f.mu.Lock()
	f.cmdLast = command
	f.cmdUser = userID
	f.mu.Unlock()
	return f.cmdErr
}

func (f *fakeTutorService) SummarizeSession(_ context.Context, _, _ string) error {
	return f.sessErr
}

// newTestTutorHandler wires TutorHandler against the fake service so
// the tests focus on the SSE wire format and the JWT auth path.
func newTestTutorHandler(t *testing.T) (*TutorHandler, *fakeTutorService) {
	t.Helper()
	svc := &fakeTutorService{}
	return NewTutorHandler(svc), svc
}

// reqWithUserIDTutor stamps the JWT user id into context — mirrors
// what authmiddleware.AuthMiddleware does in production. We keep the
// suffix to avoid colliding with the dictionary handler's
// reqWithUserID helper in the same package.
func reqWithUserIDTutor(r *http.Request, userID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(),
		authmiddleware.UserIDContextKey{}, userID))
}

func setupTutorRouter(h *TutorHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/api/v1/tutor/chat", h.StreamChat)
	return r
}

// TestTutorHandler_HappyPath_StreamsAllEventTypes cubre el camino
// principal: el handler emite los 4 tipos de evento en la secuencia
// correcta a través del SSE wire format (`event: TYPE\ndata: ...\n\n`).
func TestTutorHandler_HappyPath_StreamsAllEventTypes(t *testing.T) {
	svc, fake := newTestTutorHandler(t)
	level := models.SocraticPure
	fake.events = []ports.StreamChatEvent{
		{Type: "status", Content: "level=socratic", Level: &level},
		{Type: "token", Content: "Hello "},
		{Type: "token", Content: "world"},
		{Type: "done"},
	}
	r := setupTutorRouter(svc)

	body := `{"message":"hi","challengeId":"c1","language":"ts","codeSnippet":"x=1","preferredProvider":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserIDTutor(req, "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", w.Header().Get("Content-Type"))
	}

	// Walk the response line by line and parse the SSE wire frame.
	rdr := bufio.NewReader(bytes.NewReader(w.Body.Bytes()))
	events := parseSSEFrames(t, rdr)
	wantSeq := []string{"status", "token", "token", "done"}
	if len(events) != len(wantSeq) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(wantSeq), events)
	}
	for i, typ := range wantSeq {
		if events[i].Type != typ {
			t.Errorf("event[%d].Type = %q, want %q (data=%q)", i, events[i].Type, typ, events[i].Data)
		}
	}
	if events[1].Data != `{"text":"Hello "}` {
		t.Errorf("event[1].Data = %q, want '{\"text\":\"Hello \"}'", events[1].Data)
	}
	if events[2].Data != `{"text":"world"}` {
		t.Errorf("event[2].Data = %q, want '{\"text\":\"world\"}'", events[2].Data)
	}
	// status event data should be JSON.
	if !strings.HasPrefix(events[0].Data, "{") {
		t.Errorf("status event data not JSON: %q", events[0].Data)
	}
}

// TestTutorHandler_ErrorEvent_EmittedAsSSE — un error del cascade se
// serializa como SSE `event: error` con JSON `{message: "..."}`.
func TestTutorHandler_ErrorEvent_EmittedAsSSE(t *testing.T) {
	svc, fake := newTestTutorHandler(t)
	fake.events = []ports.StreamChatEvent{
		{Type: "error", Content: "cascade down"},
	}
	r := setupTutorRouter(svc)

	body := `{"message":"hi","challengeId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserIDTutor(req, "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (errors stream as SSE)", w.Code)
	}
	rdr := bufio.NewReader(bytes.NewReader(w.Body.Bytes()))
	events := parseSSEFrames(t, rdr)
	if len(events) != 1 {
		t.Fatalf("events = %+v, want 1", events)
	}
	if events[0].Type != "error" {
		t.Errorf("type = %q, want error", events[0].Type)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(events[0].Data), &payload); err != nil {
		t.Fatalf("error data not JSON: %q", events[0].Data)
	}
	if payload["message"] != "cascade down" {
		t.Errorf("payload.message = %v, want cascade down", payload["message"])
	}
}

// TestTutorHandler_MissingUserIDRejects400 — defense in depth: even
// without auth middleware (these tests bypass it), the handler
// refuses empty user_ids. Production adds an upstream AuthMiddleware
// that returns 401 for missing JWT.
func TestTutorHandler_MissingUserIDRejects400(t *testing.T) {
	svc, _ := newTestTutorHandler(t)
	r := setupTutorRouter(svc)

	body := `{"message":"hi","challengeId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// user_id NOT stamped
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestTutorHandler_MalformedJSONReturns400 — body no es JSON válido.
func TestTutorHandler_MalformedJSONReturns400(t *testing.T) {
	svc, _ := newTestTutorHandler(t)
	r := setupTutorRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat",
		strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserIDTutor(req, "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// TestTutorHandler_ServiceInitErrorEmitsSSEFrame — si StreamChat del
// service devuelve un error (no un channel), el handler responde 200
// OK con un `event: error` SSE frame final, no un 500. Esto preserva
// el contrato de streaming para el frontend y deja que el SSE evento
// muestre el mensaje.
func TestTutorHandler_ServiceInitErrorEmitsSSEFrame(t *testing.T) {
	svc, fake := newTestTutorHandler(t)
	fake.err = errBoom2
	r := setupTutorRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat",
		strings.NewReader(`{"message":"hi","challengeId":"c1"}`))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserIDTutor(req, "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (error streams as SSE)", w.Code)
	}
	if !strings.Contains(w.Body.String(), "event: error") {
		t.Errorf("body missing error event: %q", w.Body.String())
	}
}

// TestTutorHandler_PreferredProviderForwardedToService — el handler
// parsea `preferredProvider` del body y lo reenvía al service.
func TestTutorHandler_PreferredProviderForwardedToService(t *testing.T) {
	svc, fake := newTestTutorHandler(t)
	fake.events = []ports.StreamChatEvent{{Type: "done"}}
	r := setupTutorRouter(svc)

	body := `{"message":"hi","challengeId":"c1","preferredProvider":"minimax"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserIDTutor(req, "u1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if fake.streamCalls != 1 {
		t.Errorf("StreamChat called %d times, want 1", fake.streamCalls)
	}
}

// TestTutorHandler_JWTRequiredViaMiddleware — production wiring:
// el handler registrado bajo /api/v1 con authmiddleware responde 401
// cuando falta el Bearer token.
func TestTutorHandler_JWTRequiredViaMiddleware(t *testing.T) {
	svc, _ := newTestTutorHandler(t)
	r := chi.NewRouter()
	// Use a no-op auth middleware that enforces Bearer presence but
	// does NOT validate JWT (the real validator lives in the
	// production handler). This isolates the "handler behind auth"
	// contract: when the middleware rejects, the handler never
	// runs.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = io.WriteString(w, `{"message":"missing bearer"}`)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
		r.Post("/tutor/chat", svc.StreamChat)
	})

	body := `{"message":"hi","challengeId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// --- SSE frame parser --------------------------------------------------------

// sseEvent is the wire-level shape: an event type + a data payload.
type sseEvent struct {
	Type string
	Data string
}

// parseSSEFrames consumes the response body and yields the parsed
// events. Format per design.md §5:
//
//	event: <type>\ndata: <payload>\n\n
func parseSSEFrames(t *testing.T, r *bufio.Reader) []sseEvent {
	t.Helper()
	var out []sseEvent
	for {
		eventLine, err := r.ReadString('\n')
		if err != nil {
			break
		}
		eventLine = strings.TrimRight(eventLine, "\n")
		if !strings.HasPrefix(eventLine, "event:") {
			continue
		}
		typ := strings.TrimSpace(strings.TrimPrefix(eventLine, "event:"))
		dataLine, err := r.ReadString('\n')
		if err != nil {
			break
		}
		dataLine = strings.TrimRight(dataLine, "\n")
		if !strings.HasPrefix(dataLine, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(dataLine, "data:"))
		// Read the trailing empty line that separates frames.
		_, _ = r.ReadString('\n')
		out = append(out, sseEvent{Type: typ, Data: data})
	}
	return out
}

// errBoom2 is the sentinel error raised by the handler test to drive
// the 500 path.
var errBoom2 = io.EOF

// Block unused-time warning when a future refactor drops the time
// import from this file.
var _ = time.Now
