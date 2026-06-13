# Design: Initial Scaffolding

This design establishes the monorepo structure, Hexagonal Architecture boundaries for both frontend and backend, and the core interfaces for Auth, SSE streaming, and Docker sandboxing.

## Quick path

1. **Monorepo**: Independent `backend/` (Go) and `frontend/` (Angular) directories.
2. **Backend Architecture**: Hexagonal (Domain, Application, Ports, Infrastructure).
3. **Frontend Architecture**: Hexagonal (Domain, Application pure TS; Infrastructure, UI as Angular).
4. **Key Adapters**: Supabase Auth, Docker Sandbox, SSE HTTP Handler.

## Architecture Decisions

### Decision: Backend Directory Structure
**Choice**: `internal/domain`, `internal/application`, `internal/ports`, `internal/infrastructure`.
**Alternatives considered**: Traditional MVC or Flat structure.
**Rationale**: Hexagonal architecture isolates business logic from external frameworks, ensuring the domain is pure Go.

### Decision: Frontend Directory Structure
**Choice**: `src/app/domain`, `src/app/application`, `src/app/infrastructure`, `src/app/ui`.
**Alternatives considered**: Default Angular module-based feature folders.
**Rationale**: Aligns with the project config enforcing pure TS domain/application layers. The UI layer will contain Angular components that depend on application use cases.

### Decision: Sandbox Execution Strategy
**Choice**: Docker SDK to spin up one-shot containers with max restrictions (`no-new-privileges`, `cap-drop ALL`, `--network none`).
**Alternatives considered**: WebAssembly (Wasm) or Firecracker microVMs.
**Rationale**: Docker is already required, easy to orchestrate locally, and provides sufficient isolation for initial scaffolding without the complexity of Firecracker.

## Directory Structure

### Backend (Go)
```text
backend/
├── cmd/
│   └── api/
│       └── main.go              # Entry point, wire dependencies
├── internal/
│   ├── domain/                  # Entities and domain logic (Pure Go)
│   ├── application/             # Use cases implementing primary ports
│   ├── ports/                   # Primary & Secondary interfaces
│   └── infrastructure/
│       ├── http/                # REST handlers, SSE streaming
│       ├── auth/                # Supabase JWT validation adapter
│       └── sandbox/             # Docker executor adapter
├── go.mod
└── go.sum
```

### Frontend (Angular)
```text
frontend/
├── src/
│   ├── app/
│   │   ├── domain/              # Pure TS domain models
│   │   ├── application/         # Pure TS use cases, interfaces
│   │   ├── infrastructure/      # Angular services implementing application ports
│   │   └── ui/                  # Angular Components, Directives, Pipes
│   └── main.ts
├── angular.json
└── package.json
```

## Interfaces / Contracts

### 1. Auth Port (Secondary)
Validates JWTs from Supabase and extracts identity.
```go
package ports

import "context"

// AuthValidator defines the contract for validating authentication tokens.
type AuthValidator interface {
    // ValidateToken checks whether the provided token is valid.
    // Returns nil if valid; returns an error if invalid or expired.
    ValidateToken(ctx context.Context, token string) error

    // UserIDFromToken extracts the user ID from a valid token.
    // The caller must ensure the token has already been validated.
    UserIDFromToken(token string) (string, error)
}
```

### 2. Sandbox Port (Secondary)
Executes untrusted code in an isolated environment with streaming output.
```go
package ports

import (
    "context"
    "io"
)

// SandboxExecutor defines the contract for executing code in an isolated sandbox.
type SandboxExecutor interface {
    // Execute runs the given code snippet in an isolated environment.
    // language must be one of: "python", "javascript", "go", "bash".
    // stdout and stderr are streamed back via the returned ReadCloser.
    Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error)

    // Healthcheck verifies the sandbox runtime is responsive.
    Healthcheck(ctx context.Context) error
}
```

### 3. SSE Streaming (Driven / Secondary Port)
Connection-manager pattern pushing real-time events to frontend clients.
```go
package ports

import (
    "context"
    "encoding/json"
)

// SSEStreamer defines the contract for streaming Server-Sent Events to clients.
type SSEStreamer interface {
    // StreamEvent sends a JSON-serializable event to the client identified by clientID.
    StreamEvent(ctx context.Context, clientID string, eventType string, payload interface{}) error

    // BroadcastLLMTokens streams LLM token deltas to a client.
    BroadcastLLMTokens(ctx context.Context, clientID string, tokenDelta string) error

    // RegisterClient adds a new SSE connection. Returns a channel that closes on disconnect.
    RegisterClient(ctx context.Context, clientID string) <-chan struct{}

    // UnregisterClient removes a client and cleans up resources.
    UnregisterClient(clientID string)
}

// SSEClientMessage represents a message received from an SSE client.
type SSEClientMessage struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
```

### 4. Frontend Ports (Angular / TypeScript)

The frontend defines three domain port interfaces in pure TypeScript:

**AuthPort** — Authentication and user identity.
```typescript
export interface AuthPort {
  getCurrentUser(): Promise<User | null>;
  signIn(email: string, password: string): Promise<User>;
  signOut(): Promise<void>;
  onAuthStateChange(callback: (user: User | null) => void): () => void;
}
```

**LLMPort** — LLM interaction (Ollama), including streaming.
```typescript
export interface LLMPort {
  explainFinding(findingId: string, context: string): Promise<string>;
  summarizeSession(sessionId: string): Promise<string>;
  streamTokens(prompt: string, onToken: (token: string) => void): Promise<void>;
}
```

**AuditRepository** — Data access for audit sessions and findings.
```typescript
export interface AuditRepository {
  createSession(session: Omit<AuditSession, "id" | "createdAt" | "updatedAt">): Promise<AuditSession>;
  getSession(id: string): Promise<AuditSession | null>;
  listSessions(): Promise<AuditSession[]>;
  updateSessionStatus(id: string, status: AuditSession["status"]): Promise<void>;
  addFinding(finding: Omit<Finding, "id" | "detectedAt">): Promise<Finding>;
  getFindingsForSession(sessionId: string): Promise<Finding[]>;
}
```

### Reconciliation Notes

The following deviations from the original design were applied during implementation:

| Port | Design | Actual | Rationale |
|------|--------|--------|-----------|
| `AuthValidator` | `ValidateToken()` returns `*Identity` | `ValidateToken()` returns `error` + separate `UserIDFromToken()` | Separation of concerns: validation and identity extraction are independent. Reduces ambiguity (no partial identity on errors). |
| `SandboxExecutor` | `Execute()` uses `ExecutionRequest`/`ExecutionResult` structs | `Execute()` uses flat params `(language, code, timeoutSeconds)` + returns `io.ReadCloser` | Streaming output requires an ongoing reader, not a fixed result struct. Flat params avoid struct allocation overhead per call. Added `Healthcheck()` for runtime readiness. |
| `LLMStreamer` → `SSEStreamer` | `GenerateStream()` returns `<-chan string` | `SSEStreamer` with connection-manager pattern (`RegisterClient`, `UnregisterClient`, `StreamEvent`, `BroadcastLLMTokens`) | Channel-based API doesn't scale to multiple concurrent clients or multiple event types. Connection-manager supports client lifecycle, broadcasts, and typed events via `SSEClientMessage`. |

## Data Flow

### Sandbox Execution

    Client ──(POST /execute)──→ HTTP Adapter
                                     │
                               Application Layer
                                     │
                             Docker Infrastructure (Sandbox)
                                     │
                               ExecutionResult
                                     │
    Client ⟵──(200 OK)───────── HTTP Adapter


### SSE Streaming

    Client ──(GET /stream)────→ SSE Adapter (RegisterClient)
                                      │
                                Application Layer (BroadcastLLMTokens)
                                      │
                                   Ollama
                                      │
                                 (Token Stream)
                                      │
    Client ⟵──(SSE: data: token)── SSE Adapter (StreamEvent)
                                      │
    Client ──(disconnect)─────→ SSE Adapter (UnregisterClient)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/go.mod` | Create | Initialize Go module (`github.com/anomalyco/codeauditor/backend`) |
| `backend/cmd/api/main.go` | Create | Go entry point with Chi router, /health, /api/v1 stubs |
| `backend/internal/ports/auth.go` | Create | `AuthValidator` interface (split validation + identity) |
| `backend/internal/ports/sandbox.go` | Create | `SandboxExecutor` interface (flat params, streaming) |
| `backend/internal/ports/sse.go` | Create | `SSEStreamer` interface (connection-manager pattern) |
| `backend/internal/ports/.gitkeep` | Create | Ports package stub |
| `backend/internal/domain/.gitkeep` | Create | Domain package stub |
| `backend/internal/application/.gitkeep` | Create | Application package stub |
| `backend/internal/infrastructure/http/.gitkeep` | Create | HTTP driving adapter stub |
| `backend/internal/infrastructure/auth/.gitkeep` | Create | Auth driven adapter stub |
| `backend/internal/infrastructure/sandbox/.gitkeep` | Create | Sandbox driven adapter stub |
| `backend/pkg/.gitkeep` | Create | Shared utilities package |
| `frontend/codeauditor/angular.json` | Create | Angular 21 workspace config |
| `frontend/codeauditor/package.json` | Create | Angular 21 + dependencies |
| `frontend/codeauditor/postcss.config.js` | Create | PostCSS with tailwindcss plugin |
| `frontend/codeauditor/src/styles.css` | Create | Tailwind v4 + Dojo dark palette |
| `frontend/codeauditor/src/app/domain/models/*.ts` | Create | AuditSession, Finding, User entities |
| `frontend/codeauditor/src/app/domain/ports/*.ts` | Create | AuthPort, AuditRepository, LLMPort interfaces |
| `frontend/codeauditor/src/app/application/audit.use-case.ts` | Create | AuditUseCase stub |
| `frontend/codeauditor/src/app/infrastructure/supabase.adapter.ts` | Create | Supabase adapters |
| `frontend/codeauditor/src/app/infrastructure/ollama.adapter.ts` | Create | OllamaAdapter implementing LLMPort |
| `frontend/codeauditor/src/app/ui/home.component.ts` | Create | HomeComponent stub |
| `frontend/codeauditor/src/app/app.routes.ts` | Update | Routes pointing to HomeComponent |
| `frontend/codeauditor/src/app/app.ts` | Update | Simplified to router-outlet |
| `docker-compose.yml` | Create | Supabase (postgres, kong, studio) + Ollama services |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Pure domain & Application | Standard `go test` and `ng test`. |
| Integration | Adapters | Testcontainers for Docker sandbox testing. |
| E2E | API to Frontend flow | Playwright (future phase). |

## Migration / Rollout
No migration required. Greenfield initialization.

## Open Questions (Resolved)
- [x] Go HTTP framework: **Chi v5.1.0** — chosen for its lightweight, idiomatic handler signatures. Used in `backend/cmd/api/main.go` with route grouping for `/health` and `/api/v1`.
