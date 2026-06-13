# Dojo Audit Specification

## Purpose

Real-time streaming code analysis and feedback for Dojo challenges via SSE and local sandbox execution.

## Requirements

### Requirement: Audit Domain Model

The system MUST define an `AuditRequest` with: code (string), language (string), challengeId (string).
The system MUST define an `AuditEvent` with: type ('stdout' | 'stderr' | 'error' | 'complete'), data (string), timestamp.
The system MUST define an `AuditResult` with: exitCode, events[].

#### Scenario: Audit model creation
- GIVEN a valid code execution request
- WHEN an audit is initiated
- THEN the system MUST capture the request details in an `AuditRequest`
- AND the system MUST emit `AuditEvent` records during execution
- AND the system MUST return an `AuditResult` upon completion

### Requirement: Local Sandbox Executor

The system MUST implement `SandboxExecutor` using Go's os/exec.
For language 'typescript' → run `npx eslint --format=unix --stdin`.
For language 'go' → run `go vet`.
The sandbox MUST have a configurable timeout (default 30s).
The sandbox MUST stream stdout and stderr separately via pipes.

#### Scenario: Execution timeout
- GIVEN the command times out
- WHEN the timeout expires
- THEN the sandbox MUST kill the process and return an error event

#### Scenario: Standard execution
- GIVEN a valid execution request for a supported language
- WHEN the sandbox runs the command
- THEN it MUST stream stdout and stderr separately

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