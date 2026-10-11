package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// runRule is a small driver that invokes a rule on the supplied
// content and returns the resulting issues. Centralises the
// per-test boilerplate (File assembly + Detect call) so the
// table-driven tests below stay readable. The language parameter
// exists because LeakyAbstractionRule is gated on "go" — every
// other rule is language-agnostic and accepts "typescript" as the
// default-ish (it's the language the test data looks like).
func runRule(t *testing.T, rule Rule, path, content string) []models.CodeHealthIssue {
	t.Helper()
	return runRuleLang(t, rule, path, "typescript", content)
}

func runRuleLang(t *testing.T, rule Rule, path, lang, content string) []models.CodeHealthIssue {
	t.Helper()
	issues, err := rule.Detect(context.Background(), File{Path: path, Language: lang, Content: content, SHA: "sha-test"})
	require.NoError(t, err)
	return issues
}

// --- SecretRule (RULE-SEC-SECRET-001) ---

func TestSecretRule_Positive(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"api_key", `const api_key = "abc123def456";`},
		{"password", `password = "supersecret";`},
		{"token", `token: "ghp_abc123def456ghi";`},
		{"secret", `SECRET = "very-secret-value-99";`},
		{"apikey no underscore", `apikey="1234567890abcdef";`},
		{"mixed case", `MyApiKey = "Z9y8x7w6v5u4";`},
	}
	rule := NewSecretRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Len(t, issues, 1, "expected 1 issue, got %d", len(issues))
			if len(issues) == 1 {
				assert.Equal(t, "RULE-SEC-SECRET-001", issues[0].Rule)
				assert.Equal(t, models.SeverityCritical, issues[0].Severity)
				assert.Equal(t, models.CategorySecurity, issues[0].Category)
				assert.Equal(t, 1.0, issues[0].Confidence)
			}
		})
	}
}

func TestSecretRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"empty quotes", `password = "";`},
		{"short value", `password = "x";`},
		{"no secret keyword", `const name = "John";`},
		{"comment", `// api_key = "literal"`},
		{"number only", `const port = 8080;`},
		{"function call", `getSecret();`},
		{"import", `import { secret } from "lib";`},
	}
	rule := NewSecretRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Empty(t, issues, "expected no issues, got %d", len(issues))
		})
	}
}

// --- InjectionRule (RULE-SEC-INJECT-001) ---

func TestInjectionRule_Positive(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"js eval", `const r = eval(userInput);`},
		{"python eval", `result = eval(expr)`},
		{"python exec", `code = exec("print('hi')")`},
		{"node child_process exec", `child_process.exec("ls " + userInput);`},
		{"sql concat select", `const q = "select * from users where id = " + id;`},
		{"sql template literal", "const q = `select * from users where id = ${id}`;"},
		{"sql insert interpolation", "const q = `insert into logs values (${data})`;"},
	}
	rule := NewInjectionRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.NotEmpty(t, issues, "expected at least 1 issue, got 0")
			for _, iss := range issues {
				assert.Equal(t, "RULE-SEC-INJECT-001", iss.Rule)
				assert.Equal(t, models.SeverityCritical, iss.Severity)
			}
		})
	}
}

func TestInjectionRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"parameterized sql", `db.query("select * from users where id = $1", [id])`},
		{"plain string", `const greeting = "hello world";`},
		{"for loop", `for (let i = 0; i < 10; i++) { ... }`},
		{"comment", `// avoid eval(userInput)`},
		{"object property", `const obj = { evaluate: true };`},
		{"import", `import { exec } from "child_process";`},
		{"function decl", `function evaluate(x) { return x; }`},
	}
	rule := NewInjectionRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Empty(t, issues, "expected no issues, got %d for %q", len(issues), c.content)
		})
	}
}

// --- NPlusOneRule (RULE-PERF-N1-001) ---

func TestNPlusOneRule_Positive(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			"for loop with db.Query",
			"for (let id of ids) {\n  db.Query(\"select * from x where id = ?\", [id]);\n}\n",
		},
		{
			"while loop with .Find",
			"while (queue.length > 0) {\n  const row = repo.Find(queue.pop());\n}\n",
		},
		{
			"forEach with await db.",
			"items.forEach(async (item) => {\n  await db.query('select * from x');\n});\n",
		},
		{
			"for with .Get",
			"for (const u of users) {\n  const p = await Profile.Get(u.id);\n}\n",
		},
		{
			"for with repository.find",
			"for (const id of ids) {\n  const r = repository.find(id);\n}\n",
		},
	}
	rule := NewNPlusOneRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Len(t, issues, 1, "expected 1 issue, got %d", len(issues))
			if len(issues) == 1 {
				assert.Equal(t, "RULE-PERF-N1-001", issues[0].Rule)
				assert.Equal(t, models.SeverityHigh, issues[0].Severity)
				assert.Equal(t, models.CategoryPerformance, issues[0].Category)
			}
		})
	}
}

func TestNPlusOneRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"for without db", "for (let i = 0; i < 10; i++) { console.log(i); }\n"},
		{"while without db", "while (queue.length) { queue.pop(); }\n"},
		{"plain function", "function fetchAll() { return db.query('select *'); }\n"},
		{"if branch", "if (cond) { db.Query('select 1'); }\n"},
		{"const assignment", "const total = ids.length;\n"},
		{"class method", "class S { findAll() { return db.query('select 1'); } }\n"},
	}
	rule := NewNPlusOneRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Empty(t, issues)
		})
	}
}

// --- GodMethodRule (RULE-REFAC-GOD-001) ---

func TestGodMethodRule_Positive_LongMethod(t *testing.T) {
	// Build a 60-line function body inside a brace block.
	body := "function doWork() {\n"
	for i := 0; i < 60; i++ {
		body += "  console.log('line " + intToStr(i) + "');\n"
	}
	body += "}\n"
	rule := NewGodMethodRule()
	issues := runRule(t, rule, "app.ts", body)
	require.NotEmpty(t, issues, "expected god method hit on 60-line function")
	found := false
	for _, iss := range issues {
		if iss.Rule == "RULE-REFAC-GOD-001" && strings.Contains(iss.Message, "exceeds 50 lines") {
			found = true
		}
	}
	assert.True(t, found, "expected a long-method issue, got %+v", issues)
}

func TestGodMethodRule_Positive_GodClass(t *testing.T) {
	// Build a class with 12 public methods (over the 10 threshold).
	body := "class TooBig {\n"
	for i := 0; i < 12; i++ {
		body += "  public method" + intToStr(i) + "() { return " + intToStr(i) + "; }\n"
	}
	body += "}\n"
	rule := NewGodMethodRule()
	issues := runRule(t, rule, "app.ts", body)
	require.NotEmpty(t, issues, "expected god class hit on 12-method class")
	found := false
	for _, iss := range issues {
		if iss.Rule == "RULE-REFAC-GOD-001" && strings.Contains(iss.Message, "more than 10 public methods") {
			found = true
		}
	}
	assert.True(t, found, "expected a god-class issue, got %+v", issues)
}

func TestGodMethodRule_Positive_GoLongFunc(t *testing.T) {
	// Go flavor — receiver method.
	body := "func (s *Service) Do() {\n"
	for i := 0; i < 60; i++ {
		body += "\tlog.Println(\"line\")\n"
	}
	body += "}\n"
	rule := NewGodMethodRule()
	issues := runRule(t, rule, "app.go", body)
	require.NotEmpty(t, issues, "expected god method hit on 60-line Go func")
}

func TestGodMethodRule_Positive_GoGodStruct(t *testing.T) {
	// Multi-line struct with 12 methods declared INSIDE the body
	// (Go syntax — the `func (s *Service) X()` form is what the
	// rule looks for). The classHeaderRe matches `type Name struct`,
	// findBodyBounds locates the closing `}` at indent 0, and
	// countPublicMethods counts receiver methods inside.
	body := "type Service struct {\n\tname string\n}\n\n"
	body += "func (s *Service) A() {}\n"
	for i := 0; i < 11; i++ {
		body += "func (s *Service) M" + intToStr(i) + "() {}\n"
	}
	rule := NewGodMethodRule()
	issues := runRule(t, rule, "app.go", body)
	require.NotEmpty(t, issues, "expected god struct hit on multi-line struct with 12 methods")
	found := false
	for _, iss := range issues {
		if iss.Rule == "RULE-REFAC-GOD-001" && strings.Contains(iss.Message, "more than 10 public methods") {
			found = true
		}
	}
	assert.True(t, found, "expected a god-struct issue, got %+v", issues)
}

func TestGodMethodRule_Positive_PythonLongDef(t *testing.T) {
	// Python flavor — `def` with colon, body measured by indent.
	body := "def do_work():\n"
	for i := 0; i < 55; i++ {
		body += "    print('line " + intToStr(i) + "')\n"
	}
	rule := NewGodMethodRule()
	issues := runRule(t, rule, "app.py", body)
	require.NotEmpty(t, issues, "expected god method hit on 55-line Python def")
}

func TestGodMethodRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			"short ts function",
			"function short() {\n  return 1;\n}\n",
		},
		{
			"short ts class",
			"class Small {\n  a() { return 1; }\n  b() { return 2; }\n}\n",
		},
		{
			"no functions",
			"const a = 1;\nconst b = 2;\n",
		},
		{
			"comment",
			"// function do() { ... 60 lines ... }\n",
		},
		{
			"plain script",
			"console.log('hi');\nconsole.log('bye');\n",
		},
		{
			"imports only",
			"import { foo } from 'bar';\nimport { baz } from 'qux';\n",
		},
	}
	rule := NewGodMethodRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Empty(t, issues, "expected no god-method hits, got %d", len(issues))
		})
	}
}

// --- MagicNumberRule (RULE-STYLE-MAGIC-001) ---

func TestMagicNumberRule_Positive(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"bare 3-digit", "wait = 100;"},
		{"in function call", "setTimeout(callback, 5000);"},
		{"in arithmetic", "total = price * 150;"},
		{"in condition", "if (count > 999) { ... }"},
		{"big literal", "big = 1234567890;"},
		{"in array", "arr = [100, 200, 300];"},
	}
	rule := NewMagicNumberRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.NotEmpty(t, issues, "expected magic number issue")
			for _, iss := range issues {
				assert.Equal(t, "RULE-STYLE-MAGIC-001", iss.Rule)
				assert.Equal(t, models.SeverityLow, iss.Severity)
				assert.Equal(t, models.CategoryStyle, iss.Category)
			}
		})
	}
}

func TestMagicNumberRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"const declaration", "const MAX_RETRIES = 100;"},
		{"final declaration", "final int MAX = 5000;"},
		{"readonly", "readonly int PORT = 8080;"},
		{"enum", "enum Sizes { Small = 100, Medium = 200, Large = 300 }"},
		{"i18n key", `I18n.t("error.code.500")`},
		{"no number", "const greeting = 'hello';"},
		{"1-digit", "const i = 5;"},
		{"2-digit", "const year = 99;"},
	}
	rule := NewMagicNumberRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRule(t, rule, "app.ts", c.content)
			assert.Empty(t, issues)
		})
	}
}

// --- LeakyAbstractionRule (RULE-ARCH-LEAK-001) ---

func TestLeakyAbstractionRule_Positive_Go(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"single import", `import "github.com/foo/bar/internal/baz"`},
		{"grouped import", "import (\n\t\"some/lib/internal/util\"\n)"},
		{"nested path", `import "github.com/x/y/internal/sub/deep"`},
		{"alt module", `import "gitlab.com/team/proj/internal/api"`},
		{"go module with version", `import "github.com/x/y/internal/z/pkg/v2"`},
	}
	rule := NewLeakyAbstractionRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := runRuleLang(t, rule, "app.go", "go", c.content)
			assert.Len(t, issues, 1, "expected 1 issue, got %d", len(issues))
			if len(issues) == 1 {
				assert.Equal(t, "RULE-ARCH-LEAK-001", issues[0].Rule)
				assert.Equal(t, models.SeverityHigh, issues[0].Severity)
				assert.Equal(t, models.CategoryArchitecture, issues[0].Category)
			}
		})
	}
}

func TestLeakyAbstractionRule_Negative(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"no internal", `import "github.com/foo/bar/pkg/baz"`},
		{"stdlib", `import "fmt"`},
		{"non-Go file", `import { thing } from "foo/internal/bar";`},
		{"comment", `// import "github.com/x/y/internal/z"`},
		{"string literal", `const p = "github.com/x/y/internal/z";`},
		{"empty", ``},
	}
	rule := NewLeakyAbstractionRule()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Language gate keeps the rule silent on TS/JS files.
			issues := runRuleLang(t, rule, "app.ts", "typescript", c.content)
			assert.Empty(t, issues, "expected no issues for non-Go file, got %d", len(issues))
		})
	}
}

// intToStr is a tiny no-allocation int-to-string helper for the
// test data above. Avoids the strconv.Itoa import which would
// require a separate import block at the top of the file.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
