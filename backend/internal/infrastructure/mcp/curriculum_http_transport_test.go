package mcp_test

// Combined end-to-end smoke test for the mcp-curriculum transport. Mirrors
// dictionary http_transport_test.go and challenges_http_transport_test.go
// patterns: chi + mcpkit.NewServer + fake auth + a hand-rolled fakeService
// implementing curriculummcp.Service from outside the package (no DB, no
// filesystem). The test exercises the production main.go wiring for the 4
// curriculum tools.

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
	curriculummcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/curriculum"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

type curriculumFakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newCurriculumFakeValidator() *curriculumFakeValidator {
	return &curriculumFakeValidator{tokens: map[string]string{"TOKEN_U": "u-1"}}
}

func (f *curriculumFakeValidator) ValidateToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tokens[token]; !ok {
		return errors.New("invalid token")
	}
	return nil
}

func (f *curriculumFakeValidator) UserIDFromToken(token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.tokens[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return u, nil
}

func curriculumAuthMW(v *curriculumFakeValidator) func(http.Handler) http.Handler {
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

// curriculumHTTPFakeService is a deterministic stand-in for the production
// *services.CurriculumService -> curriculummcp.NewServiceAdapter chain.
// The tests exercise only the happy path; the path-traversal case is
// covered in the tool unit tests (curriculum_tool_test.go) — these
// transport tests prove the production wiring plumbs user_id + the JSON
// envelope through the full HTTP stack.
type curriculumHTTPFakeService struct {
	mu sync.Mutex

	getTopicsOut []services.Topic
	getTopicOut  services.TopicDetail
	suggestOut   services.Topic
	reviewOut    services.ReviewTopicResult
}

func newCurriculumHTTPFakeService() *curriculumHTTPFakeService {
	return &curriculumHTTPFakeService{
		getTopicsOut: []services.Topic{
			{ID: "variables", Title: "Variables", Difficulty: "F"},
			{ID: "types", Title: "Types", Difficulty: "E"},
		},
		getTopicOut: services.TopicDetail{
			ID: "variables", Title: "Variables", Difficulty: "F",
			Description: "Block.",
		},
		suggestOut: services.Topic{ID: "types", Title: "Types", Difficulty: "E"},
		reviewOut:  services.ReviewTopicResult{Reviewed: true},
	}
}

func (f *curriculumHTTPFakeService) GetTopics(_ context.Context, _ services.GetTopicsInput) ([]services.Topic, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getTopicsOut, nil
}

func (f *curriculumHTTPFakeService) GetTopic(_ context.Context, _, _ string) (services.TopicDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getTopicOut, nil
}

func (f *curriculumHTTPFakeService) SuggestTopic(_ context.Context, _ services.SuggestTopicInput) (services.Topic, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.suggestOut, nil
}

func (f *curriculumHTTPFakeService) ReviewTopic(_ context.Context, _ services.ReviewTopicInput) (services.ReviewTopicResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reviewOut, nil
}

// curriculumFixture wires the production-equivalent HTTP stack — same chi +
// mcpkit shape as main.go — but with a hand-rolled fake Service instead of
// a filesystem-backed *services.CurriculumService.
func curriculumFixture(t *testing.T) (http.Handler, *curriculumHTTPFakeService) {
	t.Helper()
	svc := newCurriculumHTTPFakeService()
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(curriculummcp.NewGetTopicsTool(svc))
	mcpReg.Register(curriculummcp.NewGetTopicTool(svc))
	mcpReg.Register(curriculummcp.NewSuggestTopicTool(svc))
	mcpReg.Register(curriculummcp.NewReviewTopicTool(svc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(curriculumAuthMW(newCurriculumFakeValidator())))
	return r, svc
}

func curriculumRPCCall(t *testing.T, id, name string, args any) []byte {
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

func TestMCPHTTPTransport_Curriculum_ListTools_IncludesAll4(t *testing.T) {
	stack, _ := curriculumFixture(t)
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
		"mcp-curriculum.get_topics":    false,
		"mcp-curriculum.get_topic":     false,
		"mcp-curriculum.suggest_topic": false,
		"mcp-curriculum.review_topic":  false,
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

func TestMCPHTTPTransport_Curriculum_GetTopics_HappyPath(t *testing.T) {
	stack, _ := curriculumFixture(t)
	body := curriculumRPCCall(t, "req-g", "mcp-curriculum.get_topics", map[string]any{
		"language": "typescript",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
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
		t.Fatalf("expected ok, got error: %v", env.Error)
	}
	var hits []map[string]any
	if err := json.Unmarshal(env.Result, &hits); err != nil {
		t.Fatalf("decode result: %v body=%s", err, env.Result)
	}
	if len(hits) != 2 {
		t.Errorf("len(hits) = %d, want 2", len(hits))
	}
}

func TestMCPHTTPTransport_Curriculum_PathTraversal_ReturnsInvalidParams(t *testing.T) {
	// Spec scenario: "Path Traversal in Curricula". Over HTTP the same
	// guard must trip before any filesystem read.
	// Reconfigure the fake to return the path-traversal sentinel so the
	// transport test exercises the production mapping end-to-end.
	// The fixture's fakeService is shared; swap its behaviour for this
	// single test by replacing the registry: easiest is to build a new
	// stack with a sentinel-returning fake.
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(curriculummcp.NewGetTopicsTool(curriculumTraversalFake{}))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(curriculumAuthMW(newCurriculumFakeValidator())))

	body := curriculumRPCCall(t, "req-trav", "mcp-curriculum.get_topics", map[string]any{
		"language": "../etc",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("X-Test-Token", "TOKEN_U")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

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
		t.Fatalf("expected RPC error for path traversal, got success\nbody: %s", rspBody)
	}
	if env.Error.Code != mcpkit.CodeInvalidParams {
		t.Errorf("err code = %d, want %d (-32602)", env.Error.Code, mcpkit.CodeInvalidParams)
	}
}

func TestMCPHTTPTransport_Curriculum_Unauthenticated_Request_Returns401(t *testing.T) {
	stack, _ := curriculumFixture(t)
	body := curriculumRPCCall(t, "req-noauth", "mcp-curriculum.get_topics", map[string]any{
		"language": "typescript",
	})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauth, got %d", rr.Code)
	}
}

// curriculumTraversalFake is the focused fixture for the path-traversal
// transport test. It hard-wires ErrCurriculumInvalidLanguage on GetTopics
// so the dispatcher maps to -32602 without depending on a real
// filesystem or the production service.
type curriculumTraversalFake struct{}

func (curriculumTraversalFake) GetTopics(_ context.Context, _ services.GetTopicsInput) ([]services.Topic, error) {
	return nil, services.ErrCurriculumInvalidLanguage
}

func (curriculumTraversalFake) GetTopic(_ context.Context, _, _ string) (services.TopicDetail, error) {
	return services.TopicDetail{}, services.ErrCurriculumInvalidLanguage
}

func (curriculumTraversalFake) SuggestTopic(_ context.Context, _ services.SuggestTopicInput) (services.Topic, error) {
	return services.Topic{}, services.ErrCurriculumInvalidLanguage
}

func (curriculumTraversalFake) ReviewTopic(_ context.Context, _ services.ReviewTopicInput) (services.ReviewTopicResult, error) {
	return services.ReviewTopicResult{}, services.ErrCurriculumInvalidLanguage
}
