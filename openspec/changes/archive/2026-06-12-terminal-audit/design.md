# Design: terminal-audit

## Technical Approach

We will implement a streaming code analysis pipeline. A Go backend will expose an endpoint to accept code, run it through a local sandbox using `os/exec` (`eslint` for TS, `go vet` for Go), and pipe the `stdout`/`stderr` outputs as Server-Sent Events (SSE). The Angular frontend will invoke this endpoint, consume the stream via the Fetch API `ReadableStream`, and pipe the result into an `xterm.js` terminal in real time.

## Architecture Decisions

### Decision: Real-time Communication Protocol

**Choice**: Server-Sent Events (SSE).
**Alternatives considered**: WebSockets, HTTP Long-Polling.
**Rationale**: SSE provides native, unidirectional streaming over HTTP, ideal for our use case where the server pushes analysis output to the client. It handles reconnects automatically and avoids the complexity of full-duplex WebSockets.

### Decision: Local Sandbox Execution

**Choice**: `os/exec` with `exec.CommandContext` in Go.
**Alternatives considered**: Docker containers, WASM execution.
**Rationale**: Zero-dependency integration. Given the tools (`npx eslint`, `go vet`), a simple process execution with a strict `context` timeout prevents hung processes while keeping the architecture lightweight.

### Decision: Stream Parsing on Frontend

**Choice**: `fetch()` with `ReadableStream` wrapping as an RxJS `Observable`.
**Alternatives considered**: `EventSource` API.
**Rationale**: `EventSource` doesn't natively support `POST` requests with payloads (code, language). Using `fetch()` allows sending the execution request and processing the chunked response natively.

## Data Flow

```text
  [User Action: Click "Auditar"]
         │
         ▼
   DojoPage (UI)
         │  (auditService.runAudit(code, lang, id))
         ▼
  AuditService (TS) ─────────(POST /api/v1/audit)───────┐
         ▲                                              │
         │ (ReadableStream chunks)                      ▼
         │                                       AuditHandler (Go)
         │                                              │
         │ (SSE flush)                                  ▼
         └─────────────────────────── SSEWriter ◄── AuditService (Go)
                                                        │
                                                        ▼
                                                 LocalSandbox (Go)
                                                        │ (os/exec)
                                                        ▼
                                            [ npx eslint | go vet ]
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/core/domain/models/audit_models.go` | Create | Pure Go models (`AuditRequest`, `AuditEvent`, `AuditResult`). |
| `backend/internal/infrastructure/driven/sandbox/localsandbox.go` | Create | Implements `SandboxExecutor` port with `os/exec`. |
| `backend/internal/core/services/audit_service.go` | Create | Orchestrates the sandbox and SSE writer. |
| `backend/internal/infrastructure/driving/handlers/sse_handler.go` | Create | Implements `SSEWriter` adapter over `http.ResponseWriter`. |
| `backend/internal/infrastructure/driving/handlers/audit_handler.go` | Create | HTTP handler for `POST /api/v1/audit`, handles SSE headers. |
| `backend/cmd/api/main.go` | Modify | Registers the audit route outside the global 30s timeout middleware. |
| `frontend/codeauditor/src/app/domain/models/audit-event.ts` | Create | TypeScript model for `AuditEvent` (zero Angular imports). |
| `frontend/codeauditor/src/app/domain/models/index.ts` | Modify | Exports the new domain model. |
| `frontend/codeauditor/src/app/infrastructure/services/audit.service.ts` | Create | Connects to the backend via `fetch` and processes the stream. |
| `frontend/codeauditor/src/app/infrastructure/services/index.ts` | Modify | Exports the new service. |
| `frontend/codeauditor/src/app/infrastructure/components/dojo/dojo-page.component.ts` | Modify | Adds "Auditar" button and links `AuditService` output to the terminal. |
| `frontend/codeauditor/src/app/infrastructure/components/shared/terminal-panel.component.ts` | Modify | Adds `write(data: string)` and `clear()` methods for UI updates. |

## Interfaces / Contracts

**Go Domain Models (`audit_models.go`):**
```go
package models

type AuditRequest struct {
	Code        string `json:"code"`
	Language    string `json:"language"`
	ChallengeID string `json:"challengeId"`
}

type AuditEvent struct {
	Type      string `json:"type"` // "stdout", "stderr", "error", "complete"
	Data      string `json:"data"`
	Timestamp string `json:"timestamp"`
}

type AuditResult struct {
	ExitCode int          `json:"exitCode"`
	Events   []AuditEvent `json:"events,omitempty"`
}
```

**TypeScript Domain Models (`audit-event.ts`):**
```typescript
export interface AuditEvent {
  type: 'stdout' | 'stderr' | 'error' | 'complete';
  data: string;
  timestamp: string;
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `LocalSandbox` | Test with a dummy `exec.Command` like `echo` and `sleep` to verify context timeouts and pipe separation. |
| Unit | `AuditService` (Go) | Mock `SandboxExecutor` and `SSEStreamer` to ensure events are correctly formatted and piped. |
| Integration | Audit API Route | Send a dummy `POST` request and verify that response headers (`text/event-stream`) and the initial JSON data chunk match expectations. |
| Unit | `AuditService` (TS) | Stub `window.fetch` with a fake stream and verify the `Observable` emits events and completes correctly. |

## Migration / Rollout

No migration required. This introduces a new route and frontend components.

## Open Questions

- None.