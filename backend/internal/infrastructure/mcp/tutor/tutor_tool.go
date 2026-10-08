// Package tutor implements the mcp-tutor tool group of the MCP monolith.
// It currently exposes a single driving adapter — tutor_lookup — that
// bridges the user's question to the personal dictionary via
// *services.TutorLookupService, shaping a socratic-level-aware framing for
// the tutor LLM.
//
// Hexagonal contract: this package imports core/services for the dark logic
// and mcpctx for the shared ErrInvalidParams sentinel. It does NOT import
// database/sql, any HTTP SDK, or any external service adapter.
// cmd/api/main.go wires *services.TutorLookupService to the local Service
// interface via NewServiceAdapter; tests substitute the fake for isolation.
package tutor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported here as a type alias so the tools' tools.Is
// checks (in dispatcher.go) actually match the canonical value.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// userIDKey is the local context key for the authenticated user_id. The MCP
// server's auth middleware (mcp.Server.Router) stamps this into the request
// context before invoking any tool. Tools read it via userIDFromContext;
// they MUST refuse to act without an authenticated context.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID. Production
// callers use this through mcp.Server.Router; tests use it directly so the
// tool's user-isolation contract stays identical across both.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key (ContextWithUserID); if not set,
// it falls back to the canonical MCP bridge key (mcpctx.AuthUserIDKey,
// used by mcp.Server.Router when the request came in over HTTP). An empty
// string is returned when neither was stamped — callers translate that into
// ErrInvalidParams.
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
// routing. Keeping the property array local to the package ensures the
// `description` text shows up consistently in the function-calling prompt.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single object
// parameter list. The shape matches the mcp-dictionary tools so the
// tutor LLM sees a consistent contract across groups.
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

// Service is the local interface the MCP tutor tools depend on. The
// production wiring supplies *services.TutorLookupService through
// NewServiceAdapter (below); tests substitute a fake. Keeping the interface
// local lets the mcp-tutor package stay decoupled from
// ports.DictionaryRepository and other lower-level plumbing.
type Service interface {
	TutorLookup(ctx context.Context, in services.TutorLookupInput) (*services.TutorLookupResult, error)
}

// serviceAdapter bridges *services.TutorLookupService to the local Service
// interface. It is intentionally thin — the dark logic lives in the core
// service, the adapter just forwards input and shapes the JSON output
// envelope the LLM consumes.
type serviceAdapter struct {
	svc *services.TutorLookupService
}

// NewServiceAdapter returns the production wiring for the adapter. Lives in
// this package so the JSON envelope shape (and the Source/Found booleans
// the LLM branches on) stays consistent with the tool's Schema().
func NewServiceAdapter(svc *services.TutorLookupService) Service {
	return &serviceAdapter{svc: svc}
}

// TutorLookup forwards to the core service and projects the result into a
// JSON-safe envelope. Domain-level errors (ErrDictionaryValidation raised by
// the service on empty user_id, etc.) are surfaced as ErrInvalidParams so
// the LLM gets actionable feedback.
func (a *serviceAdapter) TutorLookup(ctx context.Context, in services.TutorLookupInput) (*services.TutorLookupResult, error) {
	out, err := a.svc.TutorLookup(ctx, in)
	if err != nil {
		if errors.Is(err, services.ErrDictionaryValidation) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		if errors.Is(err, services.ErrTutorLookupUnavailable) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return out, nil
}

// encodeLookup serializes a TutorLookupResult into the exact JSON envelope
// the tutor LLM consumes. The map-shaped envelope is deterministic so the
// functional-calling prompt is stable across runs.
func encodeLookup(r *services.TutorLookupResult) (json.RawMessage, error) {
	payload := map[string]any{
		"found":          r.Found,
		"framing":        r.Framing,
		"source":         r.Source,
		"socratic_level": r.SocraticLevel,
	}
	if r.Term != nil {
		payload["term"] = encodeLookupTerm(r.Term)
	}
	return json.Marshal(payload)
}

// encodeLookupTerm projects a Term into the wire shape the tutor LLM
// consumes. UserID is dropped (cross-user safe) and timestamps stay
// optional so empty rows don't emit zeroes.
func encodeLookupTerm(t *models.Term) map[string]any {
	out := map[string]any{
		"id":          t.ID,
		"term":        t.Term,
		"explanation": t.Explanation,
		"category":    t.Category,
		"synonyms":    safeSlice(t.Synonyms),
		"tags":        safeSlice(t.Tags),
	}
	if t.Example != nil {
		out["example"] = *t.Example
	}
	return out
}

func safeSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// --- mcp-tutor.tutor_lookup ----------------------------------------------

// tutorLookupTool is the MCP tool that bridges the user's question to the
// personal dictionary and shapes a socratic-level-aware framing for the
// tutor LLM. Spec scenario "Tutor Chat Auto-Lookup".
type tutorLookupTool struct {
	svc Service
}

// NewTutorLookupTool registers the tutor_lookup action under the mcp-tutor
// group.
func NewTutorLookupTool(svc Service) *tutorLookupTool { return &tutorLookupTool{svc: svc} }

// Name is the MCP identifier used by both Registry lookups ("what tool is
// this?") and JSON-RPC routing on /mcp/tools/call. The dot-group form is
// the project-wide convention; tools MUST NOT introduce other shapes.
func (t *tutorLookupTool) Name() string { return "mcp-tutor.tutor_lookup" }

// Description is shown to the tutor LLM in the function-calling prompt.
// Keeping the description honest about the socratic-level semantics and
// silent-fallback guarantee keeps the LLM from inventing its own framing.
func (t *tutorLookupTool) Description() string {
	return "Look up a term in the user's personal tech dictionary before " +
		"answering a definition question (\"Qué es X\"). Returns a socratic-" +
		"level-aware framing shaped by the user's configured level (0=direct, " +
		"1=guided, 2=socratic, 3=discovery). When no term matches, the framing " +
		"FALLS BACK to a 'general' source and MUST NOT expose the dictionary " +
		"absence to the user. Requires an authenticated user_id in context."
}

// Schema exposes the tool's parameter shape to the LLM. `query` is the raw
// "definition question" (e.g. "Que es PR"). `level` is the socratic level
// (0..3) — out-of-range values are clamped by the service.
func (t *tutorLookupTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"query": {Type: "string", Description: "Definition question the user asked, e.g. 'Qué es PR?'."},
			"level": {Type: "integer", Description: "Socratic level 0..3 (0=direct, 3=discovery). Out-of-range values are clamped."},
		},
		[]string{"query", "level"},
	)
}

// Execute parses the JSON-RPC arguments, extracts the authenticated
// user_id from ctx (NEVER trusting any user_id from the JSON body — the spec
// mandate is that the JWT-derived id owns each dictionary row), and returns
// the JSON envelope the tutor LLM consumes.
func (t *tutorLookupTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Query string `json:"query"`
		Level int    `json:"level"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.Query == "" {
		// Empty query is a structural invalid-params — the LLM would be
		// asking "look up nothing", which would silently fall through to a
		// general framing the user wouldn't understand. Surface the
		// rejection so the LLM can re-ask the user for the term.
		return nil, fmt.Errorf("%w: query is required", ErrInvalidParams)
	}
	result, err := t.svc.TutorLookup(ctx, services.TutorLookupInput{
		UserID: userID,
		Query:  a.Query,
		Level:  a.Level,
	})
	if err != nil {
		// Service-level ErrInvalidParams is already correctly framed;
		// everything else propagates so the dispatcher can map it to the
		// appropriate JSON-RPC code.
		if errors.Is(err, ErrInvalidParams) {
			return nil, err
		}
		return nil, err
	}
	raw, err := encodeLookup(result)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
