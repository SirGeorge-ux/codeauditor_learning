// Tests for the SourceIssueID wire-arg extension to the
// mcp-challenges.generate_challenge tool (PR 2, task 2.9).
//
// Scope: validates the tool forwards sourceIssueId to the
// service's GenerateChallengeHint, and the legacy wire shape
// (without sourceIssueId) still works.
package challenges

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestGenerateChallengeTool_SourceIssueID_ForwardedToService
// is the PR-2 wire-shape contract: the wire accepts
// sourceIssueId (camelCase) alongside the S3 fields (topicId,
// level, language, difficulty, focus) so the un-stubbed
// ChallengeService.GenerateChallenge can drive the S4
// code-health origin branch.
func TestGenerateChallengeTool_SourceIssueID_ForwardedToService(t *testing.T) {
	svc := newFakeService()
	tool := NewGenerateChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "typescript",
		"difficulty": "mid",
		"sourceIssueId": "issue-3"
	}`))
	if err == nil {
		t.Fatalf("expected sentinel; got nil")
	}
	h := svc.lastGenerateHint
	if h.SourceIssueID != "issue-3" {
		t.Errorf("hint.sourceIssueId = %q, want issue-3", h.SourceIssueID)
	}
}

// TestGenerateChallengeTool_SourceIssueID_EmptyStillS3 asserts
// the legacy wire shape (no sourceIssueId) still works — the
// S3 free-practice path is unchanged.
func TestGenerateChallengeTool_SourceIssueID_EmptyStillS3(t *testing.T) {
	svc := newFakeService()
	tool := NewGenerateChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "typescript",
		"difficulty": "mid",
		"topicId": "inheritance",
		"level": "D"
	}`))
	if err == nil {
		t.Fatalf("expected sentinel; got nil")
	}
	if svc.lastGenerateHint.SourceIssueID != "" {
		t.Errorf("hint.sourceIssueId = %q, want empty (S3 path)", svc.lastGenerateHint.SourceIssueID)
	}
}

// TestGenerateChallengeTool_GenerateHintJSONTagPinsSourceIssueID
// guards the JSON tag on the new field. The wire shape is
// 'sourceIssueId' (camelCase, matches the topicId + level
// convention); a future rename to 'source_issue_id' or
// 'sourceIssueID' would break the LLM consumer.
func TestGenerateChallengeTool_GenerateHintJSONTagPinsSourceIssueID(t *testing.T) {
	hint := GenerateHint{
		SourceIssueID: "issue-3",
		Language:      "typescript",
	}
	raw, _ := json.Marshal(hint)
	body := string(raw)
	if !strings.Contains(body, `"sourceIssueId":"issue-3"`) {
		t.Errorf("wire shape missing sourceIssueId: got %s", body)
	}
}

// TestGenerateChallengeTool_GenerateHintOmitsEmptySourceIssueID
// pins omitempty so the LLM-side prompt assembly does not
// see sourceIssueId: ” when the S3 free-practice flow calls
// the service.
func TestGenerateChallengeTool_GenerateHintOmitsEmptySourceIssueID(t *testing.T) {
	hint := GenerateHint{Language: "typescript"}
	raw, _ := json.Marshal(hint)
	body := string(raw)
	if strings.Contains(body, `"sourceIssueId"`) {
		t.Errorf("omitempty leak: 'sourceIssueId' key should be absent when empty: %s", body)
	}
}

// Reference unused import guard so go vet stays clean when
// the helper above is trimmed.
var _ = context.Background
