package supabase

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// analysisColumns is the projection used by every read path. Keep
// in sync with the adapter's analysisSelectColumns constant.
var analysisColumns = []string{
	"id", "user_id", "source", "owner", "name", "branch", "status",
	"health_score", "top_issues", "languages", "summary", "error_message",
	"created_at", "completed_at", "expires_at",
}

// newCodeHealthRepoForTest wires a sqlmock-backed *sql.DB into the
// adapter. The returned closer MUST be deferred by the caller.
func newCodeHealthRepoForTest(t *testing.T) (*SupabaseCodeHealthRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	return NewSupabaseCodeHealthRepository(db), mock, func() { _ = db.Close() }
}

// jsonRaw is a tiny helper that returns a json.RawMessage so the
// sqlmock AddRow accepts JSONB values without sqlmock rejecting
// the raw []byte.
func jsonRaw(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// TestSupabaseCodeHealthRepository_ImplementsPort is the compile-time
// guard: the adapter must satisfy ports.CodeHealthRepository. If a
// method signature drifts the test fails to compile.
func TestSupabaseCodeHealthRepository_ImplementsPort(t *testing.T) {
	var _ ports.CodeHealthRepository = (*SupabaseCodeHealthRepository)(nil)
}

// --- InsertAnalysis ---

func TestCodeHealthRepo_InsertAnalysis_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	analysisID := "11111111-1111-1111-1111-111111111111"
	created := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	expires := created.Add(24 * time.Hour)

	in := &models.CodeHealthAnalysis{
		UserID:    userID,
		Source:    models.SourceGogs,
		Owner:     "ggogsmic",
		Name:      "academy-mic",
		Branch:    "master",
		Status:    models.CodeHealthStatusPending,
		Languages: map[string]float64{},
		TopIssues: []models.CodeHealthIssue{},
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.code_health_analyses`)).
		WithArgs(
			userID, string(in.Source), in.Owner, in.Name, in.Branch,
			string(in.Status),
			jsonRaw(in.TopIssues), jsonRaw(in.Languages),
		).
		WillReturnRows(sqlmock.NewRows(analysisColumns).
			AddRow(analysisID, userID, string(in.Source), in.Owner, in.Name, in.Branch,
				string(in.Status), nil, jsonRaw(in.TopIssues), jsonRaw(in.Languages),
				nil, nil, created, nil, expires))

	got, err := repo.InsertAnalysis(context.Background(), in)
	if err != nil {
		t.Fatalf("InsertAnalysis: %v", err)
	}
	if got.ID != analysisID {
		t.Errorf("ID = %q, want %q", got.ID, analysisID)
	}
	if got.UserID != userID {
		t.Errorf("UserID = %q, want %q", got.UserID, userID)
	}
	if got.Status != models.CodeHealthStatusPending {
		t.Errorf("Status = %q, want pending", got.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- UpdateStatus ---

func TestCodeHealthRepo_UpdateStatus_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.code_health_analyses`)).
		WithArgs(string(models.CodeHealthStatusRunning), "", analysisID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), analysisID, userID, string(models.CodeHealthStatusRunning), ""); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

func TestCodeHealthRepo_UpdateStatus_FailedWithErrorMessage(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	errMsg := "Gogs unreachable"
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.code_health_analyses`)).
		WithArgs(string(models.CodeHealthStatusFailed), errMsg, analysisID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), analysisID, userID, string(models.CodeHealthStatusFailed), errMsg); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
}

func TestCodeHealthRepo_UpdateStatus_NotFound(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.code_health_analyses`)).
		WithArgs(string(models.CodeHealthStatusRunning), "", analysisID, userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateStatus(context.Background(), analysisID, userID, string(models.CodeHealthStatusRunning), "")
	if !errors.Is(err, ports.ErrCodeHealthAnalysisNotFound) {
		t.Fatalf("UpdateStatus 0-rows = %v, want ErrCodeHealthAnalysisNotFound", err)
	}
}

// --- Complete ---

func TestCodeHealthRepo_Complete_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	score := 85
	topIssues := []models.CodeHealthIssue{
		{Rule: "RULE-SEC-SECRET-001", Severity: models.SeverityCritical, Category: models.CategorySecurity, FilePath: "src/main.go", Line: 1, Message: "secret", Confidence: 1.0},
	}
	languages := map[string]float64{"go": 0.7, "ts": 0.3}
	summary := "1 critical issue found"

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE public.code_health_analyses`)).
		WithArgs(
			string(models.CodeHealthStatusCompleted), score,
			jsonRaw(topIssues), jsonRaw(languages), summary,
			analysisID, userID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Complete(context.Background(), analysisID, userID, score, topIssues, languages, summary); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- FindByID ---

func TestCodeHealthRepo_FindByID_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	completed := created.Add(5 * time.Minute)
	expires := created.Add(24 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.code_health_analyses`)).
		WithArgs(analysisID, userID).
		WillReturnRows(sqlmock.NewRows(analysisColumns).
			AddRow(analysisID, userID, "gogs", "ggogsmic", "academy-mic", "master",
				"completed", 85, jsonRaw([]any{}), jsonRaw(map[string]any{"go": 0.7}),
				"summary", nil, created, completed, expires))

	got, err := repo.FindByID(context.Background(), analysisID, userID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ID != analysisID {
		t.Errorf("ID = %q, want %q", got.ID, analysisID)
	}
	if got.HealthScore == nil || *got.HealthScore != 85 {
		t.Errorf("HealthScore = %v, want 85", got.HealthScore)
	}
	if got.Status != models.CodeHealthStatusCompleted {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

func TestCodeHealthRepo_FindByID_NotFound(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.code_health_analyses`)).
		WithArgs(analysisID, userID).
		WillReturnRows(sqlmock.NewRows(analysisColumns))

	_, err := repo.FindByID(context.Background(), analysisID, userID)
	if !errors.Is(err, ports.ErrCodeHealthAnalysisNotFound) {
		t.Fatalf("FindByID no-rows = %v, want ErrCodeHealthAnalysisNotFound", err)
	}
}

// --- ListHistory ---

func TestCodeHealthRepo_ListHistory_ReturnsRanked(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	created := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	expires := created.Add(24 * time.Hour)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.code_health_analyses`)).
		WithArgs(userID, "ggogsmic", "academy-mic", "gogs", 10).
		WillReturnRows(sqlmock.NewRows(analysisColumns).
			AddRow("a1", userID, "gogs", "ggogsmic", "academy-mic", "master",
				"completed", 78, jsonRaw([]any{}), jsonRaw(map[string]any{"go": 0.6}),
				"summary-1", nil, created, created.Add(time.Minute), expires).
			AddRow("a2", userID, "gogs", "ggogsmic", "academy-mic", "master",
				"completed", 85, jsonRaw([]any{}), jsonRaw(map[string]any{"go": 0.7}),
				"summary-2", nil, created.Add(-time.Hour), created.Add(-time.Hour).Add(time.Minute), expires))

	got, err := repo.ListHistory(context.Background(), "ggogsmic", "academy-mic", "gogs", userID, 10)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListHistory len = %d, want 2", len(got))
	}
	if got[0].ID != "a1" {
		t.Errorf("ListHistory[0].ID = %q, want a1 (newest first)", got[0].ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- CountSince ---

func TestCodeHealthRepo_CountSince_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	midnight := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT`)).
		WithArgs(userID, midnight).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	got, err := repo.CountSince(context.Background(), userID, midnight)
	if err != nil {
		t.Fatalf("CountSince: %v", err)
	}
	if got != 3 {
		t.Errorf("CountSince = %d, want 3", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- InsertIssue ---

func TestCodeHealthRepo_InsertIssue_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	issueID := "22222222-2222-2222-2222-222222222222"
	created := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	in := &models.CodeHealthIssue{
		AnalysisID: analysisID,
		Severity:   models.SeverityCritical,
		Category:   models.CategorySecurity,
		FilePath:   "src/main.go",
		Line:       1,
		Message:    "secret leak",
		Evidence:   "api_key = \"abc123\"",
		Rule:       "RULE-SEC-SECRET-001",
		Confidence: 1.0,
		SHA:        "blob-main",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.code_health_issues`)).
		WithArgs(
			analysisID, string(in.Severity), string(in.Category), in.FilePath, in.Line,
			in.Message, in.Evidence, nil, in.Rule, in.Confidence, in.SHA,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(issueID, created))

	got, err := repo.InsertIssue(context.Background(), in)
	if err != nil {
		t.Fatalf("InsertIssue: %v", err)
	}
	if got.ID != issueID {
		t.Errorf("InsertIssue ID = %q, want %q", got.ID, issueID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- ListIssues ---

func TestCodeHealthRepo_ListIssues_FilterBySeverity(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	analysisID := "11111111-1111-1111-1111-111111111111"
	created := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "analysis_id", "severity", "category", "file_path", "line",
		"message", "evidence", "suggested_fix", "rule", "confidence", "sha", "created_at",
	}).AddRow(
		"i1", analysisID, "critical", "security", "src/main.go", 1,
		"secret", "ev", "fix", "RULE-SEC-SECRET-001", 1.0, "blob-1", created,
	).AddRow(
		"i2", analysisID, "critical", "security", "src/auth.go", 14,
		"another", "ev", "fix", "RULE-SEC-INJECT-001", 1.0, "blob-2", created,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.code_health_issues`)).
		WithArgs(analysisID, "critical", 50).
		WillReturnRows(rows)

	got, err := repo.ListIssues(context.Background(), analysisID, "critical", 50)
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListIssues len = %d, want 2", len(got))
	}
	for _, iss := range got {
		if iss.Severity != models.SeverityCritical {
			t.Errorf("expected severity=critical, got %q", iss.Severity)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- InsertSuppression ---

func TestCodeHealthRepo_InsertSuppression_DedupIsNoOp(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	// Nil reason: pass nil directly (the adapter handles nil
	// reason as a NULL in the INSERT).
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.code_health_suppressions`)).
		WithArgs(userID, "src/main.go", "RULE-SEC-SECRET-001", 1, nil).
		WillReturnError(&pq.Error{Code: "23505"}) // unique_violation

	// Spec contract: a duplicate suppression insert is a no-op
	// (idempotent), NOT an error. The driving layer MUST NOT see
	// a conflict surfaced to the user.
	err := repo.InsertSuppression(context.Background(), userID, "src/main.go", "RULE-SEC-SECRET-001", 1, nil)
	if err != nil {
		t.Fatalf("InsertSuppression duplicate = %v, want nil (no-op)", err)
	}
}

func TestCodeHealthRepo_InsertSuppression_HappyPath(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	reason := "false positive"
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.code_health_suppressions`)).
		WithArgs(userID, "src/main.go", "RULE-SEC-SECRET-001", 1, &reason).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.InsertSuppression(context.Background(), userID, "src/main.go", "RULE-SEC-SECRET-001", 1, &reason)
	if err != nil {
		t.Fatalf("InsertSuppression: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// --- IsSuppressed ---

func TestCodeHealthRepo_IsSuppressed_Miss(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM public.code_health_suppressions`)).
		WithArgs(userID, "src/main.go", "RULE-SEC-SECRET-001", 1).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	got, err := repo.IsSuppressed(context.Background(), userID, "src/main.go", "RULE-SEC-SECRET-001", 1)
	if err != nil {
		t.Fatalf("IsSuppressed: %v", err)
	}
	if got {
		t.Errorf("IsSuppressed = true, want false (no row)")
	}
}

func TestCodeHealthRepo_IsSuppressed_Hit(t *testing.T) {
	repo, mock, closer := newCodeHealthRepoForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT 1 FROM public.code_health_suppressions`)).
		WithArgs(userID, "src/main.go", "RULE-SEC-SECRET-001", 1).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	got, err := repo.IsSuppressed(context.Background(), userID, "src/main.go", "RULE-SEC-SECRET-001", 1)
	if err != nil {
		t.Fatalf("IsSuppressed: %v", err)
	}
	if !got {
		t.Errorf("IsSuppressed = false, want true (row present)")
	}
}
