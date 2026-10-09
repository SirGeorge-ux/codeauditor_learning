package services

// Tests for CurriculumService. PR 3 ships the path-traversal guard the spec
// (mcp-monolith threat matrix "Path Traversal in Curricula") mandates, plus
// happy-path topic extraction from .atl/curricula/<lang>.md.
//
// All filesystem tests use t.TempDir() so they never rely on the real repo
// path and run hermetically.

import (
	"context"
	"strings"
	"testing"
)

// Tests are in a separate file so the curriculum_service.go stub stays
// minimal until the RED → GREEN path lands.

func TestCurriculumService_GetTopics_RejectsParentTraversal(t *testing.T) {
	// Spec scenario (mcp-monolith threat matrix): passing "../../etc/passwd"
	// as the language MUST be rejected, not turned into a filepath traversal
	// out of the curricula root.
	dir := t.TempDir()
	svc := NewCurriculumService(dir)

	cases := []struct {
		name     string
		language string
	}{
		{"double-dot-prefix", "../etc"},
		{"double-dot-infix", "type../script"},
		{"leading-double-dot", ".."},
		{"nested-traversal", "foo/../../bar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.GetTopics(context.Background(), GetTopicsInput{Language: c.language})
			if err == nil {
				t.Fatalf("GetTopics(%q) accepted path-traversal; want error", c.language)
			}
			if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "traversal") {
				t.Errorf("GetTopics(%q) error = %q, want an 'invalid language' rejection", c.language, err)
			}
		})
	}
}
