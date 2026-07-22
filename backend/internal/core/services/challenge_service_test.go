package services

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// Helper to build standard column list for sqlmock matching the service SELECT.
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

func TestChallengeService_GetAll_ReturnsChallenges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()
	cols := challengeColumns()
	rows := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil).
		AddRow("ch-xss", "XSS", "desc2", "junior", "security", "typescript", "https://example.com", "code2", "XSS", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`)).
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
	cols := challengeColumns()
	rows := sqlmock.NewRows(cols)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`)).
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
	cols := challengeColumns()
	row := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
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

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-nonexistent", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

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

func TestChallengeService_GetChallengeWithDetails_ReturnsChallengeWithSolution(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()
	cols := challengeColumnsWithSolution()
	row := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil,
			"SELECT * FROM users WHERE...", "Use parameterized queries")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumnsWithSolution+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-sqli", userID).
		WillReturnRows(row)

	svc := NewChallengeService(db)
	challenge, err := svc.GetChallengeWithDetails(context.Background(), "ch-sqli", userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge.ID != "ch-sqli" {
		t.Errorf("expected ID 'ch-sqli', got %q", challenge.ID)
	}
	if challenge.SolutionCode != "SELECT * FROM users WHERE..." {
		t.Errorf("expected solution_code, got %q", challenge.SolutionCode)
	}
	if challenge.SolutionExplanation != "Use parameterized queries" {
		t.Errorf("expected solution_explanation, got %q", challenge.SolutionExplanation)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_GetChallengeWithDetails_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumnsWithSolution+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-nonexistent", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumnsWithSolution()))

	svc := NewChallengeService(db)
	_, err = svc.GetChallengeWithDetails(context.Background(), "ch-nonexistent", userID)
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

func TestChallengeService_GetByID_ExcludesSolutionFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"
	now := time.Now()
	cols := challengeColumns()
	row := sqlmock.NewRows(cols).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript", "https://example.com", "code", "SQL Injection", "available", now, nil, nil,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-sqli", userID).
		WillReturnRows(row)

	svc := NewChallengeService(db)
	challenge, err := svc.GetByID(context.Background(), "ch-sqli", userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if challenge.SolutionCode != "" {
		t.Errorf("expected empty solution_code in GetByID, got %q", challenge.SolutionCode)
	}
	if challenge.SolutionExplanation != "" {
		t.Errorf("expected empty solution_explanation in GetByID, got %q", challenge.SolutionExplanation)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// validV2InputFields returns valid v2 fields for a CreateChallengeInput.
// Tests that exercise the Create method beyond the validation gate need
// these fields populated so the v2 constraints pass.
func validV2InputFields() models.CreateChallengeInput {
	return models.CreateChallengeInput{
		Title:       "Test Challenge",
		Description: "A code snippet that needs review",
		Difficulty:  "junior",
		Category:    "security",
		Language:    "typescript",
		RepoURL:     "https://example.com/repo",
		SourceRepo:  "Owner/Repo",
		Code:        "code",
		CodeSmell:   "SQL Injection",
		ExpectedFindings: []models.ExpectedFinding{
			{Severity: "high", Category: "security", Message: "issue 1"},
			{Severity: "medium", Category: "security", Message: "issue 2"},
		},
		TestCases: []models.TestCase{
			{Name: "test1", ExpectedOutput: "ok", Weight: 5},
			{Name: "test2", ExpectedOutput: "ok", Weight: 5},
		},
		Hints: []models.Hint{
			{Level: 1, Content: "hint 1", CostPoints: 0},
			{Level: 2, Content: "hint 2", CostPoints: 10},
			{Level: 3, Content: "hint 3", CostPoints: 25},
		},
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
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("owner/repo", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

	// INSERT returns created_at — JSONB fields use AnyArg since exact bytes
	// depend on json.Marshal output.
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
			sqlmock.AnyArg(), "Test Challenge", "A code snippet that needs review", "junior", "security", "typescript",
			"https://example.com/repo", "owner/repo", "code", "SQL Injection", "available", userID,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 0,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", "",
			0, 0, 0, false,
			"", "", "", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	svc := NewChallengeService(db)
	input := validV2InputFields()
	// SourceRepo set by helper to "Owner/Repo" for normalization test
	input.SourceRepo = "Owner/Repo"

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

	cols := challengeColumns()
	rows := sqlmock.NewRows(cols).
		AddRow("existing-id", "Existing", "desc", "junior", "security", "typescript", "https://gogs.example.com/repo", "code", "SQL Injection", "available", now, userID, existingSourceRepo,
			emptyJSONB, emptyJSONB, emptyJSONB, 15, emptyJSONB, emptyJSONB, emptyJSONB,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("ggogsmic/academy-mic", userID).
		WillReturnRows(rows)

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.Title = "Existing"
	input.RepoURL = "https://gogs.example.com/repo"
	input.SourceRepo = "  GgogsMIC/academy-mic  " // whitespace + mixed case
	input.Description = "A code snippet for review"

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
	input := validV2InputFields()
	input.Difficulty = "expert" // invalid

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for invalid difficulty, got nil")
	}
	if err != ErrInvalidDifficulty {
		t.Errorf("expected ErrInvalidDifficulty, got %v", err)
	}
}

func TestChallengeService_Create_TooFewFindings(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.ExpectedFindings = []models.ExpectedFinding{} // 0 findings

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for 0 expected_findings, got nil")
	}
	if err != ErrInvalidExpectedFindings {
		t.Errorf("expected ErrInvalidExpectedFindings, got %v", err)
	}
}

func TestChallengeService_Create_TooManyFindings(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.ExpectedFindings = []models.ExpectedFinding{
		{Severity: "high", Category: "security", Message: "1"},
		{Severity: "high", Category: "security", Message: "2"},
		{Severity: "high", Category: "security", Message: "3"},
		{Severity: "high", Category: "security", Message: "4"}, // exceeds max
	}

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for 4 expected_findings, got nil")
	}
	if err != ErrInvalidExpectedFindings {
		t.Errorf("expected ErrInvalidExpectedFindings, got %v", err)
	}
}

func TestChallengeService_Create_TooFewTestCases(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.TestCases = []models.TestCase{
		{Name: "test1", ExpectedOutput: "ok", Weight: 5}, // only 1
	}

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for 1 test_case, got nil")
	}
	if err != ErrInvalidTestCases {
		t.Errorf("expected ErrInvalidTestCases, got %v", err)
	}
}

func TestChallengeService_Create_InvalidHintsCount(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.Hints = []models.Hint{
		{Level: 1, Content: "hint 1", CostPoints: 0},
		{Level: 2, Content: "hint 2", CostPoints: 10}, // only 2
	}

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for 2 hints, got nil")
	}
	if err != ErrInvalidHintsCount {
		t.Errorf("expected ErrInvalidHintsCount, got %v", err)
	}
}

func TestChallengeService_Create_SpoilerInDescription(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	svc := NewChallengeService(db)
	input := validV2InputFields()
	input.Description = "This challenge involves SQL Injection in the code."

	_, _, err = svc.Create(context.Background(), input, "user-1")
	if err == nil {
		t.Fatal("expected error for spoiler in description, got nil")
	}
	if err != ErrSpoilerInDescription {
		t.Errorf("expected ErrSpoilerInDescription, got %v", err)
	}
}

func TestChallengeService_Create_ValidV2Challenge(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	userID := "user-1"

	// Dedup check returns no rows
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("owner/repo", userID).
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

	// INSERT returns created_at
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
			sqlmock.AnyArg(), "Test Challenge", "A code snippet that needs review", "junior", "security", "typescript",
			"https://example.com/repo", "owner/repo", "code", "SQL Injection", "available", userID,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 0,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			"", "",
			0, 0, 0, false,
			"", "", "", "",
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	svc := NewChallengeService(db)
	input := validV2InputFields()

	challenge, created, err := svc.Create(context.Background(), input, userID)
	if err != nil {
		t.Fatalf("expected success for valid v2 challenge, got error: %v", err)
	}
	if !created {
		t.Error("expected created=true for valid new v2 challenge")
	}
	if challenge.ID == "" {
		t.Error("expected non-empty challenge ID")
	}
	// Verify v2 fields are echoed back
	if len(challenge.ExpectedFindings) != 2 {
		t.Errorf("expected 2 expected_findings, got %d", len(challenge.ExpectedFindings))
	}
	if len(challenge.TestCases) != 2 {
		t.Errorf("expected 2 test_cases, got %d", len(challenge.TestCases))
	}
	if len(challenge.Hints) != 3 {
		t.Errorf("expected 3 hints, got %d", len(challenge.Hints))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestChallengeService_GetByID_OtherUsersPrivate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// user-1 tries to access user-2's private challenge — should get not found
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`)).
		WithArgs("ch-private", "user-1").
		WillReturnRows(sqlmock.NewRows(challengeColumns()))

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