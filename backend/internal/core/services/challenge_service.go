package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrChallengeNotFound is re-exported here for backward compatibility
// with callers (HTTP handlers, MCP adapters) that historically
// branch against services.ErrChallengeNotFound. The canonical
// definition lives in ports so the driven adapter can return it
// directly without a translation layer.
var ErrChallengeNotFound = ports.ErrChallengeNotFound

// ErrInvalidDifficulty is returned when a challenge has an invalid difficulty level.
var ErrInvalidDifficulty = errors.New("invalid difficulty: must be one of junior, mid, senior, architect")

// ErrInvalidExpectedFindings is returned when expected_findings count is outside 1-3.
var ErrInvalidExpectedFindings = errors.New("expected_findings must have 1-3 items")

// ErrInvalidTestCases is returned when test_cases count is outside 2-4.
var ErrInvalidTestCases = errors.New("test_cases must have 2-4 items")

// ErrInvalidHintsCount is returned when hints count is not exactly 3.
var ErrInvalidHintsCount = errors.New("exactly 3 hints required")

// ErrSpoilerInDescription is returned when the description contains spoiler keywords.
var ErrSpoilerInDescription = errors.New("description contains spoiler content")

// ErrChallengeGenerateNotImplemented is removed in PR 2; the un-stubbed
// ChallengeService.GenerateChallenge now uses ErrChallengeGenerationNotWired
// (see below) when its dependencies are missing and three sibling
// sentinels for the orchestration failure paths.
//
// ErrChallengeGenerationNotWired is returned by ChallengeService.GenerateChallenge
// when its dependencies are not wired (specifically: when the LLM client
// is nil). The MCP mcp-challenges.generate_challenge tool surfaces this
// so an LLM caller gets an actionable "wire the LLM cascade via
// WithLLMClient" hint instead of a generic stack trace. The sentinel
// lives in core/services (not in the MCP adapter) so callers across
// transports (HTTP, MCP, future) branch with errors.Is uniformly.
var ErrChallengeGenerationNotWired = errors.New("challenge generation not wired (call WithLLMClient on ChallengeService.NewChallengeService)")

// ErrChallengeGenerationInvalidJSON is returned when the LLM cascade
// produced content that fails to parse as the v2 Challenge JSON
// schema. The orchestration loop retries up to 3 times by appending
// the parse error to the prompt; on the 3rd failure it returns this
// sentinel so the caller knows the failure mode is "LLM is emitting
// malformed payloads" rather than "sandbox gate" or "schema reject".
var ErrChallengeGenerationInvalidJSON = errors.New("challenge generation: llm response is not valid v2 challenge json")

// ErrChallengeGenerationInvalid is returned when the LLM's draft
// parses but fails validateChallengeV2 (cardinality or spoilers).
// The orchestration loop retries up to 3 times by appending the
// validation error to the prompt; on the 3rd failure this sentinel
// surfaces so callers can distinguish "LLM schema reject" from
// "sandbox gate failure" via errors.Is.
var ErrChallengeGenerationInvalid = errors.New("challenge generation: draft failed validateChallengeV2")

// ErrChallengeGenerationFailedSandbox is returned when the
// solutionCode + testCases combo fails the sandbox gate 3 times in a
// row. The orchestration retries by appending the sandbox error to
// the prompt; on the 3rd failure the sentinel surfaces and NO row is
// persisted (the service refuses to ship a known-broken challenge).
var ErrChallengeGenerationFailedSandbox = errors.New("challenge generation: sandbox gate failed 3 consecutive runs")

// validDifficulties contains allowed difficulty values.
var validDifficulties = map[string]bool{
	"junior":    true,
	"mid":       true,
	"senior":    true,
	"architect": true,
}

// spoilerKeywords is a deny-list of smell/vulnerability names that MUST NOT
// appear in a challenge description. The check is case-insensitive.
var spoilerKeywords = []string{
	"sql injection", "xss", "cross-site scripting", "god function",
	"callback hell", "prop mutation", "dead code", "silent failure",
	"poor naming", "n+1", "race condition", "memory leak",
}

// validateChallengeV2 checks v2 model constraints on a CreateChallengeInput.
// It MUST be called before persisting a new challenge.
func validateChallengeV2(input models.CreateChallengeInput) error {
	// expected_findings: 1-3 items
	if len(input.ExpectedFindings) < 1 || len(input.ExpectedFindings) > 3 {
		return ErrInvalidExpectedFindings
	}
	// test_cases: 2-4 items
	if len(input.TestCases) < 2 || len(input.TestCases) > 4 {
		return ErrInvalidTestCases
	}
	// hints: exactly 3 items
	if len(input.Hints) != 3 {
		return ErrInvalidHintsCount
	}
	// description: no spoiler keywords (case-insensitive)
	lowerDesc := strings.ToLower(input.Description)
	for _, kw := range spoilerKeywords {
		if strings.Contains(lowerDesc, kw) {
			return ErrSpoilerInDescription
		}
	}
	return nil
}

// IsValidationError returns true if the error is one of the known challenge
// validation errors. Used by the HTTP handler to decide 400 vs 500.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidDifficulty) ||
		errors.Is(err, ErrInvalidExpectedFindings) ||
		errors.Is(err, ErrInvalidTestCases) ||
		errors.Is(err, ErrInvalidHintsCount) ||
		errors.Is(err, ErrSpoilerInDescription)
}

// ChallengeService retrieves and creates challenges through the
// ChallengeRepository port. The service owns business rules
// (validation, normalization, dedup decision, response shaping);
// the port owns the SQL.
//
// PR 2 (free-practice-mode / S3) added four optional dependencies
// (CurriculumService, PedagogicalService, LLMClientPort,
// SandboxExecutor) wired through option funcs so the un-stubbed
// GenerateChallenge can drive curriculum + pedagogical + LLM + sandbox
// orchestration. Existing v1 callers keep compiling because every
// option func is additive + nil-safe.
type ChallengeService struct {
	repo      ports.ChallengeRepository
	curricula *CurriculumService
	pedagogy  *PedagogicalService
	llm       ports.LLMClientPort
	sandbox   ports.SandboxExecutor
}

// ChallengeServiceOption configures a ChallengeService. Reserved for
// future overrides (custom prompt templates, alternate sandboxes,
// etc.); keeping it as a function option keeps the zero-arg
// constructor compatible with the existing main.go call-site.
type ChallengeServiceOption func(*ChallengeService)

// WithCurriculumService wires the free-practice orchestration's
// syllabus source. When nil, GenerateChallenge skips the topic
// lookup and uses a generic prompt; when non-nil, the service
// reads .atl/curricula/<lang>.md via the supplied service.
func WithCurriculumService(s *CurriculumService) ChallengeServiceOption {
	return func(c *ChallengeService) { c.curricula = s }
}

// WithPedagogicalService wires the socratic engine that produces
// learning objectives + story contexts. When nil, GenerateChallenge
// uses generic placeholders; when non-nil, the service calls
// GetLearningObjective + GenerateStoryContext to source the
// system/prompt strings.
func WithPedagogicalService(p *PedagogicalService) ChallengeServiceOption {
	return func(c *ChallengeService) { c.pedagogy = p }
}

// WithLLMClient wires the LLM cascade adapter (Groq → DeepSeek →
// MiniMax in production). When nil, GenerateChallenge returns
// ErrChallengeGenerationNotWired without touching the repo.
func WithLLMClient(llm ports.LLMClientPort) ChallengeServiceOption {
	return func(c *ChallengeService) { c.llm = llm }
}

// WithSandboxExecutor wires the hardened sandbox the solution-code
// gate runs against. When nil, GenerateChallenge skips the sandbox
// gate and persists on the first validateChallengeV2 pass (degraded
// mode — useful for tests + dev).
func WithSandboxExecutor(s ports.SandboxExecutor) ChallengeServiceOption {
	return func(c *ChallengeService) { c.sandbox = s }
}

// NewChallengeService creates a new ChallengeService backed by the
// given repository. The repository MUST be non-nil; nil-safe wiring
// is the wiring layer's job (e.g. cmd/api/wiring_test.go passes nil
// to verify registry count without invoking any tool).
//
// Optional dependencies (CurriculumService, PedagogicalService,
// LLMClientPort, SandboxExecutor) are wired through option funcs and
// are nil-safe — every option func accepts a nil value without
// panicking, so the existing one-arg call shape keeps compiling.
func NewChallengeService(repo ports.ChallengeRepository, opts ...ChallengeServiceOption) *ChallengeService {
	c := &ChallengeService{repo: repo}
	for _, o := range opts {
		o(c)
	}
	return c
}

// difficultyForLevel maps the per-user F..S rank letter to the
// 4-bucket difficulty column on the persisted Challenge. Unknown
// letters return the empty string so the caller can branch on
// "missing level" explicitly rather than silently coercing to a
// default that would mislead the scoring service.
//
//	F,E → junior
//	D,C → mid
//	B,A → senior
//	S   → architect
func difficultyForLevel(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "F", "E":
		return "junior"
	case "D", "C":
		return "mid"
	case "B", "A":
		return "senior"
	case "S":
		return "architect"
	}
	return ""
}

// generateUUID generates a version 4 UUID using crypto/rand.
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// Create inserts a new challenge or returns the existing one if
// source_repo+user_id already exists. Returns (challenge, created,
// nil) where created is true for new inserts and false for dedup
// hits.
func (s *ChallengeService) Create(ctx context.Context, input models.CreateChallengeInput, userID string) (*models.Challenge, bool, error) {
	// Validate difficulty
	if !validDifficulties[input.Difficulty] {
		return nil, false, ErrInvalidDifficulty
	}

	// Validate v2 model constraints (expected_findings, test_cases,
	// hints, no spoilers)
	if err := validateChallengeV2(input); err != nil {
		return nil, false, err
	}

	// Normalize source_repo for dedup (lowercase + trim whitespace).
	normalizedRepo := strings.ToLower(strings.TrimSpace(input.SourceRepo))

	// Check for duplicate — the port returns ErrChallengeNotFound on
	// miss (not sql.ErrNoRows), keeping the SQL detail out of core.
	existing, err := s.repo.FindBySourceRepoAndUser(ctx, normalizedRepo, userID)
	if err == nil {
		// Found existing challenge (dedup hit)
		return &existing, false, nil
	}
	if !errors.Is(err, ports.ErrChallengeNotFound) {
		return nil, false, fmt.Errorf("checking for duplicate challenge: %w", err)
	}

	// No duplicate — generate UUID, then insert.
	id, err := generateUUID()
	if err != nil {
		return nil, false, fmt.Errorf("generating UUID: %w", err)
	}

	status := "available"

	// Build the persisted model. SourceRepo is already normalized;
	// the adapter stores exactly what we pass.
	challenge := &models.Challenge{
		ID:                   id,
		Title:                input.Title,
		Description:          input.Description,
		Difficulty:           input.Difficulty,
		Category:             input.Category,
		Language:             input.Language,
		RepoURL:              input.RepoURL,
		Code:                 input.Code,
		CodeSmell:            input.CodeSmell,
		Status:               status,
		UserID:               &userID,
		SourceRepo:           normalizedRepo,
		LearningObjectives:   input.LearningObjectives,
		Hints:                input.Hints,
		CommonMistakes:       input.CommonMistakes,
		EstimatedTimeMinutes: input.EstimatedTimeMinutes,
		ExpectedFindings:     input.ExpectedFindings,
		TestCases:            input.TestCases,
		LinterRules:          input.LinterRules,
		SolutionCode:         input.SolutionCode,
		SolutionExplanation:  input.SolutionExplanation,
		BasePoints:           input.BasePoints,
		BonusPoints:          input.BonusPoints,
		PenaltyPerHint:       input.PenaltyPerHint,
		TimeBonus:            input.TimeBonus,
		Origin:               input.Origin,
		SourcePath:           input.SourcePath,
		GeneratedBy:          input.GeneratedBy,
		CreatedBy:            input.CreatedBy,
	}

	createdAt, err := s.repo.Insert(ctx, challenge)
	if err != nil {
		return nil, false, fmt.Errorf("inserting challenge: %w", err)
	}
	challenge.CreatedAt = createdAt
	return challenge, true, nil
}

// GetAll returns all challenges visible to the given user: seeds
// (NULL user_id) plus user-owned. Solution fields (solution_code,
// solution_explanation) are NOT included in the response.
func (s *ChallengeService) GetAll(ctx context.Context, userID string) ([]models.Challenge, error) {
	return s.repo.ListVisibleToUser(ctx, userID)
}

// GetByID returns a single challenge by ID visible to the given user,
// or ErrChallengeNotFound. Solution fields are NOT included — use
// GetChallengeWithDetails for the full challenge.
func (s *ChallengeService) GetByID(ctx context.Context, id string, userID string) (models.Challenge, error) {
	return s.repo.FindByIDVisibleToUser(ctx, id, userID)
}

// GetChallengeWithDetails returns a challenge including solution_code and
// solution_explanation. Use this only when the caller is authorized to see
// the solution (tutor context or reveal flag).
func (s *ChallengeService) GetChallengeWithDetails(ctx context.Context, id string, userID string) (models.Challenge, error) {
	return s.repo.FindByIDWithSolutionVisibleToUser(ctx, id, userID)
}

// ValidateChallenge validates a CreateChallengeInput without persisting it.
// It is the public surface of validateChallengeV2 so the MCP
// mcp-challenges.validate_challenge tool can give the LLM actionable
// feedback on a draft challenge BEFORE the user commits to it. The
// returned error (when non-nil) is one of the IsValidationError sentinels
// declared above so callers can branch with errors.Is.
func (s *ChallengeService) ValidateChallenge(input models.CreateChallengeInput) error {
	return validateChallengeV2(input)
}

// GenerateChallenge returns a freshly-authored challenge driven by the
// curriculum + pedagogical + LLM + sandbox stack. The orchestration:
//
//  1. Validate inputs (unwired service → ErrChallengeGenerationNotWired;
//     missing topic/language → the matching curriculum sentinel).
//  2. Read topic (curriculum.GetTopic).
//  3. Fetch learning objective (pedagogy.GetLearningObjective).
//  4. Build story context (pedagogy.GenerateStoryContext).
//  5. Assemble the system + prompt strings.
//  6. Loop ≤ 3 attempts:
//     a. llm.Complete in JSON-mode with the v2 Challenge schema.
//     b. Parse the response into CreateChallengeInput.
//     c. Run sandbox.Execute on the solution code (the gate).
//     d. validateChallengeV2 (cardinality + spoilers).
//     e. On success → build the persisted Challenge, repo.Insert, return.
//  7. After 3 attempts, return the matching sentinel:
//     ErrChallengeGenerationInvalidJSON /
//     ErrChallengeGenerationInvalid /
//     ErrChallengeGenerationFailedSandbox.
//
// Per the spec, the persisted Challenge row carries
//
//	origin    = "generated"
//	generatedBy = "free-practice-v1"
//	createdBy   = <userID argument>
//	difficulty  = difficultyForLevel(hint.Level)  // F..S → 4-bucket
//
// The MCP mcp-challenges.generate_challenge tool delegates here so the
// business decisions (unwired / JSON parse / sandbox fail / validate
// fail) live in core/services — not in the transport adapter. Callers
// branch with errors.Is to surface stable errors regardless of
// transport.
func (s *ChallengeService) GenerateChallenge(ctx context.Context, userID string, hint models.GenerateChallengeHint) (models.Challenge, error) {
	if s.llm == nil {
		return models.Challenge{}, ErrChallengeGenerationNotWired
	}
	if hint.Language == "" || hint.TopicID == "" {
		return models.Challenge{}, ErrChallengeGenerationNotWired
	}

	// 1. Read topic — surface curriculum sentinels verbatim so the
	// UI can branch with errors.Is on ErrCurriculumTopicNotFound /
	// ErrCurriculumLanguageNotFound / ErrCurriculumInvalidLanguage.
	if s.curricula != nil {
		if _, err := s.curricula.GetTopic(ctx, hint.Language, hint.TopicID); err != nil {
			return models.Challenge{}, err
		}
	}

	// 2 + 3. Fetch the pedagogical scaffolding. PedagogicalService
	// degrades to generic placeholders when its own curricula dep is
	// nil, so a partial wiring still produces a usable prompt.
	var (
		objective string
		outcomes  []string
		story     string
		scenario  string
	)
	if s.pedagogy != nil {
		obj, err := s.pedagogy.GetLearningObjective(ctx, LearningObjectiveInput{
			Language: hint.Language,
			TopicID:  hint.TopicID,
			Level:    hint.Level,
		})
		if err != nil {
			return models.Challenge{}, err
		}
		objective = obj.Objective
		outcomes = obj.Outcomes
		sc, err := s.pedagogy.GenerateStoryContext(ctx, StoryContextInput{
			Language:  hint.Language,
			TopicID:   hint.TopicID,
			UserLevel: hint.Level,
		})
		if err != nil {
			return models.Challenge{}, err
		}
		story = sc.Story
		scenario = sc.Scenario
	}

	// 4. Assemble System + Prompt. The system string carries the
	// identity + no-spoiler rule; the prompt folds topic + level +
	// story + objective into one string the LLM parses as a single
	// request.
	system := "You are a senior code auditor generating a runnable " +
		"challenge. Do NOT include spoiler keywords in the description."
	prompt := buildGenerationPrompt(hint, objective, outcomes, story, scenario)

	// 5. Loop ≤ 3 attempts: LLM → JSON parse → sandbox gate → validate.
	const maxAttempts = 3
	var (
		draft   models.CreateChallengeInput
		lastErr error
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptPrompt := prompt
		if attempt > 1 && lastErr != nil {
			attemptPrompt += "\n\nPrevious attempt failed: " + lastErr.Error() +
				". Please correct and return the full JSON again."
		}
		resp, err := s.llm.Complete(ctx, ports.CompleteRequest{
			System:     system,
			Prompt:     attemptPrompt,
			JSONSchema: challengeJSONSchema(),
		})
		if err != nil {
			return models.Challenge{}, err
		}

		// Parse JSON. On failure, retry; on the last attempt surface
		// ErrChallengeGenerationInvalidJSON.
		if jerr := json.Unmarshal([]byte(resp.Content), &draft); jerr != nil {
			lastErr = fmt.Errorf("%w: %s", ErrChallengeGenerationInvalidJSON, jerr.Error())
			if attempt == maxAttempts {
				return models.Challenge{}, ErrChallengeGenerationInvalidJSON
			}
			continue
		}

		// Sandbox gate (Phase 5). Runs solutionCode + testCases
		// together per the spec scenario Solution-Validates Sandbox
		// Gate. When sandbox is nil (degraded mode / unit tests with
		// no fake wired), skip the gate. When draft.TestCases is empty
		// the executor returns a zero-tests result that counts as a
		// pass; if the executor surfaces a non-zero ExitCode with zero
		// tests it signals a driver crash (which we still treat as a
		// gate failure).
		if s.sandbox != nil && draft.SolutionCode != "" {
			wireTests, convErr := modelTestCasesToWire(draft.TestCases)
			if convErr != nil {
				lastErr = fmt.Errorf("%w: %s", ErrChallengeGenerationFailedSandbox, convErr.Error())
				if attempt == maxAttempts {
					return models.Challenge{}, ErrChallengeGenerationFailedSandbox
				}
				continue
			}
			res, execErr := s.sandbox.ExecuteWithTests(ctx, hint.Language, draft.SolutionCode, wireTests, 15)
			if execErr != nil {
				lastErr = fmt.Errorf("%w: %s", ErrChallengeGenerationFailedSandbox, execErr.Error())
				if attempt == maxAttempts {
					return models.Challenge{}, ErrChallengeGenerationFailedSandbox
				}
				continue
			}
			// Close any reader the executor left open; the gate's
			// pass/fail signal is the TestRunResult, not stdout.
			if res.Output != nil {
				if closer, ok := res.Output.(io.Closer); ok {
					_ = closer.Close()
				}
			}
			// Gate decision.
			if res.TestsPass < res.TestsRun || res.ExitCode != 0 {
				lastErr = fmt.Errorf("%w: testsPass=%d testsRun=%d exitCode=%d perTest=%s",
					ErrChallengeGenerationFailedSandbox,
					res.TestsPass, res.TestsRun, res.ExitCode,
					summarizePerTest(res.PerTest))
				if attempt == maxAttempts {
					return models.Challenge{}, ErrChallengeGenerationFailedSandbox
				}
				continue
			}
		}

		// v2 validation. Card + spoiler + difficulty constraints.
		if verr := validateChallengeV2(draft); verr != nil {
			lastErr = fmt.Errorf("%w: %s", ErrChallengeGenerationInvalid, verr.Error())
			if attempt == maxAttempts {
				return models.Challenge{}, ErrChallengeGenerationInvalid
			}
			continue
		}

		// Success — build the persisted Challenge.
		lastErr = nil
		break
	}
	if lastErr != nil {
		// All three attempts failed; surface the deepest matching
		// sentinel. The retry loop already set lastErr to one of the
		// three failure sentinels above so the unwrap here is just
		// defense-in-depth (e.g. if validation produced a non-v2 error
		// path somehow).
		return models.Challenge{}, lastErr
	}

	// 6. Persist. The persisted row carries the canonical
	// origin/generatedBy/createdBy per the spec.
	difficulty := difficultyForLevel(hint.Level)
	id, err := generateUUID()
	if err != nil {
		return models.Challenge{}, fmt.Errorf("generating UUID: %w", err)
	}
	userIDCopy := userID
	challenge := &models.Challenge{
		ID:                   id,
		Title:                draft.Title,
		Description:          draft.Description,
		Difficulty:           difficulty,
		Category:             draft.Category,
		Language:             draft.Language,
		Code:                 draft.Code,
		Status:               "available",
		UserID:               &userIDCopy,
		SourceRepo:           "",
		LearningObjectives:   draft.LearningObjectives,
		Hints:                draft.Hints,
		CommonMistakes:       draft.CommonMistakes,
		EstimatedTimeMinutes: draft.EstimatedTimeMinutes,
		ExpectedFindings:     draft.ExpectedFindings,
		TestCases:            draft.TestCases,
		LinterRules:          draft.LinterRules,
		SolutionCode:         draft.SolutionCode,
		SolutionExplanation:  draft.SolutionExplanation,
		BasePoints:           draft.BasePoints,
		BonusPoints:          draft.BonusPoints,
		PenaltyPerHint:       draft.PenaltyPerHint,
		TimeBonus:            draft.TimeBonus,
		Origin:               "generated",
		GeneratedBy:          "free-practice-v1",
		CreatedBy:            userID,
	}
	createdAt, err := s.repo.Insert(ctx, challenge)
	if err != nil {
		return models.Challenge{}, fmt.Errorf("inserting generated challenge: %w", err)
	}
	challenge.CreatedAt = createdAt
	return *challenge, nil
}

// buildGenerationPrompt folds the topic, level, story, objective, and
// outcomes into a single user-side prompt string the LLM parses.
// Kept as a package-private helper so a test can assert the exact
// shape (see TestGenerateChallenge_PromptAssembly_IncludesIdentityAndContext).
func buildGenerationPrompt(hint models.GenerateChallengeHint, objective string, outcomes []string, story, scenario string) string {
	var b strings.Builder
	b.WriteString("Language: ")
	b.WriteString(hint.Language)
	b.WriteString("\nTopic: ")
	b.WriteString(hint.TopicID)
	b.WriteString("\nLevel: ")
	b.WriteString(hint.Level)
	if story != "" {
		b.WriteString("\nStory: ")
		b.WriteString(story)
	}
	if scenario != "" {
		b.WriteString("\nScenario: ")
		b.WriteString(scenario)
	}
	if objective != "" {
		b.WriteString("\nLearning objective: ")
		b.WriteString(objective)
	}
	if len(outcomes) > 0 {
		b.WriteString("\nOutcomes:\n")
		for _, o := range outcomes {
			b.WriteString("- ")
			b.WriteString(o)
			b.WriteString("\n")
		}
	}
	b.WriteString("\nReturn JSON matching the v2 Challenge schema (object with title, description, language, code, hints, expectedFindings, testCases, solutionCode, solutionExplanation, commonMistakes, learningObjectives).")
	return b.String()
}

// summarizePerTest flattens the per-test outcomes into a short
// string the LLM prompt can digest on the next retry attempt. We
// keep names + pass/fail + a short error excerpt; full bodies are
// not needed because the LLM already saw the original test
// expectations in the prior attempt's prompt.
func summarizePerTest(perTest []ports.TestOutcome) string {
	if len(perTest) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, o := range perTest {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "{%s passed=%v", o.Name, o.Passed)
		if !o.Passed && o.ErrorMessage != "" {
			fmt.Fprintf(&b, " error=%q", truncateErr(o.ErrorMessage))
		}
		b.WriteByte('}')
	}
	b.WriteByte(']')
	return b.String()
}

// truncateErr keeps the error excerpt small (max ~80 chars) so the
// retry prompt stays under the LLM's input budget. Long stack
// traces are clipped with an ellipsis.
func truncateErr(s string) string {
	const max = 80
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// modelTestCasesToWire maps the domain TestCase (Input /
// ExpectedOutput are arbitrary JSON values) into the wire shape
// the test driver consumes (Input / ExpectedOutput are JSON-encoded
// strings). A marshal failure surfaces as the sentinel so the retry
// loop can record the cause.
func modelTestCasesToWire(in []models.TestCase) ([]ports.TestCase, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]ports.TestCase, len(in))
	for i, tc := range in {
		inputJSON, err := json.Marshal(tc.Input)
		if err != nil {
			return nil, fmt.Errorf("test[%d] input marshal: %w", i, err)
		}
		expectedJSON, err := json.Marshal(tc.ExpectedOutput)
		if err != nil {
			return nil, fmt.Errorf("test[%d] expectedOutput marshal: %w", i, err)
		}
		out[i] = ports.TestCase{
			Name:           tc.Name,
			Input:          string(inputJSON),
			ExpectedOutput: string(expectedJSON),
			Weight:         tc.Weight,
		}
	}
	return out, nil
}

// challengeJSONSchema returns the JSONSchema passed to
// ports.CompleteRequest. Mirrors the schema in design.md §Data
// Shapes — kept as a function so a future change to the v2 Challenge
// shape (e.g. adding a field) lands in one place.
func challengeJSONSchema() *ports.JSONSchema {
	return &ports.JSONSchema{
		Type: "object",
		Properties: map[string]any{
			"title":              map[string]any{"type": "string"},
			"description":        map[string]any{"type": "string"},
			"language":           map[string]any{"type": "string"},
			"code":               map[string]any{"type": "string"},
			"learningObjectives": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"hints": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"level":       map[string]any{"type": "integer"},
						"content":     map[string]any{"type": "string"},
						"cost_points": map[string]any{"type": "integer"},
					},
				},
			},
			"commonMistakes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"expectedFindings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"severity":      map[string]any{"type": "string"},
						"category":      map[string]any{"type": "string"},
						"message":       map[string]any{"type": "string"},
						"evidence":      map[string]any{"type": "string"},
						"suggested_fix": map[string]any{"type": "string"},
					},
				},
			},
			"testCases": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":           map[string]any{"type": "string"},
						"input":          map[string]any{},
						"expectedOutput": map[string]any{},
						"weight":         map[string]any{"type": "integer"},
					},
				},
			},
			"linterRules": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
				},
			},
			"solutionCode":        map[string]any{"type": "string"},
			"solutionExplanation": map[string]any{"type": "string"},
		},
		Required: []string{
			"title", "description", "language", "code",
			"hints", "expectedFindings", "testCases", "solutionCode",
		},
		AdditionalProperties: false,
	}
}
