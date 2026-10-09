// Package curriculum implements the mcp-curriculum tool group of the MCP
// monolith. It exposes four driving adapters — get_topics, get_topic,
// suggest_topic, review_topic — that wrap the CodeAuditor curriculum for
// the tutor LLM.
//
// Hexagonal contract: this package imports ONLY the local Service interface
// (and shared mcpkit/mcpctx helpers) plus core/services for the domain
// shapes. It MUST NOT import database/sql, net/http, or any external
// service adapter. The wiring in cmd/api/main.go adapts
// *services.CurriculumService to the local Service so the tools stay
// swappable for tests.
package curriculum

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported as a type alias so tool-local code can use
// the short name; the underlying value lives in mcpkit/mcpctx so the
// dispatcher's errors.Is check matches across the boundary.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrCurriculumInvalidLanguage mirrors services.ErrCurriculumInvalidLanguage
// so the tool can branch on it without importing services for sentinel
// detection. The production adapter maps the services-level sentinel to
// this value via errors.Is (see serviceAdapter.GetTopics below) before
// returning ErrInvalidParams to the dispatcher.
var ErrCurriculumInvalidLanguage = services.ErrCurriculumInvalidLanguage

// userIDKey is the local context key for the authenticated user_id. The MCP
// server's auth middleware (mcpkit.Server.Router) stamps this into the
// request context before invoking any tool; tools read it via
// userIDFromContext and MUST refuse to act without an authenticated
// context.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID. Production
// callers use this through mcpkit.Server.Router; tests use it directly.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key, then falls back to the canonical
// MCP bridge key. Returns "" when neither was stamped.
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
// routing. Keeping the property struct local ensures descriptions show up
// consistently in the function-calling prompt.
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

func requiredProps(names ...string) []string { return names }

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP curriculum tool depends on. The
// production wiring supplies *services.CurriculumService through
// NewServiceAdapter (below); tests substitute a fake to verify each tool's
// behavior end-to-end without booting the real filesystem reader.
//
// JSON wire shape: each method returns the canonical *services.Topic /
// TopicDetail / ReviewTopicResult the curriculum service produces. The
// adapter marshals each into a JSON envelope the LLM consumes.
type Service interface {
	// GetTopics lists topic headlines for the given language.
	// ErrCurriculumInvalidLanguage on path-traversal; underlying fs
	// errors propagate so the dispatcher maps them to -32603.
	GetTopics(ctx context.Context, input services.GetTopicsInput) ([]services.Topic, error)
	// GetTopic returns the full topic body. Same error semantics.
	GetTopic(ctx context.Context, language, topicID string) (services.TopicDetail, error)
	// SuggestTopic proposes the next topic to study.
	SuggestTopic(ctx context.Context, input services.SuggestTopicInput) (services.Topic, error)
	// ReviewTopic records a self-assessment. PR 3 acknowledges the review
	// without persisting it (S3 owns the storage layer).
	ReviewTopic(ctx context.Context, input services.ReviewTopicInput) (services.ReviewTopicResult, error)
}

// --- serviceAdapter: production *services.CurriculumService → Service ----

// serviceAdapter bridges *services.CurriculumService to the local Service
// interface. The adapter owns the translation from the service's
// path-traversal sentinel to the MCP tool's invalid-params envelope so
// the LLM caller sees actionable feedback.
type serviceAdapter struct {
	svc *services.CurriculumService
}

// NewServiceAdapter returns the production wiring. The adapter lives in
// this package so the JSON envelope + error mapping stay consistent with
// the tools' schemas.
func NewServiceAdapter(svc *services.CurriculumService) Service {
	return &serviceAdapter{svc: svc}
}

func (a *serviceAdapter) GetTopics(ctx context.Context, in services.GetTopicsInput) ([]services.Topic, error) {
	topics, err := a.svc.GetTopics(ctx, in)
	if err != nil {
		if errors.Is(err, services.ErrCurriculumInvalidLanguage) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return topics, nil
}

func (a *serviceAdapter) GetTopic(ctx context.Context, language, topicID string) (services.TopicDetail, error) {
	detail, err := a.svc.GetTopic(ctx, language, topicID)
	if err != nil {
		if errors.Is(err, services.ErrCurriculumInvalidLanguage) {
			return services.TopicDetail{}, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return services.TopicDetail{}, err
	}
	return detail, nil
}

func (a *serviceAdapter) SuggestTopic(ctx context.Context, in services.SuggestTopicInput) (services.Topic, error) {
	topic, err := a.svc.SuggestTopic(ctx, in)
	if err != nil {
		if errors.Is(err, services.ErrCurriculumInvalidLanguage) {
			return services.Topic{}, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return services.Topic{}, err
	}
	return topic, nil
}

func (a *serviceAdapter) ReviewTopic(ctx context.Context, in services.ReviewTopicInput) (services.ReviewTopicResult, error) {
	out, err := a.svc.ReviewTopic(ctx, in)
	if err != nil {
		if errors.Is(err, services.ErrCurriculumInvalidLanguage) {
			return services.ReviewTopicResult{}, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return services.ReviewTopicResult{}, err
	}
	return out, nil
}

// --- mcp-curriculum.get_topics --------------------------------------------

// getTopicsTool returns the topic headlines for a language. Rejects
// path-traversal in `language` (the spec's threat-matrix scenario).
type getTopicsTool struct {
	svc Service
}

// NewGetTopicsTool registers the get_topics action.
func NewGetTopicsTool(svc Service) *getTopicsTool { return &getTopicsTool{svc: svc} }

func (t *getTopicsTool) Name() string { return "mcp-curriculum.get_topics" }

func (t *getTopicsTool) Description() string {
	return "List the curriculum topics for a given programming language. " +
		"Returns headline topic IDs, titles, and a short description " +
		"(first paragraph). Use this when the tutor wants to scope a " +
		"lesson to a specific language. Path-traversal attempts in the " +
		"`language` arg are rejected (-32602). Requires authenticated user_id."
}

func (t *getTopicsTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key (e.g. 'typescript', 'python'). Must NOT contain '/', '..', or path separators."},
		},
		requiredProps("language"),
	)
}

func (t *getTopicsTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if strings_TrimSpaceIsEmpty(a.Language) {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	topics, err := t.svc.GetTopics(ctx, services.GetTopicsInput{Language: a.Language})
	if err != nil {
		if errors.Is(err, ErrCurriculumInvalidLanguage) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	if topics == nil {
		return []services.Topic{}, nil
	}
	return topics, nil
}

// --- mcp-curriculum.get_topic ---------------------------------------------

// getTopicTool returns the full body of a topic.
type getTopicTool struct {
	svc Service
}

// NewGetTopicTool registers the get_topic action.
func NewGetTopicTool(svc Service) *getTopicTool { return &getTopicTool{svc: svc} }

func (t *getTopicTool) Name() string { return "mcp-curriculum.get_topic" }

func (t *getTopicTool) Description() string {
	return "Return the full body of a single curriculum topic: description, " +
		"prerequisites, exercises, common mistakes, resources. Use after " +
		"get_topics when the tutor wants to dive deeper. Path-traversal " +
		"attempts in `language` or `topic_id` are rejected (-32602). " +
		"Requires authenticated user_id."
}

func (t *getTopicTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"topic_id": {Type: "string", Description: "Topic slug (e.g. 'variables'). No '/' or '..'."},
		},
		requiredProps("language", "topic_id"),
	)
}

func (t *getTopicTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if strings_TrimSpaceIsEmpty(a.Language) {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if strings_TrimSpaceIsEmpty(a.TopicID) {
		return nil, fmt.Errorf("%w: topic_id is required", ErrInvalidParams)
	}
	detail, err := t.svc.GetTopic(ctx, a.Language, a.TopicID)
	if err != nil {
		if errors.Is(err, ErrCurriculumInvalidLanguage) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return detail, nil
}

// --- mcp-curriculum.suggest_topic -----------------------------------------

// suggestTopicTool proposes the next topic to study using a deterministic
// heuristic (first F-S match not already in the user's Goal set). The S5
// LLM cascade will refine the heuristic; this tool is the wired stub.
type suggestTopicTool struct {
	svc Service
}

// NewSuggestTopicTool registers the suggest_topic action.
func NewSuggestTopicTool(svc Service) *suggestTopicTool {
	return &suggestTopicTool{svc: svc}
}

func (t *suggestTopicTool) Name() string { return "mcp-curriculum.suggest_topic" }

func (t *suggestTopicTool) Description() string {
	return "Propose the next topic for the user to study based on the supplied " +
		"language, level (F..S), and known-topics CSV (Goal). Returns the " +
		"matching Topic headline; returns an empty object when no further " +
		"topic fits. Path-traversal in `language` is rejected (-32602). " +
		"Requires authenticated user_id."
}

func (t *suggestTopicTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"level":    {Type: "string", Description: "F-S range letter (F, E, D, C, B, A, S)."},
			"goal":     {Type: "string", Description: "Optional CSV of topic IDs the user has already covered; suggestions skip these."},
		},
		requiredProps("language", "level"),
	)
}

func (t *suggestTopicTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		Level    string `json:"level"`
		Goal     string `json:"goal"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if strings_TrimSpaceIsEmpty(a.Language) {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if strings_TrimSpaceIsEmpty(a.Level) {
		return nil, fmt.Errorf("%w: level is required", ErrInvalidParams)
	}
	topic, err := t.svc.SuggestTopic(ctx, services.SuggestTopicInput{
		Language: a.Language,
		Level:    a.Level,
		Goal:     a.Goal,
	})
	if err != nil {
		if errors.Is(err, ErrCurriculumInvalidLanguage) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	// Surface empty as JSON `null` rather than `{}` so the LLM can branch
	// on "is there a topic?" cleanly.
	if topic.ID == "" {
		return map[string]any{}, nil
	}
	return topic, nil
}

// --- mcp-curriculum.review_topic ------------------------------------------

// reviewTopicTool records the user's self-assessment of a completed topic.
// PR 3 has no persistence layer for reviews (S3 spec); the tool returns
// {"reviewed": true} on success so the tutor can move on.
type reviewTopicTool struct {
	svc Service
}

// NewReviewTopicTool registers the review_topic action.
func NewReviewTopicTool(svc Service) *reviewTopicTool {
	return &reviewTopicTool{svc: svc}
}

func (t *reviewTopicTool) Name() string { return "mcp-curriculum.review_topic" }

func (t *reviewTopicTool) Description() string {
	return "Record the user's self-assessment of a completed topic. " +
		"`rating` must be 1..5. PR 3 acknowledges the review but does not " +
		"persist it; the S3 free-practice spec adds the review store. " +
		"Path-traversal attempts in `language` / `topic_id` are rejected " +
		"(-32602). Requires authenticated user_id."
}

func (t *reviewTopicTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key."},
			"topic_id": {Type: "string", Description: "Topic slug."},
			"rating":   {Type: "integer", Description: "Self-rating 1..5 (1 = struggled, 5 = mastered)."},
			"notes":    {Type: "string", Description: "Optional free-form notes from the user."},
		},
		requiredProps("language", "topic_id", "rating"),
	)
}

func (t *reviewTopicTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	if userIDFromContext(ctx) == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
		TopicID  string `json:"topic_id"`
		Rating   int    `json:"rating"`
		Notes    string `json:"notes"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if strings_TrimSpaceIsEmpty(a.Language) {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if strings_TrimSpaceIsEmpty(a.TopicID) {
		return nil, fmt.Errorf("%w: topic_id is required", ErrInvalidParams)
	}
	out, err := t.svc.ReviewTopic(ctx, services.ReviewTopicInput{
		Language: a.Language,
		TopicID:  a.TopicID,
		Rating:   a.Rating,
		Notes:    a.Notes,
	})
	if err != nil {
		if errors.Is(err, ErrCurriculumInvalidLanguage) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return out, nil
}

// strings_TrimSpaceIsEmpty is a tiny helper used by every tool's input
// validator. Kept local so the tool chain stays self-contained; importing
// strings just for this would balloon the import block.
func strings_TrimSpaceIsEmpty(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return false
		}
	}
	return true
}
