package rules

import (
	"context"
	"regexp"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// stripLineComments replaces the content of any line comment (`// ...`
// for C-family, `# ...` for Python/Ruby/shell) with whitespace so
// the byte offsets in the original string are preserved (line numbers
// stay correct) but the comment text is invisible to the regex
// matchers downstream. We keep newlines so per-line heuristics that
// look at line content (e.g. `^\s*`) still work.
//
// This is a cheap mitigation; the canonical fix is an AST-based
// detector that walks tokens. The MVP sticks with regex.
func stripLineComments(content string) string {
	// Replace `//` and `#` with spaces, but only outside string
	// literals. We do a single-pass byte scan.
	var out []byte
	inString := byte(0) // 0 means outside a string; '"' or '\'' means inside.
	for i := 0; i < len(content); i++ {
		c := content[i]
		if inString == 0 {
			if c == '"' || c == '\'' || c == '`' {
				inString = c
				out = append(out, c)
				continue
			}
			if c == '/' && i+1 < len(content) && content[i+1] == '/' {
				// Comment to end of line.
				for i < len(content) && content[i] != '\n' {
					out = append(out, ' ')
					i++
				}
				i-- // back up so the for-loop's i++ lands on '\n' or EOF
				continue
			}
			if c == '#' {
				// Comment to end of line.
				for i < len(content) && content[i] != '\n' {
					out = append(out, ' ')
					i++
				}
				i--
				continue
			}
		} else {
			if c == inString && (i == 0 || content[i-1] != '\\') {
				inString = 0
			}
		}
		out = append(out, c)
	}
	return string(out)
}

// SecretRule implements RULE-SEC-SECRET-001 (spec REQ-CODE-HEALTH-007).
// It detects hardcoded secret-looking assignments via regex. The
// pattern matches `(api[_-]?key|password|token|secret) = "..."`
// (case-insensitive) on a single line, requiring at least 4
// characters inside the quotes (filters out trivial placeholders
// like `password = "x"`).
//
// The rule is regex-only (confidence=1.0). False positives are
// expected on env-var templates and example code; the "Not an
// issue" suppression feedback (task 1.20) is the spec's prescribed
// mitigation.
type SecretRule struct {
	pattern *regexp.Regexp
}

// NewSecretRule constructs the rule with a precompiled regex.
// Splitting the literal into a raw string keeps the pattern
// readable and avoids Go's escape-rule double-escaping.
func NewSecretRule() *SecretRule {
	return &SecretRule{
		pattern: regexp.MustCompile(`(?i)(api[_-]?key|password|token|secret)\s*[:=]\s*['"][^'"]{4,}['"]`),
	}
}

func (r *SecretRule) ID() string                       { return "RULE-SEC-SECRET-001" }
func (r *SecretRule) Category() models.IssueCategory   { return models.CategorySecurity }
func (r *SecretRule) DefaultSeverity() models.Severity { return models.SeverityCritical }
func (r *SecretRule) Weight() int                      { return 15 }

func (r *SecretRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	var issues []models.CodeHealthIssue
	cleaned := stripLineComments(file.Content)
	matches := r.pattern.FindAllStringIndex(cleaned, -1)
	for _, m := range matches {
		line := lineNumberAt(file.Content, m[0])
		issues = append(issues, models.CodeHealthIssue{
			Severity:     r.DefaultSeverity(),
			Category:     r.Category(),
			FilePath:     file.Path,
			Line:         line,
			Message:      "Hardcoded secret-like assignment detected. Move the value to an environment variable or a secrets manager.",
			Evidence:     strings.TrimSpace(extractLine(file.Content, line)),
			SuggestedFix: "Read the secret from os.Getenv / process.env / a vault; never commit literal credentials.",
			Rule:         r.ID(),
			Confidence:   1.0,
			SHA:          file.SHA,
		})
	}
	return issues, nil
}

// lineNumberAt returns the 1-indexed line number that contains the
// byte offset `offset` in `content`. Used by every regex-based
// rule to convert a regex match position into a human-friendly
// line number for the issue row.
func lineNumberAt(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(content) {
		offset = len(content)
	}
	return strings.Count(content[:offset], "\n") + 1
}

// extractLine returns the (trimmed) text of the `line`-th line in
// `content`, where lines are 1-indexed. If the line is out of
// range, returns "".
func extractLine(content string, line int) string {
	if line < 1 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line-1 >= len(lines) {
		return ""
	}
	return lines[line-1]
}
