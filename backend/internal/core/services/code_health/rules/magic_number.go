package rules

import (
	"context"
	"regexp"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// MagicNumberRule implements RULE-STYLE-MAGIC-001 (spec
// REQ-CODE-HEALTH-007). It flags numeric literals with more than 2
// digits that appear OUTSIDE a `const`, `readonly`, `I18n`, or
// `enum` declaration. The 2-digit threshold filters out trivial
// numbers (0, 1, 2, ..., 99) which are usually loop counters,
// array indices, or ASCII codes.
//
// The "outside const" gate is implemented as a per-line check: if
// the line contains `const`, `readonly`, `I18n`, or `enum`, the
// match is suppressed. This is a coarse approximation; an
// AST-based gate would be more accurate but is out of scope for
// the MVP.
type MagicNumberRule struct {
	pattern *regexp.Regexp
}

// magicNumberMinDigits is the spec threshold (REQ-CODE-HEALTH-007
// row for STYLE-MAGIC-001): numeric literal > 2 digits. The
// constant is part of the rule's public docstring surface but
// is not used in code — the regex's \d{3,} literal is the
// canonical source. Kept as a named constant for future
// per-language threshold overrides.
const magicNumberMinDigits = 3

var _ = magicNumberMinDigits // referenced for future use; silences the unused-const linter

func NewMagicNumberRule() *MagicNumberRule {
	// \b\d{N,} — at least 3 consecutive digits, word-bounded.
	return &MagicNumberRule{
		pattern: regexp.MustCompile(`\b\d{3,}\b`),
	}
}

func (r *MagicNumberRule) ID() string                       { return "RULE-STYLE-MAGIC-001" }
func (r *MagicNumberRule) Category() models.IssueCategory   { return models.CategoryStyle }
func (r *MagicNumberRule) DefaultSeverity() models.Severity { return models.SeverityLow }
func (r *MagicNumberRule) Weight() int                      { return 1 }

func (r *MagicNumberRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	matches := r.pattern.FindAllStringIndex(file.Content, -1)
	issues := make([]models.CodeHealthIssue, 0, len(matches))
	for _, m := range matches {
		line := lineNumberAt(file.Content, m[0])
		lineText := strings.TrimSpace(extractLine(file.Content, line))
		if isInConstContext(lineText) {
			continue
		}
		issues = append(issues, models.CodeHealthIssue{
			Severity:     r.DefaultSeverity(),
			Category:     r.Category(),
			FilePath:     file.Path,
			Line:         line,
			Message:      "Magic number with more than 2 digits. Extract to a named constant for clarity.",
			Evidence:     lineText,
			SuggestedFix: "Declare a named constant (e.g. `const MAX_RETRIES = 100`) and reference it.",
			Rule:         r.ID(),
			Confidence:   1.0,
			SHA:          file.SHA,
		})
	}
	return issues, nil
}

// isInConstContext reports whether line contains a constant-like
// declaration marker. Used as the suppression gate for the
// magic-number rule. The marker set is the union of common
// language idioms:
//
//	const / const NAME = ...   (Go, JS, TS, Rust, C, C++)
//	final NAME = ...           (Java, Scala, Kotlin)
//	readonly NAME = ...        (C#)
//	enum NAME { ... }          (TS, Java, C#)
//	I18n.t(...) / I18n.n(...)  (Rails, i18n libraries)
//
// The check is intentionally simple: a single line containing
// any marker suppresses every numeric literal on that line. False
// negatives (a literal that should be extracted but lives on a
// `const` line) are rare in practice; the suppression path
// (task 1.20) covers the false positives.
func isInConstContext(line string) bool {
	lower := strings.ToLower(line)
	for _, marker := range []string{"const ", "const\t", "final ", "readonly ", "enum ", "i18n."} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
