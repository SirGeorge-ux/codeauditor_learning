---
name: codeauditor-add-language
description: Use this skill when an agent needs to add support for a new language in the CodeAuditor sandbox. Trigger phrases include "add language", "nuevo lenguaje", "support for X", "X language provider", "soporte para X", "añadir Kotlin", "add Rust". Loads the LanguageProvider pattern, the registry, and the file checklist.
---

# Add a Language to the CodeAuditor Sandbox

> **Outcome:** a new language supported in `LocalSandbox` + `DockerSandbox`, registered, tested, and ready to use from the frontend without further changes.
> **Time estimate:** 15-30 minutes.
> **PR size:** 2 files + 1 line edit.

## When to load

Load this skill when the user asks:
- "Add support for X language"
- "Implementar el provider de X"
- "Quiero que el dojo acepte X"
- "Sigo el workflow de añadir un lenguaje"

If the language is already supported (see `docs/sandbox-providers.md` §8), tell the user and stop.

## Pre-flight

1. Read `AGENTS.md` (root) if you haven't.
2. Read `docs/sandbox-providers.md` (it's short).
3. Skim one existing provider: `backend/internal/infrastructure/driven/sandbox/providers/python.go` is a good reference.

## Step-by-step

### Step 1 — Create the provider

Create `backend/internal/infrastructure/driven/sandbox/providers/<lang>.go`:

```go
package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

type <Lang>Provider struct{}

func New<Lang>Provider() *<Lang>Provider { return &<Lang>Provider{} }

func (p *<Lang>Provider) Language() string     { return "<lang>" }
func (p *<Lang>Provider) FileExtension() string { return ".<ext>" }
func (p *<Lang>Provider) DockerImage() string   { return "<image>:<version>-alpine" }
func (p *<Lang>Provider) LocalCommand() string  { return "<cmd>" }
func (p *<Lang>Provider) InstallHint() string   { return "<hint>" }

func (p *<Lang>Provider) DockerCommand(filename string) []string {
    return []string{"<cmd>", filename}
}

// Compile-time check that we implement the port
var _ ports.LanguageProvider = (*<Lang>Provider)(nil)
```

**Decisions to make:**
- **Image:** prefer Alpine-based, never `:latest`. Tag a specific version.
- **Local command:** the linter/interpreter in PATH (e.g. `python3`, `eslint`, `shellcheck`).
- **Install hint:** actionable (URL or command).

### Step 2 — Write the test

Create `backend/internal/infrastructure/driven/sandbox/providers/<lang>_test.go`:

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

### Step 3 — Register the provider

Edit `backend/internal/infrastructure/driven/sandbox/providers/registry.go`:

Add inside `NewDefaultRegistry()`:

```go
_ = r.Register(New<Lang>Provider())
```

Place it in the right group (scripting, JVM, systems, web+SQL, functional, crypto+niche+BEAM, legacy) with a comment.

### Step 4 — Update the port comment (optional)

In `backend/internal/ports/sandbox.go`, the `SandboxExecutor` interface has a doc comment listing supported languages. Update it:

```go
// language must be one of: "typescript", "javascript", "go", "python",
// "ruby", "php", "lua", "bash", "perl", "kotlin", "<lang>", ...
```

### Step 5 — Update the language inferrer

If files from Gogs can be imported as this language, add the extension mapping in `backend/internal/infrastructure/driving/handlers/gogs_handler.go`:

```go
func inferLanguage(path string) string {
    switch strings.ToLower(filepath.Ext(path)) {
    // ...
    case ".<ext>":
        return "<lang>"
    }
}
```

### Step 6 — Run tests

```bash
cd backend
go test ./internal/infrastructure/driven/sandbox/providers/...
go test -short ./...
```

All should pass. If you broke something, **fix it before pushing**.

### Step 7 — Open a PR (or commit if solo)

- Title: `feat(sandbox): add <lang> provider`
- Body: list the 4 changes (provider, test, registry, inferrer)
- Mention the image used and the size in MB (the project tracks lightweight).

## Don't do

- ❌ Don't add a `switch` in `LocalSandbox` or `DockerSandbox`.
- ❌ Don't use `:latest` tag.
- ❌ Don't use a non-Alpine image (>300 MB).
- ❌ Don't forget the compile-time check `var _ ports.LanguageProvider = ...`.
- ❌ Don't write the provider in a different file pattern (`<lang>_provider.go` → no, use `<lang>.go`).
- ❌ Don't add it to the registry twice (would panic on Register).

## Success criteria

- [ ] Provider file exists and compiles.
- [ ] Test file exists and passes.
- [ ] Registered in `NewDefaultRegistry()`.
- [ ] `make test-backend` passes.
- [ ] `go test -short ./...` passes (no Docker daemon required).
- [ ] Frontend accepts the new language without changes.

## Quick reference: existing providers

```bash
ls backend/internal/infrastructure/driven/sandbox/providers/*.go | grep -v _test
```

To find the most similar one to copy:
- Scripting: `python.go`, `ruby.go`, `lua.go`
- Systems: `go.go`, `rust.go`, `c.go`
- Web+SQL: `python.go` is closest to a generic interpreter
- Functional: `haskell.go`
