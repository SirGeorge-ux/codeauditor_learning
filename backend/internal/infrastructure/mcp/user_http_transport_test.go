package mcp_test

// Combined end-to-end smoke test for the mcp-user transport. Mirrors
// dictionary / challenges / curriculum http_transport_test.go patterns:
// chi + mcpkit.NewServer + fake auth + a hand-rolled fakeService
// implementing usermcp.Service from outside the package (no DB, no
// sqlmock). The test exercises the production main.go wiring for the 6
// user tools end-to-end.

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
	usermcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/user"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

type userFakeValidator struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newUserFakeValidator() *userFakeValidator {
	return &userFakeValidator{tokens: map[string]string{"TOKEN_U": "u-1"}}
}

func (f *userFakeValidator) ValidateToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.tokens[token]; !ok {
		return errors.New("invalid token")
	}
	return nil
}

func (f *userFakeValidator) UserIDFromToken(token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.tokens[token]
	if !ok {
		return "", errors.New("invalid token")
	}
	return u, nil
}

func userAuthMW(v *userFakeValidator) func(http.Handler) http.Handler {
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

// userHTTPFakeService is the deterministic stand-in for the production
// serviceAdapter. It only implements the methods the transport tests
// exercise; "unexercised" methods are kept to satisfy the interface but
// fail with a clear sentinel if a future test accidentally reaches them.
type userHTTPFakeService struct {
	mu sync.Mutex

	profileOut  *models.UserProfile
	progressOut *models.LanguageProgress
	historyOut  []models.AuditSession
	learningOut *models.LearningProfile
}

func newUserHTTPFakeService() *userHTTPFakeService {
	return &userHTTPFakeService{
		profileOut: &models.UserProfile{
			ID: "u-1", Email: "u@example.com", DisplayName: "Alice",
			RachaDias: 7, PuntosMaestria: 250, RangoActual: "C",
		},
		progressOut: &models.LanguageProgress{
			UserID: "u-1", Language: "typescript", Rango: "E", Puntos: 100,
			ChallengesCompletados: 5, ChallengesIntentados: 8, TasaExito: 0.625,
		},
		historyOut: []models.AuditSession{
			{ID: "s-1", UserID: "u-1", ChallengeTitle: "ch-xss", Language: "typescript", FindingsCount: 1},
		},
		learningOut: models.DefaultLearningProfile("u-1"),
	}
}

func (f *userHTTPFakeService) GetProfile(_ context.Context, _ string) (*models.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.profileOut, nil
}

func (f *userHTTPFakeService) GetLanguageProgress(_ context.Context, _, _ string) (*models.LanguageProgress, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.progressOut, nil
}

func (f *userHTTPFakeService) GetHistory(_ context.Context, _ string, _ int) ([]models.AuditSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.historyOut, nil
}

func (f *userHTTPFakeService) GetLearningProfile(_ context.Context, _ string) (*models.LearningProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.learningOut, nil
}

func (f *userHTTPFakeService) RecordAuditAttempt(_ context.Context, _ string, _ usermcp.RecordAuditAttemptInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return nil
}

func (f *userHTTPFakeService) UpdateLearningProfile(_ context.Context, p *models.LearningProfile) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.learningOut = p
	return nil
}

func userFixture(t *testing.T) (http.Handler, *userHTTPFakeService) {
	t.Helper()
	svc := newUserHTTPFakeService()
	mcpReg := mcpkit.NewRegistry()
	mcpReg.Register(usermcp.NewGetProfileTool(svc))
	mcpReg.Register(usermcp.NewGetLanguageProgressTool(svc))
	mcpReg.Register(usermcp.NewGetHistoryTool(svc))
	mcpReg.Register(usermcp.NewGetLearningProfileTool(svc))
	mcpReg.Register(usermcp.NewRecordAuditAttemptTool(svc))
	mcpReg.Register(usermcp.NewUpdateLearningProfileTool(svc))
	r := chi.NewRouter()
	r.Mount("/mcp", mcpkit.NewServer(mcpReg).Router(userAuthMW(newUserFakeValidator())))
	return r, svc
}

func userRPCCall(t *testing.T, id, name string, args any) []byte {
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

func TestMCPHTTPTransport_User_ListTools_IncludesAll6(t *testing.T) {
	stack, _ := userFixture(t)
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
		"mcp-user.get_profile":             false,
		"mcp-user.get_language_progress":   false,
		"mcp-user.get_history":             false,
		"mcp-user.get_learning_profile":    false,
		"mcp-user.record_audit_attempt":    false,
		"mcp-user.update_learning_profile": false,
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

func TestMCPHTTPTransport_User_GetProfile_HappyPath(t *testing.T) {
	stack, _ := userFixture(t)
	body := userRPCCall(t, "req-p", "mcp-user.get_profile", map[string]any{})
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
		t.Fatalf("expected ok, got error: %v", env.Error)
	}
	var profile map[string]any
	if err := json.Unmarshal(env.Result, &profile); err != nil {
		t.Fatalf("decode profile: %v body=%s", err, env.Result)
	}
	if got, _ := profile["email"].(string); got != "u@example.com" {
		t.Errorf("email = %q, want u@example.com", got)
	}
}

func TestMCPHTTPTransport_User_RecordAuditAttempt_HappyPath(t *testing.T) {
	stack, _ := userFixture(t)
	body := userRPCCall(t, "req-r", "mcp-user.record_audit_attempt", map[string]any{
		"language":       "typescript",
		"challenge_id":   "ch-xss",
		"score":          50,
		"completed":      true,
		"code":           "import x from 'x';",
		"findings_count": 1,
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
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("expected ok, got error: %v", env.Error)
	}
	var result map[string]any
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("decode result: %v body=%s", err, env.Result)
	}
	if result["recorded"] != true {
		t.Errorf("response = %+v, want {recorded: true}", result)
	}
}

func TestMCPHTTPTransport_User_UpdateLearningProfile_HappyPath(t *testing.T) {
	stack, svc := userFixture(t)
	args := map[string]any{
		"preferencias": map[string]any{
			"idioma":             "en",
			"nivelSocratismo":    1,
			"longitudMaximaMsg":  300,
			"bloqueosTipicos":    []string{"async"},
			"topicsQueLeCuestan": []string{"recursion"},
			"resumen":            "well-progressed",
		},
		"estiloAprendizaje": map[string]any{"aprendeMejorCon": "examples"},
	}
	body := userRPCCall(t, "req-u", "mcp-user.update_learning_profile", args)
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
	_ = json.Unmarshal(rspBody, &env)
	if env.Error != nil {
		t.Fatalf("expected ok, got error: %v", env.Error)
	}
	var result map[string]any
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("decode result: %v body=%s", err, env.Result)
	}
	pref, _ := result["preferencias"].(map[string]any)
	if pref["idioma"] != "en" {
		t.Errorf("idioma = %v, want en", pref["idioma"])
	}
	if svc.learningOut.Preferencias.NivelSocratismo != 1 {
		t.Errorf("nivel = %d, want 1", svc.learningOut.Preferencias.NivelSocratismo)
	}
}

func TestMCPHTTPTransport_User_UpdateLearningProfile_BadSocratismo_Returns422ParamsCode(t *testing.T) {
	stack, _ := userFixture(t)
	body := userRPCCall(t, "req-bad", "mcp-user.update_learning_profile", map[string]any{
		"preferencias": map[string]any{
			"idioma":            "es",
			"nivelSocratismo":   99,
			"longitudMaximaMsg": 200,
		},
	})
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
		t.Fatalf("expected RPC error for out-of-range socratismo, got success\nbody: %s", rspBody)
	}
	if env.Error.Code != mcpkit.CodeInvalidParams {
		t.Errorf("err code = %d, want %d (-32602)", env.Error.Code, mcpkit.CodeInvalidParams)
	}
}

func TestMCPHTTPTransport_User_Unauthenticated_Returns401(t *testing.T) {
	stack, _ := userFixture(t)
	body := userRPCCall(t, "req-noauth", "mcp-user.get_profile", map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/mcp/tools/call", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	stack.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}
