// mcp-code-health.suppress_issue — record a user-marked "not an
// issue" feedback (spec REQ-CODE-HEALTH-008 "not an issue"
// scenario). The tool forwards the issue_id to the production
// *codehealth.CodeHealthService, which looks up the
// (file_path, rule_id, line_number) tuple, writes the suppression
// row, and returns the deterministic suppression_id +
// suppressed_at timestamp.
//
// Spec: openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 "analyze_repo returns analysis_id
// synchronously" — this tool is the user's "I disagree"
// feedback on a finding surfaced by get_repo_issues /
// get_analysis. The next analyze_repo run for the same repo
// will skip the suppressed (file_path, rule_id, line_number)
// combination (analyzer round-trip coverage in
// analyzer_test.go).
package codehealth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"
)

// suppressIssueArgs is the wire shape (snake_case).
type suppressIssueArgs struct {
	IssueID string  `json:"issue_id"`
	Reason  *string `json:"reason,omitempty"`
}

// suppressIssueTool is the canonical MCP tool for
// mcp-code-health.suppress_issue.
type suppressIssueTool struct {
	svc Service
}

// NewSuppressIssueTool registers suppress_issue under the
// mcp-code-health group.
func NewSuppressIssueTool(svc Service) *suppressIssueTool {
	return &suppressIssueTool{svc: svc}
}

// Name returns the JSON-RPC tool name.
func (t *suppressIssueTool) Name() string { return "mcp-code-health.suppress_issue" }

// Description is the LLM-facing schema description.
func (t *suppressIssueTool) Description() string {
	return "Mark a code-health issue as a 'not an issue' (false positive). " +
		"Writes a row to code_health_suppressions so the next analysis " +
		"for the same (file_path, rule_id, line_number) skips it. " +
		"Returns {suppression_id, suppressed_at} so the frontend can " +
		"optimistic-update. Idempotent: a second call for the same " +
		"issue returns the same suppression_id. Requires an " +
		"authenticated user_id in context."
}

// Schema returns the OpenAI function schema.
func (t *suppressIssueTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{
		"issue_id": {Type: "string", Description: "Issue UUID (from get_repo_issues or get_analysis top_issues)."},
		"reason":   {Type: "string", Description: "Optional human-readable reason (e.g. 'false positive', 'outdated')."},
	}, requiredProps("issue_id"))
}

// Execute runs the tool. The wire shape returns
// {suppression_id, suppressed_at} (snake_case).
func (t *suppressIssueTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a suppressIssueArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.IssueID == "" {
		return nil, fmt.Errorf("%w: issue_id is required", ErrInvalidParams)
	}
	supID, supAtStr, err := t.svc.SuppressIssue(ctx, userID, a.IssueID, a.Reason)
	if err != nil {
		// The not-found sentinel from the service is
		// propagated verbatim. The dispatcher maps internal
		// errors to -32603; the LLM consumer can branch on
		// errors.Is(ErrCodeHealthNotFound) for a "this
		// issue isn't yours / doesn't exist" notice.
		return nil, err
	}
	// supAtStr is an RFC3339 timestamp. The service returns
	// a deterministic id; we keep the wire shape minimal.
	_ = time.Time{}
	return map[string]any{
		"suppression_id": supID,
		"suppressed_at":  supAtStr,
	}, nil
}
