package mcp_test

// Combined end-to-end smoke test for the mcp-challenges transport. Mirrors
// the dictionary http_transport_test.go and the sandbox_http_transport_test
// layout: chi + mcpkit.NewServer + fake auth + sqlmock-backed ChallengeService.
// The point of this file is to prove the production main.go wiring for the
// 5 challenges tools is structurally correct.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
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

func challengesFixture(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	svc := services.NewChallengeService(db)
	mcpSvc := challengesmcp.NewServiceAdapter(svc)
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(challengesmcp.NewListChallengesTool(mcpSvc))
	mcpReg.Register(challengesmcp.NewGetChallengeTool(mcpSvc))
	mcpReg.Register(challengesmcp.NewCreateTempChallengeTool(mcpSvc))
	mcpReg.Register(challengesmcp.NewGenerateChallengeTool(mcpSvc))
	mcpReg.Register(challengesmcp.NewValidateChallengeTool(mcpSvc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(challengesAuthMW(newChallengesFakeValidator())))
	return r, db
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
	stack, db := challengesFixture(t)
	defer db.Close()
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
		"mcp-challenges.list_challenges":     false,
		"mcp-challenges.get_challenge":       false,
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
	stack, db := challengesFixture(t)
	defer db.Close()
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
	stack, db := challengesFixture(t)
	defer db.Close()
	body := challengesRPCCall(t, "req-v", "mcp-challenges.validate_challenge", map[string]any{
		"title":             "T",
		"description":       "long enough description here",
		"difficulty":        "mid",
		"language":          "typescript",
		"expectedFindings":  []map[string]any{{"severity": "low", "category": "smell", "message": "x"}},
		"testCases":         []map[string]any{{"name": "t1"}, {"name": "t2"}},
		"hints":             []map[string]any{{"level": 1}, {"level": 2}, {"level": 3}},
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
		Error any             `json:"error"`
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
	stack, db := challengesFixture(t)
	defer db.Close()
	body := challengesRPCCall(t, "req-noauth", "mcp-challenges.generate_challenge", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
