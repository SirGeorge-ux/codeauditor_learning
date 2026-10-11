package rules

import (
	"context"
	"regexp"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// NPlusOneRule implements RULE-PERF-N1-001 (spec REQ-CODE-HEALTH-007).
// It flags loops whose body contains a database / ORM call — the
// classic N+1 anti-pattern.
//
// Detection: locate a for/while/(JS) forEach / (.forEach) loop opening
// on a line, then within the next 8 lines, look for a DB-shaped
// call (`Query(`, `.Find(`, `.FindOne(`, `.Get(`, `.First(`,
// `repository.find`, `await db.`, `db.query`). A `for` over an
// ORM result followed by `repository.find(...)` inside the body is
// the canonical hit.
//
// The 8-line window is conservative — a larger window catches more
// cases at the cost of more false positives. The rule is
// regex-only (confidence=1.0); the analyzer's LLM lane (not yet
// implemented) is the upgrade path.
type NPlusOneRule struct {
	loopRe *regexp.Regexp
	dbRe   *regexp.Regexp
}

// NewNPlusOneRule compiles the two regexes. See InjectionRule for
// the panic-on-regex-error rationale.
func NewNPlusOneRule() *NPlusOneRule {
	return &NPlusOneRule{
		// Match a line that opens a loop. The leading-line anchor
		// keeps the call from matching `// for each` comments.
		// `[^{]*` matches anything except `{` (parens, brackets,
		// spaces are all fine — the original `[^{(]` excluded `(`,
		// which broke every realistic `for (...)` / `while (...)`
		// header).
		loopRe: regexp.MustCompile(`(?m)^\s*(for|while)\b[^{]*\{|^\s*\w+\.forEach\s*\(`),
		dbRe:   regexp.MustCompile(`\.(Query|Find|FindOne|Get|First|One|All)\s*\(|\brepository\.(find|get|one)\s*\(|await\s+db\.`),
	}
}

func (r *NPlusOneRule) ID() string                       { return "RULE-PERF-N1-001" }
func (r *NPlusOneRule) Category() models.IssueCategory   { return models.CategoryPerformance }
func (r *NPlusOneRule) DefaultSeverity() models.Severity { return models.SeverityHigh }
func (r *NPlusOneRule) Weight() int                      { return 8 }

const nPlusOneWindow = 8 // lines after a loop opener to scan for DB calls

func (r *NPlusOneRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	lines := strings.Split(file.Content, "\n")

	loopHits := r.loopRe.FindAllStringIndex(file.Content, -1)
	issues := make([]models.CodeHealthIssue, 0, len(loopHits))
	for _, m := range loopHits {
		loopLine := lineNumberAt(file.Content, m[0])
		// Window: lines [loopLine, loopLine + nPlusOneWindow]
		// inclusive on both sides. Cap at len(lines).
		endLine := loopLine + nPlusOneWindow
		if endLine > len(lines) {
			endLine = len(lines)
		}
		var body string
		for i := loopLine - 1; i < endLine && i < len(lines); i++ {
			body += lines[i] + "\n"
		}
		if !r.dbRe.MatchString(body) {
			continue
		}
		issues = append(issues, models.CodeHealthIssue{
			Severity:     r.DefaultSeverity(),
			Category:     r.Category(),
			FilePath:     file.Path,
			Line:         loopLine,
			Message:      "Loop body contains a database call. This is the classic N+1 query anti-pattern; issue one batched query instead of N round-trips.",
			Evidence:     strings.TrimSpace(lines[loopLine-1]),
			SuggestedFix: "Issue a single query that returns all rows (IN clause, eager loading, JOIN) and iterate in memory.",
			Rule:         r.ID(),
			Confidence:   1.0,
			SHA:          file.SHA,
		})
	}
	return issues, nil
}
