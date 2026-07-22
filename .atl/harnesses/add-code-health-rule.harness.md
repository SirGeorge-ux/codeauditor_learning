# Harness: Add a Code Health Detection Rule

> **Step-by-step workflow** for adding a new rule to the code health analysis system.
> **Audience:** devs, security researchers, agents.
> **Estimated time:** 1-2 hours per rule.
> **Output:** a new rule that detects a specific issue in source files, with tests and docs.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/business-domain.md` §3.3 (Code health mode).
- [ ] You've read `codeauditor-code-health` skill.
- [ ] You know the issue you want to detect (security, performance, refactor, style, architecture).
- [ ] You have at least 10 test cases (5 positive, 5 negative).

---

## Step 1 — Design the rule

Answer these 5 questions:

1. **Category:** security, performance, refactor, style, architecture?
2. **Severity:** low, medium, high, critical?
3. **Detection:** How will you detect it? (regex, AST, LLM, external tool)
4. **Languages:** Which languages does the rule apply to? (any, or specific list)
5. **Output:** What does the issue look like? (severity, line, message, evidence, suggested_fix)

### Detection options

| Method | Pros | Contras | Cuándo usar |
|---|---|---|---|
| **Regex** | Fast, deterministic, easy to test | False positives, doesn't understand context | Simple patterns (e.g. `eval(`, `innerHTML = `) |
| **AST** | Precise, language-aware | Requires AST library per language | Complex patterns (e.g. N+1, deep nesting) |
| **LLM** | Flexible, understands context | Slow, costs tokens, non-deterministic | Subtle patterns (e.g. leaky abstractions, design issues) |
| **External tool** | Mature, battle-tested | Setup overhead | semgrep, eslint, go vet, etc. |

For most rules, start with **regex + LLM** (regex for fast path, LLM for verification).

---

## Step 2 — Choose the category

| Category | Ejemplos |
|---|---|
| **security** | SQLi, XSS, command injection, hardcoded secrets, insecure deserialization |
| **performance** | N+1 queries, O(n²) loops, memory leaks, missing indexes, unnecessary re-renders |
| **refactor** | God classes, long methods, duplicated code, feature envy, inappropriate intimacy |
| **style** | Magic numbers, poor naming, dead code, unused imports, console.log left in |
| **architecture** | Leaky abstractions, tight coupling, circular dependencies, god modules |

---

## Step 3 — Create the rule file

**File:** `backend/internal/core/services/code_health/rules/<rule_slug>.go`

```go
package rules

import (
    "context"
    "regexp"

    "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
)

// SQLInjectionRule detects SQL queries built with string concatenation.
type SQLInjectionRule struct {
    pattern *regexp.Regexp
}

func NewSQLInjectionRule() *SQLInjectionRule {
    return &SQLInjectionRule{
        pattern: regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE).*\+.*\$\{`),
    }
}

func (r *SQLInjectionRule) Name() string {
    return "sql_injection"
}

func (r *SQLInjectionRule) Category() string {
    return "security"
}

func (r *SQLInjectionRule) DefaultSeverity() string {
    return "critical"
}

func (r *SQLInjectionRule) Languages() []string {
    return []string{"typescript", "javascript", "python", "go", "ruby", "php"}
}

func (r *SQLInjectionRule) Detect(ctx context.Context, file *code_health.File) ([]code_health.Issue, error) {
    var issues []code_health.Issue

    // 1. Regex match (fast)
    matches := r.pattern.FindAllStringIndex(file.Content, -1)
    for _, match := range matches {
        line := lineNumberAt(file.Content, match[0])
        issues = append(issues, code_health.Issue{
            Severity:     r.DefaultSeverity(),
            Category:     r.Category(),
            Line:         line,
            Message:      "SQL query built with string concatenation. Possible SQL injection.",
            Evidence:     extractLine(file.Content, line),
            SuggestedFix: "Use parameterized queries (e.g. db.query('SELECT * FROM users WHERE id = $1', [id])).",
            Rule:         r.Name(),
        })
    }

    return issues, nil
}

func lineNumberAt(content string, offset int) int {
    return strings.Count(content[:offset], "\n") + 1
}

func extractLine(content string, line int) string {
    lines := strings.Split(content, "\n")
    if line-1 < len(lines) {
        return lines[line-1]
    }
    return ""
}
```

---

## Step 4 — Write the test

**File:** `backend/internal/core/services/code_health/rules/<rule_slug>_test.go`

```go
package rules

import (
    "context"
    "testing"

    "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSQLInjectionRule_Detect_Positive(t *testing.T) {
    tests := []struct {
        name    string
        code    string
        wantLen int
    }{
        {
            name: "concatenation with template literal",
            code: `
const query = \`SELECT * FROM users WHERE id = \${userId}\`;
db.execute(query);
`,
            wantLen: 1,
        },
        {
            name: "concatenation with +",
            code: `
const query = "SELECT * FROM users WHERE id = " + userId;
db.execute(query);
`,
            wantLen: 1,
        },
        // ... more positive cases ...
    }

    rule := NewSQLInjectionRule()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            file := &code_health.File{
                Path:     "test.ts",
                Language: "typescript",
                Content:  tt.code,
            }
            issues, err := rule.Detect(context.Background(), file)
            require.NoError(t, err)
            assert.Len(t, issues, tt.wantLen)
        })
    }
}

func TestSQLInjectionRule_Detect_Negative(t *testing.T) {
    tests := []struct {
        name string
        code string
    }{
        {
            name: "parameterized query",
            code: `
const query = "SELECT * FROM users WHERE id = $1";
db.execute(query, [userId]);
`,
        },
        {
            name: "no SQL",
            code: `
const x = 1 + 2;
console.log(x);
`,
        },
        // ... more negative cases ...
    }

    rule := NewSQLInjectionRule()
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            file := &code_health.File{
                Path:     "test.ts",
                Language: "typescript",
                Content:  tt.code,
            }
            issues, err := rule.Detect(context.Background(), file)
            require.NoError(t, err)
            assert.Empty(t, issues)
        })
    }
}
```

---

## Step 5 — Register the rule

**File:** `backend/internal/core/services/code_health/rules.go` (or `registry.go`)

```go
package code_health

import "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health/rules"

func NewDefaultRuleRegistry() *RuleRegistry {
    r := NewRuleRegistry()

    // ... existing rules ...

    // Add the new rule
    r.Register(rules.NewSQLInjectionRule())

    return r
}
```

---

## Step 6 — Test the rule on a real repo

1. Start the backend.
2. Go to a code health page for a real repo.
3. Click "Analyze now".
4. Verify the rule fires on a known SQL injection.
5. Verify it doesn't fire on a parameterized query.

---

## Step 7 — Document the rule

**File:** `docs/code-health-rules.md` (or add to the spec)

```markdown
## `sql_injection` rule

- **Category:** security
- **Default severity:** critical
- **Languages:** typescript, javascript, python, go, ruby, php
- **Detection:** regex (fast) + LLM (verify)
- **False positives:** if the template literal is used for a static SQL, you can ignore.

Example:

\`\`\`typescript
// 🚨 Detected
const query = \`SELECT * FROM users WHERE id = \${userId}\`;

// ✅ Not detected
const query = "SELECT * FROM users WHERE id = $1";
db.execute(query, [userId]);
\`\`\`
```

---

## Step 8 — Commit

```bash
git add backend/internal/core/services/code_health/rules/<rule_slug>.go \
        backend/internal/core/services/code_health/rules/<rule_slug>_test.go \
        backend/internal/core/services/code_health/registry.go \
        docs/code-health-rules.md

git commit -m "feat(code-health): add <rule_slug> rule

- Category: <category>
- Severity: <severity>
- Languages: <languages>
- Detection: regex + LLM
- 5+ positive tests
- 5+ negative tests
- Tested on <real repo>"
```

---

## Checklist

- [ ] 5 design questions answered.
- [ ] Detection method chosen.
- [ ] Rule file created.
- [ ] Test file with 5+ positive and 5+ negative cases.
- [ ] Rule registered.
- [ ] Tested on a real repo.
- [ ] Documented.
- [ ] Conventional commit.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Too many false positives | Tune the regex. Add LLM verification for ambiguous cases. |
| Too many false negatives | Add more test cases. Use a more permissive pattern + LLM check. |
| Rule only works for one language | Make `Languages()` configurable, or write per-language variants. |
| Severity is too high/low | Look at how other tools rate this issue. Critical = exploit + impact. High = exploit but mitigated. Medium = hard to exploit. Low = style. |
| Suggested fix is wrong | Reference the official docs for the fix. |
| Rule runs in O(n²) | Use precompiled regex, single pass. |
| Rule doesn't handle multi-line code | Use `(?s)` flag in regex or AST. |

---

## Resources

- **Spec:** `openspec/changes/code-health-dashboard/`
- **Doc:** `docs/business-domain.md` §3.3
- **Skill:** `codeauditor-code-health`
