# mcpkit — MCP Runtime Library Specification

## Purpose

Define the `mcpkit` Go library: the canonical MCP (Model Context Protocol) runtime extracted from `internal/infrastructure/mcp/` into `internal/mcpkit/`. The library exposes the `MCPTool` interface, `Registry`, `Dispatcher` (JSON-RPC 2.0), `Server` HTTP driving adapter, `RateLimiter`, and the `mcpctx` context-key package. It is consumed by every MCP tool group in `internal/infrastructure/mcp/<group>/` and by `cmd/api/main.go` (composition root). It is the foundation that academy-mic and the planned Gogs fork will both import.

## ADDED Requirements

### Requirement: mcpkit Package Location

The system SHALL provide the MCP runtime as a Go package at the import path `github.com/anomalyco/codeauditor/backend/internal/mcpkit`. The package name SHALL be `mcpkit`. The package SHALL live as a sibling of `internal/infrastructure/`, not inside it.

The mcpctx context-key package SHALL live at `internal/mcpkit/mcpctx/` with package name `mcpctx`, importable as `github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx`.

#### Scenario: mcpkit package exists at the canonical path

- GIVEN the repository is at a commit where this change has been applied
- WHEN `ls backend/internal/mcpkit/` is run
- THEN it contains `tool.go`, `registry.go`, `dispatcher.go`, `server.go`, `ratelimiter.go`, plus the four corresponding `*_test.go` files, plus a `mcpctx/` subdirectory holding `auth_context.go`

#### Scenario: mcpctx package exists at the new path

- GIVEN the repository is at a commit where this change has been applied
- WHEN `ls backend/internal/mcpkit/mcpctx/` is run
- THEN it contains exactly `auth_context.go`

#### Scenario: infrastructure/mcp/ contains only tool groups

- GIVEN the runtime files have been moved out
- WHEN `ls backend/internal/infrastructure/mcp/` is run
- THEN it contains only subdirectories for tool groups — `dictionary/`, `tutor/`, plus any future groups — and NO top-level Go files (no `tool.go`, `registry.go`, `dispatcher.go`, `server.go`, `ratelimiter.go`, no `*_test.go` files, no `mcpctx/` subdirectory)

### Requirement: Tool-Group Import Contract

Every tool group under `internal/infrastructure/mcp/<group>/` SHALL import runtime symbols exclusively from `internal/mcpkit` (and `internal/mcpkit/mcpctx`). No tool group SHALL import `internal/infrastructure/mcp` directly for runtime symbols.

#### Scenario: dictionary tool group imports mcpkit for mcpctx only

- GIVEN `backend/internal/infrastructure/mcp/dictionary/dictionary_tool.go`
- WHEN its imports are inspected
- THEN it imports `.../internal/mcpkit/mcpctx` (and possibly other tool-group-local symbols), and does NOT import `.../internal/infrastructure/mcp` or `.../internal/infrastructure/mcp/mcpctx`

#### Scenario: tutor tool group imports mcpkit for mcpctx only

- GIVEN `backend/internal/infrastructure/mcp/tutor/tutor_tool.go`
- WHEN its imports are inspected
- THEN it imports `.../internal/mcpkit/mcpctx`, and does NOT import `.../internal/infrastructure/mcp` or `.../internal/infrastructure/mcp/mcpctx`

#### Scenario: composition root imports mcpkit for runtime symbols

- GIVEN `backend/cmd/api/main.go`
- WHEN its imports are inspected
- THEN it imports `.../internal/mcpkit` for `mcpkit.NewRegistry`, `mcpkit.NewServer`, etc., and imports `.../internal/mcpkit/mcpctx` only if it uses context helpers
- AND it does NOT import `.../internal/infrastructure/mcp` (no runtime-symbol consumers remain)
- AND it MAY still import `.../internal/mcp/dictionary` and `.../internal/mcp/tutor` as tool-group siblings

### Requirement: Runtime Public API Preservation

The `mcpkit` package SHALL preserve, by exact name and signature, every public symbol that the existing runtime exposes. The package rename from `mcp` to `mcpkit` MUST NOT remove, rename, or change any symbol except the four re-exports deleted by the cleanup decision (see "Dead Re-export Removal" requirement).

#### Scenario: MCPTool interface preserved

- GIVEN the runtime has moved to `internal/mcpkit/`
- WHEN `grep -rn "type MCPTool interface" backend/internal/mcpkit/` is run
- THEN it returns exactly one match, in `tool.go`, with the four methods `Name() string`, `Description() string`, `Schema() openai.Tool`, `Execute(ctx context.Context, args json.RawMessage) (any, error)`

#### Scenario: Registry API preserved

- GIVEN the runtime has moved
- WHEN the registry's exported methods are enumerated via `gopls symbols` or equivalent
- THEN the list includes `NewRegistry`, `Register(tool MCPTool)`, `Get(name string) (MCPTool, error)`, `List() []MCPTool`, `Count() int`, plus the exported var `ErrToolNotFound`

#### Scenario: Dispatcher API preserved

- GIVEN the runtime has moved
- WHEN the dispatcher's exported symbols are enumerated
- THEN the list includes `NewDispatcher(*Registry)`, `(d *Dispatcher) WithTimeout(time.Duration)`, `(d *Dispatcher) Dispatch(ctx, body)`, the exported type `RPCError`, the exported error-code constants (`CodeParseError`, `CodeInvalidRequest`, `CodeMethodNotFound`, `CodeInvalidParams`, `CodeInternalError`, `CodeRateLimited`), the constant `MaxResponseBytes`, and the helper functions `TruncateResult`, `SanitizeResult`, `SanitizeErrorMessage`

#### Scenario: Server API preserved

- GIVEN the runtime has moved
- WHEN the server's exported symbols are enumerated
- THEN the list includes `NewServer(*Registry)`, `(s *Server) WithDispatcher`, `(s *Server) WithRateLimiter`, `(s *Server) Router(authMW func(http.Handler) http.Handler) http.Handler`
- AND it does NOT include `ContextWithUserID`, `UserIDFromContext`, or `AuthUserIDKey` (these are deleted; consumers use `mcpctx.X` instead)

#### Scenario: RateLimiter API preserved

- GIVEN the runtime has moved
- WHEN the rate limiter's exported symbols are enumerated
- THEN the list includes `NewRateLimiter`, `WithClock`, `Allow`, `Middleware`, the constants `DefaultRateLimit` and `DefaultRateWindow`, the helper functions `IsRateLimitedError`, `NewRateLimitedError`

#### Scenario: mcpctx symbols preserved

- GIVEN the mcpctx package has moved
- WHEN `grep -rn "^func ContextWithUserID\|^func UserIDFromContext\|^var ErrInvalidParams\|^type AuthUserIDKey" backend/internal/mcpkit/mcpctx/` is run
- THEN it returns exactly four matches — `ContextWithUserID`, `UserIDFromContext`, `ErrInvalidParams`, `AuthUserIDKey` — all in `auth_context.go`

### Requirement: Relocated Bridge Test

The end-to-end transport test currently named `http_transport_test.go` (502 lines, `package mcp_test`) SHALL be relocated to `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go` with `package dictionary_test`. The test exercises the dictionary tool group wired through the MCP server router; it is semantically a dictionary test, not a runtime test.

#### Scenario: bridge test lives in dictionary/

- GIVEN this change has been applied
- WHEN `ls backend/internal/infrastructure/mcp/dictionary/` is run
- THEN the file `http_transport_test.go` is present alongside `dictionary_tool.go`, `dictionary_tool_test.go`, `errors.go`

#### Scenario: bridge test no longer in old location

- GIVEN this change has been applied
- WHEN `ls backend/internal/infrastructure/mcp/*.go` is run
- THEN no file named `http_transport_test.go` exists in the old location (it has been moved)

#### Scenario: bridge test runs under package dictionary_test

- GIVEN the bridge test has been relocated
- WHEN `go test ./internal/infrastructure/mcp/dictionary/...` is run
- THEN the bridge test compiles and runs as `package dictionary_test`, exercising the four dictionary tools through `mcpkit.Server.Router(...)`

### Requirement: Dead Re-export Removal (with Grep Evidence)

The `mcpkit` package SHALL NOT re-export the four aliases that the old `mcp` package carried: `ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`, `ErrInvalidParams`. These are reachable via `mcpkit/mcpctx.X` only.

#### Scenario: grep proves zero consumers of the deleted re-exports

- GIVEN this change has been applied
- WHEN `grep -rn 'mcp\.\(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams\)' backend/` is run from the repo root
- THEN the output is empty — no production code or test code outside the deleted re-export declarations themselves imports any of the four aliases through the `mcp` package alias

#### Scenario: server.go and dispatcher.go no longer declare the re-exports

- GIVEN this change has been applied
- WHEN `grep -n "ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams" backend/internal/mcpkit/server.go backend/internal/mcpkit/dispatcher.go` is run
- THEN the only matches are references to `mcpctx.X` (the canonical helpers in the mcpctx package), not bare `X` or `var X = mcpctx.X` declarations

#### Scenario: tool groups still resolve ErrInvalidParams via mcpctx

- GIVEN the re-exports have been deleted
- WHEN the dictionary and tutor tool groups' `ErrInvalidParams = mcpctx.ErrInvalidParams` declarations are inspected
- THEN both tool groups continue to declare their own local aliases pointing at `mcpctx.ErrInvalidParams` (unchanged behavior); the dispatcher's `errors.Is` check uses `mcpctx.ErrInvalidParams` directly

### Requirement: No New External Dependency

This change SHALL NOT introduce any new external Go module to `backend/go.mod`. After `go mod tidy` runs at the end of the change, the only allowed effect is the existing dependency set remaining stable. The `sashabaranov/go-openai` dependency SHALL remain (used by `tool.go`'s `openai.Tool` return type).

#### Scenario: go.mod unchanged after tidy

- GIVEN this change has been applied
- WHEN `cd backend && go mod tidy && git diff -- go.mod go.sum` is run
- THEN the diff is empty — no `require` line added, no `require` line dropped, no version bump

#### Scenario: sashabaranov/go-openai still required

- GIVEN the change has been applied
- WHEN `grep "sashabaranov/go-openai" backend/go.mod` is run
- THEN a `require` line is present (the dep is still declared)

### Requirement: Hexagonal Purity Preserved

The runtime refactor SHALL NOT violate the project's hexagonal architecture. `internal/mcpkit/` SHALL NOT be imported by `internal/core/*` (only `cmd/api/main.go` and tool groups may import it). The mcpkit-internal direction `mcpkit → infrastructure/driving/authmiddleware` is documented and accepted; no reverse direction SHALL be introduced.

#### Scenario: core/ does not import mcpkit

- GIVEN this change has been applied
- WHEN `grep -rn "anomalyco/codeauditor/backend/internal/mcpkit" backend/internal/core/` is run
- THEN the output is empty (no core package imports mcpkit)

#### Scenario: no reverse direction from infrastructure to mcpkit's private internals

- GIVEN this change has been applied
- WHEN the import graph is inspected
- THEN `internal/infrastructure/driving/authmiddleware` does NOT import `internal/mcpkit` (the arrow stays one-way: mcpkit → authmiddleware)

#### Scenario: tool groups implement MCPTool without importing mcpkit runtime types

- GIVEN dictionary and tutor tool groups implement `MCPTool`-shaped methods (Name/Description/Schema/Execute)
- WHEN their import statements are inspected
- THEN they import only `internal/mcpkit/mcpctx` (for the context helpers and `ErrInvalidParams`); they do NOT import `internal/mcpkit` for `MCPTool` itself
- AND they continue to satisfy `mcpkit.MCPTool` implicitly via structural typing (no explicit `implements` declaration required in Go)

### Requirement: Tracking Note for mcp-monolith Parent Spec

This change SHALL NOT edit any file under `openspec/changes/mcp-monolith/`. The parent spec's "mcp runtime" location reference (`infrastructure/mcp/`) is updated by tracking note only. The location will be updated at mcp-monolith archive time, when phases 2-5 land.

#### Scenario: mcp-monolith files unchanged

- GIVEN this change has been applied
- WHEN `git status openspec/changes/mcp-monolith/` is run
- THEN no file in that directory is modified (the parent spec stays open with its current text)