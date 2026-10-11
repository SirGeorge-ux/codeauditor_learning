// Package codehealth implements the mcp-code-health tool group of the
// MCP monolith. It exposes five driving adapters — analyze_repo,
// get_analysis, get_repo_issues, generate_health_report,
// suppress_issue — that wrap the code-health dashboard core service
// for the LLM tutor.
//
// Hexagonal contract: this package imports the local Service interface
// (and shared mcpkit/mcpctx helpers) plus the core domain model. It
// MUST NOT import database/sql, net/http, the GitHub SDK, the Gogs
// SDK, or any driven adapter. The wiring in cmd/api/wiring.go adapts
// the production *codehealth.CodeHealthService to the local Service
// so the tools stay swappable for tests.
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"

	codehealth "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported here so tool-local code can use the
// short name; the dispatcher's errors.Is check matches the mcpkit
// canonical.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrCodeHealthNotFound mirrors the port-level
// ErrCodeHealthAnalysisNotFound + ErrCodeHealthIssueNotFound so
// the get_* + suppress_issue tools can branch with errors.Is
// without importing the service package. The production adapter
// wires the real sentinels via errors.Is comparison (see
// NewServiceAdapter for the bridge).
var ErrCodeHealthNotFound = errors.New("code-health: not found")

// userIDKey is the local context key for the authenticated user_id.
// Mirrors the canonical challenges/dictionary pattern so a tool
// can run under either this package's local key or the mcpkit
// bridge key.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in
// ctx. It first consults this package's own key, then falls back
// to the canonical MCP bridge key (mcpctx.AuthUserIDKey).
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// propertyArg is the standardized JSON envelope produced for LLM
// tool routing. Keeping it local ensures consistent descriptions.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single
// object parameter list. The shape matches the dictionary/tutor/
// challenges tools so the tutor LLM sees a consistent contract
// across groups.
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

// requiredProps materializes required-field names so all schema()
// calls keep the same convention.
func requiredProps(names ...string) []string { return names }

// Service is the local interface every MCP code-health tool
// depends on. The production wiring supplies
// *codehealth.CodeHealthService through a thin adapter
// (NewServiceAdapter below); tests substitute a fake.
//
// Each method projects the service's typed return into the JSON
// envelope the LLM consumes. The wire shape is documented per
// method; the adapter owns the marshaling so the tool side stays
// consistent.
type Service interface {
	// AnalyzeRepo is the async entry point. It validates input
	// (source ∈ {gogs, github}, owner/name non-empty, branch
	// default), runs the daily-cap defense-in-depth check, then
	// delegates to the service. Returns the analysis_id and a
	// pending status; the goroutine transitions the row
	// pending→running→completed|failed in the background.
	AnalyzeRepo(ctx context.Context, userID, owner, name, source, branch string, force bool) (analysisID string, status string, err error)

	// GetAnalysis returns the analysis row's wire shape. The
	// analysis_id must be a UUID; an unknown id returns
	// ErrCodeHealthNotFound.
	GetAnalysis(ctx context.Context, userID, analysisID string) (json.RawMessage, error)

	// GetRepoIssues returns the ranked issues for the most-recent
	// analysis of (owner, name, source). severity is optional
	// (empty = all); limit caps the returned slice (0 = 50).
	GetRepoIssues(ctx context.Context, userID, owner, name, source, severity string, limit int) (json.RawMessage, error)

	// GenerateHealthReport renders the analysis as a markdown
	// (or pdf — S5) report. Returns the report id, content,
	// format, or an error (ErrCodeHealthPDFNotImplemented for
	// pdf until S5 lands).
	GenerateHealthReport(ctx context.Context, userID, analysisID, format string) (reportID, content, formatOut string, err error)

	// SuppressIssue records a user-marked "not an issue" feedback
	// (spec REQ-CODE-HEALTH-008 "not an issue" scenario). The
	// tool maps the issue_id to (file_path, rule_id, line_number)
	// via the repository, then writes to code_health_suppressions.
	// Returns the deterministic suppression_id and the
	// suppressed_at timestamp. Unknown issue_id → ErrCodeHealthNotFound.
	SuppressIssue(ctx context.Context, userID, issueID string, reason *string) (suppressionID, suppressedAt string, err error)
}

// SuppressionResult is the structured wire shape returned by
// Service.SuppressIssue. Mirrors the mcp-code-health.suppress_issue
// tool's response: {suppression_id, suppressed_at}.
type SuppressionResult struct {
	SuppressionID string    `json:"suppression_id"`
	SuppressedAt  time.Time `json:"suppressed_at"`
}

// serviceAdapter bridges *codehealth.CodeHealthService to the local
// Service interface. Each method delegates to the production
// service, owning the JSON marshaling so the tool side stays
// consistent with the wire shapes documented in
// openspec/changes/code-health-dashboard/specs/mcp/spec.md
// (REQ-MCP-CODE-HEALTH-001) and the design.md wire-shapes table.
type serviceAdapter struct {
	svc *codehealth.CodeHealthService
}

// NewServiceAdapter returns the production wiring for the
// mcp-code-health tool group. The adapter bridges
// *codehealth.CodeHealthService to the local Service interface,
// owning the JSON wire shape so the tool side stays consistent.
func NewServiceAdapter(svc *codehealth.CodeHealthService) Service {
	return &serviceAdapter{svc: svc}
}

// --- Stubs (filled in by per-tool commits; safe to call with a
// zero-value inner service so the package compiles + the package
// boundary tests pass before the per-tool implementations land).

// AnalyzeRepo stub. Real implementation lands in analyze_repo.go
// (PR 2, task 2.2). The stub returns the input owner as the
// analysis_id so the test "TestServiceAdapter_ImplementsAllMethodSignatures"
// stays green before the per-tool commits. A real tool call from
// production NEVER hits this path (the tool-side Execute calls
// the per-tool wrapper, not the adapter directly).
func (a *serviceAdapter) AnalyzeRepo(_ context.Context, _, owner, _, _, _ string, _ bool) (string, string, error) {
	if a.svc == nil {
		return "", "", fmt.Errorf("codehealth: service is nil")
	}
	return "stub-" + owner, "pending", nil
}

// GetAnalysis stub. Real implementation lands in get_analysis.go
// (PR 2, task 2.3).
func (a *serviceAdapter) GetAnalysis(_ context.Context, _, analysisID string) (json.RawMessage, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("codehealth: service is nil")
	}
	if analysisID == "" {
		return nil, ErrCodeHealthNotFound
	}
	return json.RawMessage(`{"status":"stub"}`), nil
}

// GetRepoIssues stub. Real implementation lands in
// get_repo_issues.go (PR 2, task 2.4).
func (a *serviceAdapter) GetRepoIssues(_ context.Context, _, _, _, _, _ string, _ int) (json.RawMessage, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("codehealth: service is nil")
	}
	return json.RawMessage(`[]`), nil
}

// GenerateHealthReport stub. Real implementation lands in
// generate_health_report.go (PR 2, task 2.5).
func (a *serviceAdapter) GenerateHealthReport(_ context.Context, _, analysisID, format string) (string, string, string, error) {
	if a.svc == nil {
		return "", "", "", fmt.Errorf("codehealth: service is nil")
	}
	return "stub-rpt-" + analysisID, "# stub report\n", format, nil
}

// SuppressIssue stub. Real implementation lands in
// suppress_issue.go (PR 2, task 2.12). Returns the deterministic
// stub so the boundary test stays green.
func (a *serviceAdapter) SuppressIssue(_ context.Context, _, issueID string, _ *string) (string, string, error) {
	if a.svc == nil {
		return "", "", fmt.Errorf("codehealth: service is nil")
	}
	if issueID == "" {
		return "", "", ErrCodeHealthNotFound
	}
	return "stub-sup-" + issueID, "1970-01-01T00:00:00Z", nil
}

// Reference unused import guards so go vet stays clean when the
// per-tool files trim the package. The schema helper above uses
// openai.Tool so the import is live; the same applies to the
// mcpkit/mcpctx bridge.
var (
	_ = openai.Tool{}
	_ = propertyArg{}
)
