// Package challenges implements the mcp-challenges tool group of the MCP
// monolith. It exposes five driving adapters — list_challenges,
// get_challenge, create_temp_challenge, generate_challenge,
// validate_challenge — that wrap the CodeAuditor challenge curriculum for
// the tutor LLM.
//
// Hexagonal contract: this package imports the local Service interface
// (and shared mcpkit/mcpctx helpers) plus the core domain model
// (CreateChallengeInput). It MUST NOT import database/sql, net/http, or
// any external service adapter. The wiring in cmd/api/main.go adapts the
// production *services.ChallengeService to the local Service so the tools
// stay swappable for tests.
package challenges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported here so tool-local code can use the short
// name; the dispatcher's errors.Is check matches the mcpkit canonical.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrNotImplemented is returned by Service.GenerateChallenge while the LLM
// cascade lands in S5. The tool surfaces it as -32603 so the LLM gets an
// actionable "enable via S5 cascade" hint instead of a stack trace.
var ErrNotImplemented = fmt.Errorf("generate_challenge: not implemented (S5 LLM cascade)")

// ErrChallengeNotFound mirrors services.ErrChallengeNotFound so the get_/
// list tools can branch with errors.Is without importing services. The
// production adapter wires the real sentinel via errors.Is comparison.
var ErrChallengeNotFound = fmt.Errorf("challenge not found")

// userIDKey is the local context key for the authenticated user_id.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key, then falls back to the canonical
// MCP bridge key (mcpctx.AuthUserIDKey).
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// propertyArg is the standardized JSON envelope produced for LLM tool
// routing. Keeping it local ensures consistent descriptions.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single object
// parameter list. The shape matches the dictionary/tutor/sandbox tools so
// the tutor LLM sees a consistent contract across groups.
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

// requiredProps materializes required-field names so all schema() calls
// keep the same convention.
func requiredProps(names ...string) []string { return names }

// validateChallengeInput is the subset of CreateChallengeInput the
// validate_challenge tool accepts as JSON. Mirroring the full struct
// keeps the wire shape identical across HTTP and MCP.
type validateChallengeInput = models.CreateChallengeInput

// challengeInputEnvelope is the structural envelope CreateChallengeInput
// travels through when the LLM tool calls it. Equivalent to the service's
// input struct, kept here so the tool side stays test-isolated.
type challengeInputEnvelope = models.CreateChallengeInput

// ValidateResult is the structured wire shape returned by validate_challenge
// (mirrors what an LSP-style validator would emit). The LLM can branch on
// `valid` and present the per-field error list to the user.
type ValidateResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// GenerateHint is the optional input the tutor can pass to generate_challenge
// to bias the produced challenge (language, difficulty, smell focus).
type GenerateHint struct {
	Language   string `json:"language,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
	Focus      string `json:"focus,omitempty"`
}

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP challenges tool depends on. The
// production wiring supplies *services.ChallengeService through a thin
// adapter (NewServiceAdapter below); tests substitute a fake.
type Service interface {
	// ListChallenges returns the user's visible challenges (seeds + owned)
	// as a JSON array. Each entry is the public wire shape — solution
	// fields (code, explanation) are NEVER included.
	ListChallenges(ctx context.Context, userID string) ([]json.RawMessage, error)
	// GetChallenge fetches a single challenge by id. When reveal is true,
	// solution_code and solution_explanation are included; otherwise the
	// public shape is returned. ErrChallengeNotFound when missing.
	GetChallenge(ctx context.Context, userID, id string, reveal bool) (json.RawMessage, error)
	// CreateTempChallenge inserts a free-practice challenge. It mirrors
	// services.ChallengeService.Create exactly so the dedup behavior is
	// shared with the HTTP /api/v1/challenges endpoint.
	CreateTempChallenge(ctx context.Context, userID string, input challengeInputEnvelope) (json.RawMessage, error)
	// GenerateChallenge returns a freshly-generated challenge. PR 2 ships
	// a stub returning ErrNotImplemented — full impl lands with the S5
	// LLM cascade (OpenRouter + Ollama). The wire shape is already
	// honored so swapping the implementation later is a no-op for callers.
	GenerateChallenge(ctx context.Context, userID string, hint GenerateHint) (json.RawMessage, error)
	// ValidateChallenge runs the v2 validator on the draft input and
	// returns a structured result. Never touches the database.
	ValidateChallenge(ctx context.Context, userID string, input validateChallengeInput) (ValidateResult, error)
}

// --- mcp-challenges.list_challenges ---------------------------------------

// listChallengesTool returns the user's visible challenges. Solution
// fields are intentionally excluded so the LLM never sees solutions
// before the user unlocks them.
type listChallengesTool struct {
	svc Service
}

// NewListChallengesTool registers the list_challenges action under the
// mcp-challenges group.
func NewListChallengesTool(svc Service) *listChallengesTool {
	return &listChallengesTool{svc: svc}
}

func (t *listChallengesTool) Name() string { return "mcp-challenges.list_challenges" }

func (t *listChallengesTool) Description() string {
	return "List every challenge visible to the authenticated user (curated " +
		"seeds + user-created). Solution fields are NEVER returned. Use to " +
		"refresh the dojo before a tutor interacts with one challenge. " +
		"Requires an authenticated user_id in context."
}

func (t *listChallengesTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{}, []string{})
}

func (t *listChallengesTool) Execute(ctx context.Context, _ json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	out, err := t.svc.ListChallenges(ctx, userID)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return []json.RawMessage{}, nil
	}
	return out, nil
}

// --- mcp-challenges.get_challenge -----------------------------------------

// getChallengeTool fetches a single challenge. By default the solution is
// omitted; pass `reveal=true` to include solution_code/solution_explanation
// (tutor path).
type getChallengeTool struct {
	svc Service
}

// NewGetChallengeTool registers the get_challenge action under the
// mcp-challenges group.
func NewGetChallengeTool(svc Service) *getChallengeTool {
	return &getChallengeTool{svc: svc}
}

func (t *getChallengeTool) Name() string { return "mcp-challenges.get_challenge" }

func (t *getChallengeTool) Description() string {
	return "Fetch a single challenge by UUID. By default the solution fields " +
		"are omitted; pass `reveal=true` to include them (tutor path). " +
		"Returns the JSON-RPC error 'challenge not found' for missing IDs. " +
		"Requires an authenticated user_id in context."
}

func (t *getChallengeTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"id":     {Type: "string", Description: "Challenge UUID (e.g. from list_challenges)."},
			"reveal": {Type: "boolean", Description: "When true, includes solution_code and solution_explanation in the response."},
		},
		requiredProps("id"),
	)
}

func (t *getChallengeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		ID     string `json:"id"`
		Reveal bool   `json:"reveal"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.ID == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidParams)
	}
	out, err := t.svc.GetChallenge(ctx, userID, a.ID, a.Reveal)
	if err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			return nil, fmt.Errorf("challenge not found")
		}
		return nil, err
	}
	return out, nil
}

// --- mcp-challenges.create_temp_challenge ---------------------------------

// createTempChallengeTool inserts a free-practice challenge. It is the
// MCP counterpart to POST /api/v1/challenges; the dedup-on-source_repo
// behavior is shared through the same ChallengeService.Create method.
type createTempChallengeTool struct {
	svc Service
}

// NewCreateTempChallengeTool registers the create_temp_challenge action
// under the mcp-challenges group.
func NewCreateTempChallengeTool(svc Service) *createTempChallengeTool {
	return &createTempChallengeTool{svc: svc}
}

func (t *createTempChallengeTool) Name() string { return "mcp-challenges.create_temp_challenge" }

func (t *createTempChallengeTool) Description() string {
	return "Create a temporary challenge in the user's library. Mirrors the " +
		"HTTP POST /api/v1/challenges contract (dedup on source_repo). Use " +
		"this when the tutor wants to persist a generated draft for the " +
		"user to iterate on. Requires an authenticated user_id in context."
}

func (t *createTempChallengeTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"title":       {Type: "string", Description: "Challenge title."},
			"description": {Type: "string", Description: "Challenge description (must avoid spoiler keywords — see validate_challenge)."},
			"difficulty":  {Type: "string", Description: "One of: junior, mid, senior, architect."},
			"category":    {Type: "string", Description: "Free-form category (e.g. 'n+1', 'callback hell')."},
			"language":    {Type: "string", Description: "Primary language (e.g. typescript, python, go)."},
			"code":        {Type: "string", Description: "Source code the auditor must analyze."},
			"codeSmell":   {Type: "string", Description: "Single smell keyword (deprecated v1 field)."},
			"repoUrl":     {Type: "string", Description: "Optional repo URL the challenge was sourced from."},
			"sourceRepo":  {Type: "string", Description: "Repo identifier used for dedup (e.g. 'github.com/foo/bar')."},
		},
		requiredProps("title", "description", "difficulty", "language"),
	)
}

func (t *createTempChallengeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var in challengeInputEnvelope
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if in.Title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalidParams)
	}
	if in.Description == "" {
		return nil, fmt.Errorf("%w: description is required", ErrInvalidParams)
	}
	if in.Difficulty == "" {
		return nil, fmt.Errorf("%w: difficulty is required", ErrInvalidParams)
	}
	if in.Language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	out, err := t.svc.CreateTempChallenge(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- mcp-challenges.generate_challenge -----------------------------------

// generateChallengeTool returns a freshly-authored challenge. PR 2 ships a
// stub: it returns ErrNotImplemented because the LLM cascade (OpenRouter +
// Ollama) lands in S5. Wiring is in place so swapping the impl later
// requires only changing the production adapter's GenerateChallenge method.
type generateChallengeTool struct {
	svc Service
}

// NewGenerateChallengeTool registers the generate_challenge action under
// the mcp-challenges group.
func NewGenerateChallengeTool(svc Service) *generateChallengeTool {
	return &generateChallengeTool{svc: svc}
}

func (t *generateChallengeTool) Name() string { return "mcp-challenges.generate_challenge" }

func (t *generateChallengeTool) Description() string {
	return "Generate a brand-new challenge on the fly via the LLM cascade. " +
		"Returns the same wire shape as create_temp_challenge so the tutor " +
		"can persist it without re-shaping. NOTE: full implementation lands " +
		"with the S5 LLM cascade — until then the tool returns an explicit " +
		"'not implemented' error. Requires an authenticated user_id in context."
}

func (t *generateChallengeTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language":   {Type: "string", Description: "Target language (e.g. typescript, python)."},
			"difficulty": {Type: "string", Description: "One of: junior, mid, senior, architect."},
			"focus":      {Type: "string", Description: "Optional code-smell focus (e.g. 'n+1', 'god function')."},
		},
		[]string{},
	)
}

func (t *generateChallengeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var hint GenerateHint
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &hint); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
	}
	out, err := t.svc.GenerateChallenge(ctx, userID, hint)
	if err != nil {
		// ErrNotImplemented propagates to JSON-RPC -32603 so the LLM
		// distinguishes "infrastructure bug" from "bad params".
		return nil, err
	}
	return out, nil
}

// --- mcp-challenges.validate_challenge -----------------------------------

// validateChallengeTool runs the v2 validator on a draft without persisting
// it. The LLM uses this BEFORE create_temp_challenge / generate_challenge
// to catch spoiler descriptions or wrong cardinalities.
type validateChallengeTool struct {
	svc Service
}

// NewValidateChallengeTool registers the validate_challenge action under
// the mcp-challenges group.
func NewValidateChallengeTool(svc Service) *validateChallengeTool {
	return &validateChallengeTool{svc: svc}
}

func (t *validateChallengeTool) Name() string { return "mcp-challenges.validate_challenge" }

func (t *validateChallengeTool) Description() string {
	return "Validate a draft challenge WITHOUT persisting it. Returns a " +
		"ValidateResult with the full list of constraint violations " +
		"(difficulty, expected_findings range, test_cases range, exactly 3 " +
		"hints, no spoiler keywords in description). Run before " +
		"create_temp_challenge to surface actionable feedback."
}

func (t *validateChallengeTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"title":            {Type: "string", Description: "Challenge title."},
			"description":      {Type: "string", Description: "Description body (checked for spoiler keywords)."},
			"difficulty":       {Type: "string", Description: "One of: junior, mid, senior, architect."},
			"language":         {Type: "string", Description: "Primary language."},
			"expectedFindings": {Type: "array", Description: "Array of expected findings (must contain 1..3 items)."},
			"testCases":        {Type: "array", Description: "Array of test cases (must contain 2..4 items)."},
			"hints":            {Type: "array", Description: "Array of hints (must contain exactly 3 items)."},
		},
		requiredProps("title", "description", "difficulty", "language"),
	)
}

func (t *validateChallengeTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var in validateChallengeInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if in.Title == "" || in.Description == "" || in.Difficulty == "" || in.Language == "" {
		return nil, fmt.Errorf("%w: title, description, difficulty, and language are required", ErrInvalidParams)
	}
	result, err := t.svc.ValidateChallenge(ctx, "", in)
	if err != nil {
		return nil, err
	}
	return result, nil
}
