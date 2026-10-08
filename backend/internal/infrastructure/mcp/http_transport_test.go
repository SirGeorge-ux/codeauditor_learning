package mcp_test

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

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp"
	dictionarymcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/dictionary"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/mcpctx"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeValidator is the local stand-in for the project's AuthValidator
// (typically Supabase JWT). It stamps the user id from a known
// X-Test-User header so the test exercises the bridge WITHOUT spinning up
// a real JWT signer. Production code uses AuthMiddleware in main.go;
// this shows only the MCP HTTP transport contract.
type fakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string // token -> user id
}

func newFakeValidator() *fakeValidator {
	return &fakeValidator{tokens: map[string]string{
		"TOKEN_USER_1": "user-1",
		"TOKEN_USER_2": "user-2",
	}}
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

type fakeUserIDKey struct{}

// fakeAuthMiddlewareFactory is the mcp.Router auth callback. It stamps
// the user id under our fake key but tells the mcp bridge to read the
// same id — this is what production main.go does with
// `authmiddleware.AuthMiddleware(authAdapter)`.
func fakeAuthMiddlewareFactory(v *fakeValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Test-Token")
			if token == "" {
				token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			}
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := v.ValidateToken(r.Context(), token); err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			userID, _ := v.UserIDFromToken(token)
			// Stamp BOTH the fake user id key AND the canonical MCP key so
			// the dictionary tool's userIDFromContext can read it via the
			// bridge even when the bridge is in play.
			ctx := context.WithValue(r.Context(), fakeUserIDKey{}, userID)
			ctx = mcpctx.ContextWithUserID(ctx, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// httTransportRepo is the deterministic in-memory repo used by the HTTP
// transport test. It mirrors the production ordering + prefix behavior.
type httTransportRepo struct {
	mu  sync.Mutex
	all map[string]map[string]*models.Term // userID -> lower(term) -> term
}

func newHTTTransportRepo() *httTransportRepo {
	return &httTransportRepo{all: map[string]map[string]*models.Term{}}
}

func (r *httTransportRepo) InsertTerm(_ context.Context, userID string, term *models.Term) (*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.all[userID]; !ok {
		r.all[userID] = map[string]*models.Term{}
	}
	key := strings.ToLower(term.Term)
	if _, exists := r.all[userID][key]; exists {
		return nil, ports.ErrTermConflict
	}
	clone := *term
	clone.UserID = userID
	if clone.Synonyms == nil {
		clone.Synonyms = []string{}
	}
	if clone.Tags == nil {
		clone.Tags = []string{}
	}
	r.all[userID][key] = &clone
	out := clone
	return &out, nil
}

func (r *httTransportRepo) GetTerm(_ context.Context, userID, termID string) (*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	byTerm := r.all[userID]
	for _, t := range byTerm {
		if t.ID == termID {
			clone := *t
			return &clone, nil
		}
	}
	return nil, ports.ErrTermNotFound
}

func (r *httTransportRepo) UpdateTerm(_ context.Context, userID string, term *models.Term) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.all[userID] {
		if t.ID == term.ID {
			clone := *term
			clone.UserID = userID
			r.all[userID][strings.ToLower(term.Term)] = &clone
			return nil
		}
	}
	return ports.ErrTermNotFound
}

func (r *httTransportRepo) DeleteTerm(_ context.Context, userID, termID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, t := range r.all[userID] {
		if t.ID == termID {
			delete(r.all[userID], key)
			return nil
		}
	}
	return ports.ErrTermNotFound
}

func (r *httTransportRepo) ListTerms(_ context.Context, userID, category, tag string) ([]*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*models.Term{}
	for _, t := range r.all[userID] {
		if category != "" && t.Category != category {
			continue
		}
		// simple tag handling for tests
		out = append(out, t)
	}
	return out, nil
}

func (r *httTransportRepo) SearchTerms(_ context.Context, userID, query string) ([]*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	out := []*models.Term{}
	if len(tokens) == 0 {
		for _, t := range r.all[userID] {
			out = append(out, t)
		}
		return out, nil
	}
	for _, t := range r.all[userID] {
		haystack := strings.ToLower(t.Term) + " " + strings.ToLower(t.Explanation)
		for _, s := range t.Synonyms {
			haystack += " " + strings.ToLower(s)
		}
		for _, g := range t.Tags {
			haystack += " " + strings.ToLower(g)
		}
		matched := false
		for _, tok := range tokens {
			for _, w := range strings.Fields(haystack) {
				if strings.HasPrefix(w, tok) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			out = append(out, t)
		}
	}
	return out, nil
}

// httpFixture builds the production-equivalent HTTP stack so the test
// exercises exactly the wire shape deployed in main.go:
//
// chi.NewRouter -> /mcp mount -> mcp.Server.Router(authMW) -> /tools/{list,call}
func httpFixture(t *testing.T) (http.Handler, *httTransportRepo, *fakeValidator) {
	t.Helper()
	repo := newHTTTransportRepo()
	dictSvc := services.NewDictionaryService(repo)
	dictAdapter := dictionarymcp.NewServiceAdapter(dictSvc)

	registry := mcp.NewRegistry()
	registry.Register(dictionarymcp.NewSearchTermTool(dictAdapter))
	registry.Register(dictionarymcp.NewGetTermTool(dictAdapter))
	registry.Register(dictionarymcp.NewAddTermTool(dictAdapter))
	registry.Register(dictionarymcp.NewListTermsTool(dictAdapter))

	validator := newFakeValidator()

	r := chi.NewRouter()
	r.Mount("/mcp", mcp.NewServer(registry).Router(fakeAuthMiddlewareFactory(validator)))
	return r, repo, validator
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

func TestMCPHTTPTransport_SearchTerm_ReachesToolOverHTTP(t *testing.T) {
	stack, repo, _ := httpFixture(t)
	// Seed: user-1 has 'hardcode' with explanation mentioning literal values.
	if _, err := repo.InsertTerm(context.Background(), "user-1", &models.Term{
		Term:        "hardcode",
		Explanation: "Writing literal values directly in code.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Issue a JSON-RPC call to /mcp/tools/call as user-1 searching for "hard".
	body := rpcCall(t, "req-1", "mcp-dictionary.search_term", map[string]any{"query": "hard"})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_USER_1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}

	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   any             `json:"error"`
	}
	if err := json.Unmarshal(rspBody, &env); err != nil {
		t.Fatalf("decode rpc response: %v\nbody: %s", err, rspBody)
	}
	if env.Error != nil {
		t.Fatalf("expected success, got error: %v", env.Error)
	}
	// The result should be a JSON array containing the seeded term.
	var hits []map[string]any
	if err := json.Unmarshal(env.Result, &hits); err != nil {
		t.Fatalf("decode hits: %v\nresult: %s", err, env.Result)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for 'hard' prefix, got %d\nresult: %s", len(hits), env.Result)
	}
	if term, _ := hits[0]["term"].(string); term != "hardcode" {
		t.Errorf("hit term = %q, want hardcode", term)
	}
}

func TestMCPHTTPTransport_ListTools_IncludesDictionaryGroup(t *testing.T) {
	stack, _, _ := httpFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list", nil)
	req.Header.Set("X-Test-Token", "TOKEN_USER_1")
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
		"mcp-dictionary.add_term":    false,
		"mcp-dictionary.get_term":    false,
		"mcp-dictionary.list_terms":  false,
		"mcp-dictionary.search_term": false,
	}
	for _, it := range items {
		name, _ := it["name"].(string)
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("/mcp/tools/list missing dictionary tool %q", k)
		}
	}
}

func TestMCPHTTPTransport_AddTerm_ValidationRejectionReachesLLM(t *testing.T) {
	stack, _, _ := httpFixture(t)

	body := rpcCall(t, "req-2", "mcp-dictionary.add_term", map[string]any{
		"term":        "hardcode",
		"explanation": "too short",
		"category":    "Concepto",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_USER_1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (RPC envelope), got %d", rr.Code)
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error == nil {
		t.Fatalf("expected RPC error for short explanation, got success\nbody: %s", rspBody)
	}
	if env.Error.Code != mcp.CodeInvalidParams {
		t.Errorf("error code = %d, want %d (-32602)", env.Error.Code, mcp.CodeInvalidParams)
	}
	if !strings.Contains(strings.ToLower(env.Error.Message), "explanation") {
		t.Errorf("error message should mention explanation, got %q", env.Error.Message)
	}
}

func TestMCPHTTPTransport_GetTerm_NotFound_ReturnsNull(t *testing.T) {
	stack, _, _ := httpFixture(t)
	body := rpcCall(t, "req-m", "mcp-dictionary.get_term", map[string]any{"id": "missing-uuid"})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_USER_1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error  any         `json:"error"`
		Result interface{} `json:"result"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("expected null result, got error: %v", env.Error)
	}
	// Result should be the JSON literal `null` (encoded as the string "null").
	if resultRaw, _ := json.Marshal(env.Result); string(resultRaw) != "null" {
		t.Errorf("expected result 'null', got %q", string(resultRaw))
	}
}

func TestMCPHTTPTransport_Unauthenticated_Request_Returns401(t *testing.T) {
	stack, _, _ := httpFixture(t)
	body := rpcCall(t, "req-noauth", "mcp-dictionary.search_term", map[string]any{"query": "x"})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No token header — auth middleware must reject.
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauth, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

// Ensure multiple users see only their own rows — the dictionary spec
// RLS / cross-user scenario. The MCP transport must propagate the JWT
// user_id from auth middleware into the tool context so user B cannot
// see user A's terms via mcp-dictionary.*.
func TestMCPHTTPTransport_CrossUser_TermIsolationMcpPath(t *testing.T) {
	stack, repo, _ := httpFixture(t)

	// Seed: user-1 has 'shared-name'; user-2 has 'shared-name' too.
	// Without the bridge they'd both surface on every call.
	if _, err := repo.InsertTerm(context.Background(), "user-1", &models.Term{
		Term:        "shared-name",
		Explanation: "Owned by user 1.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("seed user-1: %v", err)
	}
	if _, err := repo.InsertTerm(context.Background(), "user-2", &models.Term{
		Term:        "shared-name",
		Explanation: "Owned by user 2.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("seed user-2: %v", err)
	}

	cases := []struct {
		token, wantExplanation string
	}{
		{"TOKEN_USER_1", "Owned by user 1."},
		{"TOKEN_USER_2", "Owned by user 2."},
	}
	for _, c := range cases {
		t.Run("token="+c.token, func(t *testing.T) {
			body := rpcCall(t, "req-cu-"+c.token, "mcp-dictionary.search_term",
				map[string]any{"query": "shared"})
			req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
			req.Header.Set("X-Test-Token", c.token)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			stack.ServeHTTP(rr, req)

			rspBody, _ := io.ReadAll(rr.Body)
			var env struct {
				Error  any             `json:"error"`
				Result json.RawMessage `json:"result"`
			}
			_ = json.Unmarshal(rspBody, &env)
			if env.Error != nil {
				t.Fatalf("expected ok, got error %v", env.Error)
			}
			var hits []map[string]any
			_ = json.Unmarshal(env.Result, &hits)
			if len(hits) != 1 {
				t.Fatalf("expected 1 row (own only), got %d; result=%s", len(hits), env.Result)
			}
			exp, _ := hits[0]["explanation"].(string)
			if exp != c.wantExplanation {
				t.Errorf("explanation = %q, want %q — context user_id bridge failed", exp, c.wantExplanation)
			}
		})
	}
}
