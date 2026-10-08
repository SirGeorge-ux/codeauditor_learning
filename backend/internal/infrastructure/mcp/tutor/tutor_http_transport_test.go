package tutor

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
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeValidator is the local stand-in for the project's AuthValidator
// (typically Supabase JWT). It stamps the user id from a known token so
// the tests exercise the bridge WITHOUT spinning up a real JWT signer.
// Production code uses authmiddleware.AuthMiddleware in main.go; this
// shows only the MCP HTTP transport contract for the tutor_lookup tool.
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

// fakeAuthMiddlewareFactory mirrors the production main.go wiring:
// stamps BOTH the local auth key AND the canonical mcpctx key so the
// tutor tool can read the user_id via its own userIDFromContext (the
// same path the dictionary tools take).
func fakeAuthMiddlewareFactory(v *fakeValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := v.ValidateToken(r.Context(), token); err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}
			userID, _ := v.UserIDFromToken(token)
			ctx := mcpctx.ContextWithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// repoStub is a deterministic DictionaryRepository stand-in used by the
// transport-level triangle. It exercises the production chain end-to-end:
// in-memory TutorLookupService + dictionary search, with the JWT-derived
// user_id acting as the ONLY cross-tenant guard.
type repoStub struct {
	mu  sync.Mutex
	all map[string]map[string]*models.Term // userID -> lower(term) -> term
}

func newRepoStubForTransport() *repoStub {
	return &repoStub{all: map[string]map[string]*models.Term{}}
}

func (r *repoStub) InsertTerm(_ context.Context, userID string, term *models.Term) (*models.Term, error) {
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
	clone.ID = strings.ToLower(term.Term) + "-" + userID
	r.all[userID][key] = &clone
	out := clone
	return &out, nil
}

func (r *repoStub) GetTerm(_ context.Context, userID, termID string) (*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.all[userID] {
		if t.ID == termID {
			clone := *t
			return &clone, nil
		}
	}
	return nil, ports.ErrTermNotFound
}

func (r *repoStub) UpdateTerm(_ context.Context, _ string, _ *models.Term) error { return nil }
func (r *repoStub) DeleteTerm(_ context.Context, _, _ string) error              { return nil }

func (r *repoStub) ListTerms(_ context.Context, _, _, _ string) ([]*models.Term, error) {
	return nil, nil
}

func (r *repoStub) SearchTerms(_ context.Context, userID, query string) ([]*models.Term, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := strings.ToLower(strings.TrimSpace(query))
	out := []*models.Term{}
	for _, t := range r.all[userID] {
		haystack := strings.ToLower(t.Term) + " " + strings.ToLower(t.Explanation)
		if q == "" || strings.Contains(haystack, q) {
			clone := *t
			out = append(out, &clone)
		}
	}
	return out, nil
}

// tutorHTTPFixture builds the production-equivalent HTTP stack for the
// tutor tool only — exercise the wire shape of mcp-tutor.tutor_lookup
// without dictionary-side noise.
func tutorHTTPFixture(t *testing.T) (*chi.Mux, *repoStub, *fakeValidator) {
	t.Helper()
	repo := newRepoStubForTransport()
	dictSvc := services.NewDictionaryService(repo)
	tutorSvc := services.NewTutorLookupService(dictSvc)
	adapter := NewServiceAdapter(tutorSvc)

	registry := mcpkit.NewRegistry()
	registry.Register(NewTutorLookupTool(adapter))
	validator := newFakeValidator()

	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(registry).Router(fakeAuthMiddlewareFactory(validator)))
	return r, repo, validator
}

// rpcCallTutor issues a JSON-RPC 2.0 payload for the tutor tool.
func rpcCallTutor(t *testing.T, id, name string, args any) []byte {
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

// --- Triangle 1: HTTP transport routes the tutor tool call ---------------

func TestTutorHTTPTransport_FoundTerm_ReturnsHitEnvelope(t *testing.T) {
	stack, repo, _ := tutorHTTPFixture(t)
	if _, err := repo.InsertTerm(context.Background(), "user-1", &models.Term{
		Term:        "PR",
		Explanation: "Pull Request on a code review platform.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	body := rpcCallTutor(t, "req-1", "mcp-tutor.tutor_lookup", map[string]any{"query": "PR", "level": 0})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer TOKEN_USER_1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
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
	var payload struct {
		Found         bool         `json:"found"`
		Term          *models.Term `json:"term,omitempty"`
		Framing       string       `json:"framing"`
		Source        string       `json:"source"`
		SocraticLevel int          `json:"socratic_level"`
	}
	if err := json.Unmarshal(env.Result, &payload); err != nil {
		t.Fatalf("decode result: %v (result=%s)", err, string(env.Result))
	}
	if !payload.Found {
		t.Errorf("payload.found = false, want true")
	}
	if payload.Source != "dictionary" {
		t.Errorf("payload.source = %q, want dictionary", payload.Source)
	}
	if !strings.Contains(strings.ToLower(payload.Framing), "pull request") {
		t.Errorf("payload.framing should reference the explanation, got %q", payload.Framing)
	}
	if payload.SocraticLevel != 0 {
		t.Errorf("payload.socratic_level = %d, want 0", payload.SocraticLevel)
	}
}

// --- Triangle 2: Miss term gets silent general-source fallback -----------

func TestTutorHTTPTransport_MissTerm_ReturnsGeneralEnvelopeNoLeak(t *testing.T) {
	stack, _, _ := tutorHTTPFixture(t)

	body := rpcCallTutor(t, "miss-1", "mcp-tutor.tutor_lookup", map[string]any{"query": "monad", "level": 1})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer TOKEN_USER_1")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	rspBody, _ := io.ReadAll(rr.Body)
	var env struct {
		Error  any             `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("miss should NOT raise a JSON-RPC error envelope, got %v", env.Error)
	}
	var payload struct {
		Found         bool   `json:"found"`
		Framing       string `json:"framing"`
		Source        string `json:"source"`
		SocraticLevel int    `json:"socratic_level"`
	}
	if err := json.Unmarshal(env.Result, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Found {
		t.Errorf("payload.found = true, want false on miss")
	}
	if payload.Source != "general" {
		t.Errorf("payload.source = %q, want general", payload.Source)
	}
	if payload.SocraticLevel != 1 {
		t.Errorf("payload.socratic_level = %d, want 1", payload.SocraticLevel)
	}
	if strings.Contains(strings.ToLower(payload.Framing), "dictionary") {
		t.Errorf("framing leaked 'dictionary' detail: %q", payload.Framing)
	}
}

// --- Triangle 3: Cross-user isolation via the auth chain ------------------

func TestTutorHTTPTransport_CrossUser_TermIsolationOverHTTP(t *testing.T) {
	// User-1 owns a term 'shared-name'. User-2 hits the tutor with
	// 'shared-name' — must NOT see user-1's explanation. RLS guarantee is
	// exercised end-to-end via the JWT-→-context bridge.
	stack, repo, _ := tutorHTTPFixture(t)
	if _, err := repo.InsertTerm(context.Background(), "user-1", &models.Term{
		Term:        "PR",
		Explanation: "Owned by user 1 explanation string.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("seed user-1: %v", err)
	}

	// user-2 calls the tutor with 'PR' — should NOT see user-1's term.
	body := rpcCallTutor(t, "x-user", "mcp-tutor.tutor_lookup", map[string]any{"query": "PR", "level": 0})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer TOKEN_USER_2")
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
	var payload struct {
		Found   bool   `json:"found"`
		Source  string `json:"source"`
		Framing string `json:"framing"`
	}
	if err := json.Unmarshal(env.Result, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Found {
		t.Errorf("user-2 should not see user-1's term under JWT isolation, got found=true")
	}
	if payload.Source != "general" {
		t.Errorf("user-2 miss should fall back to source=general, got %q", payload.Source)
	}
}

// --- Triangle 4: Unauthenticated request is rejected ----------------------

func TestTutorHTTPTransport_NoAuthHeader_Returns401(t *testing.T) {
	stack, _, _ := tutorHTTPFixture(t)
	body := rpcCallTutor(t, "no-auth", "mcp-tutor.tutor_lookup", map[string]any{"query": "PR", "level": 0})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — auth header required", rr.Code)
	}
}

// --- Triangle 5: /mcp/tools/list advertises the tutor tool ----------------

func TestTutorHTTPTransport_ToolsListAdvertisesTutorLookup(t *testing.T) {
	stack, _, _ := tutorHTTPFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list", nil)
	req.Header.Set("Authorization", "Bearer TOKEN_USER_1")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	var found bool
	for _, it := range items {
		if name, _ := it["name"].(string); name == "mcp-tutor.tutor_lookup" {
			found = true
			desc, _ := it["description"].(string)
			if !strings.Contains(strings.ToLower(desc), "personal tech dictionary") {
				t.Errorf("description missing dictionary context: %q", desc)
			}
			break
		}
	}
	if !found {
		t.Errorf("/mcp/tools/list did NOT advertise mcp-tutor.tutor_lookup; items=%v", items)
	}
}
