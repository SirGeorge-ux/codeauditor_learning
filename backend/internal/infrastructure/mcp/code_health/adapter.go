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

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	codehealth "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
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

// --- serviceAdapter method implementations ---

// AnalyzeRepo delegates to the production service. The
// production CodeHealthService.AnalyzeRepo returns the
// analysis_id; the adapter always reports status="pending"
// (the row's terminal status flips asynchronously in the
// goroutine — by the time the LLM polls, get_analysis will
// see the transition).
func (a *serviceAdapter) AnalyzeRepo(ctx context.Context, userID, owner, name, source, branch string, force bool) (string, string, error) {
	if a.svc == nil {
		return "", "", fmt.Errorf("codehealth: service is nil")
	}
	analysisID, err := a.svc.AnalyzeRepo(ctx, userID, owner, name, source, branch, force)
	if err != nil {
		return "", "", err
	}
	return analysisID, string(models.CodeHealthStatusPending), nil
}

// GetAnalysis returns the analysis row's wire shape. The
// production service.GetAnalysis does the DB lookup + 24h
// expiry check; the adapter projects the typed result into
// the JSON envelope the LLM consumes.
func (a *serviceAdapter) GetAnalysis(ctx context.Context, userID, analysisID string) (json.RawMessage, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("codehealth: service is nil")
	}
	if analysisID == "" {
		return nil, ErrCodeHealthNotFound
	}
	a2, err := a.svc.GetAnalysis(ctx, userID, analysisID)
	if err != nil {
		if errors.Is(err, ports.ErrCodeHealthAnalysisNotFound) {
			return nil, ErrCodeHealthNotFound
		}
		return nil, err
	}
	return encodeAnalysis(&a2), nil
}

// GetRepoIssues returns the ranked issues for the most-recent
// analysis of (owner, name, source), filtered by severity
// (optional) and capped at limit. The wire shape is
// {issues: [...]} per design.md.
func (a *serviceAdapter) GetRepoIssues(ctx context.Context, userID, owner, name, source, severity string, limit int) (json.RawMessage, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("codehealth: service is nil")
	}
	issues, err := a.svc.GetRepoIssues(ctx, userID, owner, name, source, severity, limit)
	if err != nil {
		return nil, err
	}
	// Project the typed slice into the wire shape.
	// The service's GetRepoIssues returns []CodeHealthIssue;
	// the wire shape is {issues: [...]} per design.md.
	// Empty slice MUST serialize as [] (never null) so the
	// LLM consumer never sees a type-mismatch on an empty
	// result.
	if issues == nil {
		issues = []models.CodeHealthIssue{}
	}
	payload, err := json.Marshal(map[string]any{"issues": encodeIssueList(issues)})
	if err != nil {
		return nil, fmt.Errorf("encode issues payload: %w", err)
	}
	return payload, nil
}

// GenerateHealthReport stub. Real implementation lands in
// generate_health_report.go (PR 2, task 2.5).
func (a *serviceAdapter) GenerateHealthReport(_ context.Context, _, analysisID, format string) (string, string, string, error) {
	if a.svc == nil {
		return "", "", "", fmt.Errorf("codehealth: service is nil")
	}
	return "stub-rpt-" + analysisID, "# stub report\n", format, nil
}

// SuppressIssue records a user-marked "not an issue" feedback.
// The production service looks up the issue, writes the
// suppression row, and returns the deterministic id + timestamp.
// Returns ErrCodeHealthNotFound when issueID does not exist
// (or belongs to a different user).
func (a *serviceAdapter) SuppressIssue(ctx context.Context, userID, issueID string, reason *string) (string, string, error) {
	if a.svc == nil {
		return "", "", fmt.Errorf("codehealth: service is nil")
	}
	if issueID == "" {
		return "", "", ErrCodeHealthNotFound
	}
	res, err := a.svc.SuppressIssue(ctx, userID, issueID, reason)
	if err != nil {
		if errors.Is(err, ports.ErrCodeHealthIssueNotFound) {
			return "", "", ErrCodeHealthNotFound
		}
		return "", "", err
	}
	return res.SuppressionID, res.SuppressedAt.UTC().Format(time.RFC3339), nil
}

// Reference unused import guards so go vet stays clean when the
// per-tool files trim the package. The schema helper above uses
// openai.Tool so the import is live; the same applies to the
// mcpkit/mcpctx bridge.
var (
	_ = openai.Tool{}
	_ = propertyArg{}
)

// encodeAnalysis projects a models.CodeHealthAnalysis into the
// JSON-RPC wire shape documented in design.md
// (mcp-code-health.get_analysis). The shape matches the spec
// verbatim so a frontend consumer sees the exact field names
// the LLM tool description promises.
//
// Nullable fields (health_score, completed_at, error_message)
// are omitted when zero so the wire shape stays minimal.
func encodeAnalysis(a *models.CodeHealthAnalysis) json.RawMessage {
	out := map[string]any{
		"analysis_id": a.ID,
		"status":      string(a.Status),
		"source":      string(a.Source),
		"owner":       a.Owner,
		"name":        a.Name,
		"branch":      a.Branch,
		"top_issues":  encodeIssueList(a.TopIssues),
		"languages":   a.Languages,
		"created_at":  a.CreatedAt.UTC().Format(time.RFC3339),
		"expires_at":  a.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if a.HealthScore != nil {
		out["health_score"] = *a.HealthScore
	}
	if a.Summary != "" {
		out["summary"] = a.Summary
	}
	if a.CompletedAt != nil {
		out["completed_at"] = a.CompletedAt.UTC().Format(time.RFC3339)
	}
	if a.ErrorMessage != "" {
		out["error_message"] = a.ErrorMessage
	}
	raw, err := json.Marshal(out)
	if err != nil {
		// Marshaling a map[string]any of primitives can only
		// fail on unsupported types — we never put any in
		// here, so this branch is unreachable in practice.
		// Return a minimal valid envelope so the dispatcher
		// never sees nil.
		return json.RawMessage(`{"error":"encode failed"}`)
	}
	return raw
}

// encodeIssueList serializes a slice of CodeHealthIssue into
// the wire shape consumed by get_analysis (top_issues) and
// get_repo_issues (issues). The shape is shared between both
// tools so the LLM consumer can deserialize them with the
// same type.
func encodeIssueList(issues []models.CodeHealthIssue) []map[string]any {
	out := make([]map[string]any, 0, len(issues))
	for i := range issues {
		iss := &issues[i]
		out = append(out, encodeIssue(iss))
	}
	return out
}

// encodeIssue serializes a single CodeHealthIssue into the
// wire shape documented in design.md
// (mcp-code-health.get_repo_issues). Mirrors the
// ExpectedFinding shape from challenge-rebuild (1:1 field
// mapping) plus rule + confidence + sha (per spec).
func encodeIssue(iss *models.CodeHealthIssue) map[string]any {
	out := map[string]any{
		"severity":   string(iss.Severity),
		"category":   string(iss.Category),
		"file_path":  iss.FilePath,
		"line":       iss.Line,
		"message":    iss.Message,
		"rule":       iss.Rule,
		"confidence": iss.Confidence,
	}
	if iss.ID != "" {
		out["id"] = iss.ID
	}
	if iss.AnalysisID != "" {
		out["analysis_id"] = iss.AnalysisID
	}
	if iss.Evidence != "" {
		out["evidence"] = iss.Evidence
	}
	if iss.SuggestedFix != "" {
		out["suggested_fix"] = iss.SuggestedFix
	}
	if iss.SHA != "" {
		out["sha"] = iss.SHA
	}
	return out
}
