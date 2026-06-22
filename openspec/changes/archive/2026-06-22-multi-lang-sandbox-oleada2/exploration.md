# Exploration: Multi-Lang Sandbox Oleada 2 — Java, Kotlin, Scala, Groovy

## Current State

The sandbox uses a clean Provider Pattern with one file per language. `LocalSandbox` and `DockerSandbox` delegate entirely to `ProviderRegistry.Get(lang)`. Each provider implements `LanguageProvider` with 6 methods. `DockerCommand(filename)` is used for **both** Docker and local execution (`buildLocalCommand` takes `args[0]` as the binary). The temp directory is mounted `:ro` inside Docker containers.

Oleada 1 has 9 providers: TypeScript, JavaScript, Go, Python, Ruby, PHP, Lua, Bash, Perl. All use single-command linting/auditing tools (`eslint`, `go vet`, `ruff check`, `php -l`, `perl -c`, etc.).

## Affected Areas

- `backend/internal/infrastructure/driven/sandbox/providers/` — 4 new provider files
- `backend/internal/infrastructure/driven/sandbox/providers/registry.go` — 4 registrations in `NewDefaultRegistry()`
- `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` — update expected language count from 9 → 13
- `backend/internal/infrastructure/driving/handlers/gogs_handler.go` — add `.groovy` to `inferLanguage()`

## Local Execution Approach

### Option A: Compilation (Recommended)

Use the compiler as a single-command syntax/type check. Fits the existing audit pattern perfectly.

| Language | Extension | Local Tool | Docker Command | Install Hint |
|----------|-----------|------------|----------------|--------------|
| Java | `.java` | `javac` | `javac -d /tmp {filename}` | SDKMAN: `sdk install java` or package manager |
| Kotlin | `.kt` | `kotlinc` | `kotlinc -d /tmp {filename}` | SDKMAN: `sdk install kotlin` |
| Scala | `.scala` | `scalac` | `scalac -d /tmp {filename}` | SDKMAN: `sdk install scala` or Coursier |
| Groovy | `.groovy` | `groovyc` | `groovyc -d /tmp {filename}` | SDKMAN: `sdk install groovy` |

**Why `-d /tmp`?** The Docker mount is `:ro`. Compilers write `.class` files; without an output dir they crash on read-only volumes. `/tmp` is already a `rw` tmpfs in `buildDockerRunArgs`.

### Option B: Execution (JEP 330 / Script Mode)

| Language | Extension | Command | Notes |
|----------|-----------|---------|-------|
| Java | `.java` | `java {filename}` | JEP 330 (Java 11+). Compiles in-memory, no `.class` files. |
| Kotlin | `.kts` | `kotlinc -script {filename}` | Requires script syntax, not standard `.kt`. |
| Scala | `.sc` | `scala {filename}` | Scala 3 script mode. Not standard `.scala`. |
| Groovy | `.groovy` | `groovy {filename}` | Runs directly, no compile step visible. |

**Problem:** Kotlin and Scala require non-standard extensions (`.kts`, `.sc`) or compound commands for standard extensions. This breaks the single-command constraint and forces `inferLanguage` to map both `.kt`/`.kts` and `.scala`/`.sc`.

## Docker Execution Approach

| Language | Image | Size (approx) | Command | Notes |
|----------|-------|---------------|---------|-------|
| Java | `eclipse-temurin:21-jdk-alpine` | ~180 MB | `javac -d /tmp code.java` | Official, Alpine, widely used. |
| Kotlin | **No official Alpine image.** | — | `kotlinc -d /tmp code.kt` | **Gotcha:** JetBrains does not publish official Docker images. Options: (1) build custom on `eclipse-temurin:21-jdk-alpine` + Kotlin zip, (2) use community image `zenika/kotlin:1.9-alpine` (if maintained), (3) accept a larger Debian-based image. |
| Scala | `scala:3.3.1-slim` | ~350 MB | `scalac -d /tmp code.scala` | Official but Debian-based (no Alpine). |
| Groovy | `groovy:4.0-jdk21-alpine` | ~200 MB | `groovyc -d /tmp code.groovy` | Official, Alpine. |

## Integration into Provider Pattern

Each provider is ~32 lines, identical structure to existing providers:

```go
type JavaProvider struct{}
func NewJavaProvider() ports.LanguageProvider { return &JavaProvider{} }
func (p *JavaProvider) Language() string      { return "java" }
func (p *JavaProvider) FileExtension() string { return ".java" }
func (p *JavaProvider) DockerImage() string   { return "eclipse-temurin:21-jdk-alpine" }
func (p *JavaProvider) DockerCommand(filename string) []string {
    return []string{"javac", "-d", "/tmp", filename}
}
func (p *JavaProvider) LocalCommand() string { return "javac" }
func (p *JavaProvider) InstallHint() string {
    return "SDKMAN: sdk install java 21-tem (or apt/brew install openjdk-21-jdk)"
}
var _ ports.LanguageProvider = (*JavaProvider)(nil)
```

Register in `registry.go`:
```go
_ = r.Register(NewJavaProvider())
_ = r.Register(NewKotlinProvider())
_ = r.Register(NewScalaProvider())
_ = r.Register(NewGroovyProvider())
```

Update `registry_test.go` `Languages()` want-slice from 9 → 13 sorted keys.

Update `gogs_handler.go` `inferLanguage()`:
```go
case "groovy":
    return "groovy"
```

## File Extension Mapping

| Extension | Language Key | Status |
|-----------|-------------|--------|
| `.java` | `java` | Already mapped |
| `.kt` | `kotlin` | Already mapped |
| `.scala` | `scala` | Already mapped |
| `.groovy` | `groovy` | **Needs addition** |

## Risks & Gotchas

1. **Read-only Docker mount + compiler output**: All JVM compilers emit `.class` files. `DockerSandbox` mounts `/code:ro`. **Mitigation:** Pass `-d /tmp` in every `DockerCommand` so bytecode lands on the `rw` tmpfs.

2. **Kotlin Docker image gap**: No official small Alpine image with Kotlin pre-installed. **Mitigation:** Build a custom image (`codeauditor/kotlin-compiler:2.0-alpine` on top of `eclipse-temurin:21-jdk-alpine`) or use `zenika/kotlin` community image with a pinned tag.

3. **Scala image size**: `scala:3.3.1-slim` is ~350 MB (Debian-based), the largest in the registry by far. **Mitigation:** Accept the size or build a custom Alpine image with Scala and sbt/Coursier.

4. **JVM startup overhead**: `javac`, `kotlinc`, `scalac` have noticeable cold-start time. Large single files or complex type-checking may approach the default 30s timeout. **Mitigation:** Monitor; increase timeout if needed, or document for users.

5. **Classpath / external deps**: Single-file compilation fails if the code imports libraries not on the default classpath. This is consistent with the existing sandbox (no dependency resolution), but JVM users may expect Maven/Gradle support. **Mitigation:** Document limitation; out of scope for Oleada 2.

6. **Kotlin/Scala standard vs script extensions**: If the team later wants execution (not compilation), `.kt` files would need `kotlinc` + `kotlin` two-step, which violates the single-command `DockerCommand` contract. **Mitigation:** Stick to compilation for Oleada 2; revisit interface extension if execution is needed later.

## Approaches Summary

| Approach | Pros | Cons | Complexity |
|----------|------|------|------------|
| **A — Compilation (recommended)** | Fits existing audit pattern; single command; standard extensions; no interface changes. | Only validates syntax/types, not style. | Low |
| **B — Execution** | Aligns with user's "run a single-file program" curiosity; JEP 330 is elegant for Java. | Kotlin/Scala require script extensions or compound commands; `gogs_handler` needs more mappings; read-only mount issues for Kotlin/Scala compiled output. | Medium |
| **C — Full linter suite** | Richer feedback (style, best practices). | Requires additional tools (checkstyle, ktlint, scalafmt, codenarc); more complex Docker images; higher maintenance. | High |

## Recommendation

**Use Approach A (compilation)** for Oleada 2. It respects the existing provider contract, requires no interface changes, uses standard file extensions, and adds 4 languages with ~128 lines of Go code plus registry wiring. The read-only mount is handled cleanly with `-d /tmp`. Kotlin's missing Docker image is the only significant blocker — resolve with a custom image or a maintained community pin.

## Ready for Proposal

**Yes.** The orchestrator should confirm:
1. Accept compilation (`javac`, `kotlinc`, `scalac`, `groovyc`) as the audit strategy for Oleada 2.
2. Preferred resolution for Kotlin Docker image (custom build vs community image).
3. Whether to build a custom Scala-on-Alpine image to avoid the 350 MB Debian image.
