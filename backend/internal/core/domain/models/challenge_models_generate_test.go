package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// Tests for the GenerateChallengeHint domain shape extended by
// free-practice-mode PR 2 (S3). The hint carries TopicID (a slug from
// .atl/curricula/<lang>.md) and Level (F..S from the per-user rank
// model). The wire-side MCP adapter translates GenerateHint into
// GenerateChallengeHint before calling the service; keeping the test
// here means the canonical input shape is enforced independently of
// the transport.

// TestGenerateChallengeHint_DefaultsToEmptyValues asserts the
// zero-value GenerateChallengeHint has every string field empty so a
// service caller can detect "no preference" via hint.Language == "".
func TestGenerateChallengeHint_DefaultsToEmptyValues(t *testing.T) {
	t.Parallel()

	var h GenerateChallengeHint
	if h.Language != "" || h.Difficulty != "" || h.Focus != "" || h.TopicID != "" || h.Level != "" {
		t.Errorf("zero hint = %+v, want all empty", h)
	}
}

// TestGenerateChallengeHint_OmitsEmptyFieldsFromJSON locks the wire
// shape consumed by the MCP adapter. omitempty on every string field
// means a partial hint serializes to only the populated keys so the
// LLM-side prompt assembly does not see `topicId: ""`.
func TestGenerateChallengeHint_OmitsEmptyFieldsFromJSON(t *testing.T) {
	t.Parallel()

	h := GenerateChallengeHint{TopicID: "inheritance", Level: "D"}
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, `"topicId":"inheritance"`) {
		t.Errorf("missing topicId in marshal: %s", body)
	}
	if !strings.Contains(body, `"level":"D"`) {
		t.Errorf("missing level in marshal: %s", body)
	}
	if strings.Contains(body, `"language":""`) {
		t.Errorf("omitempty leak: %s", body)
	}
}

// TestGenerateChallengeHint_RoundTripsTopicIDAndLevel asserts the
// added fields survive a JSON round trip. The MCP adapter uses JSON
// to translate GenerateHint (wire) into GenerateChallengeHint (domain);
// losing either field would silently break prompt assembly.
func TestGenerateChallengeHint_RoundTripsTopicIDAndLevel(t *testing.T) {
	t.Parallel()

	src := GenerateChallengeHint{
		Language:   "typescript",
		Difficulty: "mid",
		TopicID:    "async-await",
		Level:      "D",
	}
	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got GenerateChallengeHint
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.TopicID != "async-await" {
		t.Errorf("TopicID = %q, want async-await", got.TopicID)
	}
	if got.Level != "D" {
		t.Errorf("Level = %q, want D", got.Level)
	}
	if got.Language != "typescript" {
		t.Errorf("Language = %q, want typescript", got.Language)
	}
}

// Silence unused-import risk on a future refactor.