// mcp-code-health.generate_health_report — render an analysis
// row as a markdown (or pdf — S5) report. The MVP supports
// markdown only; pdf returns the service's
// ErrCodeHealthPDFNotImplemented so the dispatcher's error
// mapping can surface it as -32603 (per design.md tradeoffs).
//
// Spec: openspec/changes/code-health-dashboard/specs/mcp/spec.md
// REQ-MCP-CODE-HEALTH-001 "generate_health_report produces
// markdown" + "PDF report" deferred to S5.
package codehealth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// generateHealthReportArgs is the wire shape (snake_case).
type generateHealthReportArgs struct {
	AnalysisID string `json:"analysis_id"`
	Format     string `json:"format"`
}

// allowedReportFormats is the canonical whitelist. pdf is
// returned as a sentinel error from the service layer so the
// dispatcher can map it to -32603.
var allowedReportFormats = map[string]bool{
	"md":  true,
	"pdf": true, // surfaces ErrCodeHealthPDFNotImplemented from the service
}

// generateHealthReportTool is the canonical MCP tool for
// mcp-code-health.generate_health_report.
type generateHealthReportTool struct {
	svc Service
}

// NewGenerateHealthReportTool registers generate_health_report
// under the mcp-code-health group.
func NewGenerateHealthReportTool(svc Service) *generateHealthReportTool {
	return &generateHealthReportTool{svc: svc}
}

// Name returns the JSON-RPC tool name.
func (t *generateHealthReportTool) Name() string { return "mcp-code-health.generate_health_report" }

// Description is the LLM-facing schema description.
func (t *generateHealthReportTool) Description() string {
	return "Render a completed code-health analysis as a markdown (md) or " +
		"PDF (pdf — S5) report. Returns {report_id, content, format}. " +
		"The markdown content includes the health score, ranked issues " +
		"table, language breakdown, and a 'generate challenges' hint. " +
		"PDF returns an 'not implemented' error until S5 lands. " +
		"Requires an authenticated user_id in context."
}

// Schema returns the OpenAI function schema.
func (t *generateHealthReportTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{
		"analysis_id": {Type: "string", Description: "Analysis UUID (from analyze_repo or get_analysis)."},
		"format":      {Type: "string", Description: "Output format: 'md' (markdown) or 'pdf' (deferred to S5)."},
	}, requiredProps("analysis_id", "format"))
}

// Execute runs the tool.
func (t *generateHealthReportTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a generateHealthReportArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.AnalysisID == "" {
		return nil, fmt.Errorf("%w: analysis_id is required", ErrInvalidParams)
	}
	if !allowedReportFormats[a.Format] {
		return nil, fmt.Errorf("%w: format must be 'md' or 'pdf'", ErrInvalidParams)
	}
	reportID, content, format, err := t.svc.GenerateHealthReport(ctx, userID, a.AnalysisID, a.Format)
	if err != nil {
		// The service returns ErrCodeHealthPDFNotImplemented
		// for format=pdf; the dispatcher maps that to
		// -32603. Other errors propagate to -32603 as
		// internal errors.
		return nil, err
	}
	return map[string]any{
		"report_id": reportID,
		"content":   content,
		"format":    format,
	}, nil
}
