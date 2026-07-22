# Harness: Add a New Language to the Sandbox

> **Step-by-step workflow** for adding a new language to the CodeAuditor sandbox.
> **Audience:** devs, agents.
> **Estimated time:** 15-30 minutes.
> **Output:** a new language supported in `LocalSandbox` + `DockerSandbox`, with a passing test.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/sandbox-providers.md` §1-5.
- [ ] You have Go 1.23+ installed and `go test` working.
- [ ] The language is **not** already supported (check `backend/internal/infrastructure/driven/sandbox/providers/`).
- [ ] You have an Alpine-based Docker image in mind (<300 MB preferred).

---

## Step 1 — Open a spec (if material)

**Skip this step** if:
- The language is simple and <50 lines of code.
- You're the only one working on it.

**Open a spec** if:
- The language requires a special image (>300 MB).
- The local command needs to be installed (e.g. `brew install kotlin`).
- The detection of the language from file extensions affects multiple places.

```bash
mkdir -p openspec/changes/add-<lang>-provider/specs
cp openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/*.md \
   openspec/changes/add-<lang>-provider/
$EDITOR openspec/changes/add-<lang>-provider/proposal.md
$EDITOR openspec/changes/add-<lang>-provider/design.md
$EDITOR openspec/changes/add-<lang>-provider/tasks.md
```

In `tasks.md`, copy the structure from `add-language.template.md` below.

### `add-language.template.md` (paste in tasks.md)

```markdown
## Phase 1: Provider Implementation
- [ ] 1.1 Create `backend/internal/infrastructure/driven/sandbox/providers/<lang>.go` with the provider struct + 6 methods
- [ ] 1.2 Add compile-time check `var _ ports.LanguageProvider = (*<Lang>Provider)(nil)`
- [ ] 1.3 Create `backend/internal/infrastructure/driven/sandbox/providers/<lang>_test.go` with the standard test

## Phase 2: Registry
- [ ] 2.1 Add `_ = r.Register(New<Lang>Provider())` in `NewDefaultRegistry()` in the right group

## Phase 3: Port comment
- [ ] 3.1 Update supported languages list in `backend/internal/ports/sandbox.go` doc comment

## Phase 4: Language inference (if imported from Gogs)
- [ ] 4.1 Add extension mapping in `backend/internal/infrastructure/driving/handlers/gogs_handler.go::inferLanguage()`

## Phase 5: Verification
- [ ] 5.1 `cd backend && go test ./internal/infrastructure/driven/sandbox/providers/...` — passes
- [ ] 5.2 `cd backend && go test -short ./...` — no regression
- [ ] 5.3 `cd backend && make validate` — passes
- [ ] 5.4 (Optional) Manual: import a `<file>.<ext>` from Gogs in `/mcp`, run audit in `/dojo`
```

---

## Step 2 — Create the provider file

**File:** `backend/internal/infrastructure/driven/sandbox/providers/<lang>.go`

```go
package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

type <Lang>Provider struct{}

func New<Lang>Provider() *<Lang>Provider {
    return &<Lang>Provider{}
}

func (p *<Lang>Provider) Language() string      { return "<lang>" }
func (p *<Lang>Provider) FileExtension() string { return ".<ext>" }
func (p *<Lang>Provider) DockerImage() string   { return "<image>:<version>-alpine" }
func (p *<Lang>Provider) LocalCommand() string  { return "<cmd>" }
func (p *<Lang>Provider) InstallHint() string   { return "<install-hint>" }

func (p *<Lang>Provider) DockerCommand(filename string) []string {
    return []string{"<cmd>", filename}
}

// Compile-time check that the provider implements the port
var _ ports.LanguageProvider = (*<Lang>Provider)(nil)
```

### Decision matrix

| Question | How to decide |
|---|---|
| Image | Search Docker Hub for `<lang>` + `alpine`. Prefer official images. Tag a specific version (no `:latest`). |
| LocalCommand | The linter/interpreter in PATH. `python3`, `go`, `node`, `eslint`, `shellcheck`, `kotlinc`, `ghc`, etc. |
| InstallHint | URL or command. Should be copy-pasteable. |
| DockerCommand argv | Whatever you'd type in the shell to run the file: `python3 main.py`, `node main.js`, `eslint main.js`. |
| FileExtension | With dot: `.py`, `.ts`, `.go`, `.sh`, etc. |

---

## Step 3 — Create the test file

**File:** `backend/internal/infrastructure/driven/sandbox/providers/<lang>_test.go`

```go
package providers

import "testing"

func Test<Lang>Provider(t *testing.T) {
    p := New<Lang>Provider()

    if got, want := p.Language(), "<lang>"; got != want {
        t.Errorf("Language() = %q, want %q", got, want)
    }
    if got, want := p.FileExtension(), ".<ext>"; got != want {
        t.Errorf("FileExtension() = %q, want %q", got, want)
    }
    if p.DockerImage() == "" {
        t.Error("DockerImage() is empty")
    }
    if p.LocalCommand() == "" {
        t.Error("LocalCommand() is empty")
    }
    if p.InstallHint() == "" {
        t.Error("InstallHint() is empty")
    }
    cmd := p.DockerCommand("code.<ext>")
    if len(cmd) < 2 {
        t.Errorf("DockerCommand() too short: %v", cmd)
    }
    if cmd[len(cmd)-1] != "code.<ext>" {
        t.Errorf("DockerCommand() doesn't end with filename: %v", cmd)
    }
}
```

---

## Step 4 — Register the provider

**File:** `backend/internal/infrastructure/driven/sandbox/providers/registry.go`

Inside `NewDefaultRegistry()`, add at the right group:

```go
// <group comment>
_ = r.Register(New<Lang>Provider())
```

Groups (current):
- Existing (extracted from old switch): TypeScript, JavaScript, Go.
- Scripting (Oleada 1): Python, Ruby, PHP, Lua, Bash, Perl.
- JVM (Oleada 2): Java, Kotlin, Scala, Groovy.
- Systems (Oleada 3): Rust, C, C++, Zig.
- Web+SQL (Oleada 4): HTML, CSS, XML, JSON, YAML, SQL.
- Functional/.NET/Data/Apple (Oleada 5): C#, Swift, Haskell, Elixir, Clojure, R.
- Crypto/Niche/BEAM (Oleada 6): Solidity, Erlang, Dart, Julia, Nim.
- Legacy (Oleada 7): Cobol, PowerShell, Racket.

---

## Step 5 — Update the port comment

**File:** `backend/internal/ports/sandbox.go`

```go
// SandboxExecutor defines the contract for executing code in an isolated sandbox.
// It is a driven (secondary) port — the application use cases call it.
type SandboxExecutor interface {
    // Execute runs the given code snippet in an isolated environment.
    // language must be one of: "typescript", "javascript", "go", "python",
    // "ruby", "php", "lua", "bash", "perl", "<lang>", ...
    // stdout and stderr are streamed back via the returned ReadCloser.
    Execute(...)
}
```

---

## Step 6 — Update language inference (if needed)

**File:** `backend/internal/infrastructure/driving/handlers/gogs_handler.go`

In `inferLanguage()`:

```go
case ".<ext>":
    return "<lang>"
```

(Only if files of this type are commonly imported from Gogs. Skip for niche languages like Cobol.)

---

## Step 7 — Run the tests

```bash
cd backend

# 1. Specific test
go test -v -run Test<Lang>Provider ./internal/infrastructure/driven/sandbox/providers/...

# 2. All provider tests
go test -v ./internal/infrastructure/driven/sandbox/providers/...

# 3. All tests (short mode, no Docker daemon required)
go test -short ./...

# 4. Full quality gate
make validate
```

All must pass.

---

## Step 8 — Manual verification (optional)

1. Start the backend.
2. Open `/mcp` in the frontend.
3. Import a file with extension `.<ext>`.
4. Open the Dojo.
5. Click "Run Audit".
6. Verify the sandbox executes and returns output.

---

## Step 9 — Commit and PR

```bash
git add backend/internal/infrastructure/driven/sandbox/providers/<lang>.go \
        backend/internal/infrastructure/driven/sandbox/providers/<lang>_test.go \
        backend/internal/infrastructure/driven/sandbox/providers/registry.go \
        backend/internal/infrastructure/driven/sandbox/providers/ports/sandbox.go \
        backend/internal/infrastructure/driving/handlers/gogs_handler.go

git commit -m "feat(sandbox): add <lang> provider

- New LanguageProvider for <lang>
- Image: <image>
- Local command: <cmd>
- Tests pass: go test -short ./..."

git push origin feature/add-<lang>-provider
gh pr create --title "feat(sandbox): add <lang> provider" \
             --body "Adds support for <lang> in the sandbox. See openspec/changes/add-<lang>-provider/ for context."
```

---

## Checklist

- [ ] Provider file created (~30 lines).
- [ ] Test file created (~20 lines, table-driven if needed).
- [ ] Registered in `NewDefaultRegistry()`.
- [ ] Port comment updated.
- [ ] Language inference updated (if applicable).
- [ ] `go test -short ./...` passes.
- [ ] `make validate` passes.
- [ ] (If material) Spec in `openspec/changes/add-<lang>-provider/`.
- [ ] Commit message in conventional format.
- [ ] PR opened with clear title and body.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Used `:latest` tag | Use specific version (`python:3.12-alpine`, not `python:latest`). |
| Image too big | Check image size on Docker Hub. >500 MB is a red flag. |
| Forgot compile-time check | Add `var _ ports.LanguageProvider = (*<Lang>Provider)(nil)` at the bottom. |
| Local command not in PATH | Document the install hint clearly. Tests don't need it. |
| DockerCommand doesn't include filename | The filename MUST be the last argument (or a flag value like `eslint <file>`). |
| Registered twice | Each language can only be registered once. Check before adding. |
| Forgot to update port comment | Tests will catch it (your provider exists but the comment lies). |

---

## Reference: existing providers to copy

```bash
# Closest matches for common new languages
backend/internal/infrastructure/driven/sandbox/providers/python.go    # Scripting
backend/internal/infrastructure/driven/sandbox/providers/typescript.go # JS-like
backend/internal/infrastructure/driven/sandbox/providers/go.go        # Compiled
backend/internal/infrastructure/driven/sandbox/providers/rust.go      # Compiled + tools
backend/internal/infrastructure/driven/sandbox/providers/bash.go      # Shell-like
```

Pick the one closest in paradigm to your new language and copy-modify.
