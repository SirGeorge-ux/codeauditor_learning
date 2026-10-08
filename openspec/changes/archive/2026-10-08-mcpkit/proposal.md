# Proposal: mcpkit — Extract MCP Runtime Library

## Intent

The MCP runtime (`tool.go`, `registry.go`, `dispatcher.go`, `server.go`, `ratelimiter.go`, `mcpctx/`) currently lives inside `backend/internal/infrastructure/mcp/` alongside the tool groups (dictionary, tutor, ...). This colocates two unrelated layers — a shared **library** that every future tool group will import, and the **tool-group consumers** themselves — under one path. Future work on the planned Gogs fork will speak the same MCP protocol; both academy-mic and the fork need to import a stable runtime, but neither should import each other's tool groups.

This change carves the runtime out into a sibling library `internal/mcpkit/`, leaving the tool groups in place under `infrastructure/mcp/<group>/`. After this lands, `mcpkit` becomes the canonical Go package for the MCP runtime, importable by any tool group, by `cmd/api/main.go`, and (in the future) by external consumers.

## Scope

### In Scope

- Move `tool.go`, `registry.go`, `dispatcher.go`, `server.go`, `ratelimiter.go` from `backend/internal/infrastructure/mcp/` to the new `backend/internal/mcpkit/`.
- Move `mcpctx/auth_context.go` from `backend/internal/infrastructure/mcp/mcpctx/` to `backend/internal/mcpkit/mcpctx/` (package `mcpctx` unchanged).
- Move the four runtime test files (`dispatcher_test.go`, `registry_test.go`, `server_test.go`, `ratelimiter_test.go`) into `internal/mcpkit/`; rename their package from `mcp` to `mcpkit`.
- Relocate `http_transport_test.go` from `internal/infrastructure/mcp/` to `internal/infrastructure/mcp/dictionary/`; rename package from `mcp_test` to `dictionary_test` (the test exercises dictionary + transport, semantically a dictionary-bridge test, not a runtime test).
- Rewrite the runtime import path in every consumer: `cmd/api/main.go`, `dictionary/*.go`, `tutor/*.go`, and the two bridge test files.
- Update the `mcpctx` import path in every consumer (same consumers as above; the mcpctx path is `.../mcpkit/mcpctx`).
- Delete the four dead re-exports in `server.go` (`ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`) and the `ErrInvalidParams` re-export in `dispatcher.go` after grep confirms zero external consumers (grep evidence in `design.md`).

### Out of Scope

- No new MCP tools. The 27 tools pending in `mcp-monolith` phases 2-5 stay as separate work.
- No interface canonicalization. The `RegistryRegistrar`/`MCPToolRegistrar` duplicates in dictionary/tutor are a follow-up cleanup, not in this change.
- No changes to tool-group logic. Tool groups keep the same wire surface; only their import paths change.
- No removal of the re-export semantics for tool groups. Each tool group already declares its own `ErrInvalidParams = mcpctx.ErrInvalidParams` alias locally (dictionary, error; tutor; change did not). The mcpctx sentinel is the single source of truth; the tool-group local aliases stay.
- No `go.mod` additions. `sashabaranov/go-openai` remains; `go mod tidy` MUST NOT drop it (recheck after the move: `tool.go` uses `openai.Tool`).
- No changes to the mcp-monolith parent spec. A tracking note is added in this proposal's "Affected Areas"; mcp-monolith stays open until phases 2-5 land.

## Capabilities

### New Capabilities

- `mcpkit`: the shared MCP runtime library — `MCPTool` interface, `Registry`, `Dispatcher` (JSON-RPC 2.0), `Server` HTTP driving adapter, `RateLimiter`, and the `mcpctx` context-key package. Consumable by any tool group via a single Go import.

### Modified Capabilities

- None (mcpkit is structural; runtime behavior is preserved bit-for-bit).

### Affected Capabilities (carry forward, no edits to their specs)

- `dictionary` (already archived). Its tool group keeps the same behavior; only the `mcpctx` import path changes from `.../infrastructure/mcp/mcpctx` to `.../mcpkit/mcpctx`.
- `mcp` parent spec in `mcp-monolith` — tracking note only. The runtime leaves the `infrastructure/mcp/` dir; the spec's "mcp runtime" location is updated by reference, not by edit (mcp-monolith stays open; the delta goes into this change's `design.md` and is applied at archive time).

## Approach

Mechanically move every runtime file from `backend/internal/infrastructure/mcp/` to the new `backend/internal/mcpkit/`, rename `package mcp` → `package mcpkit` in the five runtime files and the four runtime test files, relocate the `mcpctx` package to its new home (`backend/internal/mcpkit/mcpctx/`), update the import path in every consumer (`cmd/api/main.go`, `dictionary/*`, `tutor/*`, and the bridge test files), relocate `http_transport_test.go` to `dictionary/` with `package dictionary_test`, and delete the four dead re-exports in the runtime (`ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`, `ErrInvalidParams`) after grep proves zero external use. Tool-group interfaces (`RegistryRegistrar`/`MCPToolRegistrar`) are NOT canonicalized; that is a follow-up. No `go.mod` additions. Gate: `go build ./...`, `go test -count=1 ./internal/...`, `go test -count=1 ./...`, `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` all clean.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/mcpkit/` | **New** | Created as sibling of `infrastructure/`. Houses the extracted runtime + `mcpctx/`. |
| `backend/internal/mcpkit/tool.go` | New (moved) | `MCPTool` interface; `package mcpkit`. |
| `backend/internal/mcpkit/registry.go` | New (moved) | Thread-safe `Registry`; `package mcpkit`. |
| `backend/internal/mcpkit/dispatcher.go` | New (moved) | JSON-RPC 2.0 `Dispatcher`, `RPCError`, error-code constants, sanitization. **Drops `ErrInvalidParams` re-export** (zero consumers). |
| `backend/internal/mcpkit/server.go` | New (moved) | `Server`, `Router`, `httpContextBridge`. **Drops `ContextWithUserID`/`UserIDFromContext`/`AuthUserIDKey` re-exports** (zero consumers). |
| `backend/internal/mcpkit/ratelimiter.go` | New (moved) | `RateLimiter`, `Middleware`, `IsRateLimitedError`. |
| `backend/internal/mcpkit/mcpctx/auth_context.go` | New (moved) | `package mcpctx` unchanged. |
| `backend/internal/mcpkit/*_test.go` | New (moved) | 4 runtime test files; `package mcpkit`. |
| `backend/internal/infrastructure/mcp/` | Modified | Now contains ONLY tool-group subdirs (`dictionary/`, `tutor/`). Runtime files gone. |
| `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go` | New (relocated) | Moved from `infrastructure/mcp/http_transport_test.go`. `package dictionary_test`. |
| `backend/internal/infrastructure/mcp/dictionary/dictionary_tool.go` | Modified | `mcpctx` import path rewrite only. |
| `backend/internal/infrastructure/mcp/tutor/tutor_tool.go` | Modified | `mcpctx` import path rewrite only. |
| `backend/internal/infrastructure/mcp/tutor/tutor_http_transport_test.go` | Modified | Runtime + `mcpctx` import path rewrite; symbol rename `mcp.X` → `mcpkit.X`. |
| `backend/cmd/api/main.go` | Modified | Runtime import path rewrite; `mcp.NewRegistry/NewServer` → `mcpkit.NewRegistry/NewServer`. |
| `backend/go.mod` | Unchanged | No new deps; recheck `sashabaranov/go-openai` after `tidy`. |
| `openspec/changes/mcp-monolith/` | Tracking note | Parent spec's "mcp runtime" location is now `internal/mcpkit/`. Documented in this change's `design.md`; no edit at this time (mcp-monolith stays open). |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Package rename breaks `go build ./...` (typo in import path) | Med | Mechanical + scripted; gate with `go build ./...` after every batch move; bisect by package if it breaks. |
| Hidden consumer of the four deleted re-exports (`ContextWithUserID`/`UserIDFromContext`/`AuthUserIDKey`/`ErrInvalidParams`) breaks the build | Low | Definitive grep `mcp\.(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams)` in `design.md` — only the comment in `dispatcher.go:160` matches. No code path. |
| `go mod tidy` drops `sashabaranov/go-openai` if no symbol is referenced post-rename | Low | `tool.go` keeps `openai.Tool` (schema return type). Recheck `go.mod` after `tidy`; re-add manually if dropped. |
| `http_transport_test.go` relocated wrong (still imports `package mcp`) | Low | Move file + rewrite the `package` line + rewrite both the `mcp` import and `mcp.X` symbol references in one batch. Gate with `go test ./internal/infrastructure/mcp/dictionary/...`. |
| Interface canonicalization tempting scope creep | Med | Hard scope fence in `tasks.md`: tasks beyond mechanical move + dead-re-export removal + test relocation require a new change. |
| `mcp-monolith` parent spec out of sync | Low | Tracking note in `design.md`. Sync happens at mcp-monolith archive time (phases 2-5 land). |
| Tool groups accidentally couple to mcpkit internals | Low | Only public surface (`MCPTool`, `Registry`, `NewRegistry`, `Server`, `NewServer`, `Dispatcher`, `NewDispatcher`, `RateLimiter`, `NewRateLimiter`, `RateLimitedError`, `NewRateLimitedError`, `IsRateLimitedError`, error-code constants, `RPCError`, `TruncateResult`, `SanitizeResult`, `SanitizeErrorMessage`, `MaxResponseBytes`, `DefaultRateLimit`, `DefaultRateWindow`) is intended for tool groups. Document in `design.md`. |

## Rollback Plan

1. **Code rollback**: `git revert` the single apply commit (per the work-unit-commits skill, the apply commits are reviewable work units; one revert undoes the move + import rewrites).
2. **No DB rollback** needed: mcpkit is pure Go library extraction; no migrations, no schema, no data.
3. **No frontend rollback** needed: zero frontend changes; only Go-side imports and packages change.
4. **No env / config rollback** needed: no `.env` changes, no deployment config changes.
5. **Re-exports re-add path**: if a consumer is later discovered that needs the deleted re-exports, add them back as aliases in `mcpkit` (one-line change per symbol; ~10 lines total).

## Dependencies

- Base branch: `master` + the freshly merged Dictionary commit (`6f70721`) per the explore's risk register.
- No new external Go modules.
- The `mcp-monolith` parent spec stays open (phases 2-5 still pending). This change is the foundation it builds on; it does not close any mcp-monolith tasks.

## Success Criteria

- [x] `internal/mcpkit/` exists with all runtime files (`tool.go`, `registry.go`, `dispatcher.go`, `server.go`, `ratelimiter.go`, `mcpctx/auth_context.go`) and four test files; package name `mcpkit` everywhere; tests pass.
- [x] `internal/infrastructure/mcp/` contains only the tool groups (`dictionary/`, `tutor/`) plus any future groups; no runtime files remain in this directory.
- [x] `cmd/api/main.go` and all tool groups import `.../internal/mcpkit` (not `.../infrastructure/mcp`) for runtime symbols; tool-group imports of their siblings (`.../mcp/dictionary`, `.../mcp/tutor`) remain unchanged.
- [x] `go build ./...` exit 0; `go test -count=1 ./internal/...` all packages pass; `go test -count=1 ./...` exit 0.
- [x] `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` clean.
- [x] `http_transport_test.go` lives in `dictionary/` (not in mcpkit and not in the old runtime dir); its package is `dictionary_test`.
- [x] The four deleted re-exports (`ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`, `ErrInvalidParams`) have zero external consumers (grep evidence captured in `design.md`).
- [x] No new external dependency added to `go.mod`. `sashabaranov/go-openai` remains if `go mod tidy` runs.