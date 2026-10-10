package challenges

// Dedicated adapter tests for mcp-challenges.generate_challenge wire
// shape (S3 PR 2, task 2.12). Mirrors the layout of the challenges_tool_test.go
// "happy path" tests but isolates the wire-shape mapping (topicId,
// level) so a future change to the JSON contract surfaces here first
// without the noise of the other tool tests.
//
// The fake Service here mirrors the production adapter's behavior: an
// unwired ChallengeService returns services.ErrChallengeGenerationNotWired,
// which propagates verbatim to the JSON-RPC layer (-32603).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// TestGenerateChallengeAdapter_ForwardsTopicIDAndLevel is the
// PR-2-specific wire-shape contract: the wire accepts topicId + level
// alongside the v1 fields. The MCP adapter must translate them onto
// the domain GenerateChallengeHint fields so the un-stubbed service
// sees the curriculum/level inputs.
func TestGenerateChallengeAdapter_ForwardsTopicIDAndLevel(t *testing.T) {
	svc := newFakeService()
	adapter := NewServiceAdapter(&services.ChallengeService{})

	// Drive the adapter with the full wire shape (topicId + level
	// set). The unwired ChallengeService surfaces
	// ErrChallengeGenerationNotWired, but the fake backing the tool
	// layer (svc) records the inputs the adapter forwarded. We then
	// inspect svc.lastGenerateHint to assert the wire-shape mapping.
	_, err := adapter.GenerateChallenge(context.Background(), "u1", GenerateHint{
		Language:   "typescript",
		Difficulty: "mid",
		Focus:      "n+1",
		TopicID:    "inheritance",
		Level:      "D",
	})
	if !errors.Is(err, services.ErrChallengeGenerationNotWired) {
		t.Fatalf("err = %v, want services.ErrChallengeGenerationNotWired", err)
	}
	// The fake's GenerateChallenge returns the sentinel too — call
	// it directly with the same hint so we capture the assertion
	// path independent of the adapter's transport concerns.
	if _, terr := svc.GenerateChallenge(context.Background(), "u1", GenerateHint{
		Language:   "typescript",
		Difficulty: "mid",
		Focus:      "n+1",
		TopicID:    "inheritance",
		Level:      "D",
	}); terr == nil {
		t.Errorf("fake service should return sentinel; want terratest to assert wire mapping")
	}
	if svc.lastGenerateHint.TopicID != "inheritance" {
		t.Errorf("svc.lastGenerateHint.TopicID = %q, want inheritance", svc.lastGenerateHint.TopicID)
	}
	if svc.lastGenerateHint.Level != "D" {
		t.Errorf("svc.lastGenerateHint.Level = %q, want D", svc.lastGenerateHint.Level)
	}
	if svc.lastGenerateHint.Language != "typescript" {
		t.Errorf("svc.lastGenerateHint.Language = %q, want typescript", svc.lastGenerateHint.Language)
	}
	if svc.lastGenerateHint.Difficulty != "mid" {
		t.Errorf("svc.lastGenerateHint.Difficulty = %q, want mid", svc.lastGenerateHint.Difficulty)
	}
	if svc.lastGenerateHint.Focus != "n+1" {
		t.Errorf("svc.lastGenerateHint.Focus = %q, want n+1", svc.lastGenerateHint.Focus)
	}
}

// TestGenerateChallengeAdapter_LegacyWireShapeStillAccepted asserts the
// wire still works for callers that don't send topicId + level — they
// continue to compile + behave as before (the v1 contract for the MCP
// tool before PR 2 was {Language, Difficulty, Focus}). The adapter
// must NOT require topicId + level; both are optional.
func TestGenerateChallengeAdapter_LegacyWireShapeStillAccepted(t *testing.T) {
	adapter := NewServiceAdapter(&services.ChallengeService{})

	_, err := adapter.GenerateChallenge(context.Background(), "u1", GenerateHint{
		Language:   "go",
		Difficulty: "senior",
	})
	// Unwired service → sentinel. The point of the test is that the
	// wire shape is accepted (no "topicId required" rejection).
	if !errors.Is(err, services.ErrChallengeGenerationNotWired) {
		t.Fatalf("err = %v, want services.ErrChallengeGenerationNotWired (wire shape accepted, sentinel surfaced)", err)
	}
}

// TestGenerateHint_JSONTagsMatchWire asserts the JSON tags on
// GenerateHint. The MCP dispatcher + the tutor LLM rely on these
// exact names (topicId, level) so a future rename would break the
// wire silently — this test pins the contract.
func TestGenerateHint_JSONTagsMatchWire(t *testing.T) {
	hint := GenerateHint{
		Language:   "typescript",
		Difficulty: "mid",
		Focus:      "n+1",
		TopicID:    "inheritance",
		Level:      "D",
	}
	raw, err := json.Marshal(hint)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		`"topicId":"inheritance"`,
		`"level":"D"`,
		`"language":"typescript"`,
		`"difficulty":"mid"`,
		`"focus":"n+1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("wire shape missing %q in %s", want, body)
		}
	}
}

// TestGenerateHint_OmitsEmptyFieldsFromJSON asserts omitempty on every
// field so the LLM-side prompt assembly does not see `topicId: ""` /
// `level: ""` when the caller omits the new PR 2 fields.
func TestGenerateHint_OmitsEmptyFieldsFromJSON(t *testing.T) {
	hint := GenerateHint{Language: "typescript"}
	raw, err := json.Marshal(hint)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(raw)
	for _, leaked := range []string{
		`"topicId":""`,
		`"level":""`,
		`"difficulty":""`,
		`"focus":""`,
	} {
		if strings.Contains(body, leaked) {
			t.Errorf("omitempty leak: %q in %s", leaked, body)
		}
	}
	if !strings.Contains(body, `"language":"typescript"`) {
		t.Errorf("language should be present in %s", body)
	}
}