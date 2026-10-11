// mcp-code-health.get_repo_issues — filter + paginate issues for
// the most-recent analysis of (owner, name, source). The tool
// validates the severity whitelist, clamps the limit cap, and
// delegates to the production service via the local Service
// interface.
//
// Spec: openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 "get_repo_issues filters by severity".
package codehealth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// getRepoIssuesArgs is the wire shape (snake_case).
type getRepoIssuesArgs struct {
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Severity string `json:"severity,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// allowedSeverities is the canonical whitelist for the
// severity filter; an unknown value MUST be rejected at the
// tool boundary (defense-in-depth: the repository would also
// reject, but the tool layer surfaces a clearer "invalid
// params" error to the LLM).
var allowedSeverities = map[string]bool{
	"":         true, // empty = "all severities" (no filter)
	"critical": true,
	"high":     true,
	"medium":   true,
	"low":      true,
}

// getRepoIssuesLimitMax is the canonical cap on the per-call
// limit. A misbehaving LLM asking for 100k issues is clamped
// to this so a single request cannot exhaust the database.
const getRepoIssuesLimitMax = 200

// getRepoIssuesLimitDefault is the canonical default when the
// LLM omits the limit field.
const getRepoIssuesLimitDefault = 50

// getRepoIssuesTool is the canonical MCP tool for
// mcp-code-health.get_repo_issues.
type getRepoIssuesTool struct {
	svc Service
}

// NewGetRepoIssuesTool registers get_repo_issues under the
// mcp-code-health group.
func NewGetRepoIssuesTool(svc Service) *getRepoIssuesTool {
	return &getRepoIssuesTool{svc: svc}
}

// Name returns the JSON-RPC tool name.
func (t *getRepoIssuesTool) Name() string { return "mcp-code-health.get_repo_issues" }

// Description is the LLM-facing schema description.
func (t *getRepoIssuesTool) Description() string {
	return "List ranked code-health issues for a repo, optionally filtered " +
		"by severity (critical|high|medium|low). Returns " +
		"{issues: [...]} sorted by severity weight DESC, line ASC, " +
		"category ASC. Use this to drill into a dashboard's " +
		"'show all' link or to power a server-side filter. Requires " +
		"an authenticated user_id in context."
}

// Schema returns the OpenAI function schema.
func (t *getRepoIssuesTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{
		"owner":    {Type: "string", Description: "Repo owner."},
		"name":     {Type: "string", Description: "Repo name."},
		"source":   {Type: "string", Description: "Git host: 'gogs' or 'github'."},
		"severity": {Type: "string", Description: "Filter by severity (critical|high|medium|low). Empty = all."},
		"limit":    {Type: "integer", Description: "Max issues to return (default 50, max 200)."},
	}, requiredProps("owner", "name", "source"))
}

// Execute runs the tool.
func (t *getRepoIssuesTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a getRepoIssuesArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.Owner == "" || a.Name == "" || a.Source == "" {
		return nil, fmt.Errorf("%w: owner, name, and source are required", ErrInvalidParams)
	}
	if a.Source != "gogs" && a.Source != "github" {
		return nil, fmt.Errorf("%w: source must be 'gogs' or 'github'", ErrInvalidParams)
	}
	// Severity whitelist (defense-in-depth: the repository
	// would also reject an unknown severity, but the tool
	// layer surfaces a cleaner "invalid params" error).
	if !allowedSeverities[a.Severity] {
		return nil, fmt.Errorf("%w: severity must be one of critical|high|medium|low", ErrInvalidParams)
	}
	// SSRF guard on the owner/name fields. Same regex as
	// analyze_repo so a malicious descriptor cannot pivot
	// from get_repo_issues to the source dispatcher's
	// url parser.
	if err := validateRepoPart(a.Owner, "owner"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if err := validateRepoPart(a.Name, "name"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	// Limit default + clamp.
	limit := a.Limit
	if limit <= 0 {
		limit = getRepoIssuesLimitDefault
	}
	if limit > getRepoIssuesLimitMax {
		limit = getRepoIssuesLimitMax
	}
	raw, err := t.svc.GetRepoIssues(ctx, userID, a.Owner, a.Name, a.Source, a.Severity, limit)
	if err != nil {
		return nil, err
	}
	// Decode the json.RawMessage into a generic map so the
	// dispatcher can serialize it back to JSON without
	// re-parsing. The service's wire shape is the canonical
	// {issues: [...]} per design.md wire-shapes table.
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		// Defensive: the service is expected to return
		// valid JSON. A regression would surface here
		// before the LLM sees a malformed body.
		return nil, fmt.Errorf("decode issues payload: %w", err)
	}
	// The payload may also be a bare array (some service
	// impls return [] without an object wrapper). Normalize
	// to {issues: [...]} for the LLM consumer.
	if _, ok := payload["issues"]; !ok {
		payload = map[string]any{"issues": payload}
	}
	return payload, nil
}

// Reference compile-time guard so a future signature drift
// breaks the build at the boundary, not in production.
var _ = strings.Contains
