package handlers

// Tests for the challenge HTTP handler. Mirrors the previous
// sqlmock-backed pattern, but with a hand-rolled in-memory fake port
// instead of go-sqlmock — the service no longer takes *sql.DB, so the
// fixture changes from "fake DB" to "fake ports.ChallengeRepository".
// Each test configures only the methods it needs.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

func setupChallengeRouter(handler *ChallengeHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/challenges", handler.ListChallenges)
	r.Get("/api/v1/challenges/{id}", handler.GetChallenge)
	r.Post("/api/v1/challenges", handler.CreateChallenge)
	return r
}

// reqWithUserID injects the userID into the request context so the
// handler can authorize the read/write.
func reqWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), authmiddleware.UserIDContextKey{}, userID)
	return r.WithContext(ctx)
}

// --- Fake port -----------------------------------------------------------

// fakeChallengeRepo is a hand-rolled, deterministic,
// ports.ChallengeRepository implementation. Each method has a
// pre-canned response that the test configures before constructing
// the service. The fake records every call so tests can assert on
// the (ctx, args) tuple the handler ultimately passes through.
type fakeChallengeRepo struct {
	mu sync.Mutex

	// List response
	listOut []models.Challenge
	listErr error

	// FindByID (no solution) response
	findByIDOut models.Challenge
	findByIDErr error

	// FindByID with solution response
	findByIDWithSolutionOut models.Challenge
	findByIDWithSolutionErr error

	// FindBySourceRepoAndUser response
	findBySourceOut *models.Challenge
	findBySourceErr error

	// Insert response
	insertCreatedAt time.Time
	insertErr       error
}

func (f *fakeChallengeRepo) ListVisibleToUser(_ context.Context, _ string) ([]models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listOut, f.listErr
}

func (f *fakeChallengeRepo) FindByIDVisibleToUser(_ context.Context, _, _ string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.findByIDOut, f.findByIDErr
}

func (f *fakeChallengeRepo) FindByIDWithSolutionVisibleToUser(_ context.Context, _, _ string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.findByIDWithSolutionOut, f.findByIDWithSolutionErr
}

func (f *fakeChallengeRepo) FindBySourceRepoAndUser(_ context.Context, _, _ string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findBySourceErr != nil {
		return models.Challenge{}, f.findBySourceErr
	}
	if f.findBySourceOut != nil {
		return *f.findBySourceOut, nil
	}
	return models.Challenge{}, ports.ErrChallengeNotFound
}

func (f *fakeChallengeRepo) Insert(_ context.Context, c *models.Challenge) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.insertErr != nil {
		return time.Time{}, f.insertErr
	}
	if !f.insertCreatedAt.IsZero() {
		c.CreatedAt = f.insertCreatedAt
	}
	return f.insertCreatedAt, nil
}

// Compile-time guard.
var _ ports.ChallengeRepository = (*fakeChallengeRepo)(nil)

// --- Tests ---------------------------------------------------------------

func TestChallengeHandler_ListChallenges_Success(t *testing.T) {
	now := time.Now()
	repo := &fakeChallengeRepo{
		listOut: []models.Challenge{
			{ID: "ch-sqli", Title: "SQL Injection", Difficulty: "junior", CreatedAt: now},
			{ID: "ch-xss", Title: "XSS", Difficulty: "junior", CreatedAt: now},
		},
	}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result []models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 challenges, got %d", len(result))
	}
	if result[0].ID != "ch-sqli" {
		t.Errorf("expected first ID 'ch-sqli', got %q", result[0].ID)
	}
}

func TestChallengeHandler_ListChallenges_EmptyResult(t *testing.T) {
	repo := &fakeChallengeRepo{listOut: []models.Challenge{}}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body == "null" {
		t.Error("expected empty array [], got null")
	}

	var result []models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 challenges, got %d", len(result))
	}
}

func TestChallengeHandler_GetChallenge_Success(t *testing.T) {
	repo := &fakeChallengeRepo{
		findByIDOut: models.Challenge{
			ID:         "ch-sqli",
			Title:      "SQL Injection",
			Difficulty: "junior",
		},
	}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-sqli", nil)
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.ID != "ch-sqli" {
		t.Errorf("expected ID 'ch-sqli', got %q", result.ID)
	}
	if result.Title != "SQL Injection" {
		t.Errorf("expected title 'SQL Injection', got %q", result.Title)
	}
	if result.SolutionCode != "" {
		t.Errorf("expected empty solutionCode, got %q", result.SolutionCode)
	}
}

func TestChallengeHandler_GetChallenge_RevealSolution(t *testing.T) {
	repo := &fakeChallengeRepo{
		findByIDWithSolutionOut: models.Challenge{
			ID:                  "ch-sqli",
			Title:               "SQL Injection",
			SolutionCode:        "SELECT * FROM users WHERE...",
			SolutionExplanation: "Use parameterized queries",
		},
	}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-sqli?reveal_solution=true", nil)
	req.Header.Set("X-Reveal-Authorized", "true")
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.SolutionCode != "SELECT * FROM users WHERE..." {
		t.Errorf("expected solutionCode, got %q", result.SolutionCode)
	}
	if result.SolutionExplanation != "Use parameterized queries" {
		t.Errorf("expected solutionExplanation, got %q", result.SolutionExplanation)
	}
}

// TestChallengeHandler_GetChallenge_RevealSolution_Unauthorized
// proves the handler rejects solution reveal without the
// X-Reveal-Authorized header BEFORE the service is invoked.
func TestChallengeHandler_GetChallenge_RevealSolution_Unauthorized(t *testing.T) {
	repo := &fakeChallengeRepo{
		// Set findByIDWithSolution to a sentinel that the test will
		// never see — the handler must short-circuit on the
		// missing header. If the handler reaches the service the
		// fake would return ErrChallengeNotFound, which would
		// surface as 404 (not 403), so the assertion below is
		// sufficient to prove short-circuiting.
		findByIDWithSolutionErr: ports.ErrChallengeNotFound,
	}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-sqli?reveal_solution=true", nil)
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp["error"] != "Solution reveal requires authorization" {
		t.Errorf("expected error 'Solution reveal requires authorization', got %q", errResp["error"])
	}
}

func TestChallengeHandler_GetChallenge_NotFound(t *testing.T) {
	repo := &fakeChallengeRepo{findByIDErr: ports.ErrChallengeNotFound}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-nonexistent", nil)
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp["error"] != "Challenge not found" {
		t.Errorf("expected error 'Challenge not found', got %q", errResp["error"])
	}
}

func TestChallengeHandler_CreateChallenge_Success(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeChallengeRepo{insertCreatedAt: now}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	body := `{"title":"Test Challenge","description":"A code snippet for review","difficulty":"junior","category":"security","language":"typescript","repoUrl":"https://gogs.example.com/repo","sourceRepo":"GgogsMIC/academy-mic","code":"code","codeSmell":"SQL Injection","expectedFindings":[{"severity":"high","category":"security","message":"issue1"},{"severity":"medium","category":"security","message":"issue2"}],"testCases":[{"name":"t1","expected_output":"ok","weight":5},{"name":"t2","expected_output":"ok","weight":5}],"hints":[{"level":1,"content":"h1","cost_points":0},{"level":2,"content":"h2","cost_points":10},{"level":3,"content":"h3","cost_points":25}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var result models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Title != "Test Challenge" {
		t.Errorf("expected title 'Test Challenge', got %q", result.Title)
	}
}

func TestChallengeHandler_CreateChallenge_Dedup(t *testing.T) {
	now := time.Now()
	repo := &fakeChallengeRepo{
		findBySourceOut: &models.Challenge{
			ID:         "existing-id",
			Title:      "Existing",
			Difficulty: "junior",
			Category:   "security",
			Language:   "typescript",
			SourceRepo: "ggogsmic/academy-mic",
			CreatedAt:  now,
			UserID:     stringPtr("user-1"),
		},
	}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	body := `{"title":"Existing","description":"A code snippet for review","difficulty":"junior","category":"security","language":"typescript","repoUrl":"https://gogs.example.com/repo","sourceRepo":"GgogsMIC/academy-mic","code":"code","codeSmell":"SQL Injection","expectedFindings":[{"severity":"high","category":"security","message":"issue1"},{"severity":"medium","category":"security","message":"issue2"}],"testCases":[{"name":"t1","expected_output":"ok","weight":5},{"name":"t2","expected_output":"ok","weight":5}],"hints":[{"level":1,"content":"h1","cost_points":0},{"level":2,"content":"h2","cost_points":10},{"level":3,"content":"h3","cost_points":25}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for dedup, got %d; body: %s", w.Code, w.Body.String())
	}

	var result models.Challenge
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.ID != "existing-id" {
		t.Errorf("expected existing challenge ID 'existing-id', got %q", result.ID)
	}
}

func TestChallengeHandler_CreateChallenge_InvalidDifficulty(t *testing.T) {
	repo := &fakeChallengeRepo{}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	body := `{"title":"Test","description":"desc","difficulty":"expert","category":"security","language":"typescript","repoUrl":"https://example.com","sourceRepo":"owner/repo","code":"code","codeSmell":"smell"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserID(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp["error"] == "" {
		t.Error("expected error message in response")
	}
}

// TestChallengeHandler_CreateChallenge_DBError proves an error from
// the repository surface as 500, not 400 — the validation gate has
// already passed.
func TestChallengeHandler_CreateChallenge_DBError(t *testing.T) {
	repo := &fakeChallengeRepo{insertErr: errors.New("db down")}
	svc := services.NewChallengeService(repo)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	userID := "user-1"
	body := `{"title":"Test","description":"desc","difficulty":"junior","category":"security","language":"typescript","repoUrl":"https://example.com","sourceRepo":"owner/repo","code":"code","codeSmell":"smell","expectedFindings":[{"severity":"high","category":"security","message":"1"}],"testCases":[{"name":"t1","expected_output":"ok","weight":5},{"name":"t2","expected_output":"ok","weight":5}],"hints":[{"level":1,"content":"h1","cost_points":0},{"level":2,"content":"h2","cost_points":10},{"level":3,"content":"h3","cost_points":25}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = reqWithUserID(req, userID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d; body: %s", w.Code, w.Body.String())
	}
}

// stringPtr returns a pointer to the supplied string. Used in test
// fixtures that need a *string for Challenge.UserID.
func stringPtr(s string) *string { return &s }
