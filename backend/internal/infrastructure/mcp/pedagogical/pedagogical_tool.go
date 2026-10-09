// Package pedagogical implements the mcp-pedagogical tool group of the
// MCP monolith. Per AGENTS.md this group is "the centerpiece of the
// tutor chat flow"; its six tools wrap the PedagogicalService in
// core/services for the tutor LLM.
//
// Tools exposed:
//
//	mcp-pedagogical.get_learning_objective
//	mcp-pedagogical.generate_story_context
//	mcp-pedagogical.create_scaffolded_steps
//	mcp-pedagogical.validate_solution
//	mcp-pedagogical.get_socratic_prompt        (prompt-injection guard)
//	mcp-pedagogical.peer_review_topic
//
// Hexagonal contract: this package imports ONLY the local Service
// interface (and shared mcpkit/mcpctx helpers) plus core/services for
// the domain shapes. It MUST NOT import database/sql, net/http, or any
// external service adapter. The wiring in cmd/api/main.go adapts
// *services.PedagogicalService to the local Service so the tools stay
// swappable for tests.
//
// Prompt-injection guard: get_socratic_prompt runs a defense-in-depth
// validation at the tool boundary (in addition to the matching guard
// inside PedagogicalService). The patterns mirror the service-layer
// list so a future LLM call inside the service stays safe even when a
// caller bypasses the MCP tool.
package pedagogical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported as a type alias so tool-local code can
// use the short name; the underlying value lives in mcpkit/mcpctx so
// the dispatcher's errors.Is check matches across the boundary.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrPedagogicalValidation mirrors services.ErrPedagogicalValidation
// so the tool layer can branch on it without re-importing the
// services package for the sentinel. The production adapter maps the
// service-level sentinel to this alias via errors.Is (see
// serviceAdapter below) so the six tools uniformly surface
// pedagogical-validation failures as ErrInvalidParams at the
// dispatcher.
var ErrPedagogicalValidation = services.ErrPedagogicalValidation

// ErrCurriculumInvalidLanguage is re-exported for the tool layer's
// convenience — when a peer tool's path-traversal guard fires (e.g.
// language='../etc'), the errors.Is branch maps to JSON-RPC -32602.
var ErrCurriculumInvalidLanguage = services.ErrCurriculumInvalidLanguage

// userIDKey is the local context key for the authenticated user_id.
// The MCP server's auth middleware (mcpkit.Server.Router) stamps this
// into the request context before invoking any tool; tools read it
// via userIDFromContext and MUST refuse to act without an
// authenticated context.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID.
// Production callers use this through mcpkit.Server.Router; tests use
// it directly so the user-isolation contract stays the same.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx.
// First consults this package's own key (ContextWithUserID), then
// falls back to the canonical MCP bridge key. Returns "" when neither
// was stamped — callers translate that to ErrInvalidParams.
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// propertyArg is the standardized JSON envelope produced for LLM
// tool routing. Keeping the struct local ensures descriptions show up
// consistently in the function-calling prompt.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single
// object parameter list. The shape matches the dictionary/tutor/
// sandbox/curriculum/user tools so the tutor LLM sees a consistent
// contract across groups.
func schema(name, description string, props map[string]propertyArg, required []string) openai.Tool {
	return openai.Tool{
		Type: "function",
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
}

func requiredProps(names ...string) []string { return names }

// --- Tool-layer prompt-injection guard -------------------------------------

// maxToolContextBytes caps user-controlled input at the tool layer so
// the dispatcher never hands a 2MB string to the service. Mirrors the
// service-layer limit so the two stay coherent.
const maxToolContextBytes = 2048

// toolInjectionPatterns is the tool-layer copy of the
// PedagogicalService's prompt-injection guard list. Keeping the two
// lists identical by hand is intentional — defense in depth is the
// point, not deduplication.
var toolInjectionPatterns = []string{
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

// toolGuardSocratic runs the adversarial-input guard at the MCP
// boundary. It mirrors the service-layer guard so:
//   - the dispatcher surfaces JSON-RPC -32602 immediately,
//   - the LLM is never invoked with a poisoned context,
//   - the service-layer guard is a backstop for non-MCP callers.
//
// The contract is purely additive: any input that the service would
// reject, the tool layer rejects first with the same ErrInvalidParams
// surface. Defense-in-depth never weakens the contract.
func toolGuardSocratic(level int, language, topicID, context string) error {
	if level < 0 || level > 3 {
		return fmt.Errorf("%w: level must be in [0..3], got %d", ErrInvalidParams, level)
	}
	if strings.TrimSpace(language) == "" {
		return fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if strings.TrimSpace(topicID) == "" {
		return fmt.Errorf("%w: topic_id is required", ErrInvalidParams)
	}
	if len(context) > maxToolContextBytes {
		return fmt.Errorf("%w: context exceeds %d bytes", ErrInvalidParams, maxToolContextBytes)
	}
	lowered := strings.ToLower(context)
	for _, p := range toolInjectionPatterns {
		if strings.Contains(lowered, p) {
			return fmt.Errorf("%w: context contains forbidden pattern", ErrInvalidParams)
		}
	}
	return nil
}

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP pedagogical tool depends
// on. The production wiring supplies *services.PedagogicalService
// through NewServiceAdapter (below); tests substitute a fake to
// verify each tool's behavior end-to-end.
type Service interface {
	GetLearningObjective(ctx context.Context, in services.LearningObjectiveInput) (services.LearningObjective, error)
	GenerateStoryContext(ctx context.Context, in services.StoryContextInput) (services.StoryContext, error)
	CreateScaffoldedSteps(ctx context.Context, in services.ScaffoldedStepsInput) ([]services.ScaffoldingStep, error)
	ValidateSolution(ctx context.Context, in services.ValidateSolutionInput) (services.ValidationResult, error)
	GetSocraticPrompt(ctx context.Context, in services.SocraticPromptInput) (services.SocraticPrompt, error)
	PeerReviewTopic(ctx context.Context, in services.PeerReviewInput) (services.PeerReviewResult, error)
}

// --- serviceAdapter: production *services.PedagogicalService → Service ----

// serviceAdapter bridges *services.PedagogicalService to the local
// Service interface. It owns the translation from the service's
// ErrPedagogicalValidation / ErrCurriculumInvalidLanguage sentinels to
// the MCP tool's ErrInvalidParams envelope so the LLM caller sees
// actionable feedback.
type serviceAdapter struct {
	svc *services.PedagogicalService
}

// NewServiceAdapter returns the production wiring. The adapter lives
// in this package so the JSON envelope + sentinel mapping stay
// consistent with the tools' schemas.
func NewServiceAdapter(svc *services.PedagogicalService) Service {
	return &serviceAdapter{svc: svc}
}

func (a *serviceAdapter) GetLearningObjective(ctx context.Context, in services.LearningObjectiveInput) (services.LearningObjective, error) {
	out, err := a.svc.GetLearningObjective(ctx, in)
	if err != nil {
		return services.LearningObjective{}, mapSocraticErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) GenerateStoryContext(ctx context.Context, in services.StoryContextInput) (services.StoryContext, error) {
	out, err := a.svc.GenerateStoryContext(ctx, in)
	if err != nil {
		return services.StoryContext{}, mapSocraticErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) CreateScaffoldedSteps(ctx context.Context, in services.ScaffoldedStepsInput) ([]services.ScaffoldingStep, error) {
	out, err := a.svc.CreateScaffoldedSteps(ctx, in)
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) ValidateSolution(ctx context.Context, in services.ValidateSolutionInput) (services.ValidationResult, error) {
	out, err := a.svc.ValidateSolution(ctx, in)
	if err != nil {
		return services.ValidationResult{}, mapSocraticErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) GetSocraticPrompt(ctx context.Context, in services.SocraticPromptInput) (services.SocraticPrompt, error) {
	out, err := a.svc.GetSocraticPrompt(ctx, in)
	if err != nil {
		return services.SocraticPrompt{}, mapSocraticErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) PeerReviewTopic(ctx context.Context, in services.PeerReviewInput) (services.PeerReviewResult, error) {
	out, err := a.svc.PeerReviewTopic(ctx, in)
	if err != nil {
		return services.PeerReviewResult{}, mapSocraticErr(err)
	}
	return out, nil
}

// mapSocraticErr translates service-layer errors into the MCP tool's
// ErrInvalidParams envelope. ErrPedagogicalValidation + ErrCurriculumInvalidLanguage
// both map; other errors propagate untouched so the dispatcher can
// distinguish validation from infrastructure errors.
func mapSocraticErr(err error) error {
	if errors.Is(err, ErrPedagogicalValidation) {
		return fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if errors.Is(err, ErrCurriculumInvalidLanguage) {
		return fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	return err
}

// --- mcp-pedagogical.get_learning_objective -------------------------------

type getLearningObjectiveTool struct {
	svc Service
}

// NewGetLearningObjectiveTool registers the get_learning_objective action.
func NewGetLearningObjectiveTool(svc Service) *getLearningObjectiveTool {
	return &getLearningObjectiveTool{svc: svc}
}

func (t *getLearningObjectiveTool) Name() string {
	return "mcp-pedagogical.get_learning_objective"
}

func (t *getLearningObjectiveTool) Description() string {
	return "Compose a learning objective for a (language, topic) pair. " +
		"Pulls the topic's description and the first two exercises from " +
		"the curriculum service; falls back to a generic 'Understand <topic>' " +
		"objective when the topic is unknown. Returns the objective + a list " +
		"of measurable outcomes."
}

func (t *getLearningObjectiveTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key (e.g. 'typescript')."},
			"topic_id": {Type: "string", Description: "Topic slug (e.g. 'variables')."},
			"level":    {Type: "string", Description: "Optional level hint (F..S); currently informational only."},
		},
		requiredProps("language", "topic_id"),
	)
}

func (t *getLearningObjectiveTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
		Level    string `json:"level"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	language := strings.TrimSpace(a.Language)
	topicID := strings.TrimSpace(a.TopicID)
	if language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if topicID == "" {
		return nil, fmt.Errorf("%w: topic_id is required", ErrInvalidParams)
	}
	out, err := t.svc.GetLearningObjective(ctx, services.LearningObjectiveInput{
		Language: language,
		TopicID:  topicID,
		Level:    a.Level,
	})
	if err != nil {
		// Defense in depth: the production adapter wraps the
		// pedagogical-validation / curriculum-language sentinels to
		// ErrInvalidParams at the dispatcher envelope; we replicate
		// the same mapping here so the tool layer is correct even
		// when the production adapter is bypassed in a test or a
		// future refactor swaps the wiring.
		return nil, mapSocraticErr(err)
	}
	return out, nil
}

// --- mcp-pedagogical.generate_story_context --------------------------------

type generateStoryContextTool struct {
	svc Service
}

// NewGenerateStoryContextTool registers the generate_story_context action.
func NewGenerateStoryContextTool(svc Service) *generateStoryContextTool {
	return &generateStoryContextTool{svc: svc}
}

func (t *generateStoryContextTool) Name() string {
	return "mcp-pedagogical.generate_story_context"
}

func (t *generateStoryContextTool) Description() string {
	return "Weave a relatable scenario around a topic so the tutor can frame " +
		"the lesson as a story. Returns { story, scenario } shaped for the " +
		"LLM to flavor (the deterministic PR 4 template gives the LLM a " +
		"structured hook rather than an invented narrative)."
}

func (t *generateStoryContextTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language":   {Type: "string", Description: "Lowercase language key."},
			"topic_id":   {Type: "string", Description: "Topic slug."},
			"scenario":   {Type: "string", Description: "Optional custom scenario prefix."},
			"user_level": {Type: "string", Description: "Optional F..S level hint."},
		},
		requiredProps("language", "topic_id"),
	)
}

func (t *generateStoryContextTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language  string `json:"language"`
		TopicID   string `json:"topic_id"`
		Scenario  string `json:"scenario"`
		UserLevel string `json:"user_level"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	out, err := t.svc.GenerateStoryContext(ctx, services.StoryContextInput{
		Language:  strings.TrimSpace(a.Language),
		TopicID:   strings.TrimSpace(a.TopicID),
		Scenario:  a.Scenario,
		UserLevel: a.UserLevel,
	})
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	return out, nil
}

// --- mcp-pedagogical.create_scaffolded_steps -------------------------------

type createScaffoldedStepsTool struct {
	svc Service
}

// NewCreateScaffoldedStepsTool registers the create_scaffolded_steps action.
func NewCreateScaffoldedStepsTool(svc Service) *createScaffoldedStepsTool {
	return &createScaffoldedStepsTool{svc: svc}
}

func (t *createScaffoldedStepsTool) Name() string {
	return "mcp-pedagogical.create_scaffolded_steps"
}

func (t *createScaffoldedStepsTool) Description() string {
	return "Emit progressively revealing hints toward a learning objective. " +
		"Defaults to 3 steps; clamped to [1..6] so an LLM caller cannot " +
		"manufacture an unbounded number of hints via the `levels` input."
}

func (t *createScaffoldedStepsTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language":  {Type: "string", Description: "Lowercase language key."},
			"topic_id":  {Type: "string", Description: "Topic slug."},
			"objective": {Type: "string", Description: "Optional explicit objective; defaults to 'understand <topic>'."},
			"levels":    {Type: "integer", Description: "Number of steps (1..6). Defaults to 3 when unset or <=0."},
		},
		requiredProps("language", "topic_id"),
	)
}

func (t *createScaffoldedStepsTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language  string `json:"language"`
		TopicID   string `json:"topic_id"`
		Objective string `json:"objective"`
		Levels    int    `json:"levels"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	steps, err := t.svc.CreateScaffoldedSteps(ctx, services.ScaffoldedStepsInput{
		Language:  strings.TrimSpace(a.Language),
		TopicID:   strings.TrimSpace(a.TopicID),
		Objective: a.Objective,
		Levels:    a.Levels,
	})
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	if steps == nil {
		return []services.ScaffoldingStep{}, nil
	}
	return steps, nil
}

// --- mcp-pedagogical.validate_solution -------------------------------------

type validateSolutionTool struct {
	svc Service
}

// NewValidateSolutionTool registers the validate_solution action.
func NewValidateSolutionTool(svc Service) *validateSolutionTool {
	return &validateSolutionTool{svc: svc}
}

func (t *validateSolutionTool) Name() string { return "mcp-pedagogical.validate_solution" }

func (t *validateSolutionTool) Description() string {
	return "Run the static checks the learning tool can perform WITHOUT booting " +
		"the sandbox: code is non-empty, within size limits, and references the " +
		"topic in a header comment for traceability. Returns { passed, feedback[], " +
		"score }. PR 4 ships deterministic checks; sandbox execution lands with " +
		"the S3 free-practice spec."
}

func (t *validateSolutionTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"topic_id": {Type: "string", Description: "Topic slug."},
			"code":     {Type: "string", Description: "User's solution code (max 50KB)."},
		},
		requiredProps("language", "topic_id", "code"),
	)
}

func (t *validateSolutionTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	out, err := t.svc.ValidateSolution(ctx, services.ValidateSolutionInput{
		Language: strings.TrimSpace(a.Language),
		TopicID:  strings.TrimSpace(a.TopicID),
		Code:     a.Code,
	})
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	return out, nil
}

// --- mcp-pedagogical.get_socratic_prompt -----------------------------------

type getSocraticPromptTool struct {
	svc Service
}

// NewGetSocraticPromptTool registers the get_socratic_prompt action.
func NewGetSocraticPromptTool(svc Service) *getSocraticPromptTool {
	return &getSocraticPromptTool{svc: svc}
}

func (t *getSocraticPromptTool) Name() string { return "mcp-pedagogical.get_socratic_prompt" }

func (t *getSocraticPromptTool) Description() string {
	return "Produce a socratic-level-shaped guiding question for a topic. " +
		"Level 0=direct, 1=guided, 2=socratic, 3=discovery. The tutor NEVER " +
		"gets the answer — only a question (+ hint at levels 0..2) and " +
		"breadcrumbs at higher levels. Adversarial inputs (level out of " +
		"[0..3], 'ignore previous instructions', role tags, oversize " +
		"context) are rejected at the tool boundary AND inside the service " +
		"(defense in depth). Returns -32602 before reaching the LLM prompt."
}

func (t *getSocraticPromptTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"topic_id": {Type: "string", Description: "Topic slug."},
			"level":    {Type: "integer", Description: "Socratic level (0..3). Out-of-range is rejected."},
			"context":  {Type: "string", Description: "Optional user utterance (max 2KB). Rejected when it contains adversarial patterns."},
		},
		requiredProps("language", "topic_id", "level"),
	)
}

func (t *getSocraticPromptTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
		Level    int    `json:"level"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	// Defense-in-depth prompt-injection guard runs BEFORE the service
	// is invoked. The service layer mirrors the same patterns as a
	// backstop, but the tool-layer guard short-circuits at the
	// dispatcher boundary so the LLM never sees the adversarial text.
	if err := toolGuardSocratic(a.Level, a.Language, a.TopicID, a.Context); err != nil {
		return nil, err
	}
	out, err := t.svc.GetSocraticPrompt(ctx, services.SocraticPromptInput{
		Language: a.Language,
		TopicID:  a.TopicID,
		Level:    a.Level,
		Context:  a.Context,
	})
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	return out, nil
}

// --- mcp-pedagogical.peer_review_topic -------------------------------------

type peerReviewTopicTool struct {
	svc Service
}

// NewPeerReviewTopicTool registers the peer_review_topic action.
func NewPeerReviewTopicTool(svc Service) *peerReviewTopicTool {
	return &peerReviewTopicTool{svc: svc}
}

func (t *peerReviewTopicTool) Name() string { return "mcp-pedagogical.peer_review_topic" }

func (t *peerReviewTopicTool) Description() string {
	return "Analyze a user's peer-review comment against a topic. Returns " +
		"structured strengths / improvements / missed_points so the LLM can " +
		"branch on actionable feedback. PR 4 ships deterministic analysis " +
		"based on the comment's keywords; richer analysis lands with the " +
		"S5 cascade."
}

func (t *peerReviewTopicTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"topic_id": {Type: "string", Description: "Topic slug."},
			"comment":  {Type: "string", Description: "User's review comment (max 50KB)."},
		},
		requiredProps("language", "topic_id", "comment"),
	)
}

func (t *peerReviewTopicTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
		Comment  string `json:"comment"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	out, err := t.svc.PeerReviewTopic(ctx, services.PeerReviewInput{
		Language: strings.TrimSpace(a.Language),
		TopicID:  strings.TrimSpace(a.TopicID),
		Comment:  a.Comment,
	})
	if err != nil {
		return nil, mapSocraticErr(err)
	}
	return out, nil
}
