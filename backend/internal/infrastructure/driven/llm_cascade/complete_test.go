// complete_test.go — Phase 1 / S3 PR 1. Exercises Cascade.Complete:
// provider fallthrough (Groq → DeepSeek → MiniMax), JSON-mode
// request body shape (response_format={type:"json_object"} on
// Groq/DeepSeek; text + JSON parse fallback on MiniMax), 5-second
// whole-response timeout, and ErrProviderDisabled skipping.
//
// The HTTP boundary is faked with a per-test http.RoundTripper so the
// tests stay hermetic (no live calls to Groq/OpenRouter/MiniMax).
package llmcascade

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// roundTripperFunc adapts a function to http.RoundTripper so each
// test can intercept the request body + headers without spawning a
// real listener.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// cannedResponse builds an http.Response with the supplied status +
// body. The closer is no-op; RoundTripper is single-shot per test.
func cannedResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// scriptedTransport composes a list of RoundTrip functions and
// advances per call; once exhausted it returns 500 with "no more
// scripted responses" so accidental follow-up calls fail loudly
// instead of silently passing.
type scriptedTransport struct {
	steps []roundTripperFunc
	calls int32
}

func (s *scriptedTransport) next() roundTripperFunc {
	idx := atomic.AddInt32(&s.calls, 1) - 1
	if int(idx) >= len(s.steps) {
		return func(*http.Request) (*http.Response, error) {
			return cannedResponse(500, `{"error":{"message":"no more scripted responses"}}`), nil
		}
	}
	return s.steps[idx]
}

func (s *scriptedTransport) Count() int { return int(atomic.LoadInt32(&s.calls)) }

func (s *scriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return s.next()(req)
}

// TestCascade_CompleteReturnsFirstProviderResult — el caso
// happy-path: el primer provider entrega JSON válido y la cascada lo
// devuelve con Provider=groq.
func TestCascade_CompleteReturnsFirstProviderResult(t *testing.T) {
	body := `{"choices":[{"message":{"content":"{\"x\":1}"}}]}`
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(200, body), nil
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(groq, NewDeepSeek("https://x", "k", "m"), NewMiniMax("https://x", "k", "m"))
	got, err := c.Complete(context.Background(), ports.CompleteRequest{
		System: "sys",
		Prompt: "prompt",
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if got.Provider != "groq" {
		t.Errorf("Provider = %q, want groq", got.Provider)
	}
	if got.Content != `{"x":1}` {
		t.Errorf("Content = %q, want %q", got.Content, `{"x":1}`)
	}
	if transport.Count() != 1 {
		t.Errorf("transport calls = %d, want 1 (no fallthrough)", transport.Count())
	}
}

// TestCascade_CompleteFallthroughToDeepSeekOnFiveXX — Groq falla con
// 503, la cascada transparently reintenta contra DeepSeek y el
// Provider del response es "deepseek".
func TestCascade_CompleteFallthroughToDeepSeekOnFiveXX(t *testing.T) {
	body := `{"choices":[{"message":{"content":"{\"y\":2}"}}]}`
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(503, `{"error":{"message":"service unavailable"}}`), nil
		},
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(200, body), nil
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}
	deep := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	deep.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(groq, deep, NewMiniMax("https://x", "k", "m"))
	got, err := c.Complete(context.Background(), ports.CompleteRequest{
		JSONSchema: &ports.JSONSchema{Type: "object"},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if got.Provider != "deepseek" {
		t.Errorf("Provider = %q, want deepseek", got.Provider)
	}
	if got.Content != `{"y":2}` {
		t.Errorf("Content = %q, want %q", got.Content, `{"y":2}`)
	}
	if transport.Count() != 2 {
		t.Errorf("transport calls = %d, want 2", transport.Count())
	}
}

// TestCascade_CompleteJSONModeRequestShape — cuando el caller envía
// JSONSchema, el body del request a Groq debe llevar
// `response_format: {type:"json_object"}` y un único mensaje
// system+user combinado.
func TestCascade_CompleteJSONModeRequestShape(t *testing.T) {
	var capturedBody []byte
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = b
			return cannedResponse(200, `{"choices":[{"message":{"content":"{}"}}]}`), nil
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(groq, NewDeepSeek("https://x", "k", "m"), NewMiniMax("https://x", "k", "m"))
	_, err := c.Complete(context.Background(), ports.CompleteRequest{
		System:     "sys",
		Prompt:     "give me json",
		JSONSchema: &ports.JSONSchema{Type: "object", Required: []string{"title"}},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(capturedBody, &decoded); err != nil {
		t.Fatalf("body not JSON: %v (body=%s)", err, capturedBody)
	}
	rf, ok := decoded["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format missing or not object; body=%s", capturedBody)
	}
	if rf["type"] != "json_object" {
		t.Errorf("response_format.type = %v, want json_object", rf["type"])
	}
	if _, ok := decoded["stream"]; ok {
		t.Errorf("stream flag should NOT be present on non-streaming Complete; body=%s", capturedBody)
	}
}

// TestCascade_CompleteTextModeNoResponseFormat — sin JSONSchema, el
// body NO debe llevar `response_format` (sólo se setea en JSON mode).
func TestCascade_CompleteTextModeNoResponseFormat(t *testing.T) {
	var capturedBody []byte
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = b
			return cannedResponse(200, `{"choices":[{"message":{"content":"ok"}}]}`), nil
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(groq, NewDeepSeek("https://x", "k", "m"), NewMiniMax("https://x", "k", "m"))
	_, err := c.Complete(context.Background(), ports.CompleteRequest{
		System: "sys",
		Prompt: "no schema",
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(capturedBody, &decoded); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if _, present := decoded["response_format"]; present {
		t.Errorf("response_format must be absent in text mode; body=%s", capturedBody)
	}
}

// TestCascade_CompleteMiniMaxTextFallback — MiniMax no soporta
// response_format=json_object. La cascada debe pedirle texto libre
// (sin `response_format`) y el adapter debe extraer el primer
// objeto JSON de la respuesta.
func TestCascade_CompleteMiniMaxTextFallback(t *testing.T) {
	var minimaxSaw []byte
	transport := &scriptedTransport{steps: []roundTripperFunc{
		// Groq: 500 (skip).
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(500, "groq down"), nil
		},
		// DeepSeek: 500 (skip).
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(500, "deepseek down"), nil
		},
		// MiniMax: text mode, response body has a JSON blob buried in prose.
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			minimaxSaw = b
			return cannedResponse(200, `{"choices":[{"message":{"content":"Here you go: {\"hello\":\"world\"} -- done."}}]}`), nil
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}
	deep := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	deep.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}
	m3 := NewMiniMax("https://api.MiniMax.ai/v1", "key", "MiniMax-M3")
	m3.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(groq, deep, m3)
	got, err := c.Complete(context.Background(), ports.CompleteRequest{
		JSONSchema: &ports.JSONSchema{Type: "object"},
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if got.Provider != "minimax" {
		t.Errorf("Provider = %q, want minimax (last provider standing)", got.Provider)
	}
	if got.Content != `{"hello":"world"}` {
		t.Errorf("Content = %q, want %q", got.Content, `{"hello":"world"}`)
	}
	// MiniMax request must NOT carry `response_format` (it's not a
	// JSON-mode provider).
	var decoded map[string]any
	if err := json.Unmarshal(minimaxSaw, &decoded); err != nil {
		t.Fatalf("MiniMax body not JSON: %v", err)
	}
	if _, present := decoded["response_format"]; present {
		t.Errorf("MiniMax must NOT receive response_format; body=%s", minimaxSaw)
	}
}

// TestCascade_CompleteDisabledProviderSkipped — Groq sin API key
// (ErrProviderDisabled) NO se consulta en HTTP, y la cascada pasa
// al siguiente.
func TestCascade_CompleteDisabledProviderSkipped(t *testing.T) {
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(200, `{"choices":[{"message":{"content":"{\"z\":3}"}}]}`), nil
		},
	}}
	deep := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	deep.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}

	c := NewCascade(NewGroq("https://api.groq.com/openai/v1", "", "model"), deep, NewMiniMax("https://x", "k", "m"))
	got, err := c.Complete(context.Background(), ports.CompleteRequest{})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if got.Provider != "deepseek" {
		t.Errorf("Provider = %q, want deepseek (groq disabled)", got.Provider)
	}
	if transport.Count() != 1 {
		t.Errorf("transport calls = %d, want 1 (groq disabled, no HTTP)", transport.Count())
	}
}

// TestCascade_CompleteTimeoutFiresAtFiveSeconds — the cascade must
// honor a 5s whole-response budget and return ErrLLMCompleteTimeout
// when the provider stalls. We force a shorter caller deadline via
// context.WithTimeout(200ms) so the assertion finishes in well under
// the documented 5s SLA without making CI slow.
func TestCascade_CompleteTimeoutFiresAtFiveSeconds(t *testing.T) {
	if ports.DefaultCompleteTimeout != 5*time.Second {
		t.Fatalf("DefaultCompleteTimeout = %v, want 5s (spec lock)", ports.DefaultCompleteTimeout)
	}
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			// Hold the request open until the context fires.
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(2 * time.Second):
				return cannedResponse(200, `{"choices":[{"message":{"content":"{}"}}]}`), nil
			}
		},
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Transport: transport}

	c := NewCascade(groq, NewDeepSeek("https://x", "k", "m"), NewMiniMax("https://x", "k", "m"))

	// Caller-side 200ms deadline: context.WithTimeout is the
	// effective upper bound; the cascade's 5s budget is
	// irrelevant in the presence of a shorter parent deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.Complete(ctx, ports.CompleteRequest{})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !errors.Is(err, ports.ErrLLMCompleteTimeout) {
		t.Errorf("err = %v, want ErrLLMCompleteTimeout", err)
	}
	// Must return before the 2-second provider would have
	// naturally completed. Generous slack for slow CI.
	if elapsed > 1*time.Second {
		t.Errorf("Complete did not honor deadline; elapsed=%v", elapsed)
	}
}

// TestCascade_CompletePreferredProviderShortCircuits — PreferredProvider
// limita la cascada a un único provider conocido.
func TestCascade_CompletePreferredProviderShortCircuits(t *testing.T) {
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(req *http.Request) (*http.Response, error) {
			return cannedResponse(200, `{"choices":[{"message":{"content":"{\"deep\":\"ok\"}"}}]}`), nil
		},
	}}
	deep := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	deep.http = &http.Client{Timeout: 10 * time.Second, Transport: transport}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Transport: &scriptedTransport{steps: []roundTripperFunc{
		func(*http.Request) (*http.Response, error) {
			t.Errorf("groq should not be called when PreferredProvider=deepseek")
			return cannedResponse(500, "should not see this"), nil
		},
	}}}

	c := NewCascade(groq, deep, NewMiniMax("https://x", "k", "m"))
	got, err := c.Complete(context.Background(), ports.CompleteRequest{
		PreferredProvider: "deepseek",
	})
	if err != nil {
		t.Fatalf("Complete err: %v", err)
	}
	if got.Provider != "deepseek" {
		t.Errorf("Provider = %q, want deepseek", got.Provider)
	}
}

// TestCascade_CompleteAllProvidersFail — sentinel cascade-all-failed
// se propaga cuando todo falla.
func TestCascade_CompleteAllProvidersFail(t *testing.T) {
	transport := &scriptedTransport{steps: []roundTripperFunc{
		func(*http.Request) (*http.Response, error) { return cannedResponse(500, "x"), nil },
		func(*http.Request) (*http.Response, error) { return cannedResponse(500, "x"), nil },
		func(*http.Request) (*http.Response, error) { return cannedResponse(500, "x"), nil },
	}}
	groq := NewGroq("https://api.groq.com/openai/v1", "key", "model")
	groq.http = &http.Client{Transport: transport}
	deep := NewDeepSeek("https://openrouter.ai/api/v1", "key", "model")
	deep.http = &http.Client{Transport: transport}
	m3 := NewMiniMax("https://api.MiniMax.ai/v1", "key", "MiniMax-M3")
	m3.http = &http.Client{Transport: transport}

	c := NewCascade(groq, deep, m3)
	_, err := c.Complete(context.Background(), ports.CompleteRequest{})
	if !errors.Is(err, ErrCascadeAllFailed) {
		t.Errorf("err = %v, want ErrCascadeAllFailed", err)
	}
}
