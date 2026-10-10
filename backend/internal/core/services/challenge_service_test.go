package services

// Tests for ChallengeService. Mirrors the audit_service_test.go
// pattern: hand-rolled, deterministic, in-memory fake ports so the
// service can be exercised end-to-end without a real database. The
// SQL is verified separately in
// backend/internal/infrastructure/driven/supabase/challenge_repository_test.go
// via go-sqlmock against the production adapter.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Fakes (deterministic, in-memory) ---

// fakeChallengeRepo is a minimal ports.ChallengeRepository used to
// drive ChallengeService without a real database. Each method has
// pre-canned behavior knobs + recorded calls so tests can assert on
// the exact (ctx, args) tuple the service passes through.
type fakeChallengeRepo struct {
	mu sync.Mutex

	// findBySource: when set, the call returns this challenge (and a
	// nil err) instead of the not-found sentinel.
	findBySourceOut *models.Challenge

	// findBySourceErr: when non-nil, returned from FindBySourceRepoAndUser
	// regardless of findBySourceOut.
	findBySourceErr error

	// findBySourceCalls: every (sourceRepo, userID) tuple the service
	// passed through the dedup probe.
	findBySourceCalls []fakeSourceCall

	// insertCalls: every *models.Challenge the service handed to Insert.
	insertCalls []*models.Challenge

	// insertCreatedAt: the time.Time Insert should return; mirrors the
	// DB's RETURNING created_at behavior.
	insertCreatedAt time.Time

	// insertErr: when non-nil, returned from Insert.
	insertErr error

	// listOut / listErr: response for ListVisibleToUser.
	listOut []models.Challenge
	listErr error

	// listCalls: every (ctx, userID) tuple passed to ListVisibleToUser.
	listCalls []string

	// findByIDOut / findByIDErr: response for FindByIDVisibleToUser.
	findByIDOut models.Challenge
	findByIDErr error

	// findByIDCalls: every (ctx, id, userID) tuple.
	findByIDCalls []fakeIDCall

	// findByIDWithSolutionOut / findByIDWithSolutionErr: same for the
	// solution-aware lookup.
	findByIDWithSolutionOut models.Challenge
	findByIDWithSolutionErr error

	// findByIDWithSolutionCalls: every (ctx, id, userID) tuple.
	findByIDWithSolutionCalls []fakeIDCall
}

type (
	fakeSourceCall struct{ sourceRepo, userID string }
	fakeIDCall     struct{ id, userID string }
)

func (f *fakeChallengeRepo) FindBySourceRepoAndUser(_ context.Context, sourceRepo, userID string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findBySourceCalls = append(f.findBySourceCalls, fakeSourceCall{sourceRepo, userID})
	if f.findBySourceErr != nil {
		return models.Challenge{}, f.findBySourceErr
	}
	if f.findBySourceOut != nil {
		return *f.findBySourceOut, nil
	}
	return models.Challenge{}, ports.ErrChallengeNotFound
}

func (f *fakeChallengeRepo) Insert(_ context.Context, c *models.Challenge) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertCalls = append(f.insertCalls, c)
	if f.insertErr != nil {
		return time.Time{}, f.insertErr
	}
	return f.insertCreatedAt, nil
}

func (f *fakeChallengeRepo) ListVisibleToUser(_ context.Context, userID string) ([]models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls = append(f.listCalls, userID)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listOut, nil
}

func (f *fakeChallengeRepo) FindByIDVisibleToUser(_ context.Context, id, userID string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findByIDCalls = append(f.findByIDCalls, fakeIDCall{id, userID})
	if f.findByIDErr != nil {
		return models.Challenge{}, f.findByIDErr
	}
	return f.findByIDOut, nil
}

func (f *fakeChallengeRepo) FindByIDWithSolutionVisibleToUser(_ context.Context, id, userID string) (models.Challenge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findByIDWithSolutionCalls = append(f.findByIDWithSolutionCalls, fakeIDCall{id, userID})
	if f.findByIDWithSolutionErr != nil {
		return models.Challenge{}, f.findByIDWithSolutionErr
	}
	return f.findByIDWithSolutionOut, nil
}

// Compile-time guard: fakeChallengeRepo must satisfy the port.
var _ ports.ChallengeRepository = (*fakeChallengeRepo)(nil)

// --- Tests ---

// TestChallengeService_GetAll_ProxiesToRepo proves the service
// simply delegates GetAll to the port. The fake returns a 2-row
// list and the service must hand it back unchanged.
func TestChallengeService_GetAll_ProxiesToRepo(t *testing.T) {
	t.Parallel()

	now := time.Now()
	repo := &fakeChallengeRepo{
		listOut: []models.Challenge{
			{ID: "ch-1", Title: "T1", CreatedAt: now},
			{ID: "ch-2", Title: "T2", CreatedAt: now},
		},
	}
	svc := NewChallengeService(repo)

	got, err := svc.GetAll(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetAll unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID != "ch-1" || got[1].ID != "ch-2" {
		t.Errorf("ids = (%q, %q), want (ch-1, ch-2)", got[0].ID, got[1].ID)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.listCalls) != 1 || repo.listCalls[0] != "user-1" {
		t.Errorf("ListVisibleToUser calls = %v, want [user-1]", repo.listCalls)
	}
}

func TestChallengeService_GetAll_EmptyResult(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{listOut: nil}
	svc := NewChallengeService(repo)

	got, err := svc.GetAll(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetAll unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0 (nil slice is acceptable)", len(got))
	}
}

// TestChallengeService_GetByID_Hit returns the row the port produced.
func TestChallengeService_GetByID_Hit(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{
		findByIDOut: models.Challenge{ID: "ch-1", Title: "T1", Difficulty: "junior"},
	}
	svc := NewChallengeService(repo)

	got, err := svc.GetByID(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("GetByID unexpected error: %v", err)
	}
	if got.ID != "ch-1" {
		t.Errorf("ID = %q, want ch-1", got.ID)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.findByIDCalls) != 1 {
		t.Fatalf("FindByIDVisibleToUser calls = %d, want 1", len(repo.findByIDCalls))
	}
	if c := repo.findByIDCalls[0]; c.id != "ch-1" || c.userID != "user-1" {
		t.Errorf("FindByIDVisibleToUser call = (%q, %q), want (ch-1, user-1)", c.id, c.userID)
	}
}

// TestChallengeService_GetByID_NotFound_PropagatesPortSentinel proves
// the service surfaces ports.ErrChallengeNotFound unchanged so HTTP
// handlers can branch with errors.Is.
func TestChallengeService_GetByID_NotFound_PropagatesPortSentinel(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{findByIDErr: ports.ErrChallengeNotFound}
	svc := NewChallengeService(repo)

	_, err := svc.GetByID(context.Background(), "missing", "user-1")
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound (which aliases ports.ErrChallengeNotFound)", err)
	}
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		t.Errorf("err = %v, want ports.ErrChallengeNotFound", err)
	}
}

// TestChallengeService_GetChallengeWithDetails_DelegatesToSolutionPort
// proves the solution-aware lookup goes through a different port
// method than the solution-stripped GetByID.
func TestChallengeService_GetChallengeWithDetails_DelegatesToSolutionPort(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{
		findByIDWithSolutionOut: models.Challenge{
			ID:                  "ch-1",
			SolutionCode:        "SELECT * FROM users WHERE...",
			SolutionExplanation: "Use parameterized queries",
		},
	}
	svc := NewChallengeService(repo)

	got, err := svc.GetChallengeWithDetails(context.Background(), "ch-1", "user-1")
	if err != nil {
		t.Fatalf("GetChallengeWithDetails unexpected error: %v", err)
	}
	if got.SolutionCode != "SELECT * FROM users WHERE..." {
		t.Errorf("SolutionCode = %q, want SELECT * FROM users WHERE...", got.SolutionCode)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.findByIDWithSolutionCalls) != 1 {
		t.Fatalf("FindByIDWithSolutionVisibleToUser calls = %d, want 1", len(repo.findByIDWithSolutionCalls))
	}
	if len(repo.findByIDCalls) != 0 {
		t.Errorf("GetChallengeWithDetails must NOT call FindByIDVisibleToUser; got %d calls", len(repo.findByIDCalls))
	}
}

func TestChallengeService_GetChallengeWithDetails_NotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{findByIDWithSolutionErr: ports.ErrChallengeNotFound}
	svc := NewChallengeService(repo)

	_, err := svc.GetChallengeWithDetails(context.Background(), "missing", "user-1")
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

// TestChallengeService_GetByID_OtherUsersPrivate_PropagatesNotFound
// proves that when the port returns ErrChallengeNotFound (because
// the row exists but belongs to another user or is not visible), the
// service surfaces that as ErrChallengeNotFound. Authorization is
// the adapter's responsibility (it scopes by userID at the SQL
// layer); the service just translates.
func TestChallengeService_GetByID_OtherUsersPrivate_PropagatesNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{findByIDErr: ports.ErrChallengeNotFound}
	svc := NewChallengeService(repo)

	_, err := svc.GetByID(context.Background(), "ch-private", "user-1")
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Errorf("err = %v, want ErrChallengeNotFound", err)
	}
}

// validV2InputFields returns valid v2 fields for a CreateChallengeInput.
// Tests that exercise the Create method beyond the validation gate
// need these fields populated so the v2 constraints pass.
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

// TestChallengeService_Create_NewChallenge proves the happy path:
// dedup probe returns not-found, UUID is generated, Insert is called
// with the populated model, and the service returns the persisted
// shape with the DB-supplied created_at.
func TestChallengeService_Create_NewChallenge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeChallengeRepo{insertCreatedAt: now}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.SourceRepo = "Owner/Repo" // mixed case + whitespace intentional

	got, created, err := svc.Create(context.Background(), input, "user-1")
	if err != nil {
		t.Fatalf("Create unexpected error: %v", err)
	}
	if !created {
		t.Error("created = false, want true for new challenge")
	}
	if got.Title != "Test Challenge" {
		t.Errorf("Title = %q, want Test Challenge", got.Title)
	}
	if got.SourceRepo != "owner/repo" {
		t.Errorf("SourceRepo = %q, want owner/repo (normalized lowercase + trim)", got.SourceRepo)
	}
	if got.UserID == nil || *got.UserID != "user-1" {
		t.Errorf("UserID = %v, want pointer to user-1", got.UserID)
	}
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v (the DB-supplied value)", got.CreatedAt, now)
	}
	if got.ID == "" {
		t.Error("ID is empty; service must generate a UUID before Insert")
	}

	// Verify the dedup probe ran with the normalized source_repo.
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.findBySourceCalls) != 1 {
		t.Fatalf("FindBySourceRepoAndUser calls = %d, want 1", len(repo.findBySourceCalls))
	}
	if c := repo.findBySourceCalls[0]; c.sourceRepo != "owner/repo" || c.userID != "user-1" {
		t.Errorf("FindBySourceRepoAndUser call = (%q, %q), want (owner/repo, user-1)", c.sourceRepo, c.userID)
	}

	// Verify Insert got the populated model.
	if len(repo.insertCalls) != 1 {
		t.Fatalf("Insert calls = %d, want 1", len(repo.insertCalls))
	}
	ins := repo.insertCalls[0]
	if ins.ID != got.ID {
		t.Errorf("Insert model ID = %q, want %q", ins.ID, got.ID)
	}
	if ins.SourceRepo != "owner/repo" {
		t.Errorf("Insert model SourceRepo = %q, want owner/repo (normalized)", ins.SourceRepo)
	}
}

// TestChallengeService_Create_DedupReturnsExisting proves the
// dedup-hit path: when FindBySourceRepoAndUser returns a row, the
// service returns it unchanged and does NOT call Insert.
func TestChallengeService_Create_DedupReturnsExisting(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existing := models.Challenge{
		ID:          "existing-id",
		Title:       "Existing",
		Description: "desc",
		Difficulty:  "junior",
		Category:    "security",
		Language:    "typescript",
		SourceRepo:  "ggogsmic/academy-mic",
		CreatedAt:   now,
		UserID:      stringPtr("user-1"),
	}
	repo := &fakeChallengeRepo{findBySourceOut: &existing}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.Title = "Existing"
	input.SourceRepo = "  GgogsMIC/academy-mic  " // whitespace + mixed case
	input.Description = "A code snippet for review"

	got, created, err := svc.Create(context.Background(), input, "user-1")
	if err != nil {
		t.Fatalf("Create unexpected error: %v", err)
	}
	if created {
		t.Error("created = true, want false for dedup hit")
	}
	if got.ID != "existing-id" {
		t.Errorf("ID = %q, want existing-id (the persisted row)", got.ID)
	}

	// Insert must not have been called.
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.insertCalls) != 0 {
		t.Errorf("Insert was called %d times, want 0 (dedup hit must not insert)", len(repo.insertCalls))
	}
}

// TestChallengeService_Create_DedupErrorPropagates proves that an
// unexpected error from the dedup probe (NOT ErrChallengeNotFound)
// surfaces as a wrapped error. Callers branch with errors.Is to
// distinguish the not-found case from a real DB outage.
func TestChallengeService_Create_DedupErrorPropagates(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{findBySourceErr: errors.New("db down")}
	svc := NewChallengeService(repo)

	_, _, err := svc.Create(context.Background(), validV2InputFields(), "user-1")
	if err == nil {
		t.Fatal("Create returned nil error, want non-nil")
	}
	if errors.Is(err, ports.ErrChallengeNotFound) {
		t.Errorf("err wrapped ErrChallengeNotFound, want a generic wrap of the underlying error")
	}
}

// TestChallengeService_Create_InsertErrorPropagates proves a
// real INSERT error (e.g. constraint violation) surfaces wrapped
// to the caller, with the port's raw error intact for
// errors.Is checks.
func TestChallengeService_Create_InsertErrorPropagates(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{insertErr: errors.New("constraint violation")}
	svc := NewChallengeService(repo)

	_, _, err := svc.Create(context.Background(), validV2InputFields(), "user-1")
	if err == nil {
		t.Fatal("Create returned nil error, want non-nil")
	}
	if !errors.Is(err, repo.insertErr) {
		t.Errorf("err = %v, want it to wrap %v", err, repo.insertErr)
	}
}

// --- Validation gate (pure-Go, no repo interaction) ---

func TestChallengeService_Create_InvalidDifficulty(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.Difficulty = "expert" // not in validDifficulties

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrInvalidDifficulty) {
		t.Errorf("err = %v, want ErrInvalidDifficulty", err)
	}
	// Validation must short-circuit BEFORE the repo is touched.
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.findBySourceCalls) != 0 {
		t.Errorf("FindBySourceRepoAndUser calls = %d, want 0 (validation must short-circuit)", len(repo.findBySourceCalls))
	}
}

func TestChallengeService_Create_TooFewFindings(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.ExpectedFindings = []models.ExpectedFinding{} // 0 findings

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrInvalidExpectedFindings) {
		t.Errorf("err = %v, want ErrInvalidExpectedFindings", err)
	}
}

func TestChallengeService_Create_TooManyFindings(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.ExpectedFindings = []models.ExpectedFinding{
		{Severity: "high", Category: "security", Message: "1"},
		{Severity: "high", Category: "security", Message: "2"},
		{Severity: "high", Category: "security", Message: "3"},
		{Severity: "high", Category: "security", Message: "4"},
	}

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrInvalidExpectedFindings) {
		t.Errorf("err = %v, want ErrInvalidExpectedFindings", err)
	}
}

func TestChallengeService_Create_TooFewTestCases(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.TestCases = []models.TestCase{
		{Name: "test1", ExpectedOutput: "ok", Weight: 5}, // only 1
	}

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrInvalidTestCases) {
		t.Errorf("err = %v, want ErrInvalidTestCases", err)
	}
}

func TestChallengeService_Create_InvalidHintsCount(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.Hints = []models.Hint{
		{Level: 1, Content: "hint 1", CostPoints: 0},
		{Level: 2, Content: "hint 2", CostPoints: 10}, // only 2
	}

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrInvalidHintsCount) {
		t.Errorf("err = %v, want ErrInvalidHintsCount", err)
	}
}

func TestChallengeService_Create_SpoilerInDescription(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo)

	input := validV2InputFields()
	input.Description = "This challenge involves SQL Injection in the code."

	_, _, err := svc.Create(context.Background(), input, "user-1")
	if !errors.Is(err, ErrSpoilerInDescription) {
		t.Errorf("err = %v, want ErrSpoilerInDescription", err)
	}
}

// TestChallengeService_Create_ValidV2Challenge_EchoesV2Fields
// proves the v2 fields (ExpectedFindings, TestCases, Hints) are
// preserved on the response model after a successful create.
func TestChallengeService_Create_ValidV2Challenge_EchoesV2Fields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	repo := &fakeChallengeRepo{insertCreatedAt: now}
	svc := NewChallengeService(repo)

	got, created, err := svc.Create(context.Background(), validV2InputFields(), "user-1")
	if err != nil {
		t.Fatalf("Create unexpected error: %v", err)
	}
	if !created {
		t.Error("created = false, want true for new challenge")
	}
	if len(got.ExpectedFindings) != 2 {
		t.Errorf("ExpectedFindings len = %d, want 2", len(got.ExpectedFindings))
	}
	if len(got.TestCases) != 2 {
		t.Errorf("TestCases len = %d, want 2", len(got.TestCases))
	}
	if len(got.Hints) != 3 {
		t.Errorf("Hints len = %d, want 3", len(got.Hints))
	}
}

// --- GenerateChallenge shell (PR 2 / S3) ---

// TestChallengeService_GenerateChallenge_StubReturnsErrNotImplemented
// was retired in PR 2 — the S5 placeholder was replaced by the real
// orchestration. The replacement contract is:
//
//   - Unwired service (no WithLLMClient) → ErrChallengeGenerationNotWired
//     + zero row written.
//   - Wired service + successful LLM/sandbox/validate pass → persisted
//     Challenge with origin=generated.
//
// Detailed shell coverage (sentinels, difficulty mapping, retry loop,
// sandbox gate, persistence) lives in challenge_service_generate_test.go.

// TestGenerateChallengeHint_DomainShapeMatchesWire verifies the
// domain model carries Language/Difficulty/Focus with omitempty —
// this is the shape the MCP adapter translates GenerateHint into
// before calling the service. The PR 2 extension added TopicID +
// Level (covered in challenge_models_generate_test.go); this test
// stays to lock the v1 contract that existing callers depend on.
func TestGenerateChallengeHint_DomainShapeMatchesWire(t *testing.T) {
	t.Parallel()

	hint := models.GenerateChallengeHint{Language: "go", Difficulty: "senior"}
	if hint.Language != "go" || hint.Difficulty != "senior" {
		t.Errorf("fields = (%q, %q), want (go, senior)", hint.Language, hint.Difficulty)
	}
	if hint.Focus != "" {
		t.Errorf("empty Focus = %q, want zero value", hint.Focus)
	}
	if hint.TopicID != "" || hint.Level != "" {
		t.Errorf("TopicID/Level = (%q, %q), want empty default", hint.TopicID, hint.Level)
	}
}

// stringPtr returns a pointer to the supplied string. Used in test
// fixtures that need a *string for Challenge.UserID.
func stringPtr(s string) *string { return &s }
