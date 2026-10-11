package supabase

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

// newAnalysisCacheForTest wires a sqlmock-backed *sql.DB into the
// SupabaseAnalysisCache adapter. The closer MUST be deferred.
func newAnalysisCacheForTest(t *testing.T) (*SupabaseAnalysisCache, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	return NewSupabaseAnalysisCache(db), mock, func() { _ = db.Close() }
}

// TestSupabaseAnalysisCache_ImplementsPort is the compile-time
// guard. The adapter must satisfy ports.AnalysisCache.
func TestSupabaseAnalysisCache_ImplementsPort(t *testing.T) {
	var _ ports.AnalysisCache = (*SupabaseAnalysisCache)(nil)
}

// TestAnalysisCache_Get_Hit covers the spec scenario "Cache hit
// returns prior result" (REQ-CODE-HEALTH-009). The adapter MUST
// apply the `WHERE expires_at > NOW()` filter so expired rows are
// silently treated as misses.
func TestAnalysisCache_Get_Hit(t *testing.T) {
	repo, mock, closer := newAnalysisCacheForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	fileSHA := "blob-main"

	rows := sqlmock.NewRows([]string{"result"}).
		AddRow(jsonRaw([]models.CodeHealthIssue{
			{Rule: "RULE-SEC-SECRET-001", Severity: models.SeverityCritical, Category: models.CategorySecurity, FilePath: "src/main.go", Line: 1, Message: "secret", Confidence: 1.0},
		}))

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.analysis_cache`)).
		WithArgs(userID, fileSHA).
		WillReturnRows(rows)

	got, hit, err := repo.Get(context.Background(), userID, fileSHA)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !hit {
		t.Errorf("Get hit = false, want true")
	}
	if len(got) != 1 {
		t.Fatalf("Get issues len = %d, want 1", len(got))
	}
	if got[0].Rule != "RULE-SEC-SECRET-001" {
		t.Errorf("Get issues[0].Rule = %q, want RULE-SEC-SECRET-001", got[0].Rule)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// TestAnalysisCache_Get_Miss covers the spec scenario "Cache miss
// stores new result" (REQ-CODE-HEALTH-009). sql.ErrNoRows is
// returned when the row is absent OR expired; the adapter
// translates both into (nil, false, nil).
func TestAnalysisCache_Get_Miss(t *testing.T) {
	repo, mock, closer := newAnalysisCacheForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	fileSHA := "blob-missing"

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.analysis_cache`)).
		WithArgs(userID, fileSHA).
		WillReturnRows(sqlmock.NewRows([]string{"result"}))

	got, hit, err := repo.Get(context.Background(), userID, fileSHA)
	if err != nil {
		t.Fatalf("Get miss: %v", err)
	}
	if hit {
		t.Errorf("Get miss: hit = true, want false")
	}
	if got != nil {
		t.Errorf("Get miss: got = %v, want nil", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// TestAnalysisCache_Get_DatabaseError surfaces the failure mode
// where the underlying DB call fails. The adapter returns the
// error unchanged so the service can decide whether to skip the
// cache read or surface the failure to the user.
func TestAnalysisCache_Get_DatabaseError(t *testing.T) {
	repo, mock, closer := newAnalysisCacheForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	fileSHA := "blob-1"

	mock.ExpectQuery(regexp.QuoteMeta(`FROM public.analysis_cache`)).
		WithArgs(userID, fileSHA).
		WillReturnError(errors.New("connection refused"))

	_, _, err := repo.Get(context.Background(), userID, fileSHA)
	if err == nil {
		t.Fatal("expected error from DB failure")
	}
}

// TestAnalysisCache_Set_HappyPath covers the spec scenario "Cache
// miss stores new result" (REQ-CODE-HEALTH-009) — the adapter
// MUST write the row with `expires_at = now() + ttl` (the 7-day
// default per the spec).
func TestAnalysisCache_Set_HappyPath(t *testing.T) {
	repo, mock, closer := newAnalysisCacheForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	fileSHA := "blob-main"
	issues := []models.CodeHealthIssue{
		{Rule: "RULE-PERF-N1-001", Severity: models.SeverityHigh, Category: models.CategoryPerformance, FilePath: "src/main.go", Line: 1, Message: "n+1", Confidence: 1.0},
	}
	ttl := 7 * 24 * time.Hour

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.analysis_cache`)).
		WithArgs(userID, fileSHA, jsonRaw(issues), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Set(context.Background(), userID, fileSHA, issues, ttl); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled sqlmock: %v", err)
	}
}

// TestAnalysisCache_Set_OverwriteOnConflict covers the
// force-reanalyze path: when the cache key already exists, the
// adapter MUST overwrite the row (not append or fail with a
// unique-violation). Spec REQ-CODE-HEALTH-009 "Re-analyze CTA
// bypasses cache" scenario.
func TestAnalysisCache_Set_OverwriteOnConflict(t *testing.T) {
	repo, mock, closer := newAnalysisCacheForTest(t)
	defer closer()

	userID := "00000000-0000-0000-0000-000000000001"
	fileSHA := "blob-main"
	issues := []models.CodeHealthIssue{}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO public.analysis_cache`)).
		WithArgs(userID, fileSHA, jsonRaw(issues), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Set(context.Background(), userID, fileSHA, issues, 7*24*time.Hour); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
}
