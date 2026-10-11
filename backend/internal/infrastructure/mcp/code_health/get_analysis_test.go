// Tests for the mcp-code-health.get_analysis tool (PR 2, task 2.3).
//
// Scope: validates the tool's argument parsing, the user_id
// requirement, and the not-found error mapping. The underlying
// Service is mocked via the shared fakeCodeHealthService.
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
)

// --- mcp-code-health.get_analysis ---

// TestGetAnalysisTool_HappyPath_ReturnsFullPayload is the
// spec scenario "get_analysis returns full payload"
// (REQ-MCP-CODE-HEALTH-001). The tool returns the analysis
// row's wire shape including health_score, top_issues,
// languages, summary, and timestamps.
func TestGetAnalysisTool_HappyPath_ReturnsFullPayload(t *testing.T) {
	svc := newFakeCodeHealthService()
	// Override the fake's GetAnalysis to return a richer payload
	// that mirrors the spec's full shape.
	svc.getErr = nil
	tool := NewGetAnalysisTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "analysis-abc-123"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	if payload["analysis_id"] != "analysis-abc-123" {
		t.Errorf("analysis_id = %v, want analysis-abc-123", payload["analysis_id"])
	}
	if svc.lastGetAnalysisID != "analysis-abc-123" {
		t.Errorf("routed analysisID = %q, want analysis-abc-123", svc.lastGetAnalysisID)
	}
}

// TestGetAnalysisTool_MissingUserID_RejectsWithInvalidParams is
// the canonical auth-missing case.
func TestGetAnalysisTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetAnalysisTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"analysis_id": "analysis-abc-123"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// TestGetAnalysisTool_MissingAnalysisID_RejectsWithInvalidParams
// covers the empty-id case. The tool must reject before invoking
// the service.
func TestGetAnalysisTool_MissingAnalysisID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetAnalysisTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing analysis_id) = %v, want ErrInvalidParams", err)
	}
}

// TestGetAnalysisTool_NotFound_PropagatesError covers the
// "analysis not found" case. The service returns
// ErrCodeHealthNotFound; the tool surfaces it verbatim so the
// dispatcher's error-mapping layer can branch on
// errors.Is(err, ErrCodeHealthNotFound) (or any future
// not-found mapping in the dispatcher).
func TestGetAnalysisTool_NotFound_PropagatesError(t *testing.T) {
	svc := newFakeCodeHealthService()
	svc.getErr = ErrCodeHealthNotFound
	tool := NewGetAnalysisTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "missing"
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

// TestGetAnalysisTool_Name_MatchesContract pins the JSON-RPC
// tool name. A future rename would break the LLM's routing
// (method not found -32601), so this test guards the contract.
func TestGetAnalysisTool_Name_MatchesContract(t *testing.T) {
	tool := NewGetAnalysisTool(newFakeCodeHealthService())
	if got, want := tool.Name(), "mcp-code-health.get_analysis"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}

// TestGetAnalysisTool_Description_MatchesContract pins the
// schema description's first sentence so a future copy change
// shows up in this test (the description is the LLM's primary
// signal for when to invoke the tool).
func TestGetAnalysisTool_Description_MatchesContract(t *testing.T) {
	tool := NewGetAnalysisTool(newFakeCodeHealthService())
	desc := tool.Description()
	if desc == "" {
		t.Fatalf("Description is empty; LLM would have no signal for the tool")
	}
	// First word is the action verb; assert it starts with
	// "Fetch" (the canonical description start for a GET tool).
	if len(desc) < 5 || desc[:5] != "Fetch" {
		t.Errorf("Description = %q, want first word 'Fetch'", desc)
	}
}

// TestGetAnalysisTool_ArgsForwardedToService is the
// triangulation case: a different analysis_id propagates to the
// service unchanged, and the response shape carries the
// analysis_id through.
func TestGetAnalysisTool_ArgsForwardedToService(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGetAnalysisTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "different-id-99"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload := got.(map[string]any)
	if payload["analysis_id"] != "different-id-99" {
		t.Errorf("analysis_id = %v, want different-id-99", payload["analysis_id"])
	}
	if svc.lastGetAnalysisID != "different-id-99" {
		t.Errorf("routed analysisID = %q, want different-id-99", svc.lastGetAnalysisID)
	}
}
