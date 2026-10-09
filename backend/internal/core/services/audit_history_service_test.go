package services

// Tests for AuditHistoryService. Mirrors the audit_service_test.go
// pattern: hand-rolled, deterministic, in-memory fake port so the
// service can be exercised end-to-end without a real database. The
// SQL is verified separately in
// backend/internal/infrastructure/driven/supabase/audit_history_repository_test.go
// via go-sqlmock against the production adapter.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Fake port ---

// fakeAuditHistoryRepo is a minimal ports.AuditHistoryRepository used
// to drive AuditHistoryService without a real database. Each method
// has pre-canned behavior + recorded calls so tests can assert on
// the exact (ctx, args) tuple the service passes through.
type fakeAuditHistoryRepo struct {
	mu sync.Mutex

	// saveCalls: every session the service handed to Save.
	saveCalls []models.AuditSession
	// saveErr: when non-nil, returned from Save.
	saveErr error

	// listOut / listErr: response for ListByUser.
	listOut []models.AuditSession
	listErr error
	// listCalls: every (ctx, userID, limit) tuple.
	listCalls []fakeAuditListCall

	// statsOut: the (totalAudits, totalFindings) pair returned by GetStats.
	statsTotalAudits   int
	statsTotalFindings int
	// statsErr: when non-nil, returned from GetStats.
	statsErr error
	// statsCalls: every (ctx, userID) tuple.
	statsCalls []string
}

type fakeAuditListCall struct {
	userID string
	limit  int
}

func (f *fakeAuditHistoryRepo) Save(_ context.Context, s models.AuditSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls = append(f.saveCalls, s)
	return f.saveErr
}

func (f *fakeAuditHistoryRepo) ListByUser(_ context.Context, userID string, limit int) ([]models.AuditSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls = append(f.listCalls, fakeAuditListCall{userID, limit})
	return f.listOut, f.listErr
}

func (f *fakeAuditHistoryRepo) GetStats(_ context.Context, userID string) (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statsCalls = append(f.statsCalls, userID)
	return f.statsTotalAudits, f.statsTotalFindings, f.statsErr
}

// Compile-time guard: fakeAuditHistoryRepo must satisfy the port.
var _ ports.AuditHistoryRepository = (*fakeAuditHistoryRepo)(nil)

// --- Tests ---

// TestAuditHistoryService_SaveSession_DerivesTitle proves the
// service uses the request's ChallengeID as the title when one is
// supplied.
func TestAuditHistoryService_SaveSession_DerivesTitle(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{}
	svc := NewAuditHistoryService(repo)

	err := svc.SaveSession(
		context.Background(), "user-1",
		models.AuditRequest{ChallengeID: "ch-sqli", Language: "typescript", Code: "x"},
		3,
	)
	if err != nil {
		t.Fatalf("SaveSession unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saveCalls) != 1 {
		t.Fatalf("Save calls = %d, want 1", len(repo.saveCalls))
	}
	got := repo.saveCalls[0]
	if got.ChallengeTitle != "ch-sqli" {
		t.Errorf("ChallengeTitle = %q, want ch-sqli", got.ChallengeTitle)
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", got.UserID)
	}
	if got.FindingsCount != 3 {
		t.Errorf("FindingsCount = %d, want 3", got.FindingsCount)
	}
	if got.Language != "typescript" {
		t.Errorf("Language = %q, want typescript", got.Language)
	}
}

// TestAuditHistoryService_SaveSession_CustomAuditTitle proves the
// service falls back to "Custom Audit" when ChallengeID is empty.
func TestAuditHistoryService_SaveSession_CustomAuditTitle(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{}
	svc := NewAuditHistoryService(repo)

	err := svc.SaveSession(
		context.Background(), "user-1",
		models.AuditRequest{Language: "go", Code: "x"},
		0,
	)
	if err != nil {
		t.Fatalf("SaveSession unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.saveCalls[0].ChallengeTitle != "Custom Audit" {
		t.Errorf("ChallengeTitle = %q, want Custom Audit", repo.saveCalls[0].ChallengeTitle)
	}
}

// TestAuditHistoryService_SaveSession_TruncatesSnippet proves the
// service caps CodeSnippet at 2000 chars so the DB row stays
// bounded.
func TestAuditHistoryService_SaveSession_TruncatesSnippet(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{}
	svc := NewAuditHistoryService(repo)

	longCode := strings.Repeat("a", 5000)
	err := svc.SaveSession(
		context.Background(), "user-1",
		models.AuditRequest{ChallengeID: "ch-1", Language: "go", Code: longCode},
		0,
	)
	if err != nil {
		t.Fatalf("SaveSession unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saveCalls[0].CodeSnippet) != 2000 {
		t.Errorf("CodeSnippet len = %d, want 2000 (truncated)", len(repo.saveCalls[0].CodeSnippet))
	}
}

// TestAuditHistoryService_SaveSession_DoesNotTruncateShortSnippet
// proves short snippets are stored as-is (no off-by-one in the
// truncation check).
func TestAuditHistoryService_SaveSession_DoesNotTruncateShortSnippet(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{}
	svc := NewAuditHistoryService(repo)

	shortCode := strings.Repeat("a", 1500)
	err := svc.SaveSession(
		context.Background(), "user-1",
		models.AuditRequest{ChallengeID: "ch-1", Language: "go", Code: shortCode},
		0,
	)
	if err != nil {
		t.Fatalf("SaveSession unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saveCalls[0].CodeSnippet) != 1500 {
		t.Errorf("CodeSnippet len = %d, want 1500 (no truncation)", len(repo.saveCalls[0].CodeSnippet))
	}
}

// TestAuditHistoryService_SaveSession_PropagatesDBError proves a
// real repository error surfaces to the caller so the audit flow
// can branch on it.
func TestAuditHistoryService_SaveSession_PropagatesDBError(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{saveErr: errors.New("db down")}
	svc := NewAuditHistoryService(repo)

	err := svc.SaveSession(
		context.Background(), "user-1",
		models.AuditRequest{ChallengeID: "ch-1", Language: "go", Code: "x"},
		0,
	)
	if err == nil {
		t.Fatal("SaveSession returned nil error, want non-nil")
	}
}

// TestAuditHistoryService_GetHistory_DefaultLimit proves
// GetHistory defaults limit to 20 when called with a non-positive
// value. The fake records the limit it received so the test can
// assert on it.
func TestAuditHistoryService_GetHistory_DefaultLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{listOut: nil}
	svc := NewAuditHistoryService(repo)

	_, err := svc.GetHistory(context.Background(), "user-1", 0)
	if err != nil {
		t.Fatalf("GetHistory unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.listCalls) != 1 {
		t.Fatalf("ListByUser calls = %d, want 1", len(repo.listCalls))
	}
	if repo.listCalls[0].limit != 20 {
		t.Errorf("ListByUser limit = %d, want 20 (default)", repo.listCalls[0].limit)
	}
}

func TestAuditHistoryService_GetHistory_NegativeLimitAlsoDefaults(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{listOut: nil}
	svc := NewAuditHistoryService(repo)

	_, err := svc.GetHistory(context.Background(), "user-1", -5)
	if err != nil {
		t.Fatalf("GetHistory unexpected error: %v", err)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listCalls[0].limit != 20 {
		t.Errorf("ListByUser limit = %d, want 20 (default)", repo.listCalls[0].limit)
	}
}

// TestAuditHistoryService_GetHistory_PassesLimitThrough proves a
// positive limit is forwarded unchanged.
func TestAuditHistoryService_GetHistory_PassesLimitThrough(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &fakeAuditHistoryRepo{
		listOut: []models.AuditSession{
			{ID: "sess-1", UserID: "user-1", CreatedAt: now},
		},
	}
	svc := NewAuditHistoryService(repo)

	got, err := svc.GetHistory(context.Background(), "user-1", 5)
	if err != nil {
		t.Fatalf("GetHistory unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "sess-1" {
		t.Errorf("got = %v, want one session with ID sess-1", got)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.listCalls[0].limit != 5 {
		t.Errorf("ListByUser limit = %d, want 5 (passed through)", repo.listCalls[0].limit)
	}
}

// TestAuditHistoryService_GetHistory_EmptyResult proves a
// zero-history user gets a non-nil empty slice (or nil — both
// are acceptable; the handler converts nil to [] for JSON).
func TestAuditHistoryService_GetHistory_EmptyResult(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{listOut: nil}
	svc := NewAuditHistoryService(repo)

	got, err := svc.GetHistory(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("GetHistory unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestAuditHistoryService_GetHistory_PropagatesDBError(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{listErr: errors.New("db down")}
	svc := NewAuditHistoryService(repo)

	_, err := svc.GetHistory(context.Background(), "user-1", 20)
	if err == nil {
		t.Fatal("GetHistory returned nil error, want non-nil")
	}
}

// TestAuditHistoryService_GetStats_PassesThrough proves the service
// just forwards the (totalAudits, totalFindings) pair the port
// returns — no aggregation logic in core.
func TestAuditHistoryService_GetStats_PassesThrough(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{
		statsTotalAudits:   7,
		statsTotalFindings: 23,
	}
	svc := NewAuditHistoryService(repo)

	totalAudits, totalFindings, err := svc.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetStats unexpected error: %v", err)
	}
	if totalAudits != 7 {
		t.Errorf("totalAudits = %d, want 7", totalAudits)
	}
	if totalFindings != 23 {
		t.Errorf("totalFindings = %d, want 23", totalFindings)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.statsCalls) != 1 || repo.statsCalls[0] != "user-1" {
		t.Errorf("GetStats calls = %v, want [user-1]", repo.statsCalls)
	}
}

func TestAuditHistoryService_GetStats_EmptyHistory(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{statsTotalAudits: 0, statsTotalFindings: 0}
	svc := NewAuditHistoryService(repo)

	totalAudits, totalFindings, err := svc.GetStats(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetStats unexpected error: %v", err)
	}
	if totalAudits != 0 || totalFindings != 0 {
		t.Errorf("got (%d, %d), want (0, 0)", totalAudits, totalFindings)
	}
}

func TestAuditHistoryService_GetStats_PropagatesDBError(t *testing.T) {
	t.Parallel()

	repo := &fakeAuditHistoryRepo{statsErr: errors.New("db down")}
	svc := NewAuditHistoryService(repo)

	_, _, err := svc.GetStats(context.Background(), "user-1")
	if err == nil {
		t.Fatal("GetStats returned nil error, want non-nil")
	}
}
