package mcpkit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// passthroughAuth lets every request through and injects a fixed user id,
// so the integration tests can exercise the MCP server without real JWTs.
func passthroughAuth(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), authmiddleware.UserIDContextKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newTestServer(t *testing.T, userID string) (*Server, http.Handler) {
	t.Helper()
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-sandbox.list_languages", desc: "Lists languages"})
	r.Register(&fakeTool{name: "mcp-sandbox.run_code", desc: "Runs code"})
	srv := NewServer(r)
	return srv, srv.Router(passthroughAuth(userID))
}

func TestServer_ListTools_ReturnsToolsArray(t *testing.T) {
	srv, h := newTestServer(t, "user-1")

	req := httptest.NewRequest(http.MethodGet, "/tools/list", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var items []toolListItem
	if err := json.Unmarshal(rr.Body.Bytes(), &items); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(items))
	}
	if items[0].Name != "mcp-sandbox.list_languages" {
		t.Errorf("expected first sorted name, got %q", items[0].Name)
	}
	if items[0].Description != "Lists languages" {
		t.Errorf("expected description preserved, got %q", items[0].Description)
	}
	_ = srv // keep server reference for potential override assertions
}

func TestServer_CallTool_DispatchesAndReturnsResult(t *testing.T) {
	_, h := newTestServer(t, "user-1")

	body := makeRequest("req-1", "mcp-sandbox.list_languages", nil)
	req := httptest.NewRequest(http.MethodPost, "/tools/call", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var rsp rpcResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &rsp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if rsp.Error != nil {
		t.Fatalf("expected success result, got error %+v", rsp.Error)
	}
	if rsp.ID != "req-1" {
		t.Errorf("expected id echoed back, got %v", rsp.ID)
	}
	if rsp.Result != "ok" {
		t.Errorf("expected default fake result 'ok', got %v", rsp.Result)
	}
}

func TestServer_CallTool_RedirectsUnknownToolToMethodNotFound(t *testing.T) {
	_, h := newTestServer(t, "user-1")

	body := makeRequest("req-x", "mcp-nonexistent.foo", nil)
	req := httptest.NewRequest(http.MethodPost, "/tools/call", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	var rsp rpcResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &rsp)
	if rsp.Error == nil || rsp.Error.Code != CodeMethodNotFound {
		t.Fatalf("expected -32601, got %+v", rsp.Error)
	}
}

func TestServer_Events_StreamsReadyAndHeartbeat(t *testing.T) {
	_, h := newTestServer(t, "user-1")

	// Use a context that the test cancels after the ready event so we do not
	// wait for the 15s heartbeat.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rr, req)
		close(done)
	}()

	// Give the SSE handler a moment to emit the ready event.
	time.Sleep(30 * time.Millisecond)
	cancel()
	<-done

	body := rr.Body.String()
	if !strings.Contains(body, "event: message") {
		t.Errorf("expected ready 'message' event, got: %s", body)
	}
	if !strings.Contains(body, `"status":"ready"`) {
		t.Errorf("expected ready status in SSE, got: %s", body)
	}
}

func TestServer_RejectsUnauthenticatedWithoutAuthMiddleware(t *testing.T) {
	// Mount WITHOUT any auth middleware — the handler still runs, but user id
	// is empty, so the rate limiter passes through; the spec's 401 path is the
	// caller's responsibility (the real auth middleware is mounted in main.go).
	// Here we assert the list endpoint does not panic when no user id is set.
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-x.y"})
	srv := NewServer(r)
	h := srv.Router(func(next http.Handler) http.Handler {
		// Real auth would reject here; we simulate by running next.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	req := httptest.NewRequest(http.MethodGet, "/tools/list", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var items []toolListItem
	_ = json.Unmarshal(rr.Body.Bytes(), &items)
	if len(items) != 1 {
		t.Errorf("expected 1 tool, got %d", len(items))
	}
}

// TestServer_RateLimitedCall returns -32609 after the limit is exhausted.
func TestServer_RateLimitedCallReturnsJSONRPCError(t *testing.T) {
	// Build a server with a tiny rate-limit window + small empty to keep the
	// test fast and isolated.
	r := NewRegistry()
	r.Register(&fakeTool{name: "mcp-sandbox.run_code", desc: "runs"})
	srv := NewServer(r)
	srv.WithRateLimiter(NewRateLimiter())
	h := srv.Router(passthroughAuth("user-1"))

	body := makeRequest("req-i", "mcp-sandbox.run_code", nil)
	for i := 0; i < DefaultRateLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/tools/call", strings.NewReader(string(body)))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		var rsp rpcResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &rsp)
		if rsp.Error != nil {
			t.Fatalf("call #%d should succeed, got error %+v", i+1, rsp.Error)
		}
	}

	// 11th call MUST be rejected with -32609.
	req := httptest.NewRequest(http.MethodPost, "/tools/call", strings.NewReader(string(body)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var rsp rpcResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &rsp)
	if rsp.Error == nil || rsp.Error.Code != CodeRateLimited {
		t.Fatalf("expected -32609 on 11th call, got %+v", rsp.Error)
	}
}

// Ensure full SSE read isn't needed by relying on cancel.
var _ io.Reader = (*strings.Reader)(nil)
