# Sandbox Provider Registry Specification

## Purpose

LanguageProvider interface, ProviderRegistry, and per-language providers that replace inline switch statements in the sandbox, enabling scalable multi-language support.

## Requirements

### Requirement: LanguageProvider Interface

The system MUST define a `LanguageProvider` interface in `ports/` with the following contract.

| Method | Returns | Purpose |
|--------|---------|---------|
| `Language()` | `string` | Canonical language key (e.g., "python", "ruby") |
| `FileExtension()` | `string` | Source file extension with dot (e.g., ".py", ".rb") |
| `DockerImage()` | `string` | Docker image with tag (e.g., "python:3.12-alpine") |
| `DockerCommand(filename string)` | `[]string` | Command and args for Docker execution |
| `LocalCommand()` | `string` | Tool name for healthcheck (e.g., "ruff", "shellcheck") |
| `InstallHint()` | `string` | Human-readable install suggestion (e.g., "pip install ruff") |

#### Scenario: Provider returns canonical key

- GIVEN a `PythonProvider`
- WHEN `Language()` is called
- THEN it MUST return `"python"`

#### Scenario: Provider returns correct extension

- GIVEN a `RubyProvider`
- WHEN `FileExtension()` is called
- THEN it MUST return `".rb"`

#### Scenario: Docker command includes filename

- GIVEN a `BashProvider`
- WHEN `DockerCommand("code.sh")` is called
- THEN it MUST return `["shellcheck", "code.sh"]`

#### Scenario: Install hint is actionable

- GIVEN a `PerlProvider` and `perl` is not installed
- WHEN `InstallHint()` is called
- THEN it MUST return a string the user can paste into a terminal (e.g., "Install Perl: https://www.perl.org/get.html")

---

### Requirement: ProviderRegistry

The system MUST provide a `ProviderRegistry` that maps language keys to `LanguageProvider` implementations.

#### Scenario: Get known language

- GIVEN providers for `python`, `ruby`, `php`, `lua`, `bash`, `perl`, `typescript`, `javascript`, `go` are registered
- WHEN `registry.Get("python")` is called
- THEN it MUST return the `PythonProvider`

#### Scenario: Get unknown language

- GIVEN no provider for `"cobol"` is registered
- WHEN `registry.Get("cobol")` is called
- THEN it MUST return an error indicating the language is unsupported

#### Scenario: Duplicate registration

- GIVEN a `PythonProvider` is already registered
- WHEN another provider with key `"python"` is registered
- THEN it MUST return an error or panic (config-time error)

#### Scenario: List all supported languages

- GIVEN 8 providers are registered
- WHEN `registry.Languages()` is called
- THEN it MUST return all 8 language keys

---

### Requirement: Per-Language Provider Files

The system MUST have one provider file per language under `infrastructure/driven/sandbox/providers/`.

| Language | File | Docker Image | Docker Command | Local Tool |
|----------|------|-------------|----------------|------------|
| python | `python.go` | `python:3.12-alpine` | `ruff check --output-format=text <file>` | `ruff` |
| ruby | `ruby.go` | `ruby:3.3-alpine` | `rubocop --format=simple <file>` | `rubocop` |
| php | `php.go` | `php:8.3-cli-alpine` | `php -l <file>` | `php` |
| lua | `lua.go` | `nickblah/luacheck:latest` | `luacheck <file>` | `luacheck` |
| bash | `bash.go` | `koalaman/shellcheck-alpine:latest` | `shellcheck <file>` | `shellcheck` |
| perl | `perl.go` | `perl:5.38-slim` | `perl -c <file>` | `perl` |
| typescript | `typescript.go` | `node:22-alpine` | `npx eslint --format=unix <file>` | `npx` |
| go | `go.go` | `golang:1.23-alpine` | `go vet ./...` | `go` |

#### Scenario: All providers expose correct file extension

- GIVEN each provider file
- WHEN `FileExtension()` is called
- THEN Python MUST return `".py"`, Ruby `".rb"`, PHP `".php"`, Lua `".lua"`, Bash `".sh"`, Perl `".pl"`, TypeScript `".ts"`, Go `".go"`

#### Scenario: Docker images are pinned

- GIVEN any provider
- WHEN `DockerImage()` is called
- THEN it MUST return a tagged image (never `:latest` except for tool-specific images like `koalaman/shellcheck-alpine`)

---

### Requirement: Sandbox Integration

Both `LocalSandbox` and `DockerSandbox` MUST delegate language-specific behavior to `ProviderRegistry.Get(lang)` instead of using switch statements.

#### Scenario: LocalSandbox delegates to registry

- GIVEN a `LocalSandbox` with a populated `ProviderRegistry`
- WHEN `Execute(ctx, "python", code, 30)` is called
- THEN it MUST call `registry.Get("python")` to get the provider
- AND it MUST NOT contain a switch statement on language

#### Scenario: DockerSandbox delegates to registry

- GIVEN a `DockerSandbox` with a populated `ProviderRegistry`
- WHEN `Execute(ctx, "ruby", code, 30)` is called
- THEN it MUST call `registry.Get("ruby")` to get the provider
- AND it MUST NOT contain a switch statement on language

#### Scenario: Unknown language rejects early

- GIVEN a sandbox with a populated `ProviderRegistry`
- WHEN `Execute(ctx, "fortran", code, 30)` is called
- THEN it MUST return an error before any temp directory or process is created

---

### Requirement: Healthcheck Per-Provider

DockerSandbox healthcheck MUST iterate all registered providers and verify each Docker image is available.

#### Scenario: Docker healthcheck with configurable timeout

- GIVEN a `DockerSandbox` with 8 providers
- WHEN `Healthcheck(ctx)` is called
- THEN it MUST check `docker info`
- AND for each provider, check or pull its image
- AND per-image timeout MUST be configurable via constructor

#### Scenario: LocalSandbox healthcheck reports missing tools

- GIVEN a `LocalSandbox` and `rubocop` is not installed
- WHEN `Healthcheck(ctx)` is called
- THEN it MUST report that `rubocop` is missing
- AND it MUST include `InstallHint()` in the report
