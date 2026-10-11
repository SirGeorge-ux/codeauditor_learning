// mcp-code-health.analyze_repo — fire-and-forget entry point for
// the S4 code-health dashboard. The tool validates the wire
// shape, applies the SSRF guard on the owner/name fields, and
// delegates to the production *codehealth.CodeHealthService via
// the local Service interface.
//
// Spec: openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 "analyze_repo returns analysis_id
// synchronously" — the tool returns {analysis_id, status:"pending"}
// within 500ms; the goroutine transitions the row in the
// background.
//
// Threat matrix: REQ-CODE-HEALTH threat matrix (design.md) —
// SSRF via `owner` or `name`. The tool rejects "../" patterns at
// the wire boundary BEFORE any outbound call to the Gogs/GitHub
// source clients (defense in depth; the source clients have their
// own URL parsers but the tool layer must not assume them).
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"

	codehealthsvc "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
)

// analyzeRepoArgs is the wire shape (snake_case, mirroring the
// design.md wire-shapes table).
type analyzeRepoArgs struct {
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Branch string `json:"branch,omitempty"`
	Force  bool   `json:"force,omitempty"`
}

// analyzeRepoTool is the canonical MCP tool for
// mcp-code-health.analyze_repo.
type analyzeRepoTool struct {
	svc Service
}

// NewAnalyzeRepoTool registers analyze_repo under the
// mcp-code-health group.
func NewAnalyzeRepoTool(svc Service) *analyzeRepoTool {
	return &analyzeRepoTool{svc: svc}
}

// Name returns the JSON-RPC tool name.
func (t *analyzeRepoTool) Name() string { return "mcp-code-health.analyze_repo" }

// Description is the LLM-facing schema description.
func (t *analyzeRepoTool) Description() string {
	return "Kick off a code-health analysis on a Gogs or GitHub repo. " +
		"Returns {analysis_id, status:\"pending\"} within 500ms; the row " +
		"transitions to completed|failed in the background. Poll " +
		"mcp-code-health.get_analysis(analysis_id) for the result. " +
		"Requires an authenticated user_id in context."
}

// Schema returns the OpenAI function schema. owner + name +
// source are required; branch defaults to "master" server-side;
// force (default false) bypasses the per-file cache.
func (t *analyzeRepoTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{
		"owner":  {Type: "string", Description: "Repo owner (e.g. 'anomalyco'). No slashes, no '..'."},
		"name":   {Type: "string", Description: "Repo name (e.g. 'codeauditor'). No slashes, no '..'."},
		"source": {Type: "string", Description: "Git host: 'gogs' or 'github'."},
		"branch": {Type: "string", Description: "Branch to analyze. Defaults to 'master'."},
		"force":  {Type: "boolean", Description: "Re-analyze: bypass the per-file cache. Defaults to false."},
	}, requiredProps("owner", "name", "source"))
}

// Execute runs the tool. The wire args are unmarshaled with
// strict field validation (owner + name + source required).
func (t *analyzeRepoTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a analyzeRepoArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	// SSRF + length guard: owner and name must be non-empty,
	// must not start with "/", and must not contain ".."
	// (path-traversal). Slash guards prevent URL-relative
	// tricks; ".." prevents repository descriptor pivots.
	if err := validateRepoPart(a.Owner, "owner"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if err := validateRepoPart(a.Name, "name"); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	// Source whitelist. A typo or attack via a non-canonical
	// source MUST be rejected at the tool boundary, not
	// propagated to the source dispatcher.
	if a.Source != "gogs" && a.Source != "github" {
		return nil, fmt.Errorf("%w: source must be 'gogs' or 'github'", ErrInvalidParams)
	}
	// Branch default. Empty -> "master" (the canonical default
	// the schema and the DB both honor).
	branch := a.Branch
	if branch == "" {
		branch = "master"
	}
	analysisID, status, err := t.svc.AnalyzeRepo(ctx, userID, a.Owner, a.Name, a.Source, branch, a.Force)
	if err != nil {
		// The daily-cap sentinel from the service is
		// mapped to mcpkit.NewRateLimitedError so the
		// dispatcher's IsRateLimitedError check fires and
		// the response is JSON-RPC -32609 (per
		// REQ-CODE-HEALTH-011). Other errors propagate to
		// -32603.
		if errors.Is(err, codehealthsvc.ErrCodeHealthDailyCapReached) {
			return nil, mcpkit.NewRateLimitedError(t.Name())
		}
		return nil, err
	}
	return map[string]any{
		"analysis_id": analysisID,
		"status":      status,
	}, nil
}

// validateRepoPart guards against path-traversal + URL-relative
// tricks on the owner/name fields. Empty values are rejected
// (caller must require them upstream). The ".." check is the
// threat-matrix SSRF guard from design.md.
func validateRepoPart(s, field string) error {
	if s == "" {
		return errors.New(field + " is required")
	}
	if strings.Contains(s, "..") {
		return errors.New(field + " contains path-traversal segment '..'")
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") {
		return errors.New(field + " cannot start with '/' or '\\'")
	}
	return nil
}
