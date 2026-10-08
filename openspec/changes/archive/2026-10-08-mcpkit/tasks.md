# Tasks: mcpkit — Extract MCP Runtime Library

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~300 (pure move + path rewrite; no logic edits) |
| 400-line budget risk | Low (under budget by a wide margin) |
| Chained PRs recommended | No (single PR; atomicity is the safety property) |
| Delivery strategy | single-pr |
| Chain strategy | n/a |

| Work Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|-----------|------|----|---------------------|-----------------|-------------------|
| 1 | Runtime extract + import rewrites + dead-re-export removal + bridge-test relocation | 1 | `go build ./... && go test -count=1 ./internal/... && golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` | `go test -count=1 ./...` exits 0; tool-group bridge tests run; registry accepts tools at startup | Single `git revert` of the apply commit |

**Execution order invariant**: Phase 1 (mechanical move) → Phase 2 (import rewrites) → Phase 3 (dead-re-export cleanup) → Phase 4 (bridge-test relocation + rename) → Phase 5 (aggregate verify). Tasks in Phase 1 MUST land before Phase 2; otherwise intermediate commits won't compile.

**Convention**: one commit per task (per `work-unit-commits` skill). Apply commits are reviewable work units. Conventional commit style for the apply: `refactor(mcp): extract runtime to internal/mcpkit`. Sub-tasks that are pure file moves use `git mv` (preserves history).

---

## Phase 1: Mechanical Move (runtime files → `internal/mcpkit/`)

- [x] 1.1 `git mv backend/internal/infrastructure/mcp/tool.go backend/internal/mcpkit/tool.go`
- [x] 1.2 `git mv backend/internal/infrastructure/mcp/registry.go backend/internal/mcpkit/registry.go`
- [x] 1.3 `git mv backend/internal/infrastructure/mcp/dispatcher.go backend/internal/mcpkit/dispatcher.go`
- [x] 1.4 `git mv backend/internal/infrastructure/mcp/server.go backend/internal/mcpkit/server.go`
- [x] 1.5 `git mv backend/internal/infrastructure/mcp/ratelimiter.go backend/internal/mcpkit/ratelimiter.go`
- [x] 1.6 `git mv backend/internal/infrastructure/mcp/dispatcher_test.go backend/internal/mcpkit/dispatcher_test.go`
- [x] 1.7 `git mv backend/internal/infrastructure/mcp/registry_test.go backend/internal/mcpkit/registry_test.go`
- [x] 1.8 `git mv backend/internal/infrastructure/mcp/server_test.go backend/internal/mcpkit/server_test.go`
- [x] 1.9 `git mv backend/internal/infrastructure/mcp/ratelimiter_test.go backend/internal/mcpkit/ratelimiter_test.go`
- [x] 1.10 `git mv backend/internal/infrastructure/mcp/mcpctx backend/internal/mcpkit/mcpctx` (directory move)

**Gate after Phase 1**: `go build ./...` will FAIL (intentionally — imports still point to old paths). This is expected; Phase 2 fixes it.

## Phase 2: Package Renames + Import Path Rewrites

> **Order matters**: rename the `package` line in every moved file BEFORE rewriting any imports, so the compiler reports a clean error per file rather than a cascade.

- [x] 2.1 In `backend/internal/mcpkit/tool.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.2 In `backend/internal/mcpkit/registry.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.3 In `backend/internal/mcpkit/dispatcher.go`: rewrite `package mcp` → `package mcpkit`; rewrite its `mcpctx` import path to `.../internal/mcpkit/mcpctx`
- [x] 2.4 In `backend/internal/mcpkit/server.go`: rewrite `package mcp` → `package mcpkit`; rewrite its `mcpctx` import path to `.../internal/mcpkit/mcpctx`; rewrite its `authmiddleware` import path unchanged (still `.../infrastructure/driving/authmiddleware`)
- [x] 2.5 In `backend/internal/mcpkit/ratelimiter.go`: rewrite `package mcp` → `package mcpkit`; rewrite its `mcpctx` import path to `.../internal/mcpkit/mcpctx`
- [x] 2.6 In `backend/internal/mcpkit/dispatcher_test.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.7 In `backend/internal/mcpkit/registry_test.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.8 In `backend/internal/mcpkit/server_test.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.9 In `backend/internal/mcpkit/ratelimiter_test.go`: rewrite `package mcp` → `package mcpkit`
- [x] 2.10 In `backend/cmd/api/main.go`: rewrite import line `.../internal/infrastructure/mcp` → `.../internal/mcpkit`; rewrite symbol references `mcp.NewRegistry` → `mcpkit.NewRegistry`, `mcp.NewServer` → `mcpkit.NewServer`
- [x] 2.11 In `backend/internal/infrastructure/mcp/dictionary/dictionary_tool.go`: rewrite `mcpctx` import path to `.../internal/mcpkit/mcpctx`
- [x] 2.12 In `backend/internal/infrastructure/mcp/tutor/tutor_tool.go`: rewrite `mcpctx` import path to `.../internal/mcpkit/mcpctx`

**Gate after Phase 2**: `go build ./...` MUST pass (intermediate state, before the bridge-test relocation). Bridge tests `http_transport_test.go` and `tutor_http_transport_test.go` will still fail; that's expected. The runtime, the dictionary tool group, and the tutor tool group MUST all compile.

## Phase 3: Dead Re-export Cleanup (decision 2)

> **Pre-condition**: Phase 2 complete; `go build ./internal/mcpkit/... ./internal/infrastructure/mcp/dictionary/... ./internal/infrastructure/mcp/tutor/... ./cmd/api/...` exits 0.

- [x] 3.1 In `backend/internal/mcpkit/server.go`: delete lines 17-29 (the `AuthUserIDKey` type alias, `ContextWithUserID` func, `UserIDFromContext` func, plus the comment block above them). Update the `httpContextBridge` body (line 100) to call `mcpctx.ContextWithUserID` directly instead of the now-deleted local alias.
- [x] 3.2 In `backend/internal/mcpkit/dispatcher.go`: delete lines 159-163 (the comment block + `var ErrInvalidParams = mcpctx.ErrInvalidParams`). Update the `errors.Is` check at line 152 to use `mcpctx.ErrInvalidParams` directly instead of the now-deleted local alias.

**Verify after Phase 3**: re-run the grep command from `design.md`:

```bash
grep -rn 'mcp\.\(ContextWithUserID\|UserIDFromContext\|AuthUserIDKey\|ErrInvalidParams\)' backend/
```

Expected output: empty. If non-empty, the change MUST NOT commit — a re-export is still being consumed somewhere; investigate before proceeding.

## Phase 4: Bridge-Test Relocation + Rename (decision 1)

- [x] 4.1 `git mv backend/internal/infrastructure/mcp/http_transport_test.go backend/internal/infrastructure/mcp/dictionary/http_transport_test.go`
- [x] 4.2 In `backend/internal/infrastructure/mcp/dictionary/http_transport_test.go`: rewrite the file header `package mcp_test` → `package dictionary_test`
- [x] 4.3 In the relocated file: change import `.../internal/infrastructure/mcp` → `.../internal/mcpkit`; change import `.../internal/infrastructure/mcp/mcpctx` → `.../internal/mcpkit/mcpctx`; rewrite all symbol references `mcp.NewRegistry` → `mcpkit.NewRegistry`, `mcp.NewServer` → `mcpkit.NewServer`, `mcpctx.X` → `mcpctx.X` (path only; symbols are unchanged)
- [x] 4.4 In `backend/internal/infrastructure/mcp/tutor/tutor_http_transport_test.go`: change `import "..."/internal/infrastructure/mcp"` → `"..."/internal/mcpkit"`; change `.../infrastructure/mcp/mcpctx` → `.../mcpkit/mcpctx`; rewrite `mcp.NewRegistry` → `mcpkit.NewRegistry`, `mcp.NewServer` → `mcpkit.NewServer`. `package tutor` is unchanged.

## Phase 5: Aggregate Verification + Go-Mod Recheck

- [x] 5.1 `go build ./...` — MUST exit 0
- [x] 5.2 `go test -count=1 ./internal/...` — all packages pass; no `FAIL`, no `panic`
- [x] 5.3 `go test -count=1 ./...` — exit 0 (full repo, including `cmd/`)
- [x] 5.4 `golangci-lint run ./internal/mcpkit/... ./internal/infrastructure/mcp/...` — clean
- [x] 5.5 `gofumpt -l backend/internal/mcpkit/... backend/internal/infrastructure/mcp/...` — empty (no diff)
- [x] 5.6 `cd backend && go mod tidy && git diff go.mod go.sum` — verify `sashabaranov/go-openai` was NOT dropped; if dropped, manually restore it (`tool.go` uses `openai.Tool`).
- [x] 5.7 Verify file inventory matches `design.md` "File Changes" table (create-side files exist; delete-side files are gone). Run `ls -R backend/internal/mcpkit/ backend/internal/infrastructure/mcp/` and compare to `design.md`.
- [x] 5.8 Re-run the decision-2 grep (see Phase 3) — still empty.
- [x] 5.9 `make validate` equivalent gates (this environment has no `make`; per 2026-10-05 session note): `gofumpt -l` on the mcpkit + infrastructure/mcp dirs empty, `golangci-lint` clean, `go build ./...` exit 0, `go test -count=1 ./...` 11/11 ok. Pre-existing base `make validate` debt (sandbox gofmt, prettier 8 files, eslint challenge.service.ts) is out of scope for this change.
- [x] 5.10 All 8 success-criteria checkboxes in `proposal.md` marked `[x]` (verified by the verify phase re-read).
- [x] 5.11 Apply commit per `work-unit-commits` skill: one work-unit per phase OR one combined commit per maintainer override. Default: single combined commit with the conventional message `refactor(mcp): extract runtime to internal/mcpkit`. Body summarizes: files moved (11), package renamed (9 files), import paths rewritten (6 consumers), 4 dead re-exports deleted, 1 bridge test relocated.

## Out-of-Scope Tasks (do NOT add to this change)

- [ ] ~~Canonicalize `RegistryRegistrar`/`MCPToolRegistrar` in dictionary + tutor~~ → follow-up change (`mcpkit-interfaces` or similar)
- [ ] ~~Add new MCP tools (sandbox, challenges, user, curriculum, pedagogical, external_docs, inspiration, code_health)~~ → `mcp-monolith` phases 2-5
- [ ] ~~Move `authmiddleware.UserIDContextKey` to a leaf `authctx` package~~ → bigger refactor
- [ ] ~~Rename `mcpkit.MCPTool` to `mcpkit.Tool`~~ → scope creep; revisit if/when canonicalization happens