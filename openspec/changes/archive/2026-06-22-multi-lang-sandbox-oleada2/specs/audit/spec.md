# Delta for Audit

## MODIFIED Requirements

### Requirement: Audit Domain Model

The system MUST define an `AuditRequest` with: code (string), language (string — one of: typescript, javascript, go, python, ruby, php, lua, bash, perl, java, kotlin, scala, groovy), challengeId (string).
(Previously: 9 languages, no JVM languages)

#### Scenario: JVM language keys accepted

- GIVEN an `AuditRequest` with `language: "java"`
- WHEN the audit handler receives it
- THEN it MUST pass the language key through to the sandbox unchanged

#### Scenario: All 13 language keys valid

- GIVEN valid `AuditRequest` instances for each of the 13 supported languages
- WHEN each is processed
- THEN none MUST be rejected as unsupported

---

### Requirement: Local Sandbox Executor

For language `java` → run `javac -d /tmp <file>`.
For language `kotlin` → run `kotlinc -d /tmp <file>`.
For language `scala` → run `scalac -d /tmp <file>`.
For language `groovy` → run `groovyc -d /tmp <file>`.

The sandbox MUST delegate tool selection to `ProviderRegistry.Get(language)`.
The sandbox MUST NOT contain switch statements on language.
(Previously: 9 languages, all single-command linting tools; no JVM compilers)

#### Scenario: Java compilation

- GIVEN valid Java code and `javac` is installed
- WHEN `Execute(ctx, "java", code, 30)` is called
- THEN the sandbox MUST run `javac` on the code
- AND stream stdout and stderr separately

#### Scenario: Kotlin compilation

- GIVEN valid Kotlin code and `kotlinc` is installed
- WHEN `Execute(ctx, "kotlin", code, 30)` is called
- THEN the sandbox MUST run `kotlinc` on the code

#### Scenario: Scala compilation

- GIVEN valid Scala code and `scalac` is installed
- WHEN `Execute(ctx, "scala", code, 30)` is called
- THEN the sandbox MUST run `scalac` on the code

#### Scenario: Groovy compilation

- GIVEN valid Groovy code and `groovyc` is installed
- WHEN `Execute(ctx, "groovy", code, 30)` is called
- THEN the sandbox MUST run `groovyc` on the code

---

## ADDED Requirements

### Requirement: JVM Compilation Output Directory

All JVM provider `DockerCommand` values MUST include `-d /tmp` to direct `.class` file output to the writable tmpfs, since DockerSandbox mounts `/code:ro`.

#### Scenario: Docker compilation writes to /tmp

- GIVEN a `DockerSandbox` executing Java code
- WHEN the container runs `javac -d /tmp Main.java`
- THEN `.class` files MUST be written to `/tmp`, not `/code`

#### Scenario: Read-only mount does not cause compiler failure

- GIVEN a `DockerSandbox` with `/code:ro` mount
- WHEN any JVM language (java, kotlin, scala, groovy) compiles
- THEN the compilation MUST NOT fail due to read-only filesystem

---

### Requirement: Language Key Normalization — Groovy Extension

The system MUST map `.groovy` file extensions to the `"groovy"` language key in `inferLanguage()`.

#### Scenario: Groovy file maps to groovy

- GIVEN a `.groovy` file is imported via Gogs
- WHEN `inferLanguage("src/Script.groovy")` is called
- THEN it MUST return `"groovy"`

#### Scenario: Existing JVM extensions verified

- GIVEN `.java`, `.kt`, `.scala` file extensions
- WHEN `inferLanguage()` is called for each
- THEN they MUST return `"java"`, `"kotlin"`, `"scala"` respectively (already mapped)
