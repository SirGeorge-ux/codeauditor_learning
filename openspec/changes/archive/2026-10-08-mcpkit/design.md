# Design: mcpkit — Extract MCP Runtime Library

## Technical Approach

Mechanical file relocation from `backend/internal/infrastructure/mcp/` into a new sibling package `backend/internal/mcpkit/`, followed by a single surface cleanup (delete the four dead re-exports after grep proof). The runtime's public contract — every type, every method, every exported var — is preserved bit-for-bit at the rename boundary; the only changes that can ripple to a consumer are the import path and the four removed aliases. Tool groups keep their current behavior; their only edits are import-path line changes.

This is **not** a redesign. It is a directory move plus a package rename plus a documented scope fence. The strategic motivation is forward-compatibility: academy-mic and the planned Gogs fork will both depend on `internal/mcpkit` as the canonical Go MCP runtime, but neither will import the other's tool groups.

## Architecture Decisions

### Decision 1: Relocate `http_transport_test.go` to `dictionary/`

**Choice**: Move the file from `backend/internal/infrastructure/mcp/http_transport_test.go` (current `package mcp_test`) to `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go` with `package dictionary_test`.

**Alternatives considered**:
- Keep it in `mcpkit` as a bridge test. Rejected: the file imports BOTH the runtime and the dictionary tool group, plus `core/services` and `ports`. It is not a runtime unit test; it is a dictionary end-to-end transport test. Keeping it under `mcpkit` would force mcpkit's test suite to depend on a specific tool group's services — a layering violation.
- Keep it where it is and rename `package mcp_test` → `package mcpkit_test`. Rejected: same layering violation, more visible.
- Delete it. Rejected: this is the canonical end-to-end MCP+tool-group wire test (502 lines); its coverage is unique.

**Rationale**: The test exercises `mcp.NewServer(registry).Router(...)` with the 4 dictionary tools wired in. It belongs semantically with the dictionary tool group. After relocation, `package dictionary_test` reflects what the test actually proves (dictionary wire + transport), and `mcpkit/` stays a clean runtime-only package.

### Decision 2: Delete the four dead re-exports in the runtime

**Choice**: After the file move + rename, delete these re-exports:

| Symbol | Defined where today | Alias of |
|--------|---------------------|----------|
| `ContextWithUserID` | `server.go:27` | `mcpctx.ContextWithUserID` |
| `UserIDFromContext` | `server.go:33` | `mcpctx.UserIDFromContext` |
| `AuthUserIDKey` | `server.go:20` (type alias) | `mcpctx.AuthUserIDKey` |
| `ErrInvalidParams` | `dispatcher.go:163` (var | = `mcpctx.ErrInvalidParams`) |

**Definitive grep evidence** (run 2026-10-08 against `master` + Dictionary commit `6f70721`):

```bash
$ grep -rn 'mcp\.\(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams\)' backend/
```

Returns exactly **one** match:

```text
backend/internal/infrastructure/mcp/dispatcher.go:160: // existing imports `mcp.ErrInvalidParams` keep compiling while the value
```

That match is the comment text inside the re-export definition itself. **Zero code paths consume any of the four re-exports**. The transport bridge in `server.go:100` calls its own `ContextWithUserID` (same package), not the alias. Tool groups import `mcpctx` directly. Tool groups also declare their own local `ErrInvalidParams = mcpctx.ErrInvalidParams` aliases in `dictionary/dictionary_tool.go:30` and `tutor/tutor_tool.go:30`. `errors.Is` checks across the boundary succeed because every alias points to the same `mcpctx.ErrInvalidParams` sentinel.

**Alternatives considered**:
- Keep them as a deprecation shim. Rejected: zero callers, deprecation shim is over-engineering.
- Move them to mcpkit as aliases. Rejected: same problem under a new name.

**Rationale**: Dead code is technical debt. Delete it now, while the change is already touching every line of these files.

### Decision 3: Interface canonicalization (`RegistryRegistrar`/`MCPToolRegistrar`) — DEFERRED

**Choice**: Out of scope. Both `dictionary_tool.go:489-505` and `tutor_tool.go` declare local `RegistryRegistrar` and `MCPToolRegistrar` interfaces that mirror `mcpkit.MCPTool` / `mcpkit.Registry.Register`. The duplication is intentional today — it lets tool groups satisfy `MCPTool` without importing the runtime package, breaking the import cycle between the mcp-monolith spec (which assumes `mcp.MCPTool`) and the tool groups.

**Alternatives considered**:
- Canonicalize now: expose `mcpkit.Tool` (alias of `mcpkit.MCPTool`) and have tool groups embed it. Rejected: this is a design-level decision that belongs in its own change, with its own spec and verify pass. Adding it here would balloon scope and dilute the verification focus of "the runtime moved."

**Rationale**: Hard scope fence. If/when canonicalization happens, it should:
1. Add a follow-up change (`mcpkit-interfaces` or similar).
2. Re-spec the affected tool groups' contracts.
3. Be gated on its own build + test + lint pass.

### Decision 4: One-way dependency `mcpkit` → `infrastructure/driving/authmiddleware` — ACCEPTED

**Choice**: Document and accept the direction. `mcpkit/ratelimiter.go` and `mcpkit/server.go` import `github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware`. This means `mcpkit` depends on `infrastructure/driving`, even though `mcpkit` lives as a sibling of `infrastructure/`.

**Direction diagram**:

```text
cmd/api/main.go ─────► internal/mcpkit ◄──── internal/infrastructure/mcp/dictionary
                              │                              │
                              ▼                              ▼
                     infrastructure/driving/         infrastructure/mcp/
                     authmiddleware                   tutor
                              ▲
                              │
                              └────── internal/mcpkit (one-way)
```

**Alternatives considered**:
- Reverse: have `authmiddleware` stamp both `authmiddleware.UserIDContextKey` AND `mcpkit.AuthUserIDKey`, and have `mcpkit` import only stdlib. Rejected: doubles the bridge complexity; the cost outweighs the layering purity win. The current `Server.Router(authMW)` signature takes an opaque `func(http.Handler) http.Handler` and translates internally — this is already a clean adapter boundary.
- Extract `authmiddleware.UserIDContextKey` and `GetUserID` into a leaf package (`authctx`). Rejected: bigger refactor, not in scope.

**Rationale**: One-way dependencies are acceptable in Hexagonal when the importing layer is a runtime utility (`mcpkit`) and the imported layer is a thin middleware adapter. `authmiddleware` does not import `mcpkit`; the arrow points from the higher-leverage library to the lower-leverage adapter, which is the right direction for a "runtime consumes a transport primitive" relationship. Document it; do not invert it.

## Data Flow

Unchanged. The runtime's data flow (request → middleware → `httpContextBridge` → handler → `Registry.Get` → tool `Execute`) is preserved bit-for-bit by this change. No flow diagram needed for a move.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/mcpkit/tool.go` | Create (move) | `package mcpkit`. Imports: stdlib + `sashabaranov/go-openai`. |
| `backend/internal/mcpkit/registry.go` | Create (move) | `package mcpkit`. No external deps. |
| `backend/internal/mcpkit/dispatcher.go` | Create (move) | `package mcpkit`. Imports: stdlib + `internal/mcpkit/mcpctx`. **Drops `ErrInvalidParams` re-export (line 163).** |
| `backend/internal/mcpkit/server.go` | Create (move) | `package mcpkit`. Imports: stdlib + chi + `internal/infrastructure/driving/authmiddleware` + `internal/mcpkit/mcpctx`. **Drops re-exports of `AuthUserIDKey` (line 20), `ContextWithUserID` (lines 27-29), `UserIDFromContext` (lines 33-35).** `httpContextBridge` rewrites its own internal call to use `mcpctx.ContextWithUserID` directly. |
| `backend/internal/mcpkit/ratelimiter.go` | Create (move) | `package mcpkit`. Imports: stdlib + `internal/infrastructure/driving/authmiddleware` + `internal/mcpkit/mcpctx` (for `RPCError`/`CodeRateLimited`). |
| `backend/internal/mcpkit/mcpctx/auth_context.go` | Create (move) | `package mcpctx`. Imports: stdlib (`errors`, `context`). Path: `.../mcpkit/mcpctx`. |
| `backend/internal/mcpkit/dispatcher_test.go` | Create (move) | `package mcpkit`. Same content; package rename. |
| `backend/internal/mcpkit/registry_test.go` | Create (move) | `package mcpkit`. Same content; package rename. Defines `fakeTool` helper. |
| `backend/internal/mcpkit/server_test.go` | Create (move) | `package mcpkit`. Same content; package rename. |
| `backend/internal/mcpkit/ratelimiter_test.go` | Create (move) | `package mcpkit`. Same content; package rename. |
| `backend/internal/infrastructure/mcp/tool.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/registry.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/dispatcher.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/server.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/ratelimiter.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/dispatcher_test.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/registry_test.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/server_test.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/ratelimiter_test.go` | **Delete** | Moved. |
| `backend/internal/infrastructure/mcp/mcpctx/` | **Delete** (whole dir) | Moved to `mcpkit/mcpctx/`. |
| `backend/internal/infrastructure/mcp/http_transport_test.go` | **Delete + relocate** | Becomes `dictionary/http_transport_test.go` with `package dictionary_test`. |
| `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go` | Create (relocated) | `package dictionary_test`. Rewrites: `mcp.X` → `mcpkit.X`, import path `.../mcp` → `.../mcpkit`, import path `.../mcp/mcpctx` → `.../mcpkit/mcpctx`. |
| `backend/internal/infrastructure/mcp/dictionary/dictionary_tool.go` | Modify | One-line import path rewrite: `.../mcp/mcpctx` → `.../mcpkit/mcpctx`. Local `ErrInvalidParams` alias stays. |
| `backend/internal/infrastructure/mcp/tutor/tutor_tool.go` | Modify | One-line import path rewrite: `.../mcp/mcpctx` → `.../mcpkit/mcpctx`. Local `ErrInvalidParams` alias stays. |
| `backend/internal/infrastructure/mcp/tutor/tutor_http_transport_test.go` | Modify | Rewrite import paths (`mcp` → `mcpkit`, `mcp/mcpctx` → `mcpkit/mcpctx`) and symbol references (`mcp.NewRegistry` → `mcpkit.NewRegistry`, `mcp.NewServer` → `mcpkit.NewServer`). |
| `backend/cmd/api/main.go` | Modify | Rewrite line 25 import: `.../infrastructure/mcp` → `.../mcpkit`. Lines 26-27 (tool-group paths) unchanged. Update symbol references: `mcp.NewRegistry` → `mcpkit.NewRegistry`, `mcp.NewServer` → `mcpkit.NewServer`. |
| `backend/go.mod` | Recheck (no edits expected) | Re-run `go mod tidy` after the move; confirm `sashabaranov/go-openai` is NOT dropped (it MUST stay — `tool.go` uses `openai.Tool`). |

## Interfaces / Contracts

The public contract `mcpkit` MUST preserve (verbatim from the explore's inventory):

```go
// package mcpkit
type MCPTool interface {
    Name() string
    Description() string
    Schema() openai.Tool
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}

type Registry struct{ /* ... */ }
func NewRegistry() *Registry
func (r *Registry) Register(tool MCPTool)
func (r *Registry) Get(name string) (MCPTool, error)
func (r *Registry) List() []MCPTool
func (r *Registry) Count() int

var ErrToolNotFound = /* sentinel */

type Dispatcher struct{ /* ... */ }
func NewDispatcher(*Registry) *Dispatcher
func (d *Dispatcher) WithTimeout(time.Duration) *Dispatcher
func (d *Dispatcher) Dispatch(ctx context.Context, body []byte) rpcResponse

type RPCError struct{ /* ... */ }
func (e *RPCError) Error() string

const (
    CodeParseError     = -32700
    CodeInvalidRequest = -32600
    CodeMethodNotFound = -32601
    CodeInvalidParams  = -32602
    CodeInternalError  = -32603
    CodeRateLimited    = -32609
    MaxResponseBytes   = 50 * 1000
)

func TruncateResult(any, int) (string, bool)
func SanitizeResult(any) any
func SanitizeErrorMessage(string) string

type Server struct{ /* ... */ }
func NewServer(*Registry) *Server
func (s *Server) WithDispatcher(*Dispatcher) *Server
func (s *Server) WithRateLimiter(*RateLimiter) *Server
func (s *Server) Router(authMW func(http.Handler) http.Handler) http.Handler

const (
    DefaultRateLimit   = 10
    DefaultRateWindow  = 60 * time.Second
)
type RateLimiter struct{ /* ... */ }
func NewRateLimiter() *RateLimiter
func (rl *RateLimiter) WithClock(func() time.Time) *RateLimiter
func (rl *RateLimiter) Allow(userID, toolName string) bool
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler

func IsRateLimitedError(err error) bool
func NewRateLimitedError(tool string) error

// package mcpkit/mcpctx
type AuthUserIDKey struct{}
var ErrInvalidParams = errors.New("invalid params (-32602)")
func ContextWithUserID(ctx context.Context, userID string) context.Context
func UserIDFromContext(ctx context.Context) string
```

**Removed from public surface** (post-cleanup): `ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`, `ErrInvalidParams` as `mcpkit.*` symbols (they live in `mcpkit/mcpctx`, accessible via `mcpctx.X`).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Go, `mcpkit`) | All four moved test files run as `package mcpkit` | `go test ./internal/mcpkit/...` — table-driven (`t.Run`) per `go-testing` skill. |
| Unit (Go, `infrastructure/mcp/dictionary`) | Dictionary tool tests + relocated `http_transport_test.go` | `go test ./internal/infrastructure/mcp/dictionary/...`. Verifies the relocated bridge test runs under `package dictionary_test`. |
| Unit (Go, `infrastructure/mcp/tutor`) | Tutor tool tests + bridge test | `go test ./internal/infrastructure/mcp/tutor/...`. Verifies import path rewrite. |
| Aggregate | All backend | `go test -count=1 ./internal/...` (per `openspec/config.yaml` `projects[backend].test_command` precedent). |
| Lint | mcpkit + tool groups | `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...`. |
| Build | All | `go build ./...`. |

No new tests written — every moved test exercises the same code at the same boundary. The change's verification is mechanical (file inventory + symbol inventory + package inventory + grep proof). Per `go-testing` skill: "Test behavior and state transitions, not implementation trivia." The behavior is unchanged; the move is the change.

## Migration / Rollout

No data migration. No feature flags. No phased rollout. Single PR, single apply commit per `work-unit-commits` skill:

- Commit message: `refactor(mcp): extract runtime to internal/mcpkit`.
- Body explains: files moved, package renamed, four dead re-exports deleted (with grep proof), `http_transport_test.go` relocated, import paths rewritten in 4 consumers.
- Hooks enabled; `make validate` runs (gofumpt + golangci-lint).

`go mod tidy` runs at the end of the change to confirm no dep drift. If `sashabaranov/go-openai` is unexpectedly dropped, it is manually re-added.

## Threat Matrix

**N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.**

This change moves Go source files and rewrites import paths. It does not touch HTTP routing semantics (chi router wiring is preserved), does not introduce any new shell or subprocess execution, and does not touch the VCS layer. The runtime's security posture (authmiddleware.PUT, dispatcher timeout, rate limit, response sanitization, secret-pattern redaction) is preserved bit-for-bit.

## Tracking Note: mcp-monolith Parent Spec

The `mcp-monolith` parent spec's `ADDED Requirements` and `specs/mcp/spec.md` describe the runtime location as `infrastructure/mcp/`. After this change lands, the runtime lives at `internal/mcpkit/`. The mcp-monolith spec stays OPEN (phases 2-5 still pending). The location is updated by reference:

1. **This change's spec delta** (`openspec/changes/mcpkit/specs/mcpkit/spec.md`) describes the new `mcpkit` capability and explicitly notes that the mcp-monolith parent spec's runtime location is now `internal/mcpkit/`.
2. **At mcp-monolith archive time** (when phases 2-5 land), the parent spec is updated to reference `internal/mcpkit/` instead of `infrastructure/mcp/`. This is a one-line edit, scoped to the archive.

No edits to `mcp-monolith/` files in this change.

## Open Questions

None. All four explore-level decisions resolved (this design + proposal). The change is mechanical; the verification gate is fixed.