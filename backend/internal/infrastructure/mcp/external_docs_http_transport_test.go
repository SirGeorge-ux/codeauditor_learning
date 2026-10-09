package mcp_test

// Combined end-to-end smoke test for the mcp-external_docs transport.
// Mirrors the dictionary / user / curriculum / pedagogical
// HTTP-transport-test patterns: chi + mcpkit.NewServer + fake auth + a
// hand-rolled fakeService implementing externaldocsmcp.Service from
// outside the package (no DB, no network — the Context7 + Brave HTTP
// adapters are exercised in their own test files).

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

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	externaldocsmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/external_docs"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// --- fake auth / validator -------------------------------------------------

type externalDocsFakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newExternalDocsFakeValidator() *externalDocsFakeValidator {
	return &externalDocsFakeValidator{tokens: map[string]string{"TOKEN_U": "u-1"}}
}

func (f *externalDocsFakeValidator) ValidateToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tokens[token]; !ok {
		return errors.New("invalid token")
	}
	return nil
}

func (f *externalDocsFakeValidator) UserIDFromToken(token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.tokens[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return u, nil
}

func externalDocsAuthMW(v *externalDocsFakeValidator) func(http.Handler) http.Handler {
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

// --- external_docs HTTP fake Service --------------------------------------

type externalDocsHTTPFakeService struct {
	mu sync.Mutex

	docsOut   services.LibraryDocs
	searchOut []services.SearchDocsResult
	realOut   services.RealExample
}

func newExternalDocsHTTPFakeService() *externalDocsHTTPFakeService {
	return &externalDocsHTTPFakeService{
		docsOut: services.LibraryDocs{
			Library: "react", Topic: "hooks",
			Content: "useState is a hook that...",
			Source:  "context7",
		},
		searchOut: []services.SearchDocsResult{
			{Title: "React hooks", URL: "https://example/hooks", Snippet: "the official hook guide"},
		},
		realOut: services.RealExample{
			Library: "react", Topic: "useState",
			Code:   "const [c, setC] = useState(0);",
			Source: "context7",
		},
	}
}

func (f *externalDocsHTTPFakeService) GetLibraryDocs(_ context.Context, _ services.LibraryDocsInput) (services.LibraryDocs, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.docsOut, nil
}

func (f *externalDocsHTTPFakeService) SearchDocumentation(_ context.Context, _ services.SearchDocsInput) ([]services.SearchDocsResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.searchOut == nil {
		return []services.SearchDocsResult{}, nil
	}
	return f.searchOut, nil
}

func (f *externalDocsHTTPFakeService) GetRealExample(_ context.Context, _ services.RealExampleInput) (services.RealExample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.realOut, nil
}

// externalDocsInjectionFake returns ErrCurriculumInvalidLanguage on
// every method so the transport test exercises the JSON-RPC error
// mapping end-to-end.
type externalDocsInjectionFake struct{}

func (externalDocsInjectionFake) GetLibraryDocs(_ context.Context, _ services.LibraryDocsInput) (services.LibraryDocs, error) {
	return services.LibraryDocs{}, services.ErrCurriculumInvalidLanguage
}

func (externalDocsInjectionFake) SearchDocumentation(_ context.Context, _ services.SearchDocsInput) ([]services.SearchDocsResult, error) {
	return nil, services.ErrCurriculumInvalidLanguage
}

func (externalDocsInjectionFake) GetRealExample(_ context.Context, _ services.RealExampleInput) (services.RealExample, error) {
	return services.RealExample{}, services.ErrCurriculumInvalidLanguage
}

// --- fixture + helpers ----------------------------------------------------

func externalDocsFixture(t *testing.T) http.Handler {
	t.Helper()
	svc := newExternalDocsHTTPFakeService()
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(externaldocsmcp.NewGetLibraryDocsTool(svc))
	mcpReg.Register(externaldocsmcp.NewSearchDocumentationTool(svc))
	mcpReg.Register(externaldocsmcp.NewGetRealExampleTool(svc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(externalDocsAuthMW(newExternalDocsFakeValidator())))
	return r
}

func externalDocsRPCCall(t *testing.T, id, name string, args any) []byte {
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

// --- tests ----------------------------------------------------------------

func TestMCPHTTPTransport_ExternalDocs_ListTools_IncludesAll3(t *testing.T) {
	stack := externalDocsFixture(t)
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
		"mcp-external_docs.get_library_docs":     false,
		"mcp-external_docs.search_documentation": false,
		"mcp-external_docs.get_real_example":     false,
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

func TestMCPHTTPTransport_ExternalDocs_GetLibraryDocs_HappyPath(t *testing.T) {
	stack := externalDocsFixture(t)
	body := externalDocsRPCCall(t, "req-docs", "mcp-external_docs.get_library_docs", map[string]any{
		"library": "react", "topic": "hooks",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error  any             `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(rspBody, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error != nil {
		t.Fatalf("unexpected error: %v", env.Error)
	}
	if !strings.Contains(string(env.Result), "useState is a hook") {
		t.Errorf("result = %s, want 'useState is a hook' in payload", env.Result)
	}
}

func TestMCPHTTPTransport_ExternalDocs_SearchDocumentation_HappyPath(t *testing.T) {
	stack := externalDocsFixture(t)
	body := externalDocsRPCCall(t, "req-search", "mcp-external_docs.search_documentation", map[string]any{
		"query": "react hooks",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error any `json:"error"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("unexpected error: %v", env.Error)
	}
}

func TestMCPHTTPTransport_ExternalDocs_GetRealExample_HappyPath(t *testing.T) {
	stack := externalDocsFixture(t)
	body := externalDocsRPCCall(t, "req-real", "mcp-external_docs.get_real_example", map[string]any{
		"library": "react", "topic": "useState", "language": "typescript",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error any `json:"error"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("unexpected error: %v", env.Error)
	}
}

func TestMCPHTTPTransport_ExternalDocs_MissingLibrary_ReturnsInvalidParams(t *testing.T) {
	stack := externalDocsFixture(t)
	body := externalDocsRPCCall(t, "req-empty", "mcp-external_docs.get_library_docs", map[string]any{
		"library": "  ",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error == nil || env.Error.Code != mcpkit.CodeInvalidParams {
		t.Fatalf("expected -32602 for empty library, got: %s", rspBody)
	}
}

func TestMCPHTTPTransport_ExternalDocs_ValidationSentinel_MapsToInvalidParams(t *testing.T) {
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(externaldocsmcp.NewGetLibraryDocsTool(externalDocsInjectionFake{}))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(externalDocsAuthMW(newExternalDocsFakeValidator())))

	body := externalDocsRPCCall(t, "req-val", "mcp-external_docs.get_library_docs", map[string]any{
		"library": "react",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error == nil || env.Error.Code != mcpkit.CodeInvalidParams {
		t.Fatalf("expected -32602 from sentinel mapping, got: %s", rspBody)
	}
}

func TestMCPHTTPTransport_ExternalDocs_Unauthenticated_Returns401(t *testing.T) {
	stack := externalDocsFixture(t)
	body := externalDocsRPCCall(t, "req-noauth", "mcp-external_docs.get_library_docs", map[string]any{
		"library": "react",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauth, got %d", rr.Code)
	}
}
