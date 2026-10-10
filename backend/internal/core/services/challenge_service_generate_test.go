package services

// Tests for ChallengeService.GenerateChallenge shell (S3 PR 2):
//
//   - The four new option funcs (WithCurriculumService /
//     WithPedagogicalService / WithLLMClient / WithSandboxExecutor)
//     are additive + nil-safe.
//   - ErrChallengeGenerationNotWired + the three sibling sentinels
//     (ErrChallengeGenerationInvalidJSON, ErrChallengeGenerationInvalid,
//     ErrChallengeGenerationFailedSandbox) are distinct sentinels so
//     callers branch with errors.Is.
//   - difficultyForLevel maps every F..S letter to its 4-bucket
//     counterpart.
//   - When the service is constructed without WithLLMClient,
//     GenerateChallenge returns ErrChallengeGenerationNotWired and
//     writes no row.
//
// The orchestration (curriculum → pedagogy → LLM → sandbox → persist)
// tests land in a follow-up commit to keep each work unit small.

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// --- Sentinel tests ---------------------------------------------------------

// TestErrChallengeGenerationNotWired_DistinctFromOtherSentinels locks
// the four new challenge-generation sentinels so callers can branch
// with errors.Is across transports. The four values must be distinct
// — sentinel collisions would let an HTTP layer accidentally treat a
// JSON parse error as a sandbox failure.
func TestErrChallengeGenerationNotWired_DistinctFromOtherSentinels(t *testing.T) {
	t.Parallel()

	if errors.Is(ErrChallengeGenerationNotWired, ErrChallengeGenerationInvalidJSON) {
		t.Errorf("ErrChallengeGenerationNotWired must be distinct from ErrChallengeGenerationInvalidJSON")
	}
	if errors.Is(ErrChallengeGenerationNotWired, ErrChallengeGenerationInvalid) {
		t.Errorf("ErrChallengeGenerationNotWired must be distinct from ErrChallengeGenerationInvalid")
	}
	if errors.Is(ErrChallengeGenerationNotWired, ErrChallengeGenerationFailedSandbox) {
		t.Errorf("ErrChallengeGenerationNotWired must be distinct from ErrChallengeGenerationFailedSandbox")
	}
}

// TestNewSentinels_AreDistinctAmongThemselves asserts the three new
// error sentinels are mutually distinguishable so the dispatcher's
// JSON-RPC mapping in the MCP layer does not collapse.
func TestNewSentinels_AreDistinctAmongThemselves(t *testing.T) {
	t.Parallel()

	if errors.Is(ErrChallengeGenerationInvalidJSON, ErrChallengeGenerationInvalid) {
		t.Errorf("InvalidJSON and Invalid must be distinct")
	}
	if errors.Is(ErrChallengeGenerationInvalidJSON, ErrChallengeGenerationFailedSandbox) {
		t.Errorf("InvalidJSON and FailedSandbox must be distinct")
	}
	if errors.Is(ErrChallengeGenerationInvalid, ErrChallengeGenerationFailedSandbox) {
		t.Errorf("Invalid and FailedSandbox must be distinct")
	}
}

// --- Difficulty mapping tests -----------------------------------------------

// TestDifficultyForLevel_AllLettersMap asserts the F..S →
// 4-bucket mapping every scenario in the spec describes.
func TestDifficultyForLevel_AllLettersMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level string
		want  string
	}{
		{"F", "junior"},
		{"E", "junior"},
		{"D", "mid"},
		{"C", "mid"},
		{"B", "senior"},
		{"A", "senior"},
		{"S", "architect"},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			got := difficultyForLevel(c.level)
			if got != c.want {
				t.Errorf("difficultyForLevel(%q) = %q, want %q", c.level, got, c.want)
			}
		})
	}
}

// TestDifficultyForLevel_UnknownReturnsEmpty asserts the helper does
// not silently coerce unknown letters to a default; ChallengeService
// will surface the unknown level via validation and the persisted row
// stays empty until the LLM retries with a valid letter.
func TestDifficultyForLevel_UnknownReturnsEmpty(t *testing.T) {
	t.Parallel()

	if got := difficultyForLevel("Z"); got != "" {
		t.Errorf("difficultyForLevel(\"Z\") = %q, want empty (caller decides)", got)
	}
	if got := difficultyForLevel(""); got != "" {
		t.Errorf("difficultyForLevel(\"\") = %q, want empty", got)
	}
}

// TestDifficultyForLevel_LowercaseAccepted mirrors the user-side
// reality: the language-progress model emits lowercase levels. The
// helper normalises so a frontend that sends "f" is not surprised.
func TestDifficultyForLevel_LowercaseAccepted(t *testing.T) {
	t.Parallel()

	if got := difficultyForLevel("f"); got != "junior" {
		t.Errorf("difficultyForLevel(\"f\") = %q, want junior", got)
	}
	if got := difficultyForLevel("s"); got != "architect" {
		t.Errorf("difficultyForLevel(\"s\") = %q, want architect", got)
	}
}

// --- Option-func + wiring tests --------------------------------------------

// TestWithOptionFuncs_AdditiveNilSafe asserts that every option func
// can be called with nil without panicking AND that the constructor
// still produces a usable service. The nil-safety contract is
// important so the existing wiring_test.go (which calls
// `NewChallengeService(nil)` without options) keeps compiling and
// produces a ChallengeService that satisfies the rest of the API.
func TestWithOptionFuncs_AdditiveNilSafe(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(
		repo,
		WithCurriculumService(nil),
		WithPedagogicalService(nil),
		WithLLMClient(nil),
		WithSandboxExecutor(nil),
	)
	if svc == nil {
		t.Fatal("NewChallengeService returned nil")
	}
	// Even with no options wired, the public Create API is intact so
	// downstream handlers (which keep their v1 wiring) don't crash.
	if _, _, err := svc.Create(context.Background(), validV2InputFields(), "u1"); err != nil {
		t.Errorf("Create with nil-deps service should still run validation: %v", err)
	}
}

// TestGenerateChallenge_UnwiredLLM_ReturnsErrChallengeGenerationNotWired
// asserts the unwired-service scenario from the spec: when llm is
// nil, GenerateChallenge returns ErrChallengeGenerationNotWired and
// does NOT touch the repo (no row is written).
func TestGenerateChallenge_UnwiredLLM_ReturnsErrChallengeGenerationNotWired(t *testing.T) {
	t.Parallel()

	repo := &fakeChallengeRepo{}
	svc := NewChallengeService(repo) // no options at all

	_, err := svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if !errors.Is(err, ErrChallengeGenerationNotWired) {
		t.Errorf("err = %v, want ErrChallengeGenerationNotWired", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.insertCalls) != 0 {
		t.Errorf("Insert was called %d times; an unwired service must not touch the repo", len(repo.insertCalls))
	}
}

// TestGenerateChallenge_LLMNil_NilSafetyShortCircuits confirms the
// unwired-error fires BEFORE the curricula / pedagogy lookups, so an
// unconfigured service never crashes dereferencing nil deps.
func TestGenerateChallenge_LLMNil_NilSafetyShortCircuits(t *testing.T) {
	t.Parallel()

	svc := NewChallengeService(
		&fakeChallengeRepo{},
		WithCurriculumService(nil),
		WithPedagogicalService(nil),
		WithSandboxExecutor(nil),
	)
	_, err := svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "go",
		TopicID:  "channels",
		Level:    "D",
	})
	if !errors.Is(err, ErrChallengeGenerationNotWired) {
		t.Errorf("err = %v, want ErrChallengeGenerationNotWired", err)
	}
}

// TestGenerateChallenge_NilService_DoesNotPanic is the defense-in-depth
// guard: even a zero-valued struct must not panic on GenerateChallenge.
// The MCP adapter is the production caller, but tests can construct
// the service in a dozen ways; a panic on any of them would surface
// as a 500 at runtime.
func TestGenerateChallenge_NilService_DoesNotPanic(t *testing.T) {
	t.Parallel()

	svc := &ChallengeService{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("zero-value GenerateChallenge panicked: %v", r)
		}
	}()
	_, err := svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "rust",
		TopicID:  "traits",
		Level:    "C",
	})
	if !errors.Is(err, ErrChallengeGenerationNotWired) {
		t.Errorf("err = %v, want ErrChallengeGenerationNotWired", err)
	}
}
