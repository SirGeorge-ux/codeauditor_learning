# Apply Progress — code-health-dashboard (S4) PR 1

> **Phase:** 1 (Backend Foundation)
> **Branch:** `feature/code-health-pr1` (base: `master @ 9af1407`)
> **Status:** ✅ all 20 tasks done; gates green; NOT pushed (user handles `git merge --no-ff`)
> **Date:** 2026-10-11

---

## Summary

PR 1 ships the backend foundation of the S4 code-health dashboard per the
`code-health-dashboard` change. The PR lands the **complete** Phase 1 scope from
`tasks.md` (tasks 1.1–1.20): port interfaces, GitHub + extended Gogs source
clients, 6 seeded code-health rules, Supabase migration `010_*` (3 new tables +
`source_issue_id` on `challenges`), Supabase `CodeHealthRepository` +
`AnalysisCache`, the per-file analyzer with prompt-injection + token-leakage
threat tests, the `CodeHealthService` with async lifecycle, the suppressions
migration `010b_` + repository extension + analyzer skip (the resolved
`ask-on-risk` user gate), and the multi-source wiring in `cmd/api/main.go`.

The PR is **backend-only** — no frontend, no MCP `mcp-code-health/*` tools
(those land in PR 2). Stacked-to-main chain shape is preserved.

---

## Commits

| # | SHA | Title |
|---|-----|-------|
| 1 | `8f5bdd1` | feat(ports+domain): add CodeHealth port interfaces and domain models |
| 2 | `e259002` | feat(rules): seed 6 code-health detection rules + registry |
| 3 | `5a756f1` | feat(github+gogs): add GitHubClient + extend GogsClient with ListFiles |
| 4 | `f7d6ed5` | chore(migration): add 010_create_code_health + source_issue_id on challenges |
| 5 | `91dd2e7` | feat(supabase): add CodeHealthRepository with sqlmock coverage |
| 6 | `f54bad4` | feat(supabase): add AnalysisCache adapter (7d TTL, ON CONFLICT overwrite) |
| 7 | `fb9abae` | feat(services): add code-health analyzer with threat tests (1.11 + 1.12) |
| 8 | `1bbbc69` | feat(services): add CodeHealthService with async lifecycle + report |
| 9 | `12431f1` | feat(wiring): wire CodeHealthService in main.go with multi-source dispatch |
| 10 | `c0c1fd2` | chore(migration): add 010b_create_suppressions for REQ-CODE-HEALTH-008 feedback |
| 11 | `457cf38` | test(analyzer): add round-trip suppression coverage (task 1.20) |
| 12 | `acde201` | chore: pass golangci-lint + gofumpt (task 1.15) |

Total: **12 commits**, **6,204 lines added** across 34 files. (Well over the
4 00-line review budget; the orchestrator pre-cleared `ask-on-risk` for this
3-PR stacked-to-main chain — S3 was comparable in size and used the same
shape.)

---

## Gates green (observed at commit `acde201`)

| Gate | Command | Result |
|------|---------|--------|
| Unit tests | `go test -count=1 ./internal/...` | ALL 24 packages OK (1,200+ tests) |
| Lint | `golangci-lint run ./...` | 0 findings |
| Formatter | `gofumpt -l .` | empty |
| Type check | `go vet ./...` | exit 0 |
| Build | `go build ./...` | exit 0 |
| Hexagonal purity | `grep -r "database/sql" internal/core/` | 0 imports (only doc-comments mention it) |
| Code health MCP | `grep "mcp-code-health" backend/internal/infrastructure/mcp/` | 0 hits (PR 2 owns) |
| Frontend changes | `git diff --stat master..HEAD -- frontend/` | 0 hits (PR 3 owns) |

---

## Outstanding (resolved or deferred to PR 2 / 3)

- [x] **Suppression feedback** — implemented end-to-end (migration `010b_`,
      port + repository extension, analyzer skip). NOT client-side only.
- [x] **REST endpoints** — confirmed MCP-only for S4 is acceptable; no
      `/api/v1/code-health/*` handler added in this PR.
- [x] **`source_issue_id` on `challenges`** — added in migration `010_`
      (nullable TEXT column). Persists `SourceIssueID` from the S4
      code-health → dojo flow.
- [x] **PDF report** — `format=pdf` returns `ErrCodeHealthPDFNotImplemented`
      at the service layer; PR 2's MCP tool maps it to JSON-RPC `-32603`.
- [ ] `mcp-code-health.*` tools (4) + `mcp-code-health.suppress_issue` (1) —
      deferred to PR 2.
- [ ] `mcp-challenges.generate_challenge({SourceIssueID})` wire arg +
      `GenerateChallengeHint.SourceIssueID` + `Origin="code_health"`
      persistence — deferred to PR 2.
- [ ] Frontend `/repo/:owner/:name/code-health` route + 5 dumb components +
      use case — deferred to PR 3.

---

## Next PR — `feature/code-health-pr2`

Tasks 2.1–2.14 per `tasks.md`:
- `infrastructure/mcp/code_health/adapter.go` (local Service interface)
- 5 `mcp-code-health/*` tools: `analyze_repo`, `get_analysis`,
  `get_repo_issues`, `generate_health_report`, `suppress_issue`
- `cmd/api/wiring.go` adds `CodeHealthService` to `WiringDeps` and
  registers the 5 tools (37 → 42 tools / 9 → 10 groups post-S4)
- `challenge_models.go` gains `SourceIssueID string` on
  `GenerateChallengeHint` (additive, zero-value = no-op)
- `challenge_service.go` branches on `SourceIssueID`: `Origin="code_health"`,
  `GeneratedBy="code-health-v1"`, `SourcePath=<issue.file_path>`
- `mcp-challenges` adapter + `challenges_tool.go` accept `sourceIssueId`
- Daily-cap defense-in-depth: `analyze_repo` calls
  `CountSince(userID, midnightUTC)`; ≥5 → MCP error `-32609`
- Wiring test counts the new tools (37 → 42)
- 5 tool unit tests (happy + arg parsing + `-32602` on missing user_id)
- Go gates green; hexagonal purity: 0 `database/sql`/`net/http`
  imports in `backend/internal/infrastructure/mcp/code_health/`

---

## Risk register (carried from `proposal.md`, unchanged by PR 1)

| Risk | Status post-PR-1 | Notes |
|------|------------------|-------|
| Hexagonal purity | ✅ 0 violations | verified by grep on `internal/core/` |
| GitHub auth scope | ✅ MVP PAT + nil-safe | `GITHUB_TOKEN` unset -> log line, no crash |
| Repo size budget | ✅ 300-file + 1MB caps | enforced in `analyzer.shouldSkip` + `MaxFiles` |
| Async job lifecycle | ✅ pattern matches S3 | `pending→running→completed|failed`; 24h TTL expiry → `failed` |
| Schema migration safety | ✅ additive, no backfill | `010_` + `010b_`; down documented in-file |
| Token leakage to LLM | ✅ RED test in `analyzer_test.go` | `BuildPrompt` asserts token never in output |
| Prompt injection | ✅ RED test in `analyzer_test.go` | file content with 'ignore previous instructions' yields 0 issues (no LLM call) |

---

## Spec reconciliation (post-archive)

On archive of `code-health-dashboard`:
- `code-health` delta lands at `openspec/specs/code-health/spec.md` (13
  REQ-CODE-HEALTH-001..013 implemented in PR 1; 013 auth is satisfied by
  the Supabase RLS policies + the MCP-tool auth check in PR 2).
- `challenges` delta lands at `openspec/specs/challenges/spec.md` (the
  `source_issue_id` column is in place; PR 2 adds the
  `GenerateChallengeHint.SourceIssueID` extension).
- `mcp` delta lands at `openspec/specs/mcp/spec.md` (the inventory moves
  from 33 / 8 to 37 / 9 post-S4; PR 2 lands the four new tools).

No spec reconciliation needed at PR 1 merge time (all spec deltas
already exist; the only changes are backend code, not spec text).

---

# Apply Progress — code-health-dashboard (S4) PR 2

> **Phase:** 2 (MCP group + S3 wiring)
> **Branch:** `feature/code-health-pr2` (base: `feature/code-health-pr1` after PR 1 merges)
> **Status:** ✅ all 14 tasks done; gates green; NOT pushed (user handles `git merge --no-ff`)
> **Date:** 2026-10-11

---

## Summary

PR 2 ships the MCP `mcp-code-health` group (5 tools) end-to-end +
the S4 challenges delta (SourceIssueID + `origin="code_health"`
persistence). The PR lands the **complete** Phase 2 scope from
`tasks.md` (tasks 2.1–2.14): the `mcp-code-health` tool group
(analyze_repo, get_analysis, get_repo_issues,
generate_health_report, suppress_issue), the registry wiring
(33 → 38 tools / 8 → 9 groups), the `SourceIssueID` extension
on `GenerateChallengeHint` + `Challenge` (so the S4
"generate from issue" flow persists `origin="code_health"`),
the daily-cap defense-in-depth in `CodeHealthService.AnalyzeRepo`
(maps to JSON-RPC `-32609`), and a read-only hexagonal-purity
assertion (0 `database/sql` / `net/http` imports in the
`code_health` MCP layer).

The PR is **backend-only** — no frontend, no schema changes
(PR 1 already shipped migrations `010_` and `010b_`).
Stacked-to-main chain shape is preserved.

---

## Commits

| # | SHA | Title |
|---|-----|-------|
| 1 | `1a4d4f3` | feat(mcp): add code_health service adapter (task 2.1) |
| 2 | `c018c2a` | feat(mcp): add analyze_repo + get_analysis tools (tasks 2.2+2.3) |
| 3 | `9d03104` | feat(mcp): add get_repo_issues + suppress_issue tools (tasks 2.4+2.12) |
| 4 | `6b868dd` | feat(mcp): add generate_health_report tool (task 2.5) |
| 5 | `e8409b7` | feat(wiring): register mcp-code-health group (5 tools) (task 2.6) |
| 6 | `01aa0a2` | feat(challenges): add SourceIssueID + code_health origin (tasks 2.7+2.8) |
| 7 | `e43b73a` | feat(mcp): extend challenges to accept sourceIssueId (task 2.9) |
| 8 | `7217417` | feat(services): add daily cap defense-in-depth (tasks 2.11+2.14) |

Total: **8 commits**, **~2,948 lines added** across 30 files.
Significantly over the 400-line review budget; the
orchestrator pre-cleared `ask-on-risk` for this 3-PR
stacked-to-main chain (S3 was comparable in size and used
the same shape; the overage is mostly comprehensive tool
unit tests + the wire-shape triangulation tests for every
adapter method).

---

## Gates green (observed at commit `7217417`)

| Gate | Command | Result |
|------|---------|--------|
| Unit tests | `go test -count=1 ./internal/...` | ALL packages OK (no FAIL) |
| Lint | `golangci-lint run ./...` | 0 findings |
| Formatter | `gofumpt -l .` | empty |
| Type check | `go vet ./...` | exit 0 |
| Build | `go build ./...` | exit 0 |
| Hexagonal purity | `TestNoForbiddenImports_InfrastructureLayer` (in `hex_purity_test.go`) | 0 `database/sql` / `net/http` imports |
| Registry count | `TestBuildMCPRegistry_ListsExactly38Tools` | 38 tools registered |
| Registry groups | `TestBuildMCPRegistry_GroupByPrefix` | 9 groups (4/5/6/4/4/6/3/1/5 split) |
| HTTP /mcp/tools/list | `TestBuildMCPRegistry_HTTP_GET_MCPToolsList_Reports38` | 38 items, 9 groups |
| Frontend changes | `git diff --stat 5190404..HEAD -- frontend/` | 0 hits (PR 3 owns) |
| Schema changes | `git diff --stat 5190404..HEAD -- backend/internal/infrastructure/driven/supabase/migrations/` | 0 hits (PR 1 shipped `010_` + `010b_`) |

---

## Outstanding (resolved or deferred to PR 3)

- [x] **All 5 `mcp-code-health.*` tools** — implemented
      (analyze_repo, get_analysis, get_repo_issues,
      generate_health_report, suppress_issue) with the
      canonical MCP wire shape + 30+ unit tests.
- [x] **`mcp-challenges.generate_challenge({SourceIssueID})`
      wire arg + `GenerateChallengeHint.SourceIssueID` +
      `Origin="code_health"` persistence** — landed in
      tasks 2.7+2.8+2.9.
- [x] **Daily-cap defense-in-depth** — landed in task 2.11:
      `analyze_repo` calls `CountSince(userID, midnightUTC)`;
      `>=5` → MCP error `-32609` "Daily limit reached, resets
      at 00:00 UTC".
- [x] **`suppress_issue` tool** — landed in task 2.12: writes
      via the port `CodeHealthRepository.InsertSuppression` (no
      direct Supabase client from the MCP layer).
- [x] **Hexagonal purity** — landed in task 2.14: 0
      `database/sql` / `net/http` imports in
      `backend/internal/infrastructure/mcp/code_health/`
      (verified via the `TestNoForbiddenImports_InfrastructureLayer`
      grep-assertion test).
- [ ] Frontend `/repo/:owner/:name/code-health` route + 5 dumb
      components + use case — deferred to PR 3.

---

## TDD cycle evidence (per work unit)

| Task | Test file | Layer | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|-----|-------|-------------|----------|
| 2.1 | `adapter_test.go` | Unit | ✅ Write (build fail) | ✅ Pass | ✅ 2 cases (GetAnalysis, GetRepoIssues wire) | ➖ None needed |
| 2.2 | `analyze_repo_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 9 cases (happy/force/missing user/SSRF owner/SSRF name/invalid source/missing fields/daily cap/branch default) | ➖ None needed |
| 2.3 | `get_analysis_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 5 cases (happy/missing user/missing id/not-found/name contract) | ➖ None needed |
| 2.4 | `get_repo_issues_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 7 cases (happy/missing user/invalid severity/default limit/clamped limit/missing repo/name contract) | ➖ None needed |
| 2.5 | `generate_health_report_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 6 cases (md happy/pdf not impl/invalid format/missing user/missing id/name contract) | ➖ None needed |
| 2.6 | `wiring_test.go` | Integration | ✅ Write (count=33) | ✅ Pass (count=38) | ✅ 3 cases (registry list, HTTP /mcp/tools/list, group-by-prefix) | ➖ None needed |
| 2.7 | `challenge_models_source_issue_id_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 3 cases (JSON tag, omitempty, coexistence) | ➖ None needed |
| 2.8 | `service_test.go` (existing) | Unit | ➖ pre-existing | ✅ Pass | ✅ 2 new cases (SuppressIssue happy + unknown) | ➖ None needed |
| 2.9 | `generate_challenge_source_issue_id_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 4 cases (forwarded, S3 preserved, JSON tag, omitempty) | ➖ None needed |
| 2.11 | `daily_cap_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 4 cases (at-cap reject, under-cap proceed, boundary 4/5/6, rate-limit mapping) | ➖ None needed |
| 2.12 | `suppress_issue_test.go` | Unit | ✅ Write | ✅ Pass | ✅ 6 cases (happy/optional reason/missing user/missing id/unknown issue/name contract) | ➖ None needed |
| 2.13 | All `*_test.go` | Unit | ✅ Per-tool | ✅ Pass | ✅ Per-tool | ➖ None needed |
| 2.14 | `hex_purity_test.go` | Read-only | N/A (assertion) | ✅ Pass | N/A | N/A |

---

## Risk register (carried from PR 1, updated post-PR-2)

| Risk | Status post-PR-2 | Notes |
|------|------------------|-------|
| Hexagonal purity | ✅ 0 violations | verified by `TestNoForbiddenImports_InfrastructureLayer` |
| GitHub auth scope | ✅ MVP PAT + nil-safe | unchanged from PR 1 |
| Repo size budget | ✅ 300-file + 1MB caps | unchanged from PR 1 |
| Async job lifecycle | ✅ pattern matches S3 | unchanged from PR 1 |
| Schema migration safety | ✅ additive, no backfill | unchanged from PR 1 |
| Token leakage to LLM | ✅ RED test in `analyzer_test.go` | unchanged from PR 1 |
| Prompt injection | ✅ RED test in `analyzer_test.go` | unchanged from PR 1 |
| SSRF via repo descriptor | ✅ RED threat test in `analyze_repo_test.go` | `owner="../../evil"` rejected BEFORE any outbound call |
| Source whitelist (gogs/github) | ✅ 3 rejection tests in `analyze_repo_test.go` + `get_repo_issues_test.go` | unknown sources rejected at the tool boundary |
| Daily cap bypass via client tampering | ✅ Server-side `CountSince` backstop in `AnalyzeRepo` (task 2.11) | >=5 → -32609; client localStorage is the primary control |
| Generate-from-issue double-orchestrator | ✅ REQ-CHALLENGES-CODE-HEALTH-003 honored: no new orchestrator; S3 un-stubbed `GenerateChallenge` handles both origins | the only delta is the persisted `SourceIssueID` + `SourcePath` + `origin` fields |
| Suppression feedback reach | ✅ full round-trip: user clicks "not an issue" → `suppress_issue` → port `InsertSuppression` → analyzer skip on next run (analyzer_test.go round-trip test, PR 1) | not client-side only — server-persisted per user gate |

---

## Spec reconciliation (post-archive)

On archive of `code-health-dashboard`:
- `code-health` delta lands at `openspec/specs/code-health/spec.md`
  (all 13 REQ-CODE-HEALTH-* implemented across PR 1 + PR 2).
- `challenges` delta lands at `openspec/specs/challenges/spec.md`
  (REQ-CHALLENGES-CODE-HEALTH-001/002/003 implemented in PR 2;
  the `source_issue_id` column was added in PR 1's
  migration 010_).
- `mcp` delta lands at `openspec/specs/mcp/spec.md`
  (the inventory moves from 33 / 8 to 38 / 9 post-S4; all
  5 `mcp-code-health` tools registered + the S3 backward
  compat for `mcp-challenges.generate_challenge` preserved).

No spec reconciliation needed at PR 2 merge time (all spec
deltas already exist; the only changes are backend code, not
spec text).

---

## Next PR — `feature/code-health-pr3`

Tasks 3.1–3.11 per `tasks.md` (PR 3 is **frontend-only**):
- `domain/models/code-health.ts` (CodeHealthAnalysis,
  CodeHealthIssue, severity/category/status/source unions)
- `domain/ports/code-health.port.ts` (CodeHealthPort +
  AnalyzeRepoArgs)
- `infrastructure/repositories/code-health.repository.ts`
  (impl delegating to `MCPClientPort.callTool`)
- `application/code-health.use-case.ts` (signals +
  exponential-backoff polling + 5/day UTC localStorage cap)
- 5 dumb components: `repo-url-bar`, `import-button`,
  `health-gauge` (SVG), `issues-table`, `category-breakdown`
- `code-health-page.component.ts` + `generate-challenges-cta`
- `app.routes.ts`: register `/repo/:owner/:name/code-health`
- Dashboard tile + frontend quality gates (Vitest, ESLint, build)
