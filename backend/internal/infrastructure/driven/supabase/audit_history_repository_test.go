package supabase

// Tests for SupabaseAuditHistoryRepository. Mirrors the
// dictionary_repository_test.go pattern: a sqlmock-backed
// database/sql.DB drives the adapter through the same SQL the
// production Supabase connection would see. These tests verify the
// column projections and SQL translation; the in-memory service-level
// fakes in audit_history_service_test.go verify the business rules.

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// auditSessionColumnsForTest matches the SELECT projection used by the
// adapter. Kept in sync with the adapter's ListByUser query.
var auditSessionColumnsForTest = []string{
	"id", "user_id", "challenge_title", "language", "code_snippet", "findings_count", "created_at",
}

func newAuditHistoryRepoForTest(t *testing.T) (*SupabaseAuditHistoryRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	return NewSupabaseAuditHistoryRepository(db), mock, func() { _ = db.Close() }
}

// --- Compile-time interface conformance ---

func TestSupabaseAuditHistoryRepository_ImplementsPort(t *testing.T) {
	var _ ports.AuditHistoryRepository = (*SupabaseAuditHistoryRepository)(nil)
}

// --- Save ---

func TestAuditHistoryRepo_Save_HappyPath(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	userID := "user-1"
	session := models.AuditSession{
		UserID:         userID,
		ChallengeTitle: "ch-sqli",
		Language:       "typescript",
		CodeSnippet:    "console.log(1);",
		FindingsCount:  3,
	}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.audit_sessions (user_id, challenge_title, language, code_snippet, findings_count)
		 VALUES ($1, $2, $3, $4, $5)`)).
		WithArgs(userID, "ch-sqli", "typescript", "console.log(1);", 3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Save(context.Background(), session); err != nil {
		t.Fatalf("Save unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestAuditHistoryRepo_Save_DBErrorPropagates(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.audit_sessions`)).
		WillReturnError(errFakeDB)

	if err := repo.Save(context.Background(), models.AuditSession{UserID: "u"}); err == nil {
		t.Fatal("Save returned nil error, want non-nil")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// errFakeDB is a sentinel error the sqlmock driver returns when the
// adapter's INSERT path needs to verify error propagation.
var errFakeDB = sqlmock.ErrCancelled

// --- ListByUser ---

func TestAuditHistoryRepo_ListByUser_ReturnsRows(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	now := time.Now()
	rows := sqlmock.NewRows(auditSessionColumnsForTest).
		AddRow("sess-1", "user-1", "ch-1", "typescript", "code", 2, now).
		AddRow("sess-2", "user-1", "ch-2", "go", "code2", 0, now.Add(-time.Hour))

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, challenge_title, language, code_snippet, findings_count, created_at
		 FROM public.audit_sessions
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`)).
		WithArgs("user-1", 20).
		WillReturnRows(rows)

	got, err := repo.ListByUser(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("ListByUser unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != "sess-1" {
		t.Errorf("got[0].ID = %q, want sess-1", got[0].ID)
	}
	if got[0].FindingsCount != 2 {
		t.Errorf("got[0].FindingsCount = %d, want 2", got[0].FindingsCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestAuditHistoryRepo_ListByUser_EmptyResult(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, challenge_title, language, code_snippet, findings_count, created_at`)).
		WithArgs("user-1", 20).
		WillReturnRows(sqlmock.NewRows(auditSessionColumnsForTest))

	got, err := repo.ListByUser(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("ListByUser unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

// --- GetStats ---

func TestAuditHistoryRepo_GetStats_NonEmpty(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*), COALESCE(SUM(findings_count), 0)
		 FROM public.audit_sessions WHERE user_id = $1`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(5, 12))

	totalAudits, totalFindings, err := repo.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetStats unexpected error: %v", err)
	}
	if totalAudits != 5 {
		t.Errorf("totalAudits = %d, want 5", totalAudits)
	}
	if totalFindings != 12 {
		t.Errorf("totalFindings = %d, want 12", totalFindings)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestAuditHistoryRepo_GetStats_EmptyHistory(t *testing.T) {
	repo, mock, closer := newAuditHistoryRepoForTest(t)
	defer closer()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*), COALESCE(SUM(findings_count), 0)`)).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count", "sum"}).AddRow(0, 0))

	totalAudits, totalFindings, err := repo.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetStats unexpected error: %v", err)
	}
	if totalAudits != 0 {
		t.Errorf("totalAudits = %d, want 0", totalAudits)
	}
	if totalFindings != 0 {
		t.Errorf("totalFindings = %d, want 0", totalFindings)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock expectations: %v", err)
	}
}
