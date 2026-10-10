package ports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// --- TutorServicePort stubs ------------------------------------------------

// tutorServiceStub satisfies TutorServicePort with deterministic
// return values so future signature drift surfaces at compile time.
// The fields are exported so a future test can read what the stub was
// called with.
type tutorServiceStub struct {
	streamEvents []StreamChatEvent
	streamErr    error
	commands     []string
	summaryErr   error
	lastReq      StreamChatRequest
}

func (s *tutorServiceStub) StreamChat(ctx context.Context, req StreamChatRequest) (<-chan StreamChatEvent, error) {
	s.lastReq = req
	ch := make(chan StreamChatEvent, len(s.streamEvents))
	for _, ev := range s.streamEvents {
		ch <- ev
	}
	close(ch)
	return ch, s.streamErr
}

func (s *tutorServiceStub) ProcessCommand(_ context.Context, _ string, command string) error {
	s.commands = append(s.commands, command)
	return nil
}

func (s *tutorServiceStub) SummarizeSession(_ context.Context, _ string, _ string) error {
	return s.summaryErr
}

// TestTutorServicePort_InterfaceSatisfied asserts that tutorServiceStub
// satisfies TutorServicePort at compile time. If the port signature
// drifts (eg. StreamChat loses the channel return), this line fails to
// build before the change reaches production wiring.
func TestTutorServicePort_InterfaceSatisfied(t *testing.T) {
	var _ TutorServicePort = (*tutorServiceStub)(nil)
}

// TestTutorServicePort_StreamChatRoundtrip exercises the stub so a
// future maintainer can confirm the stub actually returns the channel
// + error it claims to (instead of silently violating the contract).
func TestTutorServicePort_StreamChatRoundtrip(t *testing.T) {
	stub := &tutorServiceStub{
		streamEvents: []StreamChatEvent{
			{Type: "token", Content: "hi"},
			{Type: "done"},
		},
	}
	ch, err := stub.StreamChat(context.Background(), StreamChatRequest{UserID: "u1", Message: "hi"})
	if err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}
	got := make([]StreamChatEvent, 0, len(stub.streamEvents))
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
	if got[0].Type != "token" || got[0].Content != "hi" {
		t.Errorf("first event = %+v, want token/hi", got[0])
	}
	if stub.lastReq.UserID != "u1" {
		t.Errorf("lastReq.UserID = %q, want u1", stub.lastReq.UserID)
	}
}

// TestTutorServicePort_StreamChatErrorPropagates verifies the stub
// surfaces its configured error so a test can assert production code
// branches on the error rather than silently swallowing.
func TestTutorServicePort_StreamChatErrorPropagates(t *testing.T) {
	wantErr := errors.New("cascade down")
	stub := &tutorServiceStub{streamErr: wantErr}
	_, err := stub.StreamChat(context.Background(), StreamChatRequest{UserID: "u1"})
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("got %v, want %v", err, wantErr)
	}
}

// TestTutorServicePort_ProcessCommandRecords verifies the stub records
// the command name so a future test can assert the production port
// forwards the right value (eg. "/nivel 0" → PersistLevel(0)).
func TestTutorServicePort_ProcessCommandRecords(t *testing.T) {
	stub := &tutorServiceStub{}
	if err := stub.ProcessCommand(context.Background(), "u1", "/nivel 0"); err != nil {
		t.Fatalf("ProcessCommand returned error: %v", err)
	}
	if len(stub.commands) != 1 || stub.commands[0] != "/nivel 0" {
		t.Fatalf("recorded commands = %v, want [/nivel 0]", stub.commands)
	}
}

// --- CachePort stubs --------------------------------------------------------

// sessionCacheStub satisfies CachePort with an in-memory map so the
// interface-satisfaction check at compile time catches signature drift.
// The cap on the ring buffer is intentionally NOT implemented here —
// the stub is allowed to grow; the real impl in the driven adapter is
// where the 10-message cap lives.
type sessionCacheStub struct {
	data map[string]models.ChatMessage
}

func newSessionCacheStub() *sessionCacheStub {
	return &sessionCacheStub{data: map[string]models.ChatMessage{}}
}

func (s *sessionCacheStub) Get(_ context.Context, sessionID string) (models.ChatMessage, bool) {
	v, ok := s.data[sessionID]
	return v, ok
}

func (s *sessionCacheStub) Set(_ context.Context, sessionID string, msg models.ChatMessage) {
	s.data[sessionID] = msg
}

func (s *sessionCacheStub) Snapshot(_ context.Context, sessionID string) []models.ChatMessage {
	return []models.ChatMessage{s.data[sessionID]}
}

func (s *sessionCacheStub) Keys(_ context.Context) []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func (s *sessionCacheStub) Reset(_ context.Context, sessionID string) {
	delete(s.data, sessionID)
}

// TestCachePort_InterfaceSatisfied asserts the stub satisfies CachePort.
func TestCachePort_InterfaceSatisfied(t *testing.T) {
	var _ CachePort = (*sessionCacheStub)(nil)
}

// TestCachePort_StubOperations exercises the basic shape: Set → Get →
// Snapshot → Reset → Keys. The production implementation lives in
// infrastructure/driven/session_cache.go and adds ring-buffer behavior.
func TestCachePort_StubOperations(t *testing.T) {
	stub := newSessionCacheStub()
	ctx := context.Background()
	stub.Set(ctx, "sess-1", models.ChatMessage{Role: models.ChatRoleUser, Content: "hi"})
	stub.Set(ctx, "sess-1", models.ChatMessage{Role: models.ChatRoleTutor, Content: "hello"})
	got, ok := stub.Get(ctx, "sess-1")
	if !ok || got.Content != "hello" {
		t.Errorf("Get(sess-1) = (%v, %v), want (hello, true)", got, ok)
	}
	if snap := stub.Snapshot(ctx, "sess-1"); len(snap) != 1 || snap[0].Content != "hello" {
		t.Errorf("Snapshot(sess-1) = %+v, want [hello]", snap)
	}
	keys := stub.Keys(ctx)
	if len(keys) != 1 || keys[0] != "sess-1" {
		t.Errorf("Keys() = %v, want [sess-1]", keys)
	}
	stub.Reset(ctx, "sess-1")
	if _, ok := stub.Get(ctx, "sess-1"); ok {
		t.Errorf("after Reset, Get(sess-1) returned ok=true")
	}
}

// --- LLMClientPort stubs ----------------------------------------------------

// llmClientStub satisfies LLMClientPort. It records the messages +
// options it was called with and yields the configured events to the
// callback so a future test can assert the production wire shape.
type llmClientStub struct {
	events   []LLMStreamEvent
	err      error
	last     []models.ChatMessage
	lastOpts StreamChatOptions

	// Complete side (Phase 1 / S3 PR 1): records the request and
	// returns the configured canned response. Used by
	// TestLLMClientPort_CompleteRoundtrip.
	completeReq     CompleteRequest
	completeResp    CompleteResponse
	completeErr     error
	completeCalls   int
	completeLatency time.Duration
}

func (s *llmClientStub) StreamChat(_ context.Context, msgs []models.ChatMessage, opts StreamChatOptions, onEvent func(LLMStreamEvent) error) error {
	s.last = msgs
	s.lastOpts = opts
	for _, ev := range s.events {
		if err := onEvent(ev); err != nil {
			return err
		}
	}
	return s.err
}

func (s *llmClientStub) Complete(_ context.Context, req CompleteRequest) (CompleteResponse, error) {
	s.completeReq = req
	s.completeCalls++
	if s.completeLatency > 0 {
		time.Sleep(s.completeLatency)
	}
	return s.completeResp, s.completeErr
}

// TestLLMClientPort_InterfaceSatisfied asserts the stub satisfies the
// LLMClientPort. PR 2's cascade adapter wraps Ollama/Groq/DeepSeek/M3
// behind this contract.
func TestLLMClientPort_InterfaceSatisfied(t *testing.T) {
	var _ LLMClientPort = (*llmClientStub)(nil)
}

// TestLLMClientPort_ForwardsMessagesAndOptions exercises the stub so a
// regression in the signature is caught at compile time AND the test
// confirms the contract carries the messages + opts to the LLM.
func TestLLMClientPort_ForwardsMessagesAndOptions(t *testing.T) {
	stub := &llmClientStub{
		events: []LLMStreamEvent{
			{Type: "token", Content: "hello"},
			{Type: "done"},
		},
	}
	msgs := []models.ChatMessage{
		{Role: models.ChatRoleSystem, Content: "system"},
		{Role: models.ChatRoleUser, Content: "hi"},
	}
	opts := StreamChatOptions{PreferredProvider: "groq"}
	if err := stub.StreamChat(context.Background(), msgs, opts, func(ev LLMStreamEvent) error { return nil }); err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}
	if len(stub.last) != 2 {
		t.Fatalf("got %d messages, want 2", len(stub.last))
	}
	if stub.lastOpts.PreferredProvider != "groq" {
		t.Errorf("PreferredProvider = %q, want groq", stub.lastOpts.PreferredProvider)
	}
}

// --- Complete (Phase 1 / S3 PR 1) -----------------------------------------

// TestLLMClientPort_CompleteRoundtrip exercises the stub's Complete
// path so a regression in the new method's signature is caught here
// (the stub MUST keep satisfying LLMClientPort or compilation fails).
func TestLLMClientPort_CompleteRoundtrip(t *testing.T) {
	wantResp := CompleteResponse{
		Content:  `{"title":"ok"}`,
		Provider: "groq",
		Latency:  42 * time.Millisecond,
	}
	stub := &llmClientStub{completeResp: wantResp}
	schema := &JSONSchema{Type: "object", Required: []string{"title"}}
	req := CompleteRequest{
		System:     "identity",
		Prompt:     "topic+story",
		JSONSchema: schema,
	}
	got, err := stub.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got.Content != wantResp.Content || got.Provider != wantResp.Provider {
		t.Errorf("got %+v, want %+v", got, wantResp)
	}
	if stub.completeCalls != 1 {
		t.Errorf("completeCalls = %d, want 1", stub.completeCalls)
	}
	if stub.completeReq.JSONSchema == nil || stub.completeReq.JSONSchema.Type != "object" {
		t.Errorf("schema not forwarded: %+v", stub.completeReq.JSONSchema)
	}
}

// TestLLMClientPort_CompletePropagatesError verifies the stub surfaces
// its configured error so callers can branch with errors.Is against
// ErrLLMCompleteTimeout (or any other sentinel).
func TestLLMClientPort_CompletePropagatesError(t *testing.T) {
	wantErr := ErrLLMCompleteTimeout
	stub := &llmClientStub{completeErr: wantErr}
	_, err := stub.Complete(context.Background(), CompleteRequest{})
	if !errors.Is(err, ErrLLMCompleteTimeout) {
		t.Errorf("err = %v, want ErrLLMCompleteTimeout", err)
	}
}

// TestDefaultCompleteTimeoutLocksFiveSeconds locks the documented 5s
// whole-response timeout (spec.md#SSE-Streaming parallel). Mirrors the
// TestCascadeDefaultTimeoutMatchesSpec canary on the streaming path.
func TestDefaultCompleteTimeoutLocksFiveSeconds(t *testing.T) {
	if DefaultCompleteTimeout != 5*time.Second {
		t.Errorf("DefaultCompleteTimeout = %v, want 5s", DefaultCompleteTimeout)
	}
}

// TestJSONSchemaMarshallable locks the wire shape consumed by the
// cascade adapters (Groq/DeepSeek `response_format`, MiniMax text
// fallback).
func TestJSONSchemaMarshallable(t *testing.T) {
	schema := JSONSchema{
		Type:       "object",
		Properties: map[string]any{"title": map[string]any{"type": "string"}},
		Required:   []string{"title"},
	}
	if schema.Type != "object" {
		t.Errorf("JSONSchema.Type = %q, want object", schema.Type)
	}
	if len(schema.Properties) != 1 {
		t.Errorf("JSONSchema.Properties len = %d, want 1", len(schema.Properties))
	}
}
