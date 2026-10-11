package rules

import (
	"context"
	"regexp"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// InjectionRule implements RULE-SEC-INJECT-001 (spec
// REQ-CODE-HEALTH-007). It detects three classes of dynamic
// execution / injection patterns:
//
//  1. eval(...) calls — JavaScript / Lua / Python (eval) shells.
//  2. exec(...) calls — Python / Ruby / Node child_process.
//  3. SQL string concatenation into a query — TypeScript / JS /
//     Go: a `select|insert|update|delete` keyword followed by `+`
//     or template-literal interpolation within ~120 chars.
//
// All three match on a single line. Confidence=1.0 (regex fast
// path). The SQL case is intentionally fuzzy on purpose: a stricter
// AST-based check would catch more cases but requires a per-language
// parser, which the MVP defers to a future PR.
type InjectionRule struct {
	evalRe *regexp.Regexp
	execRe *regexp.Regexp
	sqlRe  *regexp.Regexp
}

// NewInjectionRule compiles the three regexes. Errors here would
// be a programming bug (the patterns are constants) so we panic
// rather than propagate — the boot-time init in default.go would
// catch a broken regex before any user request.
func NewInjectionRule() *InjectionRule {
	return &InjectionRule{
		evalRe: regexp.MustCompile(`\beval\s*\(`),
		execRe: regexp.MustCompile(`\bexec\s*\(`),
		sqlRe:  regexp.MustCompile(`(?i)\b(select|insert|update|delete)\b[^;\n]{0,120}(\$\{|` + "`" + `|\+\s*\w+)`),
	}
}

func (r *InjectionRule) ID() string                       { return "RULE-SEC-INJECT-001" }
func (r *InjectionRule) Category() models.IssueCategory   { return models.CategorySecurity }
func (r *InjectionRule) DefaultSeverity() models.Severity { return models.SeverityCritical }
func (r *InjectionRule) Weight() int                      { return 15 }

func (r *InjectionRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	var issues []models.CodeHealthIssue
	cleaned := stripLineComments(file.Content)

	appendHit := func(re *regexp.Regexp, label, fix string) {
		matches := re.FindAllStringIndex(cleaned, -1)
		for _, m := range matches {
			line := lineNumberAt(file.Content, m[0])
			issues = append(issues, models.CodeHealthIssue{
				Severity:     r.DefaultSeverity(),
				Category:     r.Category(),
				FilePath:     file.Path,
				Line:         line,
				Message:      label,
				Evidence:     strings.TrimSpace(extractLine(file.Content, line)),
				SuggestedFix: fix,
				Rule:         r.ID(),
				Confidence:   1.0,
				SHA:          file.SHA,
			})
		}
	}

	appendHit(r.evalRe,
		"Dynamic eval() detected. Avoid eval — it enables code-injection from untrusted input.",
		"Replace eval with a typed dispatch table (function map, command pattern, or a parser).")
	appendHit(r.execRe,
		"Dynamic exec() detected. Pass arguments as an array, not a concatenated string, to prevent shell injection.",
		"Use exec([cmd, arg1, arg2]) with explicit argv — never pass a concatenated shell string.")
	appendHit(r.sqlRe,
		"SQL string built with concatenation or interpolation. Use parameterized queries to prevent SQL injection.",
		"Use $1 / ? placeholders and pass values as bound parameters.")

	return issues, nil
}
