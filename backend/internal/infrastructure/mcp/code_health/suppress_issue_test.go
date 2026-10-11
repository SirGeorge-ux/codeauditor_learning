// Tests for the mcp-code-health.suppress_issue tool (PR 2, task 2.12).
//
// Scope: validates the tool's argument parsing, the user_id
// requirement, the not-found error mapping (unknown issue_id),
// and the happy path. The underlying Service is mocked via the
// shared fakeCodeHealthService.
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
)

// --- mcp-code-health.suppress_issue ---

// TestSuppressIssueTool_HappyPath_ReturnsSuppressionID is the
// spec scenario "User marks 'not an issue' invalidates the
// finding" (REQ-CODE-HEALTH-008). The tool returns
// {suppression_id, suppressed_at} so the frontend can
// optimistic-update the UI.
func TestSuppressIssueTool_HappyPath_ReturnsSuppressionID(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewSuppressIssueTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"issue_id": "issue-3",
		"reason": "false positive"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	if payload["suppression_id"] != "sup-stub" {
		t.Errorf("suppression_id = %v, want sup-stub", payload["suppression_id"])
	}
	if payload["suppressed_at"] != "2026-10-11T00:00:00Z" {
		t.Errorf("suppressed_at = %v, want 2026-10-11T00:00:00Z", payload["suppressed_at"])
	}
	if svc.lastSuppressIssueID != "issue-3" {
		t.Errorf("routed issueID = %q, want issue-3", svc.lastSuppressIssueID)
	}
	if svc.lastSuppressReason == nil || *svc.lastSuppressReason != "false positive" {
		t.Errorf("reason = %v, want 'false positive'", svc.lastSuppressReason)
	}
}

// TestSuppressIssueTool_ReasonOptional covers the case when the
// LLM omits the reason field. The tool must still call the
// service with reason=nil so the row is written with a NULL
// reason column.
func TestSuppressIssueTool_ReasonOptional(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewSuppressIssueTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"issue_id": "issue-3"
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastSuppressReason != nil {
		t.Errorf("reason = %v, want nil (omitted)", *svc.lastSuppressReason)
	}
}

// TestSuppressIssueTool_MissingUserID_RejectsWithInvalidParams
// covers the auth-missing case.
func TestSuppressIssueTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewSuppressIssueTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"issue_id": "issue-3"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// TestSuppressIssueTool_MissingIssueID_RejectsWithInvalidParams
// covers the missing-id case.
func TestSuppressIssueTool_MissingIssueID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewSuppressIssueTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing issue_id) = %v, want ErrInvalidParams", err)
	}
}

// TestSuppressIssueTool_UnknownIssue_PropagatesNotFoundError
// covers the "unknown issue" case. The service returns
// ErrCodeHealthNotFound (the canonical sentinel); the tool
// surfaces it verbatim so the LLM can present a "this issue
// isn't yours / doesn't exist" notice.
func TestSuppressIssueTool_UnknownIssue_PropagatesNotFoundError(t *testing.T) {
	svc := newFakeCodeHealthService()
	svc.suppressErr = ErrCodeHealthNotFound
	tool := NewSuppressIssueTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"issue_id": "missing-issue"
	}`))
	if err == nil {
		t.Fatalf("expected not-found error, got nil")
	}
	if !errors.Is(err, ErrCodeHealthNotFound) {
		t.Errorf("err = %v, want errors.Is(ErrCodeHealthNotFound)", err)
	}
	// The error message MUST mention "not found" so the LLM
	// consumer can surface a useful notice.
	if matched := regexp.MustCompile(`(?i)not found`).MatchString(err.Error()); !matched {
		t.Errorf("err message should contain 'not found', got %q", err.Error())
	}
}

// TestSuppressIssueTool_Name_MatchesContract pins the JSON-RPC
// tool name.
func TestSuppressIssueTool_Name_MatchesContract(t *testing.T) {
	tool := NewSuppressIssueTool(newFakeCodeHealthService())
	if got, want := tool.Name(), "mcp-code-health.suppress_issue"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}
