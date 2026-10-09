package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// PedagogicalService is the socratic engine behind the mcp-pedagogical
// tool group: learning objectives, story context, scaffolded steps,
// solution validation, socratic prompts, and peer-review feedback.
//
// Per AGENTS.md this service is "the centerpiece of the tutor chat
// flow" and the spec mandates a prompt-injection guard on
// GetSocraticPrompt. The guard lives HERE (service layer) AND on the
// MCP tool layer in infrastructure/mcp/pedagogical — defense in depth
// so an LLM-driven caller that bypasses the tool layer still cannot
// poison the prompt constructor.
//
// PR 1 shipped a compilable stub; PR 4 fills it with deterministic
// logic (no LLM dependency yet — the S5 cascade refines the prompt
// quality). The optional *CurriculumService dependency is injected by
// the cmd/api composition root and short-circuits topic lookups to
// deterministic defaults when nil.
type PedagogicalService struct {
	curricula *CurriculumService
}

// ErrPedagogicalValidation is returned for input-validation failures
// that are pedagogical-specific (level range, context injection,
// missing required fields). The MCP tool layer maps this to
// ErrInvalidParams (-32602) so the dispatcher surfaces a uniform JSON-RPC
// error envelope.
//
// Routing language / topic_id validation through ErrCurriculumInvalidLanguage
// (the curriculum service's sentinel) lets the MCP tool layer branch
// with errors.Is while the service returns the most-actionable error:
// pedagogical problems first (level out of range), curriculum problems
// second (path-traversal).
var ErrPedagogicalValidation = errors.New("pedagogical validation failed")

// maxSocraticContextBytes caps the user-supplied `context` payload at
// 2KB. Real-world socratic contexts are conversational (a few
// sentences); clamping prevents the LLM window from being blown out
// by adversarial large-payload calls before the injection patterns can
// run.
const maxSocraticContextBytes = 2048

// maxValidateSolutionBytes caps the user's `code` payload. The MCP
// dispatcher's MaxResponseBytes is 50KB; validating against the same
// scale keeps both ends coherent.
const maxValidateSolutionBytes = 50 * 1024

// socraticInjectionPatterns are lowercased substrings a malicious
// caller can use to take over the system prompt. The list covers the
// three documented shapes the design.md threat matrix calls out plus
// a few lower-friction variants:
//
//   - "ignore all previous instructions" (canonical jailbreak)
//   - role-tag prefixes ("system:", "assistant:", "user:")
//   - chat-template role markers (<|system|>, <|user|>, <|assistant|>)
//   - "disregard" and "forget" variants on the "ignore" theme
//
// The guard rejects the call rather than stripping the substring so
// the LLM never sees the adversarial text even partially. The MCP
// tool layer runs the same guard (defense in depth) and surfaces
// ErrInvalidParams before invoking the service.
var socraticInjectionPatterns = []string{
	"ignore all previous instructions",
	"ignore previous instructions",
	"ignore the above",
	"ignore the prompt",
	"disregard all prior",
	"disregard the system",
	"forget all previous",
	"system:",
	"assistant:",
	"user:",
	"<|system|>",
	"<|user|>",
	"<|assistant|>",
}

// NewPedagogicalService wires the pedagogical engine to an optional
// CurriculumService. A nil curricula is acceptable in tests; the
// service degrades to its deterministic defaults and skips topic
// lookup. Production main.go passes a fully-wired CurriculumService so
// GetLearningObjective can read real curriculum bodies.
func NewPedagogicalService(curricula *CurriculumService) *PedagogicalService {
	return &PedagogicalService{curricula: curricula}
}

// --- Input/output shapes ---------------------------------------------------

// LearningObjectiveInput scopes a learning objective lookup.
type LearningObjectiveInput struct {
	Language string `json:"language"`
	TopicID  string `json:"topic_id"`
	Level    string `json:"level,omitempty"`
}

// LearningObjective is the structured outcome of get_learning_objective.
type LearningObjective struct {
	Objective string   `json:"objective"`
	Outcomes  []string `json:"outcomes,omitempty"`
}

// StoryContextInput carries the scenario for generate_story_context.
type StoryContextInput struct {
	Language  string `json:"language"`
	TopicID   string `json:"topic_id"`
	Scenario  string `json:"scenario,omitempty"`
	UserLevel string `json:"user_level,omitempty"`
}

// StoryContext is the narrative wrapper produced for a topic.
type StoryContext struct {
	Story    string `json:"story"`
	Scenario string `json:"scenario,omitempty"`
}

// ScaffoldedStepsInput scopes a scaffolding request.
type ScaffoldedStepsInput struct {
	Language  string `json:"language"`
	TopicID   string `json:"topic_id"`
	Objective string `json:"objective,omitempty"`
	Levels    int    `json:"levels,omitempty"`
}

// ScaffoldingStep is one progressive hint toward the objective.
type ScaffoldingStep struct {
	Level  int    `json:"level"`
	Prompt string `json:"prompt"`
	Detail string `json:"detail,omitempty"`
}

// ValidateSolutionInput carries the user's attempt for validation.
type ValidateSolutionInput struct {
	Language string `json:"language"`
	TopicID  string `json:"topic_id"`
	Code     string `json:"code"`
}

// ValidationResult is the outcome of validate_solution.
type ValidationResult struct {
	Passed   bool     `json:"passed"`
	Feedback []string `json:"feedback,omitempty"`
	Score    int      `json:"score,omitempty"`
}

// SocraticPromptInput tunes the socratic level (0=direct … 3=discovery).
type SocraticPromptInput struct {
	Language string `json:"language"`
	TopicID  string `json:"topic_id"`
	Level    int    `json:"level"`
	Context  string `json:"context,omitempty"`
}

// SocraticPrompt is a guiding question (never the answer).
type SocraticPrompt struct {
	Question    string   `json:"question"`
	Hint        string   `json:"hint,omitempty"`
	Breadcrumbs []string `json:"breadcrumbs,omitempty"`
}

// PeerReviewInput scopes a peer-review request.
type PeerReviewInput struct {
	Language string `json:"language"`
	TopicID  string `json:"topic_id"`
	Comment  string `json:"comment"`
}

// PeerReviewResult is the structured peer-review analysis.
type PeerReviewResult struct {
	Strengths    []string `json:"strengths,omitempty"`
	Improvements []string `json:"improvements,omitempty"`
	MissedPoints []string `json:"missed_points,omitempty"`
}

// --- GetLearningObjective --------------------------------------------------

// GetLearningObjective composes a learning objective for the supplied
// topic. With a wired CurriculumService it pulls the topic's body and
// extracts the first two exercises as outcomes; with a nil curricula
// (test path / wiring race) it returns a generic "Understand <topic>"
// objective so the LLM still has something to work with.
func (s *PedagogicalService) GetLearningObjective(_ context.Context, in LearningObjectiveInput) (LearningObjective, error) {
	language := strings.TrimSpace(in.Language)
	topicID := strings.TrimSpace(in.TopicID)
	if language == "" || topicID == "" {
		return LearningObjective{}, fmt.Errorf("%w: language and topic_id are required", ErrPedagogicalValidation)
	}
	if s.curricula == nil {
		return LearningObjective{
			Objective: fmt.Sprintf("Understand %s in %s.", topicID, language),
			Outcomes:  []string{"identify the concept", "use it in a small example"},
		}, nil
	}
	detail, err := s.curricula.GetTopic(context.Background(), language, topicID)
	if err != nil {
		// Topic not found → degrade to a generic objective rather than
		// surface 404 to the LLM. The MCP tool layer maps unknown-
		// topic sentinel back to ErrInvalidParams upstream, but
		// log on the service path it remains a "best-effort" outcome.
		if errors.Is(err, ErrCurriculumInvalidLanguage) {
			return LearningObjective{}, err
		}
		return LearningObjective{
			Objective: fmt.Sprintf("Understand %s.", topicID),
			Outcomes:  []string{"identify the concept"},
		}, nil
	}
	out := []string{}
	if len(detail.Exercises) > 0 {
		upto := len(detail.Exercises)
		if upto > 2 {
			upto = 2
		}
		out = detail.Exercises[:upto]
	}
	objective := strings.TrimSpace(detail.Description)
	if objective == "" {
		objective = fmt.Sprintf("Understand %s.", topicID)
	}
	return LearningObjective{Objective: objective, Outcomes: out}, nil
}

// --- GenerateStoryContext --------------------------------------------------

// GenerateStoryContext weaves a relatable scenario around the topic
// for the tutor to surface as a story-style prompt. PR 4 ships a
// deterministic template — the S5 LLM cascade refines the narrative;
// for now the function gives the LLM a structured hook (the topic's
// description) so the LLM only has to flavor, not invent.
func (s *PedagogicalService) GenerateStoryContext(_ context.Context, in StoryContextInput) (StoryContext, error) {
	language := strings.TrimSpace(in.Language)
	topicID := strings.TrimSpace(in.TopicID)
	scenario := strings.TrimSpace(in.Scenario)
	userLevel := strings.TrimSpace(in.UserLevel)
	if language == "" || topicID == "" {
		return StoryContext{}, fmt.Errorf("%w: language and topic_id are required", ErrPedagogicalValidation)
	}
	if userLevel == "" {
		userLevel = "F"
	}
	hook := "Imagine you are building a small TypeScript project and need to"
	if strings.ToLower(language) != "typescript" {
		hook = fmt.Sprintf("Imagine you are building a small %s project and need to", language)
	}
	story := fmt.Sprintf("%s %s. Your task: write it in a way another developer could read in under a minute.", hook, topicID)
	scenarioText := fmt.Sprintf("Open a %s scratch file and try %s at the %s level. Focus on naming and shape — correctness comes from clarity.", language, topicID, userLevel)
	if scenario != "" {
		scenarioText = scenario + " " + scenarioText
	}
	return StoryContext{Story: story, Scenario: scenarioText}, nil
}

// --- CreateScaffoldedSteps -------------------------------------------------

// CreateScaffoldedSteps emits progressively revealing hints toward the
// supplied objective. The count defaults to 3 (a tasteful arc that
// matches the docs/curricula-format.md "exercises" guidance) and clamps
// to [1..6] so an LLM caller cannot manufacture 100 hints via the
// Levels input.
func (s *PedagogicalService) CreateScaffoldedSteps(_ context.Context, in ScaffoldedStepsInput) ([]ScaffoldingStep, error) {
	language := strings.TrimSpace(in.Language)
	topicID := strings.TrimSpace(in.TopicID)
	if language == "" || topicID == "" {
		return nil, fmt.Errorf("%w: language and topic_id are required", ErrPedagogicalValidation)
	}
	objective := strings.TrimSpace(in.Objective)
	if objective == "" {
		objective = fmt.Sprintf("understand %s", topicID)
	}
	levels := in.Levels
	if levels <= 0 {
		levels = 3
	}
	if levels > 6 {
		levels = 6
	}
	out := make([]ScaffoldingStep, 0, levels)
	for i := 1; i <= levels; i++ {
		out = append(out, s.scaffoldOne(language, topicID, objective, i, levels))
	}
	return out, nil
}

// scaffoldOne shapes a single hint per the level position. The level
// number reflects progressive disclosure: step 1 hints the
// shape/structure, the last step hints the answer's flavor without
// crossing the socratic line (the tutor never solves the problem).
func (s *PedagogicalService) scaffoldOne(language, topicID, objective string, pos, total int) ScaffoldingStep {
	switch {
	case pos == 1:
		return ScaffoldingStep{
			Level:  pos,
			Prompt: fmt.Sprintf("Let's start with %s in %s. What's the very first shape this needs?", topicID, language),
			Detail: "Name the language construct you're reaching for and why it fits the goal: " + objective,
		}
	case pos == total:
		return ScaffoldingStep{
			Level:  pos,
			Prompt: fmt.Sprintf("One last nudge — how does %s express the answer most clearly here?", topicID),
			Detail: fmt.Sprintf("Compare your approach to the most idiomatic %s pattern; refactor toward that.", language),
		}
	default:
		return ScaffoldingStep{
			Level:  pos,
			Prompt: fmt.Sprintf("Step %d of %d: what changes in %s if you trade the common shortcut for the safer one?", pos, total, language),
			Detail: "Look for the edge case your approach can't represent yet; that's the next surface to cover.",
		}
	}
}

// --- ValidateSolution ------------------------------------------------------

// ValidateSolution runs the static checks a learning tool can perform
// without booting the sandbox: the code is non-empty, within size
// limits, and identifies the topic by header comment for traceability.
// PR 4 keeps it deterministic and side-effect-free; sandbox execution
// lands with the S3 free-practice spec (where mcp-sandbox.run_code
// becomes the deep validator).
func (s *PedagogicalService) ValidateSolution(_ context.Context, in ValidateSolutionInput) (ValidationResult, error) {
	language := strings.TrimSpace(in.Language)
	topicID := strings.TrimSpace(in.TopicID)
	code := in.Code
	if language == "" || topicID == "" {
		return ValidationResult{}, fmt.Errorf("%w: language and topic_id are required", ErrPedagogicalValidation)
	}
	if strings.TrimSpace(code) == "" {
		return ValidationResult{}, fmt.Errorf("%w: code is required", ErrPedagogicalValidation)
	}
	if len(code) > maxValidateSolutionBytes {
		return ValidationResult{}, fmt.Errorf("%w: code exceeds %d bytes", ErrPedagogicalValidation, maxValidateSolutionBytes)
	}
	feedback := []string{}
	headerMatch := false
	for _, line := range strings.Split(code, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.HasPrefix(strings.TrimSpace(line), "#") {
			low := strings.ToLower(line)
			if strings.Contains(low, strings.ToLower(topicID)) {
				headerMatch = true
				break
			}
		}
	}
	if !headerMatch {
		feedback = append(feedback, fmt.Sprintf("Add a header comment that references the %s topic for traceability.", topicID))
	}
	if len(strings.TrimSpace(code)) < 40 {
		feedback = append(feedback, "Solution is very short — try to express the idea with a small example you can actually run.")
	}
	if len(strings.Split(code, "\n")) <= 2 {
		feedback = append(feedback, "Break the solution into named steps so a reviewer can follow the intent.")
	}
	score := 100 - 15*len(feedback)
	if score < 60 {
		score = 60
	}
	return ValidationResult{
		Passed:   len(feedback) == 0,
		Feedback: feedback,
		Score:    score,
	}, nil
}

// --- GetSocraticPrompt -----------------------------------------------------

// GetSocraticPrompt produces a socratic-level-shaped guiding question.
// Per AGENTS.md: the tutor must NEVER solve the problem — the tool's
// output is a question (and breadcrumbs at higher levels), never an
// answer.
//
// The function runs the prompt-injection guard BEFORE any curriculum
// lookup. The MCP tool layer runs the same guard (defense in depth).
func (s *PedagogicalService) GetSocraticPrompt(_ context.Context, in SocraticPromptInput) (SocraticPrompt, error) {
	if err := s.validateSocraticInput(in); err != nil {
		return SocraticPrompt{}, err
	}
	topicTitle := in.TopicID
	if s.curricula != nil {
		detail, lookupErr := s.curricula.GetTopic(context.Background(), in.Language, in.TopicID)
		if lookupErr == nil {
			topicTitle = detail.Title
		}
	}
	return s.socraticFor(in.Language, topicTitle, in.Context, in.Level), nil
}

func (s *PedagogicalService) validateSocraticInput(in SocraticPromptInput) error {
	if in.Level < 0 || in.Level > 3 {
		return fmt.Errorf("%w: level must be in [0..3], got %d", ErrPedagogicalValidation, in.Level)
	}
	if strings.TrimSpace(in.Language) == "" {
		return fmt.Errorf("%w: language is required", ErrPedagogicalValidation)
	}
	if strings.TrimSpace(in.TopicID) == "" {
		return fmt.Errorf("%w: topic_id is required", ErrPedagogicalValidation)
	}
	ctx := in.Context
	if len(ctx) > maxSocraticContextBytes {
		return fmt.Errorf("%w: context exceeds %d bytes", ErrPedagogicalValidation, maxSocraticContextBytes)
	}
	lowered := strings.ToLower(ctx)
	for _, p := range socraticInjectionPatterns {
		if strings.Contains(lowered, p) {
			return fmt.Errorf("%w: context contains forbidden pattern", ErrPedagogicalValidation)
		}
	}
	return nil
}

// socraticFor emits the question + breadcrumbs for a (language, topic,
// context, level) tuple. The level shaping mirrors tutor_lookup_service's
// framing taxonomy:
//
//	Level 0 (direct)      — Hint + a concrete next step.
//	Level 1 (guided)      — One question + a clarifying hint.
//	Level 2 (socratic)    — A question that nudges without answering.
//	Level 3 (discovery)   — Breadcrumbs only — let the user connect them.
func (s *PedagogicalService) socraticFor(language, topicTitle, context string, level int) SocraticPrompt {
	cleanCtx := strings.TrimSpace(context)
	var (
		question string
		hint     string
		breads   []string
	)
	switch level {
	case 0:
		question = fmt.Sprintf("To make progress on %s in %s, what's the smallest concrete next step?", topicTitle, language)
		hint = "Sketch one variable / function signature that captures the intent, then iterate."
		breads = []string{"name the shape", "implement the smallest version", "run it and read the output"}
	case 1:
		question = fmt.Sprintf("You mentioned %q — what part of %s is still unclear after that?", cleanCtx, topicTitle)
		hint = "Look for the concept you keep circling around; that's where the answer lives."
		breads = []string{"restate the goal in your own words", "name the smallest ambiguous term"}
	case 2:
		question = fmt.Sprintf("Imagine explaining %s in %s to a peer who's never seen the language. What's the FIRST thing they'd need?", topicTitle, language)
		hint = "Hold back the answer; the goal is to help the user find the named gap, not solve it for them."
		breads = []string{"the user's mental model", "the language's idiomatic shape"}
	default: // level 3 (discovery) and anything defensively off-range
		question = fmt.Sprintf("What does %s remind you of in %s — and where does the analogy fall short?", topicTitle, language)
		breads = []string{"a familiar pattern", "the part the pattern doesn't cover", "the smallest experiment that reveals the gap"}
	}
	return SocraticPrompt{
		Question:    question,
		Hint:        hint,
		Breadcrumbs: breads,
	}
}

// --- PeerReviewTopic -------------------------------------------------------

// PeerReviewTopic analyzes a user's peer-review comment against the
// topic without an LLM: deterministic feedback built from the
// comment's length, presence of structural keywords, and the configured
// level. PR 4 keeps it side-effect-free; richer analysis lands with the
// S5 cascade.
func (s *PedagogicalService) PeerReviewTopic(_ context.Context, in PeerReviewInput) (PeerReviewResult, error) {
	language := strings.TrimSpace(in.Language)
	topicID := strings.TrimSpace(in.TopicID)
	comment := strings.TrimSpace(in.Comment)
	if language == "" || topicID == "" {
		return PeerReviewResult{}, fmt.Errorf("%w: language and topic_id are required", ErrPedagogicalValidation)
	}
	if comment == "" {
		return PeerReviewResult{}, fmt.Errorf("%w: comment is required", ErrPedagogicalValidation)
	}
	if len(comment) > maxValidateSolutionBytes {
		return PeerReviewResult{}, fmt.Errorf("%w: comment exceeds %d bytes", ErrPedagogicalValidation, maxValidateSolutionBytes)
	}
	strengths := []string{}
	improvements := []string{}
	missed := []string{}

	low := strings.ToLower(comment)
	kwStrengths := []string{"clear", "good", "consistent", "named", "tests pass", "idomatic", "idiomatic"}
	for _, kw := range kwStrengths {
		if strings.Contains(low, kw) {
			strengths = append(strengths, fmt.Sprintf("Noted positive: %q.", kw))
		}
	}
	kwImprovements := []string{"should", "could", "missing", "lacks", "would be better", "tradeoff"}
	for _, kw := range kwImprovements {
		if strings.Contains(low, kw) {
			improvements = append(improvements, fmt.Sprintf("Action: address the %q you raised.", kw))
		}
	}
	if len(comment) < 80 {
		missed = append(missed, "Be more concrete — quote the line/function and propose the smallest alternative.")
	}
	if !strings.Contains(low, "why") {
		missed = append(missed, "Explain WHY the suggestion improves the code; reviewers value intent over prescription.")
	}
	if len(strengths) == 0 {
		strengths = []string{"You engaged with the topic — that's the entry point."}
	}
	if len(improvements) == 0 {
		improvements = []string{"Add a concrete code-level suggestion (one line is enough) so the author can act on it."}
	}
	if len(missed) == 0 {
		missed = []string{fmt.Sprintf("Reference the %s exercises in your note so the link back to the curriculum is obvious.", topicID)}
	}
	return PeerReviewResult{
		Strengths:    strengths,
		Improvements: improvements,
		MissedPoints: missed,
	}, nil
}
