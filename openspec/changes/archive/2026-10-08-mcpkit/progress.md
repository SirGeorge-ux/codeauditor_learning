# mcpkit Apply Progress

## Branch
`feature/mcpkit` (from `feature/dictionary` at `6f70721` — the orchestrator's
prompt referenced master but master has not yet merged the dictionary commit;
the runtime that the change refactors only exists on `feature/dictionary`).
Flagged for orchestrator awareness.

## Phase A — Mechanical move (atomic, one commit, PENDING)
- `git mv` of 9 runtime files + `mcpctx/` dir into `backend/internal/mcpkit/`
  (preserves rename detection; `git status` shows `R` for all 11).
- `git mv` of `http_transport_test.go` from `infrastructure/mcp/` into
  `infrastructure/mcp/dictionary/` (Phase 4 task).
- `package mcp` → `package mcpkit` in 9 files (5 runtime + 4 test).
- `package mcp_test` → `package dictionary_test` in the relocated bridge test.
- Import path rewrites:
  - `internal/infrastructure/mcp` → `internal/mcpkit`
  - `internal/infrastructure/mcp/mcpctx` → `internal/mcpkit/mcpctx`
- Symbol rewrites `mcp.X` → `mcpkit.X`:
  - `cmd/api/main.go`: `mcp.NewRegistry`, `mcp.NewServer` (2 sites)
  - `dictionary/dictionary_tool.go`: comments only (`mcp.Server`, `mcp.Registry`, `mcp.MCPTool`)
  - `tutor/tutor_tool.go`: comments only (`mcp.Server`)
  - `dictionary/http_transport_test.go`: `mcp.NewRegistry`, `mcp.NewServer`,
    `mcp.CodeInvalidParams` (3 sites + comments)
  - `tutor/tutor_http_transport_test.go`: `mcp.NewRegistry`, `mcp.NewServer` (2 sites)
- `mcpkit/mcpctx/auth_context.go`: package name stays `mcpctx`; only comments updated.

### Verification (gate before commit)
- `go build ./...` → exit 0 (no errors)
- `go test -count=1 ./internal/...` → 11/11 ok, no FAIL
- `go test -count=1 ./...` → 11/11 ok, no FAIL
- `gofmt -l backend/internal/mcpkit/ backend/internal/infrastructure/mcp/` → empty
- `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` → empty

## Phase B — Re-export cleanup (next commit)
- DELETE the 4 re-exports in `mcpkit/server.go`:
  - `AuthUserIDKey` (type alias of `mcpctx.AuthUserIDKey`)
  - `ContextWithUserID` (function re-export)
  - `UserIDFromContext` (function re-export)
  - Update `httpContextBridge` (line 100) to call `mcpctx.ContextWithUserID` directly.
- DELETE the `ErrInvalidParams` re-export in `mcpkit/dispatcher.go`:
  - Remove `var ErrInvalidParams = mcpctx.ErrInvalidParams` (lines 159-163)
  - Update `errors.Is(err, ErrInvalidParams)` at line 152 to `mcpctx.ErrInvalidParams`.

### Verification (gate before commit)
- `go build ./...` → exit 0
- `go test -count=1 ./internal/...` → all ok
- `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` → clean
- Grep `mcp\.\(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams\)` → empty

## Phase C — Aggregate verify (evidence)
- `go build ./...` → exit 0
- `go test -count=1 ./...` → all 11 ok
- `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` → clean
- `go mod tidy` review: confirm `sashabaranov/go-openai` remains in `go.mod`