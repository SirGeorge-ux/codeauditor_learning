package services

// Orchestration tests for ChallengeService.GenerateChallenge (S3 PR 2).
//
// The orchestration wires:
//
//	curriculum.GetTopic   →  validate topic exists
//	pedagogy.GetLearningObjective  →  build the prompt
//	pedagogy.GenerateStoryContext  →  build the prompt
//	llm.Complete          →  emit a v2 Challenge JSON
//	sandbox.Execute       →  gate the solution code (≤3 attempts)
//	validateChallengeV2   →  cardinality + spoiler check
//	repo.Insert           →  persist with origin=generated
//
// We use real CurriculumService / PedagogicalService against fixtures in
// `t.TempDir()` so the test exercises the production prompt-time path
// without dragging an LLM into the unit-test suite. The LLM and sandbox are
// hand-rolled fakes (FIFO queues of canned responses); the repo is a
// fake that records Insert calls.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Orchestration fakes ---------------------------------------------------

// genFakeLLMClient implements ports.LLMClientPort. completeResponses is
// a FIFO queue: each Complete call pops the head; when the queue is
// empty the default response is returned. Lets a test simulate
// "fail twice, pass third" without leaking the queue across sub-tests.
//
// The name carries a `_gen_` infix so it does not collide with the
// audit_service_test.go fake (same package).
type genFakeLLMClient struct {
	mu sync.Mutex

	completeResponses []ports.CompleteResponse
	completeErrs      []error // parallel slice

	completeCalls int
	lastReq       ports.CompleteRequest
}

func (f *genFakeLLMClient) StreamChat(_ context.Context, _ []models.ChatMessage, _ ports.StreamChatOptions, _ func(ports.LLMStreamEvent) error) error {
	return errors.New("genFakeLLMClient: StreamChat not used by GenerateChallenge")
}

func (f *genFakeLLMClient) Complete(_ context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	f.lastReq = req
	idx := f.completeCalls - 1
	var (
		resp ports.CompleteResponse
		err  error
	)
	if idx < len(f.completeResponses) {
		resp = f.completeResponses[idx]
	}
	if idx < len(f.completeErrs) {
		err = f.completeErrs[idx]
	}
	return resp, err
}

// genFakeSandbox implements ports.SandboxExecutor. executeBehaviour is
// a FIFO queue of canned (body, err) tuples — one per Execute call.
// Each Execute also records the (language, code) tuple so a test can
// assert the solution code was actually passed to the sandbox.
type genFakeSandbox struct {
	mu sync.Mutex

	executeBehaviour []genFakeSandboxExec
	executeCalls     int
	lastLang         string
	lastCode         string

	healthErr error
}

type genFakeSandboxExec struct {
	Body string
	Err  error
}

func (f *genFakeSandbox) Execute(_ context.Context, language, code string, _ int) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastLang = language
	f.lastCode = code
	idx := f.executeCalls
	if idx < len(f.executeBehaviour) {
		b := f.executeBehaviour[idx]
		f.executeCalls++
		if b.Err != nil {
			return nil, b.Err
		}
		body := b.Body
		if body == "" {
			body = "ok\n"
		}
		return io.NopCloser(strings.NewReader(body)), nil
	}
	f.executeCalls++
	return io.NopCloser(strings.NewReader("ok\n")), nil
}

func (f *genFakeSandbox) Healthcheck(_ context.Context) error {
	return f.healthErr
}

// genFakeRepoForGenerate is a minimal ports.ChallengeRepository used by
// the orchestration tests. GenerateChallenge only calls Insert
// (per the design decision: no dedup probe — daily cap + client
// hash guard duplicates server-side); the other methods are stubbed
// for port satisfaction.
type genFakeRepoForGenerate struct {
	mu sync.Mutex

	insertOut time.Time
	insertErr error

	insertCalls []*models.Challenge
}

func (f *genFakeRepoForGenerate) FindBySourceRepoAndUser(_ context.Context, _, _ string) (models.Challenge, error) {
	return models.Challenge{}, ports.ErrChallengeNotFound
}

func (f *genFakeRepoForGenerate) Insert(_ context.Context, c *models.Challenge) (time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertCalls = append(f.insertCalls, c)
	if f.insertErr != nil {
		return time.Time{}, f.insertErr
	}
	return f.insertOut, nil
}

func (f *genFakeRepoForGenerate) ListVisibleToUser(_ context.Context, _ string) ([]models.Challenge, error) {
	return nil, nil
}

func (f *genFakeRepoForGenerate) FindByIDVisibleToUser(_ context.Context, _, _ string) (models.Challenge, error) {
	return models.Challenge{}, ports.ErrChallengeNotFound
}

func (f *genFakeRepoForGenerate) FindByIDWithSolutionVisibleToUser(_ context.Context, _, _ string) (models.Challenge, error) {
	return models.Challenge{}, ports.ErrChallengeNotFound
}

var _ ports.ChallengeRepository = (*genFakeRepoForGenerate)(nil)

// --- Fixtures --------------------------------------------------------------

// minimalTypeScriptFixture is the smallest valid .atl/curricula/typescript.md
// the parser accepts. It declares ONE topic (`inheritance`) at level D
// so the orchestration tests can target a known slug.
const minimalTypeScriptFixture = `# TypeScript Curriculum

## Topics

### inheritance

- **slug:** inheritance
- **difficulty:** D
- **prerequisites:** []
- **category:** oop
- **description:** |
  Inheritance lets a class reuse and extend another's behavior.
- **exercises:**
  - "Extend a base class and override a method."
  - "Use protected to share state with subclasses."
- **common_mistakes:**
  - "Overriding without calling super.method()."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/classes.html
`

// fixtureBundle wires the real CurriculumService + PedagogicalService
// against a fixture file plus fakes for LLM/sandbox/repo. Returns the
// wired ChallengeService + the fakes a test needs to assert on.
type fixtureBundle struct {
	svc             *ChallengeService
	repo            *genFakeRepoForGenerate
	llm             *genFakeLLMClient
	sandbox         *genFakeSandbox
	curricula       *CurriculumService
	pedagogy        *PedagogicalService
	insertCreatedAt time.Time
}

// newFixtureBundle returns a fully wired ChallengeService against the
// minimal TypeScript fixture. Tests can swap the fakes' canned
// responses before calling GenerateChallenge.
func newFixtureBundle(t *testing.T) *fixtureBundle {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "typescript.md"), []byte(minimalTypeScriptFixture), 0o644); err != nil {
		t.Fatalf("write curriculum fixture: %v", err)
	}

	curricula := NewCurriculumService(WithBaseDir(dir))
	pedagogy := NewPedagogicalService(curricula)
	llm := &genFakeLLMClient{}
	sandbox := &genFakeSandbox{}
	repo := &genFakeRepoForGenerate{insertOut: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}

	svc := NewChallengeService(
		repo,
		WithCurriculumService(curricula),
		WithPedagogicalService(pedagogy),
		WithLLMClient(llm),
		WithSandboxExecutor(sandbox),
	)
	return &fixtureBundle{
		svc:             svc,
		repo:            repo,
		llm:             llm,
		sandbox:         sandbox,
		curricula:       curricula,
		pedagogy:        pedagogy,
		insertCreatedAt: repo.insertOut,
	}
}

// validChallengeJSON returns a v2 Challenge JSON the LLM would emit
// that passes validateChallengeV2. The fields are deliberately
// small so the assertion focus stays on the orchestration outcome
// rather than content shape.
func validChallengeJSON(t *testing.T, lang, solutionBody string) string {
	t.Helper()
	draft := models.CreateChallengeInput{
		Title:        "Inheritance Hunt",
		Description:  "Extend a base class without regressing its method.",
		Difficulty:   "junior",
		Category:     "oop",
		Language:     lang,
		Code:         "class Animal {}",
		SolutionCode: solutionBody,
		Hints: []models.Hint{
			{Level: 1, Content: "hint 1", CostPoints: 0},
			{Level: 2, Content: "hint 2", CostPoints: 10},
			{Level: 3, Content: "hint 3", CostPoints: 25},
		},
		ExpectedFindings: []models.ExpectedFinding{
			{Severity: "high", Category: "oop", Message: "missing super call"},
		},
		TestCases: []models.TestCase{
			{Name: "test1", ExpectedOutput: "ok", Weight: 5},
			{Name: "test2", ExpectedOutput: "ok", Weight: 5},
		},
	}
	raw, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	return string(raw)
}

// --- Orchestration tests --------------------------------------------------

// TestGenerateChallenge_HappyPath_PersistsAndMapsFields drives the
// full orchestration: curriculum + pedagogy + LLM + sandbox gate +
// validate + persist. The persisted row must carry
// origin="generated", generatedBy="free-practice-v1", createdBy=userID,
// the difficulty derived from the F..S level, and the LLM-emitted
// Title / Description / Language.
func TestGenerateChallenge_HappyPath_PersistsAndMapsFields(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "class Dog extends Animal {}"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Body: "ok\n"},
	}

	got, err := b.svc.GenerateChallenge(context.Background(), "user-1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if err != nil {
		t.Fatalf("GenerateChallenge unexpected error: %v", err)
	}

	if got.Origin != "generated" {
		t.Errorf("Origin = %q, want generated", got.Origin)
	}
	if got.GeneratedBy != "free-practice-v1" {
		t.Errorf("GeneratedBy = %q, want free-practice-v1", got.GeneratedBy)
	}
	if got.CreatedBy != "user-1" {
		t.Errorf("CreatedBy = %q, want user-1", got.CreatedBy)
	}
	// D maps to mid per the spec.
	if got.Difficulty != "mid" {
		t.Errorf("Difficulty = %q, want mid (level D → mid)", got.Difficulty)
	}
	if got.Language != "typescript" {
		t.Errorf("Language = %q, want typescript", got.Language)
	}
	if got.Title == "" {
		t.Errorf("Title = empty; LLM-emitted title was not copied")
	}
	if got.SolutionCode == "" {
		t.Errorf("SolutionCode = empty; LLM-emitted solution was not copied")
	}

	// Repo must have received exactly one insert with the same model.
	if len(b.repo.insertCalls) != 1 {
		t.Fatalf("Insert calls = %d, want 1", len(b.repo.insertCalls))
	}
	ins := b.repo.insertCalls[0]
	if ins.Origin != "generated" {
		t.Errorf("Insert.Origin = %q, want generated", ins.Origin)
	}
	if ins.GeneratedBy != "free-practice-v1" {
		t.Errorf("Insert.GeneratedBy = %q, want free-practice-v1", ins.GeneratedBy)
	}
	if ins.CreatedBy != "user-1" {
		t.Errorf("Insert.CreatedBy = %q, want user-1", ins.CreatedBy)
	}
	if !got.CreatedAt.Equal(b.insertCreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, b.insertCreatedAt)
	}
}

// TestGenerateChallenge_PromptAssembly_IncludesIdentityAndContext
// asserts the System + Prompt strings contain the topic, level, and
// a topic-relevant description so the LLM is grounded in the
// curriculum (not just the hint). The test is hermetic — no LLM
// call, just an inspection of the captured request.
func TestGenerateChallenge_PromptAssembly_IncludesIdentityAndContext(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "x"), Provider: "groq"},
	}

	if _, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	}); err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}

	system := b.llm.lastReq.System
	prompt := b.llm.lastReq.Prompt

	if !strings.Contains(system, "auditor") && !strings.Contains(system, "code") {
		t.Errorf("system prompt missing identity: %q", system)
	}
	if !strings.Contains(prompt, "typescript") {
		t.Errorf("prompt missing language: %q", prompt)
	}
	if !strings.Contains(prompt, "inheritance") {
		t.Errorf("prompt missing topic: %q", prompt)
	}
	if !strings.Contains(prompt, "D") {
		t.Errorf("prompt missing level D: %q", prompt)
	}
	if !strings.Contains(prompt, "Inheritance lets a class reuse") {
		t.Errorf("prompt missing curriculum description: %q", prompt)
	}
	if b.llm.lastReq.JSONSchema == nil {
		t.Errorf("JSONSchema must be non-nil (the LLM call is JSON-mode)")
	}
}

// TestGenerateChallenge_LevelMapping_F_E_Junior pins the F->junior
// boundary from the spec scenarios "Level F maps to junior".
func TestGenerateChallenge_LevelMapping_F_E_Junior(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level, want string
	}{
		{"F", "junior"},
		{"E", "junior"},
		{"S", "architect"},
	}
	for _, c := range cases {
		t.Run(c.level, func(t *testing.T) {
			b := newFixtureBundle(t)
			b.llm.completeResponses = []ports.CompleteResponse{
				{Content: validChallengeJSON(t, "typescript", "x"), Provider: "groq"},
			}
			got, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
				Language: "typescript",
				TopicID:  "inheritance",
				Level:    c.level,
			})
			if err != nil {
				t.Fatalf("GenerateChallenge(%s): %v", c.level, err)
			}
			if got.Difficulty != c.want {
				t.Errorf("Difficulty = %q, want %q (level %s)", got.Difficulty, c.want, c.level)
			}
		})
	}
}

// TestGenerateChallenge_TopicNotInCurriculum_ReturnsSentinel covers
// the spec scenario "Topic not in curriculum" — a missing topic slug
// surfaces ErrCurriculumTopicNotFound and writes NO row.
func TestGenerateChallenge_TopicNotInCurriculum_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "no-such-topic",
		Level:    "D",
	})
	if !errors.Is(err, ErrCurriculumTopicNotFound) {
		t.Errorf("err = %v, want ErrCurriculumTopicNotFound", err)
	}
	if len(b.repo.insertCalls) != 0 {
		t.Errorf("Insert calls = %d, want 0 (missing-topic must not persist)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_LanguageNotInCurriculum_ReturnsSentinel
// covers the spec scenario "Unsupported language" — a missing
// curriculum file surfaces ErrCurriculumLanguageNotFound and writes
// no row.
func TestGenerateChallenge_LanguageNotInCurriculum_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "rust",
		TopicID:  "traits",
		Level:    "C",
	})
	if !errors.Is(err, ErrCurriculumLanguageNotFound) {
		t.Errorf("err = %v, want ErrCurriculumLanguageNotFound", err)
	}
	if len(b.repo.insertCalls) != 0 {
		t.Errorf("Insert calls = %d, want 0 (missing-language must not persist)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_SandboxGatePassesOnFirstRun exercises the
// happy path of the solution-code gate: the LLM emits a clean
// solution, the sandbox returns ok, the orchestrator persists.
func TestGenerateChallenge_SandboxGatePassesOnFirstRun(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "class Dog extends Animal {}"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Body: "ok\n"},
	}

	got, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}
	if got.ID == "" {
		t.Errorf("ID = empty; persisted row must carry a UUID")
	}
	if b.sandbox.executeCalls != 1 {
		t.Errorf("sandbox.executeCalls = %d, want 1 (gate runs once)", b.sandbox.executeCalls)
	}
}

// TestGenerateChallenge_SandboxGateFailsThreeTimes_ReturnsSentinel
// covers the spec scenario "Solution fails three times" — three
// consecutive sandbox failures return ErrChallengeGenerationFailedSandbox
// and write NO row.
func TestGenerateChallenge_SandboxGateFailsThreeTimes_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "broken 1"), Provider: "groq"},
		{Content: validChallengeJSON(t, "typescript", "broken 2"), Provider: "groq"},
		{Content: validChallengeJSON(t, "typescript", "broken 3"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Err: errors.New("compile error: broken 1")},
		{Err: errors.New("compile error: broken 2")},
		{Err: errors.New("compile error: broken 3")},
	}

	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if !errors.Is(err, ErrChallengeGenerationFailedSandbox) {
		t.Errorf("err = %v, want ErrChallengeGenerationFailedSandbox", err)
	}
	if b.sandbox.executeCalls != 3 {
		t.Errorf("sandbox.executeCalls = %d, want 3 (full retry budget)", b.sandbox.executeCalls)
	}
	if b.llm.completeCalls != 3 {
		t.Errorf("llm.completeCalls = %d, want 3 (full retry budget)", b.llm.completeCalls)
	}
	if len(b.repo.insertCalls) != 0 {
		t.Errorf("Insert calls = %d, want 0 (failed sandbox must not persist)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_SandboxGateFailsTwiceThenPasses covers the
// spec scenario "Solution fails twice, passes third" — the
// orchestrator persists the third-pass success and the failure
// history is not in the persisted row.
func TestGenerateChallenge_SandboxGateFailsTwiceThenPasses(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "broken 1"), Provider: "groq"},
		{Content: validChallengeJSON(t, "typescript", "broken 2"), Provider: "groq"},
		{Content: validChallengeJSON(t, "typescript", "ok now"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Err: errors.New("compile error 1")},
		{Err: errors.New("compile error 2")},
		{Body: "ok\n"},
	}

	got, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}
	if got.SolutionCode != "ok now" {
		t.Errorf("SolutionCode = %q, want the third-pass solution", got.SolutionCode)
	}
	if len(b.repo.insertCalls) != 1 {
		t.Errorf("Insert calls = %d, want 1 (3rd attempt persists)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_LLMInvalidJSON_ReturnsSentinel covers the
// spec scenario "LLM returns invalid JSON" — the LLM emits text the
// JSON parser rejects, the orchestrator surfaces
// ErrChallengeGenerationInvalidJSON after the retry budget and
// writes NO row.
func TestGenerateChallenge_LLMInvalidJSON_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: "not json at all", Provider: "groq"},
		{Content: "still not json", Provider: "groq"},
		{Content: "{garbage}", Provider: "groq"},
	}
	// Sandbox passes (we never get there because JSON parse fails first).

	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if !errors.Is(err, ErrChallengeGenerationInvalidJSON) {
		t.Errorf("err = %v, want ErrChallengeGenerationInvalidJSON", err)
	}
	if b.llm.completeCalls != 3 {
		t.Errorf("llm.completeCalls = %d, want 3 (full retry budget)", b.llm.completeCalls)
	}
	if len(b.repo.insertCalls) != 0 {
		t.Errorf("Insert calls = %d, want 0 (invalid JSON must not persist)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_LLMInvalidJSON_PassesThirdTry covers the
// retry-then-recover path on the JSON-parse branch — the orchestrator
// retries and the third attempt's valid JSON persists.
func TestGenerateChallenge_LLMInvalidJSON_PassesThirdTry(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: "garbage 1", Provider: "groq"},
		{Content: "garbage 2", Provider: "groq"},
		{Content: validChallengeJSON(t, "typescript", "ok now"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Body: "ok\n"},
	}

	got, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if err != nil {
		t.Fatalf("GenerateChallenge: %v", err)
	}
	if got.SolutionCode != "ok now" {
		t.Errorf("SolutionCode = %q, want third-pass payload", got.SolutionCode)
	}
}

// TestGenerateChallenge_DraftFailsValidateChallengeV2 covers the
// spec scenario "Draft fails validateChallengeV2" — the LLM emits
// valid JSON but with cardinality violations (e.g. 0 expected
// findings). The orchestrator returns ErrChallengeGenerationInvalid
// after 3 attempts and writes no row.
func TestGenerateChallenge_DraftFailsValidateChallengeV2(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	// Build a draft that parses as JSON but has 0 expected findings.
	badDraft := `{"title":"x","description":"y","difficulty":"junior","language":"typescript","code":"","hints":[{"level":1,"content":"a","cost_points":0},{"level":2,"content":"b","cost_points":10},{"level":3,"content":"c","cost_points":25}],"expected_findings":[],"test_cases":[{"name":"a","expected_output":"x","weight":1},{"name":"b","expected_output":"y","weight":1}],"solution_code":"foo"}`
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: badDraft, Provider: "groq"},
		{Content: badDraft, Provider: "groq"},
		{Content: badDraft, Provider: "groq"},
	}

	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if !errors.Is(err, ErrChallengeGenerationInvalid) {
		t.Errorf("err = %v, want ErrChallengeGenerationInvalid", err)
	}
	if len(b.repo.insertCalls) != 0 {
		t.Errorf("Insert calls = %d, want 0 (invalid draft must not persist)", len(b.repo.insertCalls))
	}
}

// TestGenerateChallenge_RepoInsertError_Propagates ensures the
// orchestration does NOT swallow a real DB error. The sentinel is
// wrapped so the caller can branch on it AND see the underlying
// error message in the chain.
func TestGenerateChallenge_RepoInsertError_Propagates(t *testing.T) {
	t.Parallel()

	b := newFixtureBundle(t)
	b.llm.completeResponses = []ports.CompleteResponse{
		{Content: validChallengeJSON(t, "typescript", "ok"), Provider: "groq"},
	}
	b.sandbox.executeBehaviour = []genFakeSandboxExec{
		{Body: "ok\n"},
	}
	b.repo.insertErr = errors.New("db constraint violation")

	_, err := b.svc.GenerateChallenge(context.Background(), "u1", models.GenerateChallengeHint{
		Language: "typescript",
		TopicID:  "inheritance",
		Level:    "D",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "db constraint violation") {
		t.Errorf("err = %v, want it to wrap db constraint violation", err)
	}
}
