package mcp_test

// Combined end-to-end smoke test for the mcp-challenges transport. Mirrors
// the dictionary http_transport_test.go and the sandbox http_transport_test
// layout: chi + mcpkit.NewServer + fake auth + a hand-rolled fake Service
// implementing challengesmcp.Service from outside the package (no DB, no
// sqlmock). The point of this file is to prove the production main.go wiring
// for the 5 challenges tools is structurally correct.
//
// PR 2 amendment: the previous sqlmock-backed fixture leaked database/sql +
// DATA-DOG/go-sqlmock into the MCP test layer; the spec's "transport tests
// do not touch the DB" assumption was therefore violated. This refactor
// replaces it with a deterministic fake that satisfies challengesmcp.Service
// directly — same JSON-RPC paths exercised, none of them via a real driver.

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
	challengesmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/challenges"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

type challengesFakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newChallengesFakeValidator() *challengesFakeValidator {
	return &challengesFakeValidator{tokens: map[string]string{"TOKEN_U": "u-1"}}
}

func (f *challengesFakeValidator) ValidateToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tokens[token]; !ok {
		return errors.New("invalid token")
	}
	return nil
}

func (f *challengesFakeValidator) UserIDFromToken(token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.tokens[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return u, nil
}

func challengesAuthMW(v *challengesFakeValidator) func(http.Handler) http.Handler {
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

// challengesHTTPFakeService is a deterministic stand-in for the production
// *services.ChallengeService -> challengesmcp.NewServiceAdapter chain. The
// HTTP transport tests only need:
//   - ListChallenges: a stable non-empty JSON array for /mcp/tools/list
//   - GetChallenge:   not exercised by these tests
//   - CreateTempChallenge: not exercised by these tests
//   - GenerateChallenge: configured error (the S5 stub sentinel)
//   - ValidateChallenge: a known good ValidateResult for the happy path
//
// Each test sets only the knobs it exercises; default values make
// "untouched" methods safe-but-obviously-unused so a future test that
// accidentally invokes one fails with a clear sentinel rather than a
// silent success.
type challengesHTTPFakeService struct {
	mu sync.Mutex

	// Knobs exercised by these tests.
	generateErr error
	validateOut challengesmcp.ValidateResult
}

func newChallengesHTTPFakeService() *challengesHTTPFakeService {
	return &challengesHTTPFakeService{
		// Mirror the production adapter's stub: when the service
		// returns ErrChallengeGenerateNotImplemented, the dispatcher
		// maps it to JSON-RPC -32603. The package-level ErrNotImplemented
		// sentinel in challengesmcp carries the same dispatch behavior,
		// so we use it directly — no reason to round-trip through a
		// service import that the spec says the transport layer should
		// not depend on.
		generateErr: challengesmcp.ErrNotImplemented,
	}
}

func (f *challengesHTTPFakeService) ListChallenges(_ context.Context, _ string) ([]json.RawMessage, error) {
	// /mcp/tools/list never invokes Execute, but the registry iterates
	// tools, not services. Returning an empty slice is safe and
	// avoids any test that *does* invoke Execute depending on DB-shaped
	// data.
	return []json.RawMessage{}, nil
}

func (f *challengesHTTPFakeService) GetChallenge(_ context.Context, _, _ string, _ bool) (json.RawMessage, error) {
	return nil, challengesmcp.ErrChallengeNotFound
}

func (f *challengesHTTPFakeService) CreateTempChallenge(_ context.Context, _ string, _ models.CreateChallengeInput) (json.RawMessage, error) {
	// Not exercised by these tests. If a future test accidentally calls
	// this, the explicit sentinel surfaces the regression rather than
	// silent data corruption from a fake that "succeeds".
	return nil, errors.New("challengesHTTPFakeService.CreateTempChallenge: not exercised by HTTP transport tests")
}

func (f *challengesHTTPFakeService) GenerateChallenge(_ context.Context, _ string, _ challengesmcp.GenerateHint) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return nil, f.generateErr
}

func (f *challengesHTTPFakeService) ValidateChallenge(_ context.Context, _ string, _ models.CreateChallengeInput) (challengesmcp.ValidateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.validateOut, nil
}

// challengesFixture wires the production-equivalent HTTP stack — same chi +
// mcpkit shape as main.go — but with a hand-rolled fake Service instead of
// a sqlmock-backed *services.ChallengeService. The returned Service lets
// individual tests configure GenerateErr / ValidateOut without re-creating
// the fixture.
func challengesFixture(t *testing.T) (http.Handler, *challengesHTTPFakeService) {
	t.Helper()
	svc := newChallengesHTTPFakeService()
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(challengesmcp.NewListChallengesTool(svc))
	mcpReg.Register(challengesmcp.NewGetChallengeTool(svc))
	mcpReg.Register(challengesmcp.NewCreateTempChallengeTool(svc))
	mcpReg.Register(challengesmcp.NewGenerateChallengeTool(svc))
	mcpReg.Register(challengesmcp.NewValidateChallengeTool(svc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(challengesAuthMW(newChallengesFakeValidator())))
	return r, svc
}

func challengesRPCCall(t *testing.T, id, name string, args any) []byte {
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

func TestMCPHTTPTransport_Challenges_ListTools_IncludesAll5(t *testing.T) {
	stack, _ := challengesFixture(t)
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
		"mcp-challenges.list_challenges":       false,
		"mcp-challenges.get_challenge":         false,
		"mcp-challenges.create_temp_challenge": false,
		"mcp-challenges.generate_challenge":    false,
		"mcp-challenges.validate_challenge":    false,
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

func TestMCPHTTPTransport_Challenges_Generate_InternalErrorOverHTTP(t *testing.T) {
	stack, _ := challengesFixture(t)
	body := challengesRPCCall(t, "req-g", "mcp-challenges.generate_challenge", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 RPC envelope, got %d", rr.Code)
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
		t.Fatalf("expected RPC error from generate stub")
	}
	if env.Error.Code != mcpkit.CodeInternalError {
		t.Errorf("err code = %d, want %d (-32603)",
			env.Error.Code, mcpkit.CodeInternalError)
	}
}

func TestMCPHTTPTransport_Challenges_Validate_HappyPathOverHTTP(t *testing.T) {
	stack, svc := challengesFixture(t)
	// Configure the fake to mimic a successful v2 validation result.
	// A production v2 pass with no violations is {Valid:true, Errors:[]};
	// we set it explicitly so this test is hermetic.
	svc.validateOut = challengesmcp.ValidateResult{Valid: true, Errors: []string{}}

	body := challengesRPCCall(t, "req-v", "mcp-challenges.validate_challenge", map[string]any{
		"title":            "T",
		"description":      "long enough description here",
		"difficulty":       "mid",
		"language":         "typescript",
		"expectedFindings": []map[string]any{{"severity": "low", "category": "smell", "message": "x"}},
		"testCases":        []map[string]any{{"name": "t1"}, {"name": "t2"}},
		"hints":            []map[string]any{{"level": 1}, {"level": 2}, {"level": 3}},
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
		Error  any             `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(rspBody, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error != nil {
		t.Fatalf("expected success, got error: %v", env.Error)
	}
	var result map[string]any
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("decode result: %v body=%s", err, env.Result)
	}
	if valid, _ := result["valid"].(bool); !valid {
		t.Errorf("expected valid=true, got %+v", result)
	}
}

func TestMCPHTTPTransport_Challenges_Unauthenticated_Request_Returns401(t *testing.T) {
	stack, _ := challengesFixture(t)
	body := challengesRPCCall(t, "req-noauth", "mcp-challenges.generate_challenge", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
