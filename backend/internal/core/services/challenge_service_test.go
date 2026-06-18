package services

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestChallengeService_GetAll_ReturnsChallenges(t *testing.T) {
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

	svc := NewChallengeService(db)
	challenges, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(challenges) != 2 {
		t.Fatalf("expected 2 challenges, got %d", len(challenges))
	}
	if challenges[0].ID != "ch-sqli" {
		t.Errorf("expected first challenge ID 'ch-sqli', got %q", challenges[0].ID)
	}
	if challenges[1].ID != "ch-xss" {
		t.Errorf("expected second challenge ID 'ch-xss', got %q", challenges[1].ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_GetAll_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE status = 'available' ORDER BY created_at DESC`)).
		WillReturnRows(rows)

	svc := NewChallengeService(db)
	challenges, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Service returns nil when no rows — handler converts nil to empty array
	if len(challenges) != 0 {
		t.Fatalf("expected 0 challenges, got %d", len(challenges))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_GetByID_ReturnsChallenge(t *testing.T) {
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

	svc := NewChallengeService(db)
	challenge, err := svc.GetByID(context.Background(), "ch-sqli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge.ID != "ch-sqli" {
		t.Errorf("expected ID 'ch-sqli', got %q", challenge.ID)
	}
	if challenge.Title != "SQL Injection" {
		t.Errorf("expected title 'SQL Injection', got %q", challenge.Title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_GetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at FROM public.challenges WHERE id = $1 AND status = 'available'`)).
		WithArgs("ch-nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at"}))

	svc := NewChallengeService(db)
	_, err = svc.GetByID(context.Background(), "ch-nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrChallengeNotFound {
		t.Errorf("expected ErrChallengeNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}