// Tests for the mcp-code-health.generate_health_report tool
// (PR 2, task 2.5).
//
// Scope: validates the format whitelist (md vs pdf), the
// user_id requirement, and the happy path. The underlying
// Service is mocked via the shared fakeCodeHealthService.
package codehealth

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
)

// --- mcp-code-health.generate_health_report ---

// TestGenerateHealthReportTool_HappyPath_ReturnsMarkdown is the
// spec scenario "generate_health_report produces markdown"
// (REQ-MCP-CODE-HEALTH-001). The tool returns
// {report_id, content, format} where content is the rendered
// markdown report.
func TestGenerateHealthReportTool_HappyPath_ReturnsMarkdown(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGenerateHealthReportTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "a-1",
		"format": "md"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("got %T, want map[string]any", got)
	}
	if payload["report_id"] != "rpt-stub" {
		t.Errorf("report_id = %v, want rpt-stub", payload["report_id"])
	}
	if payload["format"] != "md" {
		t.Errorf("format = %v, want md", payload["format"])
	}
	if !regexp.MustCompile(`(?s)stub report`).MatchString(payload["content"].(string)) {
		t.Errorf("content = %q, want 'stub report' in the body", payload["content"])
	}
	if svc.lastReportAnalysisID != "a-1" {
		t.Errorf("routed analysisID = %q, want a-1", svc.lastReportAnalysisID)
	}
	if svc.lastReportFormat != "md" {
		t.Errorf("routed format = %q, want md", svc.lastReportFormat)
	}
}

// TestGenerateHealthReportTool_PdfFormat_NotImplemented covers
// the design-deferred PDF case: PR 2's tool returns the
// service's ErrCodeHealthPDFNotImplemented so the dispatcher's
// error-mapping can surface it as -32603.
func TestGenerateHealthReportTool_PdfFormat_NotImplemented(t *testing.T) {
	svc := newFakeCodeHealthService()
	svc.reportErr = errors.New("code-health: pdf report format is not implemented yet (S5)")
	tool := NewGenerateHealthReportTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "a-1",
		"format": "pdf"
	}`))
	if err == nil {
		t.Fatalf("expected pdf-not-implemented error, got nil")
	}
	if !regexp.MustCompile(`(?i)pdf`).MatchString(err.Error()) {
		t.Errorf("err message should mention 'pdf', got %q", err.Error())
	}
}

// TestGenerateHealthReportTool_InvalidFormat_RejectsWithInvalidParams
// covers the format whitelist. An unknown format is rejected
// at the tool boundary, not propagated to the service (which
// would surface as a 500).
func TestGenerateHealthReportTool_InvalidFormat_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGenerateHealthReportTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"analysis_id": "a-1",
		"format": "docx"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(format=docx) = %v, want ErrInvalidParams", err)
	}
}

// TestGenerateHealthReportTool_MissingUserID_RejectsWithInvalidParams
// covers the auth-missing case.
func TestGenerateHealthReportTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGenerateHealthReportTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"analysis_id": "a-1",
		"format": "md"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// TestGenerateHealthReportTool_MissingAnalysisID_RejectsWithInvalidParams
// covers the missing-id case.
func TestGenerateHealthReportTool_MissingAnalysisID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeCodeHealthService()
	tool := NewGenerateHealthReportTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{
		"format": "md"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing analysis_id) = %v, want ErrInvalidParams", err)
	}
}

// TestGenerateHealthReportTool_Name_MatchesContract pins the
// JSON-RPC tool name.
func TestGenerateHealthReportTool_Name_MatchesContract(t *testing.T) {
	tool := NewGenerateHealthReportTool(newFakeCodeHealthService())
	if got, want := tool.Name(), "mcp-code-health.generate_health_report"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
}
