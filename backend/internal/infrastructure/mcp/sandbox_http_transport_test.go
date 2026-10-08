package mcp_test

// Combined end-to-end smoke test for the mcp-sandbox transport that mirrors
// the dictionary http_transport_test.go pattern: chi + mcpkit.NewServer +
// fake auth + a real *sandbox.DockerSandbox-backed service. The point of
// this file is to prove the production main.go wiring is structurally
// correct — a JSON-RPC call reaches the tool with the right user_id, and
// the registry lists all 4 sandbox tools.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	sandboxdrv "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	sandboxmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

type fakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newFakeValidator() *fakeValidator {
	return &fakeValidator{tokens: map[string]string{"TOKEN_U": "u-1"}}
}
func (f *fakeValidator) ValidateToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tokens[token]; !ok {
		return errors.New("invalid token")
	}
	return nil
}
func (f *fakeValidator) UserIDFromToken(token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.tokens[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return u, nil
}

func authMW(v *fakeValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Test-Token")
			if token == "" {
				token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			}
			if token == "" || v.ValidateToken(r.Context(), token) != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			uid, _ := v.UserIDFromToken(token)
			next.ServeHTTP(w, r.WithContext(mcpctx.ContextWithUserID(r.Context(), uid)))
		})
	}
}

func sandboxFixture(t *testing.T) (http.Handler, sandboxmcp.Service) {
	t.Helper()
	registry := providers.NewDefaultRegistry()
	docker := sandboxdrv.NewDockerSandbox(0, registry)
	svc := sandboxmcp.NewServiceAdapter(docker, registry)
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(sandboxmcp.NewRunCodeTool(svc))
	mcpReg.Register(sandboxmcp.NewListLanguagesTool(svc))
	mcpReg.Register(sandboxmcp.NewHealthcheckTool(svc))
	mcpReg.Register(sandboxmcp.NewGetInstallHintTool(svc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(authMW(newFakeValidator())))
	return r, svc
}

func rpcCall(t *testing.T, id, name string, args any) []byte {
	t.Helper()
	argJSON, _ := json.Marshal(args)
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": json.RawMessage(argJSON),
		},
	})
	return body
}

func TestMCPHTTPTransport_Sandbox_ListTools_IncludesAll4(t *testing.T) {
	stack, _ := sandboxFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list", nil)
	req.Header.Set("X-Test-Token", "TOKEN_U")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	want := map[string]bool{
		"mcp-sandbox.run_code":         false,
		"mcp-sandbox.list_languages":   false,
		"mcp-sandbox.healthcheck":      false,
		"mcp-sandbox.get_install_hint": false,
	}
	for _, it := range items {
		name, _ := it["name"].(string)
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("/mcp/tools/list missing %q", k)
		}
	}
}

func TestMCPHTTPTransport_Sandbox_ListLanguages_ReturnsCatalogOverHTTP(t *testing.T) {
	stack, _ := sandboxFixture(t)
	body := rpcCall(t, "req-1", "mcp-sandbox.list_languages", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", rr.Code, rr.Body.String())
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error any             `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(rspBody, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error != nil {
		t.Fatalf("expected success, got error: %v", env.Error)
	}
	var langs []map[string]any
	if err := json.Unmarshal(env.Result, &langs); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(langs) < 1 {
		t.Fatalf("expected non-empty catalog, got %d", len(langs))
	}
}

func TestMCPHTTPTransport_Sandbox_Unauthenticated_Request_Returns401(t *testing.T) {
	stack, _ := sandboxFixture(t)
	body := rpcCall(t, "req-noauth", "mcp-sandbox.list_languages", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
