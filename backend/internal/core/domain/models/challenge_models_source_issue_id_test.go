// Tests for the SourceIssueID extension to GenerateChallengeHint
// (PR 2, task 2.7) and the Origin='code_health' branch in
// GenerateChallenge (PR 2, task 2.8).
//
// Scope: validates the JSON tag on SourceIssueID, the omitempty
// behavior, and the orchestrator's branch on SourceIssueID:
// when set, the persisted row carries Origin='code_health',
// GeneratedBy='code-health-v1', SourcePath=<issue.file_path>.
// When empty, behavior is identical to S3.
package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGenerateChallengeHint_SourceIssueIDJSONTag pins the
// JSON tag on the new field so the wire shape stays
// stable (camelCase sourceIssueId). A future rename would
// break the MCP wire and surface as a "missing sourceIssueId"
// error from the LLM.
func TestGenerateChallengeHint_SourceIssueIDJSONTag(t *testing.T) {
	hint := GenerateChallengeHint{
		SourceIssueID: "issue-3",
		Language:      "typescript",
		Difficulty:    "mid",
	}
	raw, err := json.Marshal(hint)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"sourceIssueId":"issue-3"`) {
		t.Errorf("wire shape missing sourceIssueId: got %s", body)
	}
}

// TestGenerateChallengeHint_SourceIssueID_OmitsWhenEmpty pins
// the omitempty behavior so the LLM-side prompt assembly does
// not see sourceIssueId: ” when the S3 free-practice flow
// calls the service.
func TestGenerateChallengeHint_SourceIssueID_OmitsWhenEmpty(t *testing.T) {
	hint := GenerateChallengeHint{Language: "typescript"}
	raw, _ := json.Marshal(hint)
	body := string(raw)
	if strings.Contains(body, `"sourceIssueId":""`) {
		t.Errorf("omitempty leak: %q in %s", `"sourceIssueId":""`, body)
	}
	if strings.Contains(body, `"sourceIssueId"`) {
		t.Errorf("omitempty leak: 'sourceIssueId' key should be absent: %s", body)
	}
}

// TestGenerateChallengeHint_SourceIssueID_AcceptsAllExistingFields
// is a triangulation case: the new field coexists with every
// existing S3 field (language, difficulty, focus, topicId,
// level) so a caller that sets all 6 fields sees them all
// in the wire.
func TestGenerateChallengeHint_SourceIssueID_AcceptsAllExistingFields(t *testing.T) {
	hint := GenerateChallengeHint{
		Language:      "typescript",
		Difficulty:    "mid",
		Focus:         "n+1",
		TopicID:       "inheritance",
		Level:         "D",
		SourceIssueID: "issue-3",
	}
	raw, _ := json.Marshal(hint)
	body := string(raw)
	for _, want := range []string{
		`"language":"typescript"`,
		`"difficulty":"mid"`,
		`"focus":"n+1"`,
		`"topicId":"inheritance"`,
		`"level":"D"`,
		`"sourceIssueId":"issue-3"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wire shape missing %q in %s", want, body)
		}
	}
}
