package supabase

// Tests for SupabaseChallengeRepository. Mirrors the
// dictionary_repository_test.go pattern: a sqlmock-backed
// database/sql.DB drives the adapter through the same SQL the
// production Supabase connection would see. These tests verify the
// column projections, NULL handling, and SQLSTATE / ErrNoRows
// translations are correct; the in-memory service-level fakes in
// challenge_service_test.go verify the business rules.

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// challengeColumnsForTest matches the SELECT projection used by the
// adapter. Kept in sync with the adapter's challengeSelectColumns
// constant.
var challengeColumnsForTest = []string{
	"id", "title", "description", "difficulty", "category", "language",
	"repo_url", "code", "code_smell", "status", "created_at", "user_id", "source_repo",
	"learning_objectives", "hints", "common_mistakes", "estimated_time_minutes",
	"expected_findings", "test_cases", "linter_rules",
	"base_points", "bonus_points", "penalty_per_hint", "time_bonus",
	"origin", "source_path", "generated_by", "created_by",
}

func challengeColumnsWithSolutionForTest() []string {
	cols := append([]string{}, challengeColumnsForTest...)
	return append(cols, "solution_code", "solution_explanation")
}

var emptyJSONBForTest = []byte("[]")

// newChallengeRepoForTest builds a SupabaseChallengeRepository backed
// by a sqlmock-backed database/sql.DB. The returned closer MUST be
// deferred.
func newChallengeRepoForTest(t *testing.T) (*SupabaseChallengeRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	return NewSupabaseChallengeRepository(db), mock, func() { _ = db.Close() }
}

// --- Compile-time interface conformance ---

func TestSupabaseChallengeRepository_ImplementsPort(t *testing.T) {
	var _ ports.ChallengeRepository = (*SupabaseChallengeRepository)(nil)
}

// --- FindBySourceRepoAndUser (dedup probe) ---

func TestChallengeRepo_FindBySourceRepoAndUser_Hit(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	userID := "user-1"
	now := time.Now()
	row := sqlmock.NewRows(challengeColumnsForTest).
		AddRow("ch-sqli", "SQL Injection", "desc", "junior", "security", "typescript",
			"https://example.com", "code", "SQL Injection", "available", now, userID, "owner/repo",
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest, 15,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest,
			100, 50, 0, false,
			"curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`)).
		WithArgs("owner/repo", userID).
		WillReturnRows(row)

	got, err := repo.FindBySourceRepoAndUser(context.Background(), "owner/repo", userID)
	if err != nil {
		t.Fatalf("FindBySourceRepoAndUser unexpected error: %v", err)
	}
	if got.ID != "ch-sqli" {
		t.Errorf("ID = %q, want ch-sqli", got.ID)
	}
	if got.SourceRepo != "owner/repo" {
		t.Errorf("SourceRepo = %q, want owner/repo", got.SourceRepo)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestChallengeRepo_FindBySourceRepoAndUser_NotFound(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns)).
		WithArgs("missing/repo", "user-1").
		WillReturnRows(sqlmock.NewRows(challengeColumnsForTest))

	_, err := repo.FindBySourceRepoAndUser(context.Background(), "missing/repo", "user-1")
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		t.Fatalf("err = %v, want ports.ErrChallengeNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// --- Insert ---

func TestChallengeRepo_Insert_ReturnsCreatedAt(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := &models.Challenge{
		ID:                 "ch-new",
		Title:              "Test",
		Description:        "desc",
		Difficulty:         "junior",
		Category:           "security",
		Language:           "typescript",
		RepoURL:            "https://example.com",
		SourceRepo:         "owner/repo",
		Code:               "code",
		CodeSmell:          "SQL Injection",
		Status:             "available",
		LearningObjectives: []string{},
		Hints:              []models.Hint{},
		CommonMistakes:     []string{},
		ExpectedFindings:   []models.ExpectedFinding{},
		TestCases:          []models.TestCase{},
		LinterRules:        []models.LinterRule{},
		BasePoints:         100,
		BonusPoints:        50,
		PenaltyPerHint:     0,
		TimeBonus:          false,
	}
	uid := "user-1"
	c.UserID = &uid

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.challenges`)).
		WithArgs(
			c.ID, c.Title, c.Description, c.Difficulty, c.Category, c.Language,
			c.RepoURL, c.SourceRepo, c.Code, c.CodeSmell, c.Status, uid,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), c.EstimatedTimeMinutes,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			c.SolutionCode, c.SolutionExplanation,
			c.BasePoints, c.BonusPoints, c.PenaltyPerHint, c.TimeBonus,
			c.Origin, c.SourcePath, c.GeneratedBy, c.CreatedBy,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	got, err := repo.Insert(context.Background(), c)
	if err != nil {
		t.Fatalf("Insert unexpected error: %v", err)
	}
	if !got.Equal(now) {
		t.Errorf("createdAt = %v, want %v", got, now)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// --- ListVisibleToUser ---

func TestChallengeRepo_ListVisibleToUser_ReturnsRows(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	now := time.Now()
	rows := sqlmock.NewRows(challengeColumnsForTest).
		AddRow("ch-1", "T1", "d", "junior", "security", "typescript", "", "c", "smell", "available", now, nil, nil,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest, 15,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest,
			100, 50, 0, false, "curated", nil, nil, nil).
		AddRow("ch-2", "T2", "d", "junior", "security", "typescript", "", "c", "smell", "available", now, nil, nil,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest, 15,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + challengeSelectColumns + `
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`)).
		WithArgs("user-1").
		WillReturnRows(rows)

	got, err := repo.ListVisibleToUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListVisibleToUser unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != "ch-1" || got[1].ID != "ch-2" {
		t.Errorf("ids = (%q, %q), want (ch-1, ch-2)", got[0].ID, got[1].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestChallengeRepo_ListVisibleToUser_EmptyResult(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT ` + challengeSelectColumns)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows(challengeColumnsForTest))

	got, err := repo.ListVisibleToUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListVisibleToUser unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (nil slice is acceptable)", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// --- FindByIDVisibleToUser ---

func TestChallengeRepo_FindByIDVisibleToUser_Hit(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	now := time.Now()
	row := sqlmock.NewRows(challengeColumnsForTest).
		AddRow("ch-1", "T1", "d", "junior", "security", "typescript", "", "c", "smell", "available", now, nil, nil,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest, 15,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest,
			100, 50, 0, false, "curated", nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2`)).
		WithArgs("ch-1", "user-1").
		WillReturnRows(row)

	got, err := repo.FindByIDVisibleToUser(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("FindByIDVisibleToUser unexpected error: %v", err)
	}
	if got.ID != "ch-1" {
		t.Errorf("ID = %q, want ch-1", got.ID)
	}
	// FindByIDVisibleToUser must not include solution fields.
	if got.SolutionCode != "" {
		t.Errorf("SolutionCode = %q, want empty (no solution fields)", got.SolutionCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestChallengeRepo_FindByIDVisibleToUser_NotFound(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumns)).
		WithArgs("missing", "user-1").
		WillReturnRows(sqlmock.NewRows(challengeColumnsForTest))

	_, err := repo.FindByIDVisibleToUser(context.Background(), "missing", "user-1")
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		t.Fatalf("err = %v, want ports.ErrChallengeNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// --- FindByIDWithSolutionVisibleToUser ---

func TestChallengeRepo_FindByIDWithSolutionVisibleToUser_IncludesSolution(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	now := time.Now()
	row := sqlmock.NewRows(challengeColumnsWithSolutionForTest()).
		AddRow("ch-1", "T1", "d", "junior", "security", "typescript", "", "c", "smell", "available", now, nil, nil,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest, 15,
			emptyJSONBForTest, emptyJSONBForTest, emptyJSONBForTest,
			100, 50, 0, false, "curated", nil, nil, nil,
			"SELECT * FROM users WHERE...", "Use parameterized queries")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumnsWithSolution+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2`)).
		WithArgs("ch-1", "user-1").
		WillReturnRows(row)

	got, err := repo.FindByIDWithSolutionVisibleToUser(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("FindByIDWithSolutionVisibleToUser unexpected error: %v", err)
	}
	if got.SolutionCode != "SELECT * FROM users WHERE..." {
		t.Errorf("SolutionCode = %q, want SELECT * FROM users WHERE...", got.SolutionCode)
	}
	if got.SolutionExplanation != "Use parameterized queries" {
		t.Errorf("SolutionExplanation = %q, want 'Use parameterized queries'", got.SolutionExplanation)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestChallengeRepo_FindByIDWithSolutionVisibleToUser_NotFound(t *testing.T) {
	repo, mock, closer := newChallengeRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `+challengeSelectColumnsWithSolution)).
		WithArgs("missing", "user-1").
		WillReturnRows(sqlmock.NewRows(challengeColumnsWithSolutionForTest()))

	_, err := repo.FindByIDWithSolutionVisibleToUser(context.Background(), "missing", "user-1")
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		t.Fatalf("err = %v, want ports.ErrChallengeNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}
