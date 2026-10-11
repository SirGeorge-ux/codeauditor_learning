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
