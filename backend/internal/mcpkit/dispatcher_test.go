package mcpkit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// makeRequest builds a JSON-RPC 2.0 tools/call body.
func makeRequest(id, toolName string, args any) []byte {
	if args == nil {
		args = map[string]any{}
	}
	raw, _ := json.Marshal(args)
	body, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"` + id + `"`),
		Method:  "tools/call",
		Params:  rpcParams{Name: toolName, Arguments: raw},
	})
	return body
}

// ---- RED 1.1: dispatcher enforces 30s timeout per tool call ----

func TestDispatcher_EnforcesTimeout(t *testing.T) {
	// RED assertion (threat matrix: Tool Exhaustion / RCE): a tool that
	// exceeds the per-call budget MUST be cut off and reported as an
	// internal error (mapped to -32603).
	r := NewRegistry()
	tool := &fakeTool{
		name: "mcp-sandbox.run_code",
		exec: func(ctx context.Context, _ json.RawMessage) (any, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return "should-not-return", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	r.Register(tool)

	// Use a tiny timeout (10ms) for fast verification. Production keeps 30s.
	d := NewDispatcher(r).WithTimeout(10 * time.Millisecond)

	rsp := d.Dispatch(context.Background(), makeRequest("req-1", "mcp-sandbox.run_code", nil))

	var got rpcResponse
	out, _ := json.Marshal(rsp)
	_ = json.Unmarshal(out, &got)
	if got.Error == nil {
		t.Fatalf("expected timeout error, got result: %v", got.Result)
	}
	if got.Error.Code != CodeInternalError {
		t.Errorf("expected code %d (Internal), got %d", CodeInternalError, got.Error.Code)
	}
	if !strings.Contains(got.Error.Message, "timed out") {
		t.Errorf("expected 'timed out' in message, got %q", got.Error.Message)
	}
}

func TestDispatcher_FastToolSucceedsWithinTimeout(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-sandbox.list_languages"})
	d := NewDispatcher(r).WithTimeout(30 * time.Second)

	rsp := d.Dispatch(context.Background(), makeRequest("req-1", "mcp-sandbox.list_languages", nil))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error != nil {
		t.Fatalf("expected success, got error: %v", got.Error)
	}
}

// ---- RED 1.1: dispatcher truncates responses > 50KB ----

func TestDispatcher_TruncatesResponseOverCap(t *testing.T) {
	r := NewRegistry()
	oversized := strings.Repeat("x", MaxResponseBytes+10_000) // > 50KB
	r.Register(&fakeTool{
		name: "mcp-verbose.dump",
		desc: "big",
		exec: func(_ context.Context, _ json.RawMessage) (any, error) {
			return oversized, nil
		},
	})
	d := NewDispatcher(r)

	rsp := d.Dispatch(context.Background(), makeRequest("r", "mcp-verbose.dump", nil))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error != nil {
		t.Fatalf("unexpected error: %v", got.Error)
	}
	// The truncated result carries a `truncated: true` flag.
	obj, ok := got.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected truncated envelope, got %T: %v", got.Result, got.Result)
	}
	if trunc, _ := obj["truncated"].(bool); !trunc {
		t.Errorf("expected truncated=true, got %v", obj["truncated"])
	}
	content, _ := obj["content"].(string)
	if len(content) > MaxResponseBytes {
		t.Errorf("expected content capped at %d bytes, got %d", MaxResponseBytes, len(content))
	}
}

func TestDispatcher_ShortResponseNotTruncated(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-sandbox.list_languages", exec: func(_ context.Context, _ json.RawMessage) (any, error) {
		return []string{"go", "ts"}, nil
	}})
	d := NewDispatcher(r)
	rsp := d.Dispatch(context.Background(), makeRequest("r", "mcp-sandbox.list_languages", nil))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error != nil {
		t.Fatalf("unexpected error: %v", got.Error)
	}
	// Plain result is returned directly (no truncated envelope).
	if _, isEnvelope := got.Result.(map[string]any); isEnvelope {
		if _, ok := got.Result.(map[string]any)["truncated"]; ok {
			t.Fatalf("did not expect truncation envelope for short response: %v", got.Result)
		}
	}
}

// ---- Error mapping ----

func TestDispatcher_UnknownToolReturnsMethodNotFound(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)
	rsp := d.Dispatch(context.Background(), makeRequest("r", "mcp-nonexistent.foo", nil))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error == nil || got.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected -32601, got %+v", got.Error)
	}
}

func TestDispatcher_MissingMethodReturnsInvalidRequest(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "r", "params": rpcParams{Name: "x"}})
	rsp := d.Dispatch(context.Background(), body)
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error == nil || got.Error.Code != CodeInvalidRequest {
		t.Fatalf("expected -32600, got %+v", got.Error)
	}
}

func TestDispatcher_WrongMethodReturnsMethodNotFound(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-x.y"})
	d := NewDispatcher(r)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "r", "method": "foo/bar"})
	rsp := d.Dispatch(context.Background(), body)
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error == nil || got.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected -32601, got %+v", got.Error)
	}
}

func TestDispatcher_MalformedJSONReturnsParseError(t *testing.T) {
	r := NewRegistry()
	d := NewDispatcher(r)
	rsp := d.Dispatch(context.Background(), []byte(`{not json`))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error == nil || got.Error.Code != CodeParseError {
		t.Fatalf("expected -32700, got %+v", got.Error)
	}
}

func TestDispatcher_ServiceErrorMapsToInternalError(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-bad.svc", exec: func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("boom in services/something.go:42: db exploded with token=ghp_abcdef")
	}})
	d := NewDispatcher(r)
	rsp := d.Dispatch(context.Background(), makeRequest("r", "mcp-bad.svc", nil))
	out, _ := json.Marshal(rsp)
	var got rpcResponse
	_ = json.Unmarshal(out, &got)
	if got.Error == nil || got.Error.Code != CodeInternalError {
		t.Fatalf("expected -32603, got %+v", got.Error)
	}
	// Error message MUST be sanitized: file/line + leaked token removed.
	if strings.Contains(got.Error.Message, "services/something.go") {
		t.Errorf("expected file/line scrubbed, got %q", got.Error.Message)
	}
	if strings.Contains(got.Error.Message, "ghp_abcdef") {
		t.Errorf("expected token redacted, got %q", got.Error.Message)
	}
}

// ---- RED 1.8: response sanitization (no secrets leaked) ----

func TestDispatcher_SanitizesSecretsInResult(t *testing.T) {
	cases := []struct {
		name    string
		result  any
		mustNot string
	}{
		{"api_key string", map[string]any{"api_key": "sk-leak-1234567890123456789012", "ok": true}, "sk-leak"},
		{"token pair", map[string]any{"token": "ghp_abcdefghijklmnopqr", "value": 1}, "ghp_abcdefghijklmnopqr"},
		{"password", map[string]any{"password": "hunter2", "u": "a"}, "hunter2"},
		{"nested secret", map[string]any{"cfg": map[string]any{"secret": "sb_publish_01234567890123456789"}}, "sb_publish_"},
		{"jwt in string field", map[string]any{"trace": "auth=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ4In0.sig123"}, "eyJ"},
		{"bearer header", map[string]any{"authorization": "bearer eyJhbGci.eyJzdWI.sig"}, "eyJ"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded, _ := json.Marshal(SanitizeResult(c.result))
			if strings.Contains(string(encoded), c.mustNot) {
				t.Errorf("secret %q leaked in sanitized result: %s", c.mustNot, encoded)
			}
		})
	}
}

func TestDispatcher_SanitizePreservesNonSecretData(t *testing.T) {
	in := map[string]any{"language": "typescript", "level": 5, "topic": "closures"}
	out := SanitizeResult(in)
	encoded, _ := json.Marshal(out)
	var decoded map[string]any
	_ = json.Unmarshal(encoded, &decoded)
	if decoded["language"] != "typescript" {
		t.Errorf("expected language preserved, got %v", decoded["language"])
	}
	if decoded["topic"] != "closures" {
		t.Errorf("expected topic preserved, got %v", decoded["topic"])
	}
}
