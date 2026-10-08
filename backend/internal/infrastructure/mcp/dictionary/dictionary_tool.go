// Package dictionary implements the mcp-dictionary tool group of the MCP
// monolith. It exposes four driving adapters — search_term, get_term,
// add_term, list_terms — that wrap the user's personal tech Dictionary
// service for the tutor LLM.
//
// Hexagonal contract: this package imports the local in-package Service
// interface AND ports/core helpers, NOT the concrete service in
// core/services. The wiring in cmd/api/main.go adapts *services.DictionaryService
// to the local interface so the tools stay swappable for tests.
package dictionary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported here as a type alias so tool-local code
// can keep using the short name; the underlying value lives in
// `mcpkit/mcpctx` so the dispatcher's `errors.Is` check
// (in `mcpkit/dispatcher.go`) actually matches.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// userIDKey is the context key for the authenticated user_id. The MCP
// server's auth middleware stores the verified user id here before
// invoking any tool. Tools read it via userIDFromContext; tools MUST refuse
// to act without an authenticated context.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID. Production
// callers (cmd/api MCP auth middleware) use this; tests use it too so the
// tool's user-isolation contract stays the same across both.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key (ContextWithUserID); if not set,
// it falls back to the canonical MCP bridge key (mcpctx.AuthUserIDKey,
// used by mcpkit.Server.Router when the request came in over HTTP). An
// empty string is returned when neither was stamped — callers translate
// that into ErrInvalidParams.
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// PropertyArg is the standardized JSON envelope produced for LLM tool
// routing. Keeping the property array local to the package ensures the
// `description` text shows up consistently in the function-calling prompt.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single object
// parameter list. The schema is a CONSTANT for our tools — keep the helper
// here so all four tools produce identical wire shape.
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

// requiredProps materializes required-field and property names. Keeping a
// helper prevents typo drift between schema calls.
func requiredProps(names ...string) []string { return names }

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP dictionary tool depends on. The
// production wiring supplies *services.DictionaryService through a small
// adapter that satisfies this interface (see main.go). Keeping the
// interface local to this package lets tests substitute a fake without
// importing core/services.
//
// JSON wire shape: Each method returns / consumes JSON-shaped values so
// tool result objects can be passed straight back to the LLM without
// re-marshaling by the caller. The service is the canonical REST shape.
// The adapter in main.go handles the translation to
// *models.Term / services.AddTermInput.
type Service interface {
	Add(ctx context.Context, in addTermInput) (json.RawMessage, error)
	Get(ctx context.Context, userID, id string) (json.RawMessage, error)
	List(ctx context.Context, userID, category, tag string) ([]json.RawMessage, error)
	Search(ctx context.Context, userID, query string) ([]json.RawMessage, error)
}

// addTermInput is the validated input shape consumed by Service.Add. It
// mirrors services.AddTermInput; the alias lets the public surface stay
// stable if the service struct ever changes shape internally.
type addTermInput = services.AddTermInput

// --- serviceAdapter: production *services.DictionaryService → Service ----

// serviceAdapter bridges *services.DictionaryService to the local Service
// interface. It owns the JSON marshaling / unmarshaling because the MCP
// tool chain speaks JSON end-to-end.
type serviceAdapter struct {
	svc *services.DictionaryService
}

// NewServiceAdapter returns the production wiring. The adapter lives in
// this package so the JSON shape stays consistent with the tools' schema.
func NewServiceAdapter(svc *services.DictionaryService) Service {
	return &serviceAdapter{svc: svc}
}

func (a *serviceAdapter) Add(ctx context.Context, in addTermInput) (json.RawMessage, error) {
	term, err := a.svc.AddTerm(ctx, in)
	if err != nil {
		if errors.Is(err, services.ErrDictionaryValidation) {
			// Validation errors are mapped to JSON-RPC invalid-params at the
			// tool layer (caller-side enrichment); surface raw detail for
			// the dispatcher to massage.
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	if term == nil {
		return json.RawMessage("null"), nil
	}
	raw, err := json.Marshal(mustEncodeTerm(term))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *serviceAdapter) Get(ctx context.Context, userID, id string) (json.RawMessage, error) {
	term, err := a.svc.GetTerm(ctx, userID, id)
	if err != nil {
		// `not found` is a normal null find for MCP, per spec scenario
		// "MCP get_term returns null for missing ID".
		return a.mapTermErr(err)
	}
	if term == nil {
		return json.RawMessage("null"), nil
	}
	raw, err := json.Marshal(mustEncodeTerm(term))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (a *serviceAdapter) List(ctx context.Context, userID, category, tag string) ([]json.RawMessage, error) {
	terms, err := a.svc.ListTerms(ctx, userID, category, tag)
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(terms))
	for _, t := range terms {
		raw, err := json.Marshal(mustEncodeTerm(t))
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func (a *serviceAdapter) Search(ctx context.Context, userID, query string) ([]json.RawMessage, error) {
	terms, err := a.svc.SearchTerms(ctx, userID, query)
	if err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(terms))
	for _, t := range terms {
		raw, err := json.Marshal(mustEncodeTerm(t))
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// mapTermErr translates dictionary service errors into the JSON-RPC tool
// envelope. Handles the "not found → null" semantic GetTerm uses.
func (a *serviceAdapter) mapTermErr(err error) (json.RawMessage, error) {
	// We can't import ports.ErrTermNotFound directly here without a cycle;
	// we use errors.As on the service's known sentinel via the service
	// package import. ports.ErrTermNotFound re-exported as
	// dictionaryErrNotFound below (see errors.go).
	if errors.Is(err, dictionaryErrNotFound) {
		return json.RawMessage("null"), nil
	}
	if errors.Is(err, services.ErrDictionaryValidation) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	return nil, err
}

// mustEncodeTerm projects a Term into the wire shape the LLM/tool consumes.
// The Term.UserID field is intentionally omitted so cross-user terms never
// leak through the tutor.
func mustEncodeTerm(t *models.Term) map[string]any {
	out := map[string]any{
		"id":          t.ID,
		"term":        t.Term,
		"explanation": t.Explanation,
		"category":    t.Category,
		"synonyms":    stringSliceSafe(t.Synonyms),
		"tags":        stringSliceSafe(t.Tags),
	}
	if t.Example != nil {
		out["example"] = *t.Example
	}
	if !t.CreatedAt.IsZero() {
		out["createdAt"] = t.CreatedAt
	}
	if !t.UpdatedAt.IsZero() {
		out["updatedAt"] = t.UpdatedAt
	}
	return out
}

func stringSliceSafe(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// --- mcp-dictionary.search_term -------------------------------------------

// searchTermTool runs a ranked full-text search over the user's textbook
// and returns up to MaxSearchResults hits. An empty query falls back to
// returning the entire personal glossary.
type searchTermTool struct {
	svc Service
}

// NewSearchTermTool registers the search_term action under the
// mcp-dictionary group.
func NewSearchTermTool(svc Service) *searchTermTool { return &searchTermTool{svc: svc} }

func (t *searchTermTool) Name() string { return "mcp-dictionary.search_term" }

func (t *searchTermTool) Description() string {
	return "Search the user's personal tech dictionary with full-text ranking (max 20). " +
		"Use when the tutor needs to look up an existing definition or surface " +
		"related terminology before answering."
}

func (t *searchTermTool) Schema() openai.Tool {
	return schema(
		"mcp-dictionary.search_term", t.Description(),
		map[string]propertyArg{
			"query": {Type: "string", Description: "Free-text search; matches term name, explanation, synonyms, and tags."},
		},
		requiredProps("query"),
	)
}

func (t *searchTermTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	results, err := t.svc.Search(ctx, userID, a.Query)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []json.RawMessage{}, nil
	}
	return results, nil
}

// --- mcp-dictionary.get_term ----------------------------------------------

// getTermTool retrieves a single dictionary entry by its UUID. Missing
// IDs are returned as JSON `null` per the spec (NOT an error), so callers
// can branch on a single line.
type getTermTool struct {
	svc Service
}

// NewGetTermTool registers the get_term action under the mcp-dictionary group.
func NewGetTermTool(svc Service) *getTermTool { return &getTermTool{svc: svc} }

func (t *getTermTool) Name() string { return "mcp-dictionary.get_term" }

func (t *getTermTool) Description() string {
	return "Fetch a single dictionary term by its UUID. Returns JSON null when " +
		"the ID is not found in the user's dictionary."
}

func (t *getTermTool) Schema() openai.Tool {
	return schema(
		"mcp-dictionary.get_term", t.Description(),
		map[string]propertyArg{
			"id": {Type: "string", Description: "Term UUID (e.g. from search_term)."},
		},
		requiredProps("id"),
	)
}

func (t *getTermTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.ID == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidParams)
	}
	term, err := t.svc.Get(ctx, userID, a.ID)
	if err != nil {
		// "Not found" maps to JSON null per spec scenario "MCP get_term
		// returns null for missing ID". Other errors propagate untouched
		// so the dispatcher can map to internal error.
		if errors.Is(err, dictionaryErrNotFound) {
			return json.RawMessage("null"), nil
		}
		return nil, err
	}
	if term == nil {
		return json.RawMessage("null"), nil
	}
	return term, nil
}

// --- mcp-dictionary.add_term ----------------------------------------------

// addTermTool creates a new dictionary entry. Validation failures
// (short explanation, invalid category) surface as ErrInvalidParams
// (-32602) so the LLM gets actionable feedback; the user-confirm gate
// lives in the tutor prompt, not in the tool.
type addTermTool struct {
	svc Service
}

// NewAddTermTool registers the add_term action under the mcp-dictionary group.
func NewAddTermTool(svc Service) *addTermTool { return &addTermTool{svc: svc} }

func (t *addTermTool) Name() string { return "mcp-dictionary.add_term" }

func (t *addTermTool) Description() string {
	return "Add a new term to the user's personal dictionary. The user MUST " +
		"explicitly confirm dictionary writes from chat — this tool is for " +
		"the tutor's auto-add flow once the user agrees."
}

func (t *addTermTool) Schema() openai.Tool {
	return schema(
		"mcp-dictionary.add_term", t.Description(),
		map[string]propertyArg{
			"term":        {Type: "string", Description: "Term name (max 50 chars, stored lowercase)."},
			"explanation": {Type: "string", Description: "Definition body (10..500 chars)."},
			"category":    {Type: "string", Description: "One of: Concepto, Patrón, Herramienta, Abreviatura, Anti-patrón, Otro."},
			"example":     {Type: "string", Description: "Optional code example (max 1000 chars)."},
			"synonyms":    {Type: "array", Description: "Optional alternate names for the term."},
			"tags":        {Type: "array", Description: "Optional tagging strings."},
		},
		requiredProps("term", "explanation", "category"),
	)
}

func (t *addTermTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var in addTermInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	in.UserID = userID

	// Tool-level shape gating keeps validation feedback uniform across MCP
	// and HTTP. The production service performs the same gating (defense
	// in depth); the tool pre-check ensures the LLM caller always sees
	// ErrInvalidParams (-32602) for spec scenario "MCP add_term validates
	// input" without depending on internal sentinel-detection.
	if len(in.Explanation) < 10 {
		return nil, fmt.Errorf("%w: explanation must be >= 10 chars", ErrInvalidParams)
	}
	if len(in.Term) == 0 || len(in.Term) > 50 {
		return nil, fmt.Errorf("%w: term must be 1..50 chars", ErrInvalidParams)
	}
	if in.Category == "" {
		return nil, fmt.Errorf("%w: category is required", ErrInvalidParams)
	}
	created, err := t.svc.Add(ctx, in)
	if err != nil {
		// Validation errors coming out of the service layer are also
		// surface as invalid params so the LLM gets consistent feedback.
		if errors.Is(err, services.ErrDictionaryValidation) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	if created == nil {
		return json.RawMessage("null"), nil
	}
	return created, nil
}

// --- mcp-dictionary.list_terms --------------------------------------------

// listTermTool returns the user's personal dictionary, optionally filtered
// by category and/or single tag.
type listTermTool struct {
	svc Service
}

// NewListTermsTool registers the list_terms action under the mcp-dictionary
// group.
func NewListTermsTool(svc Service) *listTermTool { return &listTermTool{svc: svc} }

func (t *listTermTool) Name() string { return "mcp-dictionary.list_terms" }

func (t *listTermTool) Description() string {
	return "List the user's dictionary terms with optional category / tag filters. " +
		"Returns an array (possibly empty)."
}

func (t *listTermTool) Schema() openai.Tool {
	return schema(
		"mcp-dictionary.list_terms", t.Description(),
		map[string]propertyArg{
			"category": {Type: "string", Description: "Optional category filter."},
			"tag":      {Type: "string", Description: "Optional tag filter (single tag, exact match)."},
		},
		[]string{}, // no required fields
	)
}

func (t *listTermTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Category string `json:"category"`
		Tag      string `json:"tag"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	results, err := t.svc.List(ctx, userID, a.Category, a.Tag)
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []json.RawMessage{}, nil
	}
	return results, nil
}

// --- Registry bridge -----------------------------------------------------

// RegistryRegistrar is the surface mcpkit.Registry exposes for adding tools.
// Defining a tiny interface keeps this package decoupled from the
// underlying registry while preserving compile-time safety.
type RegistryRegistrar interface {
	Register(tool MCPToolRegistrar)
}

// MCPToolRegistrar matches the name / description / schema / execute
// surface of mcpkit.MCPTool without importing the upstream package. The
// production adapter in main.go implements this contract by passing the
// real mcpkit.MCPTool value through.
type MCPToolRegistrar interface {
	Name() string
	Description() string
	Schema() openai.Tool
	Execute(ctx context.Context, args json.RawMessage) (any, error)
}
