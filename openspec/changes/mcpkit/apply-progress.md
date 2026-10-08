# Apply Progress — mcpkit

## Branch
`feature/mcpkit` (created from `feature/dictionary` at `6f70721`).

> **Note for orchestrator**: the prompt said "from master", but `master` is at
> `3ef8f32` (no dictionary commit, no `infrastructure/mcp/` directory). The
> runtime that this change refactors exists only on `feature/dictionary`
> (the S2 dictionary commit). Branching from `feature/dictionary` is the only
> way to operate on the runtime — branching from `master` would result in an
> empty apply. Two valid follow-ups: (a) fast-forward `master` to include
> the dictionary commit before merging `feature/mcpkit`, or (b) accept this
> branch's divergence and rebase later.

## Commits

| Phase | SHA | Subject |
|-------|-----|---------|
| A — mechanical move | `306e2ce666a0d362edffc619d3c2833cdaf0fc43` | `refactor(mcp): extract runtime to internal/mcpkit` |
| B — re-export cleanup | `a8ac3c1` (resolved locally) | `refactor(mcp): drop dead re-exports from mcpkit` |
| Tracking | `0951635` | `docs(openspec): mark mcpkit apply tasks complete` |

The first two are the work-unit commits the prompt specified. The third is
the openspec persistence artifact (tasks/proposal checkboxes per the sdd-apply
openspec-mode contract) — no code change.

## Tasks completed (openspec/changes/mcpkit/tasks.md)

- Phase 1 (1.1–1.10): all 10 mechanical `git mv` operations.
- Phase 2 (2.1–2.12): all 12 package renames + import path rewrites.
- Phase 3 (3.1, 3.2): all 4 re-exports deleted.
- Phase 4 (4.1–4.4): bridge-test relocated + package renamed + symbol rewrites.
- Phase 5 (5.1–5.8, 5.11): aggregate verify pass.
- Phase 5.9 (make validate): **left unchecked**. The two files I touched
  (`./internal/mcpkit/...`, `./internal/infrastructure/mcp/...`) pass
  `gofumpt -l` and `golangci-lint run` clean. Pre-existing gofumpt drift
  in `internal/infrastructure/driven/sandbox/{dockersandbox,localsandbox}.go`
  predates this branch (commit `39c35a9`, 2026-06-22) and is out of scope.

## Phase A — Verification (gate passed before commit)

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | exit 0 |
| Internal tests | `go test -count=1 ./internal/...` | 11/11 ok |
| Full tests | `go test -count=1 ./...` | 11/11 ok |
| Format | `gofmt -l internal/mcpkit/ internal/infrastructure/mcp/` | empty |
| Lint | `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` | empty |

## Phase B — Verification (gate passed before commit)

| Gate | Command | Result |
|------|---------|--------|
| Build | `go build ./...` | exit 0 |
| Tests | `go test -count=1 ./internal/...` | 11/11 ok |
| Lint | `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` | empty |
| Decision-2 grep | `grep -rn --include='*.go' 'mcp\.\(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams\)' backend/` | empty (exit 1) |
| mcpctx only | `grep -n 'ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams' internal/mcpkit/{server,dispatcher}.go` | 2 matches, both `mcpctx.X` references |
| Hexagonal purity | `grep -rn 'anomalyco/codeauditor/backend/internal/mcpkit' backend/internal/core/` | empty |

Decision-2 grep evidence captured at `openspec/changes/mcpkit/grep-after-cleanup.txt`.

## Phase C — Aggregate verify

| Check | Result |
|-------|--------|
| `go build ./...` | exit 0 |
| `go test -count=1 ./...` | 11/11 ok |
| `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` | clean |
| `go mod tidy` | No diff in `go.mod`/`go.sum` after revert (a `// indirect`→direct comment fix for `sashabaranov/go-openai v1.41.2` was reverted to keep the refactor diff minimal; the dep remains, tool.go still uses `openai.Tool` directly) |
| File inventory vs `design.md` "File Changes" | matches |

## mcp.X → mcpkit.X rewrites

The prompt asked for a list of every rewrite. Each was a deliberate edit, not
a batch sed transform; count and breakdown below.

| File | Rewrites | Sites |
|------|----------|-------|
| `backend/cmd/api/main.go` | import line + `mcp.NewRegistry` + `mcp.NewServer` | 3 sites |
| `backend/internal/infrastructure/mcp/dictionary/dictionary_tool.go` | comments only (`mcp.Server`, `mcp.Registry`, `mcp.MCPTool`) + `mcpctx` import | 4 sites |
| `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go` | `package mcp_test`→`dictionary_test`, 2 imports, `mcp.NewRegistry`, `mcp.NewServer`, `mcp.CodeInvalidParams` (×2), comments | 9 sites |
| `backend/internal/infrastructure/mcp/tutor/tutor_tool.go` | `mcpctx` import + 3 comments (`mcp.Server`) | 4 sites |
| `backend/internal/infrastructure/mcp/tutor/tutor_http_transport_test.go` | 2 imports + `mcp.NewRegistry` + `mcp.NewServer` | 4 sites |
| `backend/internal/mcpkit/mcpctx/auth_context.go` | comments only | 2 sites |
| `backend/internal/mcpkit/dispatcher.go` | `package mcp`→`mcpkit` + `mcpctx` import + internal `errors.Is(ErrInvalidParams)`→`mcpctx.ErrInvalidParams` | 3 sites |
| `backend/internal/mcpkit/server.go` | `package mcp`→`mcpkit` + `mcpctx` import + `ContextWithUserID`→`mcpctx.ContextWithUserID` in `httpContextBridge` | 3 sites |
| `backend/internal/mcpkit/{tool,registry,ratelimiter}.go` | `package mcp`→`mcpkit` | 3 sites |
| 4 test files (dispatcher_test, registry_test, server_test, ratelimiter_test) | `package mcp`→`mcpkit` | 4 sites |

No `mcp.X` symbol reference survived the rename in source code outside
the deleted re-export block. Verified via `grep -rn --include='*.go'
'\bmcp\.(NewRegistry|NewServer|NewDispatcher|NewRateLimiter|MCPTool|Registry|
Dispatcher|RateLimiter|RPCError|CodeInvalidParams|CodeParseError|CodeInvalidRequest|
CodeMethodNotFound|CodeInternalError|CodeRateLimited|MaxResponseBytes|DefaultRateLimit|
DefaultRateWindow|IsRateLimitedError|NewRateLimitedError|TruncateResult|SanitizeResult|
SanitizeErrorMessage|ErrToolNotFound|ErrInvalidParams|ContextWithUserID|
UserIDFromContext|AuthUserIDKey)' backend/` → only the post-cleanup
comment line that was deleted in commit B.

## Deviations from design.md

None. Implementation matches every decision.

## Issues encountered

- `master` does not contain `6f70721` (the dictionary commit) — the runtime
  that this change refactors only exists on `feature/dictionary`. Branched
  from `feature/dictionary` accordingly. See Branch note above.
- Pre-existing gofumpt drift in `internal/infrastructure/driven/sandbox/*.go`
  predates this branch (`39c35a9`, 2026-06-22). Out of scope; not caused
  by this refactor.
- Untracked files in the working tree (curriculum_service.go, brave/,
  context7/, etc.) belong to other changes; not staged or committed.

## Risks for verify

- None known. The refactor is mechanical; the test suite is the green gate,
  and all 11 packages pass. Behavior is preserved.

## Suggested follow-up changes

- Merge `master` → `feature/mcpkit` once `feature/dictionary` lands on
  master (per Branch note).
- `mcpkit-interfaces` (out of scope per design.md Decision 3): canonicalize
  `RegistryRegistrar`/`MCPToolRegistrar` duplicates in dictionary and tutor.
- `mcp-monolith` archive (out of scope per design.md): at archive time,
  update parent spec's "mcp runtime" location reference from
  `infrastructure/mcp/` → `internal/mcpkit/`.

## Next recommended step

`sd verify` — independent verification against the proposal/design/spec/tasks.