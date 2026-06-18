package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

func setupChallengeRouter(handler *ChallengeHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/challenges", handler.ListChallenges)
	r.Get("/api/v1/challenges/{id}", handler.GetChallenge)
	return r
}

func TestChallengeHandler_ListChallenges_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"}).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now).
		AddRow("ch-xss", "XSS", "desc2", "junior", "security", "typescript", "https://example.com", "code2", "XSS", "available", now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE status = 'available' ORDER BY created_at DESC`)).
		WillReturnRows(rows)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
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

	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE status = 'available' ORDER BY created_at DESC`)).
		WillReturnRows(rows)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Should be [] not null
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
	row := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"}).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE id = $1 AND status = 'available'`)).
		WithArgs("ch-sqli").
		WillReturnRows(row)

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-sqli", nil)
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
}

func TestChallengeHandler_GetChallenge_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE id = $1 AND status = 'available'`)).
		WithArgs("ch-nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"}))

	svc := services.NewChallengeService(db)
	handler := NewChallengeHandler(svc)
	r := setupChallengeRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-nonexistent", nil)
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