package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All filesystem tests use t.TempDir() so they never depend on the real
// repo path. The tests cover:
//
//   - 3.1: RED → GREEN path traversal guard (table-driven, above)
//   - 3.1: language-only safe characters accepted
//   - 3.3 happy paths for get_topics / get_topic / suggest_topic / review_topic
//   - The markdown parser — feed a real specimen into a temp file and
//     assert the structured output.

func writeMD(t *testing.T, baseDir, language, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(baseDir, language+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("seed %s.md: %v", language, err)
	}
}

const sampleCurriculum = `---
language: typescript
display_name: TypeScript
---

# TypeScript Curriculum

## Topics

### variables

- **slug:** variables
- **difficulty:** F
- **prerequisites:** []
- **category:** syntax
- **description:** |
  Variables are the most basic building block. TypeScript has ` + "`" + `let` + "`" + `, ` + "`" + `const` + "`" + `, and ` + "`" + `var` + "`" + ` (avoid).
- **exercises:**
  - "Declare a const for a user's name."
  - "Reassign a let and observe the type error."
- **common_mistakes:**
  - "Using var instead of let/const."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/variable-declarations.html

### types

- **slug:** types
- **difficulty:** E
- **prerequisites:** [variables]
- **category:** types
- **description:** |
  Primitives, arrays, tuples, unions, and intersections.
- **exercises:**
  - "Type a function returning string | number."
- **common_mistakes:**
  - "Using any instead of a specific type."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/everyday-types.html
`

func TestCurriculumService_GetTopics_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "typescript", sampleCurriculum)
	svc := NewCurriculumService(WithBaseDir(dir))

	got, err := svc.GetTopics(context.Background(), GetTopicsInput{Language: "typescript"})
	if err != nil {
		t.Fatalf("GetTopics: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(topics) = %d, want 2; got=%+v", len(got), got)
	}
	if got[0].ID != "variables" || got[0].Difficulty != "F" {
		t.Errorf("first topic = %+v, want id=variables difficulty=F", got[0])
	}
	if got[1].ID != "types" || got[1].Difficulty != "E" {
		t.Errorf("second topic = %+v, want id=types difficulty=E", got[1])
	}
	// Headline description must include the lede, not the whole prose.
	if got[0].Description == "" {
		t.Errorf("first topic description = empty, want first paragraph")
	}
}

func TestCurriculumService_GetTopics_MissingFile_ReturnsEmptySlice(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	got, err := svc.GetTopics(context.Background(), GetTopicsInput{Language: "kotlin"})
	if err != nil {
		t.Fatalf("GetTopics(missing): %v", err)
	}
	if got == nil {
		t.Errorf("GetTopics(missing) returned nil slice, want empty (LLM branches on len not nil)")
	}
	if len(got) != 0 {
		t.Errorf("len(topics) = %d, want 0", len(got))
	}
}

func TestCurriculumService_GetTopic_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "typescript", sampleCurriculum)
	svc := NewCurriculumService(WithBaseDir(dir))

	detail, err := svc.GetTopic(context.Background(), "typescript", "types")
	if err != nil {
		t.Fatalf("GetTopic: %v", err)
	}
	if detail.ID != "types" {
		t.Errorf("id = %q, want types", detail.ID)
	}
	if detail.Difficulty != "E" {
		t.Errorf("difficulty = %q, want E", detail.Difficulty)
	}
	if len(detail.Prerequisites) != 1 || detail.Prerequisites[0] != "variables" {
		t.Errorf("prerequisites = %v, want [variables]", detail.Prerequisites)
	}
	if len(detail.Exercises) != 1 || !strings.Contains(detail.Exercises[0], "string | number") {
		t.Errorf("exercises = %v, want at least one with 'string | number'", detail.Exercises)
	}
}

func TestCurriculumService_GetTopic_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	// Topic IDs MUST face the same guard — a malicious LLM caller asking
	// for topic "../etc" must be rejected before we read from disk.
	cases := []string{"../etc", "foo/..", "../variables"}
	for _, bad := range cases {
		t.Run(bad, func(t *testing.T) {
			_, err := svc.GetTopic(context.Background(), "typescript", bad)
			if err == nil {
				t.Fatalf("GetTopic(%q) accepted path-traversal; want error", bad)
			}
			if !errors.Is(err, ErrCurriculumInvalidLanguage) {
				t.Errorf("err = %v, want ErrCurriculumInvalidLanguage", err)
			}
		})
	}
}

func TestCurriculumService_GetTopic_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "typescript", sampleCurriculum)
	svc := NewCurriculumService(WithBaseDir(dir))

	_, err := svc.GetTopic(context.Background(), "typescript", "missing")
	if err == nil {
		t.Fatal("expected not-found, got nil")
	}
	if errors.Is(err, ErrCurriculumInvalidLanguage) {
		t.Errorf("missing-topic must not be reported as invalid-language: %v", err)
	}
}

func TestCurriculumService_SuggestTopic_PrefersRequestedDifficulty(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "typescript", sampleCurriculum)
	svc := NewCurriculumService(WithBaseDir(dir))

	got, err := svc.SuggestTopic(context.Background(), SuggestTopicInput{
		Language: "typescript",
		Level:    "E",
		Goal:     "",
	})
	if err != nil {
		t.Fatalf("SuggestTopic: %v", err)
	}
	if got.ID != "types" {
		t.Errorf("SuggestTopic(level=E) = %q, want 'types' (E is the second topic)", got.ID)
	}
}

func TestCurriculumService_SuggestTopic_SkipsKnownTopics(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "typescript", sampleCurriculum)
	svc := NewCurriculumService(WithBaseDir(dir))

	// "I've already covered variables" → suggest the F-level topic that
	// is NOT in the goal set. With only one F-level topic, we expect the
	// loop to fall through and return the empty Topic sentinel.
	got, err := svc.SuggestTopic(context.Background(), SuggestTopicInput{
		Language: "typescript",
		Level:    "F",
		Goal:     "variables",
	})
	if err != nil {
		t.Fatalf("SuggestTopic: %v", err)
	}
	if got.ID != "" {
		t.Errorf("SuggestTopic(level=F, goal=variables) = %+v, want zero (no further F-level topic available)", got)
	}
}

func TestCurriculumService_SuggestTopic_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	_, err := svc.SuggestTopic(context.Background(), SuggestTopicInput{Language: "../etc", Level: "F"})
	if err == nil {
		t.Fatal("SuggestTopic accepted ../etc; want error")
	}
}

func TestCurriculumService_ReviewTopic_HappyPath(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	out, err := svc.ReviewTopic(context.Background(), ReviewTopicInput{
		Language: "typescript",
		TopicID:  "variables",
		Rating:   4,
		Notes:    "felt solid",
	})
	if err != nil {
		t.Fatalf("ReviewTopic: %v", err)
	}
	if !out.Reviewed {
		t.Errorf("Reviewed = false, want true")
	}
}

func TestCurriculumService_ReviewTopic_RejectsBadRating(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	for _, bad := range []int{0, 6, -1} {
		t.Run("rating="+itoaForTest(bad), func(t *testing.T) {
			_, err := svc.ReviewTopic(context.Background(), ReviewTopicInput{
				Language: "typescript",
				TopicID:  "variables",
				Rating:   bad,
			})
			if err == nil {
				t.Fatalf("rating %d must be rejected", bad)
			}
		})
	}
}

func TestCurriculumService_ReviewTopic_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	svc := NewCurriculumService(WithBaseDir(dir))

	_, err := svc.ReviewTopic(context.Background(), ReviewTopicInput{
		Language: "typescript",
		TopicID:  "../etc",
		Rating:   3,
	})
	if err == nil {
		t.Fatal("ReviewTopic accepted ../topicID; want error")
	}
}

func TestCurriculumService_NoBaseDir_FailsClosed(t *testing.T) {
	// Constructed without WithBaseDir — every read must fail closed.
	svc := NewCurriculumService()
	_, err := svc.GetTopics(context.Background(), GetTopicsInput{Language: "typescript"})
	if err == nil {
		t.Fatal("GetTopics without baseDir returned nil err; want ErrCurriculumInvalidLanguage")
	}
}

// itoaForTest avoids importing strconv for one call.
func itoaForTest(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
