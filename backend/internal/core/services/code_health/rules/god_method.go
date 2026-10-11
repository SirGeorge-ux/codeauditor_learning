package rules

import (
	"context"
	"regexp"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// GodMethodRule implements RULE-REFAC-GOD-001 (spec REQ-CODE-HEALTH-007).
// It flags two smells:
//
//  1. A function body longer than 50 source lines (the canonical
//     "long method" smell).
//  2. A class / struct with more than 10 public methods (the
//     "god class" smell).
//
// Detection is line-based and language-agnostic. For #1, we look
// for a function header (e.g. `func name(...) {`,
// `function name(...) {`, `def name(...):`) and count lines until
// the matching closing brace at the same indent. For #2, we count
// method-like declarations inside a class / struct block.
//
// Both are regex-only (confidence=1.0). The rule is intentionally
// conservative — better a few false positives the user can
// suppress than missed god methods on a 200-line function.
type GodMethodRule struct {
	funcHeaderRe  *regexp.Regexp
	classHeaderRe *regexp.Regexp
	methodRe      *regexp.Regexp
}

// godMethodMaxLines is the threshold for the "long method" smell
// (spec REQ-CODE-HEALTH-007 row for REFAC-GOD-001).
const godMethodMaxLines = 50

// godMethodMaxPublicMethods is the threshold for the "god class"
// smell (spec REQ-CODE-HEALTH-007 row for REFAC-GOD-001).
const godMethodMaxPublicMethods = 10

func NewGodMethodRule() *GodMethodRule {
	return &GodMethodRule{
		// function name(args) { or func name(args) { or def name(args):
		// We accept the optional `async` prefix in JS/TS and the
		// Go receiver `(s *Service)` form.
		funcHeaderRe: regexp.MustCompile(`(?m)^\s*(public\s+|private\s+|protected\s+|async\s+|static\s+|export\s+)*(function\s+\w+|def\s+\w+|func\s+(\([^)]+\)\s+)?\w+)\s*\(`),
		// class Name { or struct Name { or type Name struct {
		// — the Go form is `type Name struct { ... }` (the
		// keyword is `type`, not `struct` at column 0).
		classHeaderRe: regexp.MustCompile(`(?m)^(class\s+\w+|struct\s+\w+|type\s+\w+\s+struct)`),
		// A method declaration: `func (r *R) Name(` for Go,
		// `Name(args) {` for JS/TS/Python. Public = not prefixed
		// with `_` in Python or lowercase in Go.
		methodRe: regexp.MustCompile(`(?m)^\s*((public|private|protected)\s+)?(func\s+\([^)]+\)\s+|function\s+|def\s+|(\w+)\s*\()`),
	}
}

func (r *GodMethodRule) ID() string                       { return "RULE-REFAC-GOD-001" }
func (r *GodMethodRule) Category() models.IssueCategory   { return models.CategoryRefactor }
func (r *GodMethodRule) DefaultSeverity() models.Severity { return models.SeverityMedium }
func (r *GodMethodRule) Weight() int                      { return 3 }

func (r *GodMethodRule) Detect(_ context.Context, file File) ([]models.CodeHealthIssue, error) {
	lines := strings.Split(file.Content, "\n")
	cleaned := stripLineComments(file.Content)

	// Pre-allocate for the long-method + god-class detection
	// passes. The function header regex + class header regex
	// combined is the upper bound on per-file findings.
	funcHits := r.funcHeaderRe.FindAllStringIndex(cleaned, -1)
	classHits := r.classHeaderRe.FindAllStringIndex(cleaned, -1)
	issues := make([]models.CodeHealthIssue, 0, len(funcHits)+len(classHits))

	// --- Long methods: count lines between the opening brace of a
	// function and the matching closing brace at the same indent.
	for _, m := range funcHits {
		headerLine := lineNumberAt(cleaned, m[0])
		bodyStart, bodyEnd := findBodyBounds(lines, headerLine-1)
		if bodyStart < 0 || bodyEnd <= bodyStart {
			continue
		}
		bodyLen := bodyEnd - bodyStart + 1
		if bodyLen <= godMethodMaxLines {
			continue
		}
		issues = append(issues, models.CodeHealthIssue{
			Severity:     r.DefaultSeverity(),
			Category:     r.Category(),
			FilePath:     file.Path,
			Line:         headerLine,
			Message:      "Function body exceeds 50 lines. Extract cohesive sub-procedures; long methods hide bugs and resist reuse.",
			Evidence:     strings.TrimSpace(lines[headerLine-1]),
			SuggestedFix: "Split by responsibility: each extracted helper should do one thing and be testable in isolation.",
			Rule:         r.ID(),
			Confidence:   1.0,
			SHA:          file.SHA,
		})
	}

	// --- God classes: count public methods in a class/struct block.
	// For Go, methods are declared at package level with a receiver
	// `(s *Name)`, NOT inside the `type Name struct { ... }` block.
	// We handle this by collecting `type Name struct` declarations
	// separately and counting receiver methods on `Name` across the
	// whole file. For C-style classes (TS/JS/Python) we count methods
	// inside the block as before.
	for _, m := range classHits {
		classLine := lineNumberAt(cleaned, m[0])
		matched := cleaned[m[0]:m[1]]
		switch {
		case strings.HasPrefix(matched, "type "):
			// Go: collect receiver methods across the whole file.
			name := extractGoTypeName(matched)
			if name == "" {
				continue
			}
			publicMethods := countGoReceiverMethods(cleaned, name)
			if publicMethods <= godMethodMaxPublicMethods {
				continue
			}
			issues = append(issues, models.CodeHealthIssue{
				Severity:     r.DefaultSeverity(),
				Category:     r.Category(),
				FilePath:     file.Path,
				Line:         classLine,
				Message:      "Class has more than 10 public methods. It likely carries too many responsibilities; split by cohesion.",
				Evidence:     strings.TrimSpace(lines[classLine-1]),
				SuggestedFix: "Apply the Single Responsibility Principle: extract a focused collaborator (e.g. a Repository, a Validator) and re-delegate.",
				Rule:         r.ID(),
				Confidence:   1.0,
				SHA:          file.SHA,
			})
		default:
			// TS/JS/Python class block: count methods inside the
			// brace-delimited body.
			_, classEnd := findBodyBounds(lines, classLine-1)
			if classEnd < 0 {
				continue
			}
			publicMethods := countPublicMethods(lines, classLine-1, classEnd)
			if publicMethods <= godMethodMaxPublicMethods {
				continue
			}
			issues = append(issues, models.CodeHealthIssue{
				Severity:     r.DefaultSeverity(),
				Category:     r.Category(),
				FilePath:     file.Path,
				Line:         classLine,
				Message:      "Class has more than 10 public methods. It likely carries too many responsibilities; split by cohesion.",
				Evidence:     strings.TrimSpace(lines[classLine-1]),
				SuggestedFix: "Apply the Single Responsibility Principle: extract a focused collaborator (e.g. a Repository, a Validator) and re-delegate.",
				Rule:         r.ID(),
				Confidence:   1.0,
				SHA:          file.SHA,
			})
		}
	}

	return issues, nil
}

// extractGoTypeName pulls the type name from a matched
// `type Name struct` header. Returns "" if the matched string
// doesn't fit the canonical shape.
func extractGoTypeName(matched string) string {
	// matched is like "type Service struct" or "type Foo struct".
	// Strip "type " and " struct" to get the name.
	rest := strings.TrimPrefix(matched, "type ")
	rest = strings.TrimSuffix(rest, " struct")
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.ContainsAny(rest, " \t{") {
		return ""
	}
	return rest
}

// countGoReceiverMethods counts the public-looking receiver
// methods on `name` in content. A method is `func (r *Name) X()`
// or `func (r Name) X()`. Public = first letter of X is
// uppercase (Go convention).
func countGoReceiverMethods(content, name string) int {
	// Pattern: `func (recv *Name) X(` or `func (recv Name) X(`.
	// The receiver can be named anything; we just need the type
	// to match `name` (with possible pointer prefix).
	pattern := regexp.MustCompile(`(?m)^\s*func\s*\([^)]*\*?\s*` + regexp.QuoteMeta(name) + `\s*\)\s*(\w+)\s*\(`)
	matches := pattern.FindAllStringSubmatch(content, -1)
	count := 0
	for _, m := range matches {
		methodName := m[1]
		if methodName == "" {
			continue
		}
		first := methodName[0]
		// Lowercase first letter = unexported in Go.
		if first >= 'a' && first <= 'z' {
			continue
		}
		count++
	}
	return count
}

// findBodyBounds returns the 0-indexed start and end lines of the
// block whose header is at `headerIdx`. The block is delimited by
// the FIRST `{` or `:` after the header line and the matching
// closing `}` / `end` / blank-line-after-indent-drop. The end is
// the line whose indent is <= the header's. We return -1, -1 if
// the block has no recognisable opening.
//
// The implementation is intentionally simple — it doesn't track
// string literals or nested braces perfectly; the goal is a
// fast approximation that catches the canonical "long method" case
// while accepting a small false-positive rate. The suppression
// path (task 1.20) covers the false positives.
func findBodyBounds(lines []string, headerIdx int) (int, int) {
	if headerIdx < 0 || headerIdx >= len(lines) {
		return -1, -1
	}
	// Find the opening brace (Go/JS/TS/Java/C#) or the
	// end-of-line colon (Python). Brace-based languages get a
	// brace-counting loop; Python gets a dedent-based loop.
	header := lines[headerIdx]
	openBrace := strings.LastIndex(header, "{")
	hasColon := strings.HasSuffix(strings.TrimSpace(header), ":")

	if openBrace >= 0 {
		// Brace-counting pass.
		depth := 0
		for i := headerIdx; i < len(lines); i++ {
			depth += strings.Count(lines[i], "{")
			depth -= strings.Count(lines[i], "}")
			if depth == 0 && i > headerIdx {
				return headerIdx, i
			}
		}
		return headerIdx, len(lines) - 1
	}
	if hasColon {
		// Python-style: end is the first non-blank line whose
		// indent is <= the header's. The header is at indent 0
		// (we matched `^class` / `^def` at column 0).
		// The first non-empty line inside the block is at
		// indent > 0; the block ends when we see a line at
		// indent 0 again.
		end := len(lines) - 1
		for i := headerIdx + 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if indent == 0 {
				end = i - 1
				break
			}
		}
		return headerIdx, end
	}
	// One-liner: no recognisable body delimiter. Return
	// headerIdx, headerIdx (length 1) so the threshold check
	// rejects it as "not a god method".
	return headerIdx, headerIdx
}

// countPublicMethods returns the count of public-looking method
// declarations in lines[start..end] (inclusive). Public = no
// leading underscore (Python convention) AND not lowercase-first
// (Go convention). C-style `public foo()` is always public; the
// default (no modifier) in C# is private but we treat it as
// public — the suppression path catches the false positives.
func countPublicMethods(lines []string, start, end int) int {
	count := 0
	for i := start + 1; i <= end && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		// Method-shaped: `def name(`, `func (r *R) name(`, or
		// `name(args) {`. Exclude `if (`, `for (`, `while (`,
		// `switch (`, `return (`.
		if strings.HasPrefix(line, "def ") ||
			strings.HasPrefix(line, "func (") ||
			(methodLike(line) && !controlFlowLike(line)) {
			if isPublicMethodName(line) {
				count++
			}
		}
	}
	return count
}

// methodLike reports whether line looks like a method definition
// (NAME(args) opening). It excludes lines that start with control
// flow keywords.
func methodLike(line string) bool {
	// Cheap regex-free check: line contains `(` and either
	// `func ` or matches `<word>(`. We exclude `if(`, `for(`,
	// `while(`, `switch(`, `return(`, `await(`.
	prefixes := []string{"if(", "for(", "while(", "switch(", "return(", "await(", "import(", "from(", "package("}
	lower := strings.ToLower(line)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	return strings.Contains(line, "(") && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#")
}

// controlFlowLike reports whether line starts with a control-flow
// keyword. Used as a belt to methodLike's suspenders — most
// control flow is filtered by methodLike already.
func controlFlowLike(line string) bool {
	lower := strings.ToLower(line)
	for _, kw := range []string{"if ", "for ", "while ", "switch ", "return ", "await ", "import ", "from ", "package "} {
		if strings.HasPrefix(lower, kw) {
			return true
		}
	}
	return false
}

// isPublicMethodName reports whether a method line looks public.
// Heuristic: name doesn't start with `_` (Python private) and
// doesn't start with a lowercase letter (Go unexported). C-style
// `public ` modifier is always public.
func isPublicMethodName(line string) bool {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "public ") {
		return true
	}
	if strings.HasPrefix(lower, "private ") || strings.HasPrefix(lower, "protected ") {
		return false
	}
	// Pull the first identifier-ish word.
	trimmed := strings.TrimLeft(line, " \t")
	// Skip `def ` / `func ` / `function ` keywords.
	for _, kw := range []string{"def ", "func ", "function ", "async "} {
		if strings.HasPrefix(trimmed, kw) {
			trimmed = strings.TrimSpace(trimmed[len(kw):])
			break
		}
	}
	if trimmed == "" {
		return false
	}
	first := trimmed[0]
	// Underscore-prefixed = private in Python.
	if first == '_' {
		return false
	}
	// Lowercase first letter = unexported in Go.
	if first >= 'a' && first <= 'z' {
		return false
	}
	return true
}
