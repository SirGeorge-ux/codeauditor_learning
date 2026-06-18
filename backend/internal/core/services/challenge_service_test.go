package services

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

func TestChallengeService_GetAll_ReturnsChallenges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil).
		AddRow("ch-xss", "XSS", "desc2", "junior", "security", "typescript", "https://example.com", "code2", "XSS", "available", now, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE status = 'available' AND (user_id IS NULL OR user_id = $1) ORDER BY created_at DESC`)).
		WithArgs(userID).
		WillReturnRows(rows)

	svc := NewChallengeService(db)
	challenges, err := svc.GetAll(context.Background(), userID)
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

	userID := "user-1"
	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE status = 'available' AND (user_id IS NULL OR user_id = $1) ORDER BY created_at DESC`)).
		WithArgs(userID).
		WillReturnRows(rows)

	svc := NewChallengeService(db)
	challenges, err := svc.GetAll(context.Background(), userID)
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

	userID := "user-1"
	now := time.Now()
	row := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-sqli", userID).
		WillReturnRows(row)

	svc := NewChallengeService(db)
	challenge, err := svc.GetByID(context.Background(), "ch-sqli", userID)
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

	userID := "user-1"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-nonexistent", userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}))

	svc := NewChallengeService(db)
	_, err = svc.GetByID(context.Background(), "ch-nonexistent", userID)
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

func TestChallengeService_Create_NewChallenge(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	// Dedup check returns no rows
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("owner/repo", userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}))

	// INSERT returns created_at
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.challenges (id, title, description, difficulty, category, language, repo_url, source_repo, code, code_smell, status, user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING created_at`)).
		WithArgs(sqlmock.AnyArg(), "Test Challenge", "desc", "junior", "security", "typescript", "https://example.com/repo", "owner/repo", "code", "SQL Injection", "available", userID).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	svc := NewChallengeService(db)
	input := models.CreateChallengeInput{
		Title:       "Test Challenge",
		Description: "desc",
		Difficulty:  "junior",
		Category:    "security",
		Language:    "typescript",
		RepoURL:     "https://example.com/repo",
		SourceRepo:  "Owner/Repo", // should be normalized
		Code:        "code",
		CodeSmell:   "SQL Injection",
	}

	challenge, created, err := svc.Create(context.Background(), input, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new challenge")
	}
	if challenge.Title != "Test Challenge" {
		t.Errorf("expected title 'Test Challenge', got %q", challenge.Title)
	}
	if challenge.SourceRepo != "owner/repo" {
		t.Errorf("expected normalized source_repo 'owner/repo', got %q", challenge.SourceRepo)
	}
	if challenge.UserID == nil || *challenge.UserID != userID {
		t.Errorf("expected userID %q, got %v", userID, challenge.UserID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_Create_DedupReturnsExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()
	existingSourceRepo := "ggogsmic/academy-mic"

	// Dedup check returns existing row
	rows := sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}).
		AddRow("existing-id", "Existing", "desc", "junior", "security", "typescript", "https://gogs.example.com/repo", "code", "SQL Injection", "available", now, userID, existingSourceRepo)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("ggogsmic/academy-mic", userID).
		WillReturnRows(rows)

	svc := NewChallengeService(db)
	input := models.CreateChallengeInput{
		Title:       "Existing",
		Description: "desc",
		Difficulty:  "junior",
		Category:    "security",
		Language:    "typescript",
		RepoURL:     "https://gogs.example.com/repo",
		SourceRepo:  "  GgogsMIC/academy-mic  ", // whitespace + mixed case
		Code:        "code",
		CodeSmell:   "SQL Injection",
	}

	challenge, created, err := svc.Create(context.Background(), input, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false for dedup hit")
	}
	if challenge.ID != "existing-id" {
		t.Errorf("expected existing challenge ID 'existing-id', got %q", challenge.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_Create_InvalidDifficulty(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := models.CreateChallengeInput{
		Title:       "Test",
		Description: "desc",
		Difficulty:  "expert", // invalid
		Category:    "security",
		Language:    "typescript",
		RepoURL:     "https://example.com",
		SourceRepo:  "owner/repo",
		Code:        "code",
		CodeSmell:   "smell",
	}

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for invalid difficulty, got nil")
	}
	if err != ErrInvalidDifficulty {
		t.Errorf("expected ErrInvalidDifficulty, got %v", err)
	}
}

func TestChallengeService_GetByID_OtherUsersPrivate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// user-1 tries to access user-2's private challenge — should get not found
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo FROM public.challenges WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-private", "user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "difficulty", "category", "language", "repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo"}))

	svc := NewChallengeService(db)
	_, err = svc.GetByID(context.Background(), "ch-private", "user-1")
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