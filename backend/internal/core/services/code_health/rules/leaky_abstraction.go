package rules

import (
	"context"
	"regexp"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// LeakyAbstractionRule implements RULE-ARCH-LEAK-001 (spec
// REQ-CODE-HEALTH-007). It flags imports of another module's
// `internal/` package — the canonical leaky-abstraction signal in
// Go, where `internal/` is the language-blessed privacy boundary.
//
// Detection: any import line that references a path containing
// `/internal/` is flagged. We restrict the rule to Go imports for
// the MVP because that is the only language in the project's
// polyglot stack where `internal/` carries that semantic. Other
// languages' analogues (TS barrel files, Python `_internal` by
// convention) are out of scope.
type LeakyAbstractionRule struct {
	importRe *regexp.Regexp
}

// internalPathRe is the canonical pattern: a path segment
// `internal` between slashes. We anchor on `/` on both sides to
// avoid matching the bare word "internal" in a comment or
// variable name.
var internalPathRe = regexp.MustCompile(`(?:^|/)internal/`)

func NewLeakyAbstractionRule() *LeakyAbstractionRule {
	return &LeakyAbstractionRule{
		// Three Go import shapes:
		//   1. `import "path"`           (single-line)
		//   2. `import "alias/path"`     (aliased)
		//   3. `import (\n\t"path"\n)`   (grouped)
		// We match the path in shapes 1+2 via the `import "..."`
		// pattern and in shape 3 by capturing each quoted string
		// inside the group. The second alternative uses
		// `[^"]+` for the path content (no double-quote).
		importRe: regexp.MustCompile(`(?m)(?:import\s+"([^"]+)"|^\s*"([^"]+)"\s*$)`),
	}
}

func (r *LeakyAbstractionRule) ID() string                       { return "RULE-ARCH-LEAK-001" }
func (r *LeakyAbstractionRule) Category() models.IssueCategory   { return models.CategoryArchitecture }
func (r *LeakyAbstractionRule) DefaultSeverity() models.Severity { return models.SeverityHigh }
func (r *LeakyAbstractionRule) Weight() int                      { return 8 }

func (r *LeakyAbstractionRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	if file.Language != "go" {
		// Rule is Go-only; the analyzer dispatches it on every
		// file but the language gate keeps the cost low.
		return nil, nil
	}
	matches := r.importRe.FindAllStringSubmatch(file.Content, -1)
	issues := make([]models.CodeHealthIssue, 0, len(matches))
	for _, m := range matches {
		// m[1] is the single-line `import "..."` path; m[2] is
		// the grouped `import ( ... )` path. Use whichever is
		// non-empty.
		path := m[1]
		if path == "" {
			path = m[2]
		}
		if path == "" {
			continue
		}
		if !internalPathRe.MatchString(path) {
			continue
		}
		line := lineNumberAt(file.Content, indexOf(file.Content, m[0]))
		issues = append(issues, models.CodeHealthIssue{
			Severity:     r.DefaultSeverity(),
			Category:     r.Category(),
			FilePath:     file.Path,
			Line:         line,
			Message:      "Importing another module's `internal/` package is a leaky-abstraction smell. The Go visibility boundary is being crossed.",
			Evidence:     m[0],
			SuggestedFix: "Expose the required surface through a public API on the owning module; never reach into its `internal/` tree from outside.",
			Rule:         r.ID(),
			Confidence:   1.0,
			SHA:          file.SHA,
		})
	}
	return issues, nil
}

// indexOf is a small helper that returns the byte offset of the
// first occurrence of substr in s, or -1 if absent. We use it to
// compute the line number for a regex submatch without re-running
// the matcher.
func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
