// Tests for the mcp-code-health.get_repo_issues tool (PR 2, task 2.4).
//
// Scope: validates the tool's argument parsing, the
// severity whitelist, the limit cap, and the user_id
// requirement. The underlying Service is mocked via the
// shared fakeCodeHealthService.
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// --- mcp-code-health.get_repo_issues ---

// TestGetRepoIssuesTool_HappyPath_ReturnsIssues is the spec
// scenario "get_repo_issues filters by severity"
// (REQ-MCP-CODE-HEALTH-001). The tool returns {issues: [...]}.
// The fake's GetRepoIssues captures the routed arguments so the
// test can assert the wire shape survives the tool boundary.
func TestGetRepoIssuesTool_HappyPath_ReturnsIssues(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs",
		"severity": "critical",
		"limit": 50
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	issues, ok := payload["issues"].([]any)
	if !ok {
		t.Fatalf("issues field is not a JSON array; got %T (payload=%+v)", payload["issues"], payload)
	}
	if len(issues) != 0 {
		t.Errorf("fake returns empty issues; got %d", len(issues))
	}
	// Argument forward assertions
	if svc.lastGetIssuesArgs.UserID != "u-1" {
		t.Errorf("routed userID = %q, want u-1", svc.lastGetIssuesArgs.UserID)
	}
	if svc.lastGetIssuesArgs.Owner != "anomalyco" {
		t.Errorf("owner = %q, want anomalyco", svc.lastGetIssuesArgs.Owner)
	}
	if svc.lastGetIssuesArgs.Source != "gogs" {
		t.Errorf("source = %q, want gogs", svc.lastGetIssuesArgs.Source)
	}
	if svc.lastGetIssuesArgs.Severity != "critical" {
		t.Errorf("severity = %q, want critical", svc.lastGetIssuesArgs.Severity)
	}
	if svc.lastGetIssuesArgs.Limit != 50 {
		t.Errorf("limit = %d, want 50", svc.lastGetIssuesArgs.Limit)
	}
}

// TestGetRepoIssuesTool_MissingUserID_RejectsWithInvalidParams
// covers the auth-missing case.
func TestGetRepoIssuesTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// TestGetRepoIssuesTool_InvalidSeverity_RejectsWithInvalidParams
// covers the severity whitelist. An invalid value is rejected
// at the tool boundary, not propagated to the repository (which
// would surface as a 500).
func TestGetRepoIssuesTool_InvalidSeverity_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs",
		"severity": "explosive"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(severity=explosive) = %v, want ErrInvalidParams", err)
	}
}

// TestGetRepoIssuesTool_LimitDefaultsTo50 is the wire default:
// when the LLM omits limit the service uses 50 (the spec's
// canonical cap).
func TestGetRepoIssuesTool_LimitDefaultsTo50(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs"
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetIssuesArgs.Limit != 50 {
		t.Errorf("limit = %d, want 50 (default)", svc.lastGetIssuesArgs.Limit)
	}
}

// TestGetRepoIssuesTool_LimitClampedToMax is the wire cap: a
// limit > 200 is clamped to 200 so a misbehaving LLM cannot
// ask for an unbounded scan.
func TestGetRepoIssuesTool_LimitClampedToMax(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"owner": "anomalyco",
		"name": "codeauditor",
		"source": "gogs",
		"limit": 100000
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetIssuesArgs.Limit != 200 {
		t.Errorf("limit = %d, want 200 (clamped)", svc.lastGetIssuesArgs.Limit)
	}
}

// TestGetRepoIssuesTool_MissingRepoDescriptor_RejectsWithInvalidParams
// covers the missing owner/name case.
func TestGetRepoIssuesTool_MissingRepoDescriptor_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetRepoIssuesTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"source": "gogs"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing owner/name) = %v, want ErrInvalidParams", err)
	}
}

// TestGetRepoIssuesTool_Name_MatchesContract pins the JSON-RPC
// tool name.
func TestGetRepoIssuesTool_Name_MatchesContract(t *testing.T) {
	tool := NewGetRepoIssuesTool(newFakeCodeHealthService())
	if got, want := tool.Name(), "mcp-code-health.get_repo_issues"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}
