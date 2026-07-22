package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

func setupChallengeRouter(handler *ChallengeHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/challenges", handler.ListChallenges)
	r.Get("/api/v1/challenges/{id}", handler.GetChallenge)
	r.Post("/api/v1/challenges", handler.CreateChallenge)
	return r
}

// Helper to inject userID into request context
func reqWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), authmiddleware.UserIDContextKey{}, userID)
	return r.WithContext(ctx)
}

// challengeColumns returns the column list matching the service SELECT (without solution).
func challengeColumns() []string {
	return []string{
		"id", "title", "description", "difficulty", "category", "language",
		"repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo",
		"learning_objectives", "hints", "common_mistakes", "estimated_time_minutes",
		"expected_findings", "test_cases", "linter_rules",
		"base_points", "bonus_points", "penalty_per_hint", "time_bonus",
		"origin", "source_path", "generated_by", "created_by",
	}
}

func challengeColumnsWithSolution() []string {
	cols := challengeColumns()
	return append(cols, "solution_code", "solution_explanation")
}

var emptyJSONB = []byte("[]")

func TestChallengeHandler_ListChallenges_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	userID := "user-1"
	cols := challengeColumns()
	rows := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil).
		AddRow("ch-xss", "XSS", "desc2", "junior", "security", "typescript", "https://example.com", "code2", "XSS", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`)).
		WithArgs(userID).
		WillReturnRows(rows)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
		t.Errorf("expected 2 challenges, got %d", len(result))
	}
	if result[0].ID != "ch-sqli" {
		t.Errorf("expected first ID 'ch-sqli', got %q", result[0].ID)
	}
}

func TestChallengeHandler_ListChallenges_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	cols := challengeColumns()
	rows := sqlmock.NewRows(cols)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`)).
		WithArgs(userID).
		WillReturnRows(rows)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	userID := "user-1"
	cols := challengeColumns()
	row := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-sqli", userID).
		WillReturnRows(row)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	// Solution fields should be absent in the default response
	if result.SolutionCode != "" {
		t.Errorf("expected empty solutionCode, got %q", result.SolutionCode)
	}
}

func TestChallengeHandler_GetChallenge_RevealSolution(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	userID := "user-1"
	cols := challengeColumnsWithSolution()
	row := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil,
			"SELECT * FROM users WHERE...", "Use parameterized queries")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumnsWithSolution() + `
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-sqli", userID).
		WillReturnRows(row)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	if result.ID != "ch-sqli" {
		t.Errorf("expected ID 'ch-sqli', got %q", result.ID)
	}
	if result.SolutionCode != "SELECT * FROM users WHERE..." {
		t.Errorf("expected solutionCode 'SELECT * FROM users WHERE...', got %q", result.SolutionCode)
	}
	if result.SolutionExplanation != "Use parameterized queries" {
		t.Errorf("expected solutionExplanation, got %q", result.SolutionExplanation)
	}
}

func TestChallengeHandler_GetChallenge_RevealSolution_Unauthorized(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	// No DB query expectation: the handler should reject before hitting the service
	// when the X-Reveal-Authorized header is missing.

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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

	// Ensure no DB queries were made (handler returned before calling service)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB interaction: %v", err)
	}
}

func TestChallengeHandler_GetChallenge_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-nonexistent", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	// Expect dedup check — no existing row found
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("ggogsmic/academy-mic", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

	// Expect INSERT with RETURNING
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.challenges (
			id, title, description, difficulty, category, language, repo_url, source_repo, code, code_smell, status, user_id,
			learning_objectives, hints, common_mistakes, estimated_time_minutes,
			expected_findings, test_cases, linter_rules,
			solution_code, solution_explanation,
			base_points, bonus_points, penalty_per_hint, time_bonus,
			origin, source_path, generated_by, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		RETURNING created_at`)).
		WithArgs(
			sqlmock.AnyArg(), "Test Challenge", "A code snippet for review", "junior", "security", "typescript",
			"https://gogs.example.com/repo", "ggogsmic/academy-mic", "code", "SQL Injection", "available", userID,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 0,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", "",
			0, 0, 0, false,
			"", "", "", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()

	// Expect dedup check — existing row found
	cols := challengeColumns()
	rows := sqlmock.NewRows(cols).
		AddRow("existing-id", "Existing", "desc", "junior", "security", "typescript", "https://gogs.example.com/repo", "code", "SQL Injection", "available", now, userID, "ggogsmic/academy-mic",
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + services.ChallengeSelectColumns() + `
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("ggogsmic/academy-mic", userID).
		WillReturnRows(rows)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

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
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := services.NewChallengeService(db)
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