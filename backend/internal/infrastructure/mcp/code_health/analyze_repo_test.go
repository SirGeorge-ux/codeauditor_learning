// Tests for the mcp-code-health.analyze_repo tool (PR 2, task 2.2).
//
// Scope: validates the tool's argument parsing, the SSRF guard on
// the owner field, the source whitelist, the daily-cap defense
// (forwarded from the service), and the user_id requirement. The
// underlying *codehealth.CodeHealthService is mocked via a fake
// Service so the test never touches the database, the network, or
// Docker.
//
// RED-then-GREEN: the test file is written first; the
// implementation (in analyze_repo.go) lands in the same commit so
// the suite stays green. The two companions of this file are
// get_analysis_test.go (task 2.3) and the adapter-level
// serviceAdapter.AnalyzeRepo wiring (same package, separate file).
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	codehealthsvc "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
)

// fakeCodeHealthService is the hand-rolled stand-in for the local
// Service interface. The MCP tool tests substitute it via
// NewAnalyzeRepoTool(fake). The fake records the last call so each
// test can assert the wire-shape mapping without booting the
// production *codehealth.CodeHealthService.
type fakeCodeHealthService struct {
	mu sync.Mutex

	// Behavior knobs (set per-test).
	analyzeErr  error
	getErr      error
	issuesErr   error
	reportErr   error
	suppressErr error
	dailyCapHit bool // when true, AnalyzeRepo returns the daily-cap sentinel

	// Recorded inputs (last call).
	lastAnalyzeUserID    string
	lastAnalyzeOwner     string
	lastAnalyzeName      string
	lastAnalyzeSource    string
	lastAnalyzeBranch    string
	lastAnalyzeForce     bool
	lastGetAnalysisID    string
	lastGetIssuesArgs    recordedGetRepoIssues
	lastReportAnalysisID string
	lastReportFormat     string
	lastSuppressIssueID  string
	lastSuppressReason   *string
}

type recordedGetRepoIssues struct {
	UserID, Owner, Name, Source, Severity string
	Limit                                 int
}

func newFakeCodeHealthService() *fakeCodeHealthService {
	return &fakeCodeHealthService{}
}

var _ = recordedGetRepoIssues{} // ensure the type stays referenced when the fakes are trimmed

func (f *fakeCodeHealthService) AnalyzeRepo(_ context.Context, userID, owner, name, source, branch string, force bool) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAnalyzeUserID = userID
	f.lastAnalyzeOwner = owner
	f.lastAnalyzeName = name
	f.lastAnalyzeSource = source
	f.lastAnalyzeBranch = branch
	f.lastAnalyzeForce = force
	if f.dailyCapHit {
		// Return the canonical sentinel so the tool's
		// errors.Is branch fires and the response is
		// JSON-RPC -32609 (defense-in-depth: the
		// production service returns this exact sentinel
		// from CountSince(midnightUTC) >= 5).
		return "", "", codehealthsvc.ErrCodeHealthDailyCapReached
	}
	if f.analyzeErr != nil {
		return "", "", f.analyzeErr
	}
	return "analysis-abc-123", "pending", nil
}

func (f *fakeCodeHealthService) GetAnalysis(_ context.Context, userID, analysisID string) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGetAnalysisID = analysisID
	if f.getErr != nil {
		return nil, f.getErr
	}
	return json.RawMessage(`{"analysis_id":"` + analysisID + `","status":"pending"}`), nil
}

func (f *fakeCodeHealthService) GetRepoIssues(_ context.Context, userID, owner, name, source, severity string, limit int) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGetIssuesArgs = recordedGetRepoIssues{
		UserID: userID, Owner: owner, Name: name, Source: source, Severity: severity, Limit: limit,
	}
	if f.issuesErr != nil {
		return nil, f.issuesErr
	}
	// Return the canonical {issues: [...]} wire shape so the
	// tool's decoder round-trips correctly.
	return json.RawMessage(`{"issues":[]}`), nil
}

func (f *fakeCodeHealthService) GenerateHealthReport(_ context.Context, userID, analysisID, format string) (string, string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastReportAnalysisID = analysisID
	f.lastReportFormat = format
	if f.reportErr != nil {
		return "", "", "", f.reportErr
	}
	return "rpt-stub", "# stub report\n", format, nil
}

func (f *fakeCodeHealthService) SuppressIssue(_ context.Context, userID, issueID string, reason *string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSuppressIssueID = issueID
	f.lastSuppressReason = reason
	if f.suppressErr != nil {
		return "", "", f.suppressErr
	}
	return "sup-stub", "2026-10-11T00:00:00Z", nil
}

// ctxWithUser is a small helper that mirrors the canonical pattern
// in challenges/dictionary test files: stamp the user_id via the
// local context key.
func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-code-health.analyze_repo ---

// TestAnalyzeRepoTool_HappyPath_ReturnsPendingID asserts the
// canonical happy path: a valid user_id, valid owner/name/branch,
// and source="gogs" produces a JSON object {analysis_id, status}
// where status is "pending" and the analysis_id matches the
// service's return.
func TestAnalyzeRepoTool_HappyPath_ReturnsPendingID(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs",
		"branch": "master"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	if payload["status"] != "pending" {
		t.Errorf("status = %v, want pending", payload["status"])
	}
	if payload["analysis_id"] != "analysis-abc-123" {
		t.Errorf("analysis_id = %v, want analysis-abc-123", payload["analysis_id"])
	}
	if svc.lastAnalyzeUserID != "u-1" {
		t.Errorf("routed userID = %q, want u-1", svc.lastAnalyzeUserID)
	}
	if svc.lastAnalyzeSource != "gogs" {
		t.Errorf("source = %q, want gogs", svc.lastAnalyzeSource)
	}
	if svc.lastAnalyzeOwner != "anomalyco" {
		t.Errorf("owner = %q, want anomalyco", svc.lastAnalyzeOwner)
	}
	if svc.lastAnalyzeForce {
		t.Errorf("force must default to false; got true")
	}
}

// TestAnalyzeRepoTool_ForceTrue_ForwardedToService asserts the
// wire's "force":true is forwarded to the service so a Re-analyze
// CTA can bypass the per-file cache.
func TestAnalyzeRepoTool_ForceTrue_ForwardedToService(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "github",
		"force": true
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !svc.lastAnalyzeForce {
		t.Errorf("force=true not forwarded to service")
	}
	if svc.lastAnalyzeSource != "github" {
		t.Errorf("source = %q, want github", svc.lastAnalyzeSource)
	}
}

// TestAnalyzeRepoTool_MissingUserID_RejectsWithInvalidParams is
// the spec scenario "Missing user_id in MCP tool call"
// (REQ-CODE-HEALTH-013). The dispatcher maps ErrInvalidParams to
// JSON-RPC -32602.
func TestAnalyzeRepoTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// TestAnalyzeRepoTool_SSRFGuard_OwnerTraversalRejected is the
// threat matrix RED test (design.md §Threat matrix). The MCP
// layer MUST reject `owner="../../evil"` BEFORE any outbound call
// to the Gogs/GitHub source clients — escaping the fixed baseURL
// would let an attacker pivot to internal infrastructure.
func TestAnalyzeRepoTool_SSRFGuard_OwnerTraversalRejected(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "../../evil",
		"name": "codeauditor",
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(SSRF owner) = %v, want ErrInvalidParams (rejected before any outbound call)", err)
	}
	// Defense-in-depth: the service MUST NOT have been called.
	if svc.lastAnalyzeOwner != "" {
		t.Errorf("service.AnalyzeRepo was called with owner=%q; SSRF guard must reject BEFORE the service", svc.lastAnalyzeOwner)
	}
}

// TestAnalyzeRepoTool_SSRFGuard_NameTraversalRejected covers the
// same threat on the `name` field — a path-traversal in `name`
// must be rejected at the tool boundary, not at the source
// client.
func TestAnalyzeRepoTool_SSRFGuard_NameTraversalRejected(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "../etc/passwd",
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(SSRF name) = %v, want ErrInvalidParams", err)
	}
}

// TestAnalyzeRepoTool_InvalidSource_RejectsWithInvalidParams
// covers the source whitelist ("gogs" or "github"). A typo or
// attack via a non-canonical source must be rejected at the tool
// boundary, not propagated to the source dispatch (which would
// surface as a 500).
func TestAnalyzeRepoTool_InvalidSource_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "bitbucket"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(source=bitbucket) = %v, want ErrInvalidParams", err)
	}
}

// TestAnalyzeRepoTool_MissingRequiredFields_RejectsWithInvalidParams
// covers the missing owner/name case. The tool must reject
// without invoking the service.
func TestAnalyzeRepoTool_MissingRequiredFields_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing owner/name) = %v, want ErrInvalidParams", err)
	}
}

// TestAnalyzeRepoTool_DailyCap_ReturnsRateLimitedError is the
// daily-cap defense-in-depth (REQ-CODE-HEALTH-011). When the
// user has hit the cap, the service returns the daily-cap
// sentinel; the tool must surface a rate-limit-flavored error so
// the dispatcher maps it to JSON-RPC -32609.
//
// The fake's dailyCapHit knob simulates the service's
// CountSince(midnightUTC) >= 5 branch (PR 2 task 2.11). The tool
// MUST surface the error verbatim so the dispatcher's
// IsRateLimitedError branch fires.
func TestAnalyzeRepoTool_DailyCap_ReturnsRateLimitedError(t *testing.T) {
	svc := newFakeCodeHealthService()
	svc.dailyCapHit = true
	tool := NewAnalyzeRepoTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs"
	}`))
	if err == nil {
		t.Fatalf("expected daily-cap error, got nil")
	}
	// The tool maps the service's daily-cap sentinel to
	// mcpkit.NewRateLimitedError so the dispatcher's
	// IsRateLimitedError check fires and the response is
	// JSON-RPC -32609 (defense-in-depth: the client
	// localStorage cap is the primary control; this is the
	// backstop).
	if !mcpkit.IsRateLimitedError(err) {
		t.Errorf("daily-cap error should be a rate-limit-flavored error, got %T %v", err, err)
	}
}

// TestAnalyzeRepoTool_BranchDefaultsToMaster asserts the wire
// defaults branch to "master" when omitted. The MCP wire shape
// uses an optional branch field; absent means "use the repo
// default" which the service persists as master.
func TestAnalyzeRepoTool_BranchDefaultsToMaster(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewAnalyzeRepoTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs"
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastAnalyzeBranch != "master" {
		t.Errorf("branch = %q, want master (default)", svc.lastAnalyzeBranch)
	}
}
