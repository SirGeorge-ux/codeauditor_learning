// Package external_docs implements the mcp-external_docs tool group of
// the MCP monolith. Per AGENTS.md this group wraps the Context7 +
// Brave Search integrations behind three tools for the tutor LLM:
//
//	mcp-external_docs.get_library_docs       (Context7 docs lookup)
//	mcp-external_docs.search_documentation   (Brave Search)
//	mcp-external_docs.get_real_example       (Context7-first, Brave fallback)
//
// Hexagonal contract: this package imports ONLY the local Service
// interface (and shared mcpkit/mcpctx helpers) plus core/services for
// the domain shapes. It MUST NOT import database/sql, net/http, or any
// external service adapter. The wiring in cmd/api/main.go adapts
// *services.ExternalDocsService to the local Service so the tools
// stay swappable for tests.
package external_docs

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

// ErrCurriculumInvalidLanguage mirrors services.ErrCurriculumInvalidLanguage
// so the tool layer can branch on it without re-importing services
// for the sentinel. The production adapter maps the
// services-level sentinel to this value via errors.Is before
// returning ErrInvalidParams to the dispatcher.
var ErrCurriculumInvalidLanguage = services.ErrCurriculumInvalidLanguage

// userIDKey is the local context key for the authenticated user_id.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx.
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
// routing.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single
// object parameter list.
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

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP external_docs tool depends
// on. The production wiring supplies *services.ExternalDocsService
// through NewServiceAdapter (below); tests substitute a fake.
type Service interface {
	GetLibraryDocs(ctx context.Context, in services.LibraryDocsInput) (services.LibraryDocs, error)
	SearchDocumentation(ctx context.Context, in services.SearchDocsInput) ([]services.SearchDocsResult, error)
	GetRealExample(ctx context.Context, in services.RealExampleInput) (services.RealExample, error)
}

// --- serviceAdapter: production *services.ExternalDocsService → Service ----

// serviceAdapter bridges *services.ExternalDocsService to the local
// Service interface. It owns the translation from the service's
// ErrCurriculumInvalidLanguage sentinel (used by the service to
// reject empty library) to the MCP tool's ErrInvalidParams envelope.
type serviceAdapter struct {
	svc *services.ExternalDocsService
}

// NewServiceAdapter returns the production wiring.
func NewServiceAdapter(svc *services.ExternalDocsService) Service {
	return &serviceAdapter{svc: svc}
}

func (a *serviceAdapter) GetLibraryDocs(ctx context.Context, in services.LibraryDocsInput) (services.LibraryDocs, error) {
	out, err := a.svc.GetLibraryDocs(ctx, in)
	if err != nil {
		return services.LibraryDocs{}, mapExtDocsErr(err)
	}
	return out, nil
}

func (a *serviceAdapter) SearchDocumentation(ctx context.Context, in services.SearchDocsInput) ([]services.SearchDocsResult, error) {
	hits, err := a.svc.SearchDocumentation(ctx, in)
	if err != nil {
		return nil, mapExtDocsErr(err)
	}
	return hits, nil
}

func (a *serviceAdapter) GetRealExample(ctx context.Context, in services.RealExampleInput) (services.RealExample, error) {
	out, err := a.svc.GetRealExample(ctx, in)
	if err != nil {
		return services.RealExample{}, mapExtDocsErr(err)
	}
	return out, nil
}

// mapExtDocsErr translates service-layer errors into the MCP tool's
// ErrInvalidParams envelope. ErrCurriculumInvalidLanguage (which the
// service uses for empty library) is mapped; other errors propagate
// untouched so the dispatcher can distinguish validation from
// upstream / configuration errors.
func mapExtDocsErr(err error) error {
	if errors.Is(err, ErrCurriculumInvalidLanguage) {
		return fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	return err
}

// --- mcp-external_docs.get_library_docs -----------------------------------

type getLibraryDocsTool struct {
	svc Service
}

// NewGetLibraryDocsTool registers the get_library_docs action.
func NewGetLibraryDocsTool(svc Service) *getLibraryDocsTool {
	return &getLibraryDocsTool{svc: svc}
}

func (t *getLibraryDocsTool) Name() string { return "mcp-external_docs.get_library_docs" }

func (t *getLibraryDocsTool) Description() string {
	return "Fetch a documentation excerpt for a library and (optional) topic. " +
		"Backed by Context7 in production. The `library` arg is required " +
		"(no path separators or traversal). The API key is server-side; " +
		"this tool never returns it."
}

func (t *getLibraryDocsTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"library": {Type: "string", Description: "Library name (e.g. 'react', 'typescript')."},
			"topic":   {Type: "string", Description: "Optional topic within the library (e.g. 'hooks')."},
		},
		requiredProps("library"),
	)
}

func (t *getLibraryDocsTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Library string `json:"library"`
		Topic   string `json:"topic"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	library := strings.TrimSpace(a.Library)
	if library == "" {
		return nil, fmt.Errorf("%w: library is required", ErrInvalidParams)
	}
	out, err := t.svc.GetLibraryDocs(ctx, services.LibraryDocsInput{
		Library: library,
		Topic:   strings.TrimSpace(a.Topic),
	})
	if err != nil {
		return nil, mapExtDocsErr(err)
	}
	return out, nil
}

// --- mcp-external_docs.search_documentation -------------------------------

type searchDocumentationTool struct {
	svc Service
}

// NewSearchDocumentationTool registers the search_documentation action.
func NewSearchDocumentationTool(svc Service) *searchDocumentationTool {
	return &searchDocumentationTool{svc: svc}
}

func (t *searchDocumentationTool) Name() string {
	return "mcp-external_docs.search_documentation"
}

func (t *searchDocumentationTool) Description() string {
	return "Search the web for documentation snippets via Brave Search. " +
		"Returns up to 20 hits (title, url, snippet). An empty query " +
		"returns an empty list (no upstream call)."
}

func (t *searchDocumentationTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"query": {Type: "string", Description: "Free-text search; passed verbatim to Brave Search."},
		},
		requiredProps("query"),
	)
}

func (t *searchDocumentationTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	hits, err := t.svc.SearchDocumentation(ctx, services.SearchDocsInput{Query: a.Query})
	if err != nil {
		return nil, mapExtDocsErr(err)
	}
	if hits == nil {
		return []services.SearchDocsResult{}, nil
	}
	return hits, nil
}

// --- mcp-external_docs.get_real_example -----------------------------------

type getRealExampleTool struct {
	svc Service
}

// NewGetRealExampleTool registers the get_real_example action.
func NewGetRealExampleTool(svc Service) *getRealExampleTool {
	return &getRealExampleTool{svc: svc}
}

func (t *getRealExampleTool) Name() string { return "mcp-external_docs.get_real_example" }

func (t *getRealExampleTool) Description() string {
	return "Return a real-world code example for a library + topic. " +
		"Fuses Context7 (first fenced code block) with Brave Search " +
		"(top hit snippet) when Context7 returns empty. The Source " +
		"field identifies which provider the payload came from."
}

func (t *getRealExampleTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"library":  {Type: "string", Description: "Library name (e.g. 'react')."},
			"topic":    {Type: "string", Description: "Optional topic within the library (e.g. 'useState')."},
			"language": {Type: "string", Description: "Optional programming language hint."},
		},
		requiredProps("library"),
	)
}

func (t *getRealExampleTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Library  string `json:"library"`
		Topic    string `json:"topic"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	library := strings.TrimSpace(a.Library)
	if library == "" {
		return nil, fmt.Errorf("%w: library is required", ErrInvalidParams)
	}
	out, err := t.svc.GetRealExample(ctx, services.RealExampleInput{
		Library:  library,
		Topic:    strings.TrimSpace(a.Topic),
		Language: strings.TrimSpace(a.Language),
	})
	if err != nil {
		return nil, mapExtDocsErr(err)
	}
	return out, nil
}
