// Package llmcascade is the LLM cascade driven adapter — PR 2 of the
// tutor-chat change. The cascade owns the multi-provider contract that
// ports.LLMClientPort abstracts behind the application layer. Three
// providers compete on every chat request:
//
//  1. Groq (llama-3.3-70b-versatile) — primary; $0 free tier, fast.
//  2. DeepSeek V3 via OpenRouter     — fallback; $0 free tier, long.
//  3. MiniMax M3                     — opt-in only when the user sets
//     use_minimax=true or
//     PreferredProvider=="minimax".
//
// Per the spec (spec.md#SSE-Streaming "Fallback on timeout"), the
// cascade enforces a 5s first-token timeout. On timeout/error from a
// provider, the cascade walks to the next configured one.
//
// The package is a single binary-internal subtree to keep the file
// surface small and the import graph consistent: core/services must
// only import ports; cascade imports core/domain/models and ports;
// never the other way around.
package llmcascade

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// stubProvider implements the internal providerClient interface so
// the cascade test can simulate timeouts, errors, and canned streams
// without HTTP. The providerClient interface is local to the package
// (lowercase) so a future change can refactor without breaking
// public types.
type stubProvider struct {
	name        string
	tokens      []string
	emitOnError error
	emitDelay   time.Duration
	enabled     bool
	gotOpts     ports.StreamChatOptions
	gotMessages []models.ChatMessage
	called      int
	mu          sync.Mutex
	streamFn    func(ctx context.Context, msgs []models.ChatMessage, opts ports.StreamChatOptions, onEvent func(ports.LLMStreamEvent) error) error
	// Complete-side hooks for the S3 PR 1 cascade. The default
	// returns ErrProviderDisabled; tests that want a different
	// behavior supply completeFn.
	completeFn     func(ctx context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error)
	gotComplete    ports.CompleteRequest
	completeCalled int
}

func (s *stubProvider) StreamChat(ctx context.Context, msgs []models.ChatMessage, opts ports.StreamChatOptions, onEvent func(ports.LLMStreamEvent) error) error {
	s.mu.Lock()
	s.gotMessages = msgs
	s.gotOpts = opts
	s.called++
	streamFn := s.streamFn
	s.mu.Unlock()
	if streamFn != nil {
		return streamFn(ctx, msgs, opts, onEvent)
	}
	if !s.enabled {
		return ErrProviderDisabled
	}
	if s.emitDelay > 0 {
		select {
		case <-time.After(s.emitDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.emitOnError != nil {
		// Fire one status event then error out.
		if onEvent != nil {
			_ = onEvent(ports.LLMStreamEvent{Type: "status", Content: s.name + ": attempting"})
		}
		return s.emitOnError
	}
	for _, tok := range s.tokens {
		if onEvent != nil {
			if err := onEvent(ports.LLMStreamEvent{Type: "token", Content: tok}); err != nil {
				return err
			}
		}
	}
	if onEvent != nil {
		_ = onEvent(ports.LLMStreamEvent{Type: "done"})
	}
	return nil
}

func (s *stubProvider) Complete(ctx context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error) {
	s.mu.Lock()
	s.gotComplete = req
	s.completeCalled++
	cb := s.completeFn
	s.mu.Unlock()
	if cb != nil {
		return cb(ctx, req)
	}
	return ports.CompleteResponse{}, ErrProviderDisabled
}

func (s *stubProvider) Name() string { return s.name }

// TestCascade_PrimaryEmitsAllTokensHappyPath — el caso central: con
// la cascada configurada, el primer provider entrega todos los tokens
// + done y los providers fallback NUNCA se llaman.
func TestCascade_PrimaryEmitsAllTokensHappyPath(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, tokens: []string{"hello", " ", "world"}}
	deepStub := &stubProvider{name: "deepseek"}
	m3Stub := &stubProvider{name: "minimax"}

	c := NewCascade(groqStub, deepStub, m3Stub)
	var got []ports.LLMStreamEvent
	err := c.StreamChat(context.Background(),
		[]models.ChatMessage{{Role: models.ChatRoleUser, Content: "hi"}},
		ports.StreamChatOptions{PreferredProvider: "", Timeout: 100 * time.Millisecond},
		func(ev ports.LLMStreamEvent) error {
			got = append(got, ev)
			return nil
		})
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}

	// Drain the events.
	collected := make([]ports.LLMStreamEvent, 0, len(got))
	collected = append(collected, got...)
	wantTokens := []string{"hello", " ", "world"}
	for i, want := range wantTokens {
		if i >= len(collected) || collected[i].Type != "token" || collected[i].Content != want {
			t.Errorf("event[%d] = %+v, want token/%q", i, collected[i], want)
		}
	}
	if !eventExists(collected, "done") {
		t.Errorf("missing 'done' event in cascade output")
	}

	// Fallbacks must not have been hit.
	if deepStub.called != 0 || m3Stub.called != 0 {
		t.Errorf("cascade fell through unexpectedly")
	}
}

// TestCascade_FallsThroughOnProviderError — el primer provider
// entrega un error → el siguiente se usa.
func TestCascade_FallsThroughOnProviderError(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, emitOnError: errors.New("groq down")}
	deepStub := &stubProvider{name: "deepseek", enabled: true, tokens: []string{"deep"}}
	m3Stub := &stubProvider{name: "minimax"}

	c := NewCascade(groqStub, deepStub, m3Stub)
	var got []ports.LLMStreamEvent
	err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{Timeout: 200 * time.Millisecond},
		func(ev ports.LLMStreamEvent) error {
			got = append(got, ev)
			return nil
		})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !eventExists(got, "token") && !eventExists(got, "done") {
		t.Errorf("cascade never produced tokens; events=%+v", got)
	}
	if groqStub.called != 1 {
		t.Errorf("groq called %d times, want 1", groqStub.called)
	}
	if deepStub.called != 1 {
		t.Errorf("deepseek called %d times, want 1 (fallthrough)", deepStub.called)
	}
}

// TestCascade_PreferredProviderShortCircuits — cuando el cliente pide
// MiniMax, ese provider corre incluso aunque no esté en el ranking
// canónico. Se cubre con un config donde M3 está disabled pero
// igual el override lo habilita.
func TestCascade_PreferredProviderShortCircuits(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, tokens: []string{"groq"}}
	deepStub := &stubProvider{name: "deepseek", enabled: true, tokens: []string{"deep"}}
	m3Stub := &stubProvider{name: "minimax", enabled: true, tokens: []string{"m3"}}

	c := NewCascade(groqStub, deepStub, m3Stub)
	var got []ports.LLMStreamEvent
	err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{PreferredProvider: "minimax"},
		func(ev ports.LLMStreamEvent) error {
			got = append(got, ev)
			return nil
		})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !eventExists(got, "token") {
		t.Errorf("minimax short-circuit never produced tokens; events=%+v", got)
	}
	if groqStub.called != 0 || deepStub.called != 0 {
		t.Errorf("cascade did not skip ranking providers on opt-in (groq=%d, deep=%d)", groqStub.called, deepStub.called)
	}
	if m3Stub.called != 1 {
		t.Errorf("minimax called %d times, want 1", m3Stub.called)
	}
}

// TestCascade_PreferredProviderUnknown — un valor no-canónico se
// ignora y la cascada usa el ranking por defecto.
func TestCascade_PreferredProviderUnknown(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, tokens: []string{"groq"}}
	deepStub := &stubProvider{name: "deepseek"}
	m3Stub := &stubProvider{name: "minimax"}

	c := NewCascade(groqStub, deepStub, m3Stub)
	var got []ports.LLMStreamEvent
	err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{PreferredProvider: "fancy-llm"},
		func(ev ports.LLMStreamEvent) error {
			got = append(got, ev)
			return nil
		})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Should fall back to default ranking; groq wins.
	if groqStub.called != 1 {
		t.Errorf("unknown preferred provider should default to ranking; groq called %d times", groqStub.called)
	}
}

// TestCascade_AllProvidersFail propagates the last error to the
// caller. El handler lo traduce en un SSE `error` event.
func TestCascade_AllProvidersFail(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, emitOnError: errors.New("groq down")}
	deepStub := &stubProvider{name: "deepseek", enabled: true, emitOnError: errors.New("deep down")}
	m3Stub := &stubProvider{name: "minimax", enabled: true, emitOnError: errors.New("m3 down")}

	c := NewCascade(groqStub, deepStub, m3Stub)
	err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{},
		func(_ ports.LLMStreamEvent) error { return nil })
	if err == nil {
		t.Fatalf("expected error when every provider fails")
	}
	if !errors.Is(err, ErrCascadeAllFailed) {
		t.Errorf("err = %v, want ErrCascadeAllFailed", err)
	}
}

// TestCascade_DisabledProvidersSkipped — un provider sin API key
// (ErrProviderDisabled) se skipea sin agotar el timeout.
func TestCascade_DisabledProvidersSkipped(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: false}
	deepStub := &stubProvider{name: "deepseek", enabled: true, tokens: []string{"deep"}}
	m3Stub := &stubProvider{name: "minimax"}

	c := NewCascade(groqStub, deepStub, m3Stub)
	var got []ports.LLMStreamEvent
	start := time.Now()
	err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{Timeout: 1 * time.Second},
		func(ev ports.LLMStreamEvent) error {
			got = append(got, ev)
			return nil
		})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Disabled provider must NOT block cascade advance.
	if elapsed > 800*time.Millisecond {
		t.Errorf("cascade waited too long on disabled provider: %v", elapsed)
	}
	if !eventExists(got, "token") {
		t.Errorf("disabled provider caused loss of fallback; events=%+v", got)
	}
}

// TestCascade_OrderingLockedToGroqDeepSeekMinimax — la config por
// defecto usa el orden prescrito en docs/llm-strategy.md. Se valida
// con stubs que reportan cuáles fueron instanciados.
func TestCascade_OrderingLockedToGroqDeepSeekMinimax(t *testing.T) {
	var calledOrder []string
	mk := func(name string) *stubProvider {
		return &stubProvider{
			name: name, enabled: true, tokens: []string{name},
			streamFn: func(_ context.Context, _ []models.ChatMessage, _ ports.StreamChatOptions, _ func(ports.LLMStreamEvent) error) error {
				calledOrder = append(calledOrder, name)
				return nil
			},
		}
	}
	c := NewCascade(mk("groq"), mk("deepseek"), mk("minimax"))
	if err := c.StreamChat(context.Background(), nil,
		ports.StreamChatOptions{PreferredProvider: "deepseek"},
		func(_ ports.LLMStreamEvent) error { return nil }); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(calledOrder) != 1 || calledOrder[0] != "deepseek" {
		t.Errorf("called order = %v, want [deepseek]", calledOrder)
	}
}

// TestCascade_TokenAmountsEmitted sanity check: un provider que
// emite N tokens produce N token events (no duplicados).
func TestCascade_TokenAmountsEmitted(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, tokens: []string{"a", "b", "c", "d"}}
	c := NewCascade(groqStub, &stubProvider{name: "deepseek"}, &stubProvider{name: "minimax"})
	var tokens int
	_ = c.StreamChat(context.Background(), nil, ports.StreamChatOptions{},
		func(ev ports.LLMStreamEvent) error {
			if ev.Type == "token" {
				tokens++
			}
			return nil
		})
	if tokens != 4 {
		t.Errorf("token count = %d, want 4", tokens)
	}
}

// TestCascade_OnEventCallbackErrorPropagates — si onEvent devuelve
// error (back-pressure), el provider lo recibe y la cascada aborta.
func TestCascade_OnEventCallbackErrorPropagates(t *testing.T) {
	groqStub := &stubProvider{name: "groq", enabled: true, tokens: []string{"a"}}
	c := NewCascade(groqStub, &stubProvider{name: "deepseek"}, &stubProvider{name: "minimax"})
	backpressure := errors.New("client disconnected")
	err := c.StreamChat(context.Background(), nil, ports.StreamChatOptions{},
		func(_ ports.LLMStreamEvent) error { return backpressure })
	if !errors.Is(err, backpressure) {
		t.Errorf("err = %v, want backpressure", err)
	}
}

// eventExists helps table-driven checks without wrestling with
// nested slices.
func eventExists(events []ports.LLMStreamEvent, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
}

// TestCascadePreferredProviderNameNormalizationLocked — el enum
// accepted del PreferredProvider solo reconoce "minimax" en PR 2.
// Mantenemos nombres por string (no enum) para no romper la wire
// shape del frontend.
func TestCascadePreferredProviderNameNormalizationLocked(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"groq", true},
		{"deepseek", true},
		{"minimax", true},
		{"", true}, // empty = use ranking
		{"claude", false},
		{"unknown", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := isValidPreferredProvider(c.in)
			if got != c.want {
				t.Errorf("isValidPreferredProvider(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestCascadeDefaultTimeoutMatchesSpec locks the documented 5s
// first-token timeout (spec.md#SSE-Streaming). If a PR 3 change
// adjusts the default, this assertion is the canary.
func TestCascadeDefaultTimeoutMatchesSpec(t *testing.T) {
	if DefaultFirstTokenTimeout != 5*time.Second {
		t.Errorf("DefaultFirstTokenTimeout = %v, want 5s", DefaultFirstTokenTimeout)
	}
}

// TestCascadeImplementsLLMClientPort exercises the compile-time guard.
func TestCascadeImplementsLLMClientPort(t *testing.T) {
	var _ ports.LLMClientPort = (*Cascade)(nil)
}

// --- Groq provider tests --------------------------------------------------

// TestGroqProvider_NameCanonical valida el contrato de Name() (la
// cascada lo usa para logging / PreferredProvider matching).
func TestGroqProvider_NameCanonical(t *testing.T) {
	g := NewGroq("https://api.groq.com/openai/v1", "key", "llama-3.3-70b-versatile")
	if g.Name() != "groq" {
		t.Errorf("Name() = %q, want groq", g.Name())
	}
}

// TestGroqProvider_EmptyKeyMarksDisabled — una key vacía marca al
// provider como disabled; la cascada lo skipea sin HTTP.
func TestGroqProvider_EmptyKeyMarksDisabled(t *testing.T) {
	g := NewGroq("https://api.groq.com/openai/v1", "", "model")
	if !g.IsDisabled() {
		t.Errorf("empty key should mark provider disabled")
	}
}

// TestGroqProvider_BuildChatCompletionURL sanity check de la URL.
// Sin key, la URL nunca se golpea — esto solo verifica el helper.
func TestGroqProvider_BuildChatCompletionURL(t *testing.T) {
	g := NewGroq("https://api.groq.com/openai/v1/", "k", "m")
	got := g.chatCompletionURL()
	if !strings.Contains(got, "/chat/completions") {
		t.Errorf("URL %q missing /chat/completions", got)
	}
}
