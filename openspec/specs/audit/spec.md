# Dojo Audit Specification

## Purpose

Real-time streaming code analysis and feedback for Dojo challenges via SSE and local sandbox execution.

## Requirements

### Requirement: Audit Domain Model

The system MUST define an `AuditRequest` with: code (string), language (string — one of: typescript, javascript, go, python, ruby, php, lua, bash, perl, java, kotlin, scala, groovy), challengeId (string).
(Previously: 9 languages, no JVM languages)

The system MUST define an `AuditEvent` with: type ('stdout' | 'stderr' | 'error' | 'complete'), data (string), timestamp.
The system MUST define an `AuditResult` with: exitCode, events[].

#### Scenario: Audit model creation
- GIVEN a valid code execution request
- WHEN an audit is initiated
- THEN the system MUST capture the request details in an `AuditRequest`
- AND the system MUST emit `AuditEvent` records during execution
- AND the system MUST return an `AuditResult` upon completion

#### Scenario: JVM language keys accepted
- GIVEN an `AuditRequest` with `language: "java"`
- WHEN the audit handler receives it
- THEN it MUST pass the language key through to the sandbox unchanged

#### Scenario: All 13 language keys valid
- GIVEN valid `AuditRequest` instances for each of the 13 supported languages
- WHEN each is processed
- THEN none MUST be rejected as unsupported

### Requirement: Local Sandbox Executor

The system MUST implement `SandboxExecutor` using Go's os/exec.
For language 'typescript' → run `npx eslint --format=unix --stdin`.
For language 'go' → run `go vet`.
For language 'java' → run `javac -d /tmp <file>`.
For language 'kotlin' → run `kotlinc -d /tmp <file>`.
For language 'scala' → run `scalac -d /tmp <file>`.
For language 'groovy' → run `groovyc -d /tmp <file>`.
The sandbox MUST delegate tool selection to `ProviderRegistry.Get(language)`.
The sandbox MUST NOT contain switch statements on language.
The sandbox MUST have a configurable timeout (default 30s).
The sandbox MUST stream stdout and stderr separately via pipes.
(Previously: 9 languages, all single-command linting tools; no JVM compilers)

#### Scenario: Execution timeout
- GIVEN the command times out
- WHEN the timeout expires
- THEN the sandbox MUST kill the process and return an error event

#### Scenario: Standard execution
- GIVEN a valid execution request for a supported language
- WHEN the sandbox runs the command
- THEN it MUST stream stdout and stderr separately

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

### Requirement: Audit Handler (SSE)

The system MUST provide `POST /api/v1/audit` accepting JSON body.
The response MUST be `Content-Type: text/event-stream`.
The handler MUST stream events as SSE: `data: {"type":"stdout","data":"...","timestamp":"..."}\n\n`.
The handler MUST send a final `data: {"type":"complete","data":{"exitCode":0}}\n\n` when done.
The endpoint MUST NOT be subject to the global 30s timeout.

#### Scenario: Client disconnects
- GIVEN a client disconnects
- WHEN the handler detects the disconnection
- THEN it MUST cancel the sandbox process

#### Scenario: Complete execution
- GIVEN the sandbox finishes execution
- WHEN the process exits
- THEN the handler MUST send a final complete event

### Requirement: Audit Service (Frontend)

The system MUST provide an Angular `AuditService`.
It MUST expose a method `runAudit(code: string, language: string, challengeId: string): Observable<AuditEvent>`.
It MUST use `fetch()` with `ReadableStream` to parse SSE events.

#### Scenario: Normal stream end
- GIVEN the stream ends normally
- WHEN the complete event is received
- THEN the Observable MUST complete

#### Scenario: Network error
- GIVEN a network error
- WHEN the stream breaks
- THEN the Observable MUST error with a user-friendly message

### Requirement: Dojo Audit Button

The DojoPage MUST display an "Auditar" button in the top-right area.

#### Scenario: No challenge selected
- GIVEN no challenge is selected
- WHEN the user clicks "Auditar"
- THEN the button MUST be disabled

#### Scenario: Valid challenge audit
- GIVEN a challenge is loaded
- WHEN the user clicks "Auditar"
- THEN the button MUST show a loading state (spinner or "Auditando...")
- AND the terminal MUST clear and begin receiving output

#### Scenario: Audit completes
- GIVEN the audit completes
- WHEN the complete event arrives
- THEN the button MUST return to its normal state

#### Scenario: Audit fails
- GIVEN the audit fails
- WHEN an error event arrives
- THEN the terminal MUST show the error in red and the button MUST re-enable

### Requirement: Terminal Output Display

The TerminalPanelComponent MUST expose a `write(data: string)` method.

#### Scenario: Receive streaming events
- GIVEN SSE events arrive
- WHEN a stdout or stderr event is received
- THEN the terminal MUST display the data with ANSI color support
- WHEN an error event is received
- THEN the terminal MUST display the error in red

#### Scenario: Start new audit
- GIVEN a new audit starts
- WHEN the user clicks "Auditar"
- THEN the terminal MUST clear its content before new output appears

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

---

### Requirement: Hexagonal Isolation

Domain models (AuditRequest, AuditEvent) in Go backend MUST be in `internal/domain/models/`.
Application service (AuditService) in Go MUST be in `internal/core/services/`.
Frontend audit-event.ts MUST be in `domain/models/` (ZERO Angular imports).
Frontend AuditService MUST be in `infrastructure/services/`.

#### Scenario: Backend domain structure
- GIVEN the Go backend
- WHEN structuring domain logic
- THEN AuditRequest and AuditEvent MUST be isolated in `internal/domain/models/`

#### Scenario: Frontend domain structure
- GIVEN the Angular frontend
- WHEN defining the audit event model
- THEN `audit-event.ts` MUST NOT contain Angular imports

---

### Requirement: Systems Language Audit

The system MUST support audit execution for Rust, C, C++, and Zig languages through the sandbox executor.

#### Scenario: Rust audit compilation and execution

- GIVEN valid Rust code and `rustc` is available
- WHEN `Execute(ctx, "rust", code, 30)` is called
- THEN the sandbox MUST run `rustc -o /tmp/out /tmp/code.rs && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: C audit compilation and execution

- GIVEN valid C code and `gcc` is available
- WHEN `Execute(ctx, "c", code, 30)` is called
- THEN the sandbox MUST run `gcc -o /tmp/out /tmp/code.c && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: C++ audit compilation and execution

- GIVEN valid C++ code and `g++` is available
- WHEN `Execute(ctx, "cpp", code, 30)` is called
- THEN the sandbox MUST run `g++ -o /tmp/out /tmp/code.cpp && /tmp/out`
- AND stream stdout and stderr separately

#### Scenario: Zig audit compilation and execution

- GIVEN valid Zig code and `zig` is available
- WHEN `Execute(ctx, "zig", code, 30)` is called
- THEN the sandbox MUST run the `sh -c` wrapper that copies source to `/tmp`, compiles with `zig build-exe`, and executes

#### Scenario: Rust compilation error

- GIVEN invalid Rust code with syntax errors
- WHEN `Execute(ctx, "rust", code, 30)` is called
- THEN the sandbox MUST return a non-zero exit code
- AND stderr MUST contain the compiler error message

#### Scenario: C compilation timeout

- GIVEN C code that enters an infinite loop during compilation
- WHEN `Execute(ctx, "c", code, 5)` is called with a 5-second timeout
- THEN the sandbox MUST kill the process
- AND return a timeout error event

#### Scenario: Zig compilation error

- GIVEN invalid Zig code with type errors
- WHEN `Execute(ctx, "zig", code, 30)` is called
- THEN the sandbox MUST return a non-zero exit code
- AND stderr MUST contain the Zig compiler error message

#### Scenario: All 17 language keys valid

- GIVEN valid `AuditRequest` instances for each of the 17 supported languages
- WHEN each is processed
- THEN none MUST be rejected as unsupported