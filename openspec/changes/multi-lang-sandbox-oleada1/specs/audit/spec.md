# Delta Spec: audit

## MODIFIED Requirements

### Requirement: Local Sandbox Executor

**MODIFIED**: Expand supported languages from 2 to 8.

For language `typescript` → run `npx eslint --format=unix --stdin`.
For language `go` → run `go vet`.
For language `python` → run `ruff check --output-format=text`.
For language `ruby` → run `rubocop --format=simple`.
For language `php` → run `php -l`.
For language `lua` → run `luacheck`.
For language `bash` → run `shellcheck`.
For language `perl` → run `perl -c`.

The sandbox MUST delegate tool selection to `ProviderRegistry.Get(language)`.
The sandbox MUST NOT contain switch statements on language.

#### Scenario: Python execution

- GIVEN a valid Python code string and `ruff` is installed
- WHEN `Execute(ctx, "python", code, 30)` is called
- THEN the sandbox MUST run `ruff check` on the code
- AND stream stdout and stderr separately

#### Scenario: Ruby execution

- GIVEN a valid Ruby code string and `rubocop` is installed
- WHEN `Execute(ctx, "ruby", code, 30)` is called
- THEN the sandbox MUST run `rubocop --format=simple`

#### Scenario: Bash execution

- GIVEN a valid Bash code string and `shellcheck` is installed
- WHEN `Execute(ctx, "bash", code, 30)` is called
- THEN the sandbox MUST run `shellcheck`

#### Scenario: Perl execution

- GIVEN a valid Perl code string
- WHEN `Execute(ctx, "perl", code, 30)` is called
- THEN the sandbox MUST run `perl -c`

#### Scenario: PHP execution

- GIVEN a valid PHP code string
- WHEN `Execute(ctx, "php", code, 30)` is called
- THEN the sandbox MUST run `php -l`

#### Scenario: Lua execution

- GIVEN a valid Lua code string and `luacheck` is installed
- WHEN `Execute(ctx, "lua", code, 30)` is called
- THEN the sandbox MUST run `luacheck`

#### Scenario: Unknown language

- GIVEN an unsupported language `"fortran"`
- WHEN `Execute(ctx, "fortran", code, 30)` is called
- THEN it MUST return an error before any process is started

---

### Requirement: Audit Domain Model

**MODIFIED**: Update language field to reflect expanded support.

The system MUST define an `AuditRequest` with: code (string), language (string — one of: typescript, javascript, go, python, ruby, php, lua, bash, perl), challengeId (string).

#### Scenario: Valid language keys accepted

- GIVEN an `AuditRequest` with `language: "python"`
- WHEN the audit handler receives it
- THEN it MUST pass the language key through to the sandbox unchanged

---

## ADDED Requirements

### Requirement: Language Key Normalization

The system MUST use canonical lowercase keys for all languages. No aliases.

#### Scenario: Shell files map to bash

- GIVEN a `.sh` file is imported via Gogs
- WHEN the language is detected
- THEN it MUST return `"bash"` (not `"shell"`)

#### Scenario: JavaScript maps to javascript

- GIVEN a `.js` file
- WHEN the language is detected
- THEN it MUST return `"javascript"` (not `"js"`)

---

### Requirement: Install Hint for Missing Tools

The sandbox healthcheck MUST report which local tools are missing with actionable install instructions.

#### Scenario: Missing tool reported

- GIVEN `ruff` is not installed on the system
- WHEN `Healthcheck(ctx)` is called on `LocalSandbox`
- THEN it MUST report `"ruff: not found. Install with: pip install ruff"`

#### Scenario: All tools available

- GIVEN all 8 local tools are installed
- WHEN `Healthcheck(ctx)` is called on `LocalSandbox`
- THEN it MUST return nil (healthy)
