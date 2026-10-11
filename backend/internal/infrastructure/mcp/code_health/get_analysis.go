// mcp-code-health.get_analysis — poll the status of an analysis
// job. The tool forwards the analysis_id to the production
// *codehealth.CodeHealthService and returns the wire shape
// unchanged. Status transitions are observed by the client
// polling at 2s→30s exponential backoff (see frontend
// CodeHealthUseCase, PR 3).
//
// Spec: openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 "get_analysis returns full payload".
package codehealth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// getAnalysisArgs is the wire shape (snake_case).
type getAnalysisArgs struct {
	AnalysisID string `json:"analysis_id"`
}

// getAnalysisTool is the canonical MCP tool for
// mcp-code-health.get_analysis.
type getAnalysisTool struct {
	svc Service
}

// NewGetAnalysisTool registers get_analysis under the
// mcp-code-health group.
func NewGetAnalysisTool(svc Service) *getAnalysisTool {
	return &getAnalysisTool{svc: svc}
}

// Name returns the JSON-RPC tool name.
func (t *getAnalysisTool) Name() string { return "mcp-code-health.get_analysis" }

// Description is the LLM-facing schema description.
func (t *getAnalysisTool) Description() string {
	return "Fetch the current status + payload of a code-health analysis. " +
		"Status transitions: pending → running → completed|failed. " +
		"Completed payloads include health_score, top_issues (max 10), " +
		"languages, summary, and timestamps. Use this in a poll loop " +
		"after mcp-code-health.analyze_repo. Requires an authenticated " +
		"user_id in context."
}

// Schema returns the OpenAI function schema.
func (t *getAnalysisTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{
		"analysis_id": {Type: "string", Description: "Analysis UUID returned by analyze_repo."},
	}, requiredProps("analysis_id"))
}

// Execute runs the tool.
func (t *getAnalysisTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a getAnalysisArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.AnalysisID == "" {
		return nil, fmt.Errorf("%w: analysis_id is required", ErrInvalidParams)
	}
	raw, err := t.svc.GetAnalysis(ctx, userID, a.AnalysisID)
	if err != nil {
		// Not-found surfaces verbatim. The dispatcher maps
		// internal errors to -32603; the LLM consumer can
		// branch on errors.Is(ErrCodeHealthNotFound) for a
		// "this analysis isn't yours / doesn't exist" notice.
		return nil, err
	}
	// Decode the json.RawMessage into a generic map so the
	// dispatcher can serialize it back to JSON without
	// re-parsing. The service's wire shape is the canonical
	// one (see design.md wire-shapes table).
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode analysis payload: %w", err)
	}
	return payload, nil
}
