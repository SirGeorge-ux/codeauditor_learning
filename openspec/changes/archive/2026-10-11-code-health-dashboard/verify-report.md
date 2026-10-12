# Verify Report: code-health-dashboard (S4)

> **Generated:** 2026-10-12
> **Repo HEAD:** `2144d59` (master, post-merge of PR 1 + PR 2 + PR 3)
> **Mode:** Strict TDD (`openspec/config.yaml` → `testing.strict_tdd: true`)
> **Verdict:** **PASS WITH WARNINGS** — all automated gates green, 3 manual smokes deferred, 6 drift items to reconcile at archive.

## Completeness

| Metric | Value |
|--------|-------|
| Implementation tasks (P1+P2+P3) | 45 of 45 reported ✅ in `apply-progress.md` (P1: 20, P2: 14, P3: 11) |
| Phase 4 automated gates | 13 run, all green (4.6 skipped — node_modules present) |
| Manual smokes | 3 deferred_to_user (need live backend + LLM + browser) |
| Requirements (spec total) | 21 (code-health 13 + challenges 3 + mcp 5) |
| Scenarios (spec total) | 59 |

## Automated Gates (4.1–4.13)

| # | Gate | Status | Observed |
|---|------|--------|----------|
| 4.1 | `go test -count=1 ./internal/...` | 🟢 GREEN | exit 0, 26 packages `ok` |
| 4.2 | `golangci-lint run ./...` | 🟢 GREEN | exit 0, 0 findings |
| 4.3 | `gofumpt -l .` | 🟢 GREEN | empty (no unformatted files) |
| 4.4 | `go vet ./...` | 🟢 GREEN | exit 0 |
| 4.5 | `go build ./...` | 🟢 GREEN | exit 0 |
| 4.6 | `pnpm install --frozen-lockfile` | ⏭️ SKIPPED | `node_modules` already present |
| 4.7 | `pnpm lint` (ESLint) | 🟢 GREEN | exit 0, 0 errors / 0 warnings |
| 4.8 | `pnpm exec prettier --check "src/**/*.{ts,html,css}"` | 🟢 GREEN | "All matched files use Prettier code style!" |
| 4.9 | `npx ng test --watch=false` | 🟢 GREEN | 40 passed + 1 skipped files; 321 passed + 10 skipped tests |
| 4.10 | `npx ng build` | 🟢 GREEN | exit 0; bundle 1.07 MB |
| 4.11 | Hex purity backend (`database/sql` in `core/`) | 🟢 GREEN | 0 matches (only comment matches in dictionary_service.go + code_health/service.go) |
| 4.11b | Hex purity frontend (`@angular` in `domain/`) | 🟢 GREEN | 0 matches (only comment matches in 3 port files) |
| 4.12 | MCP registry count (`TestBuildMCPRegistry_ListsExactly38Tools`) | 🟢 GREEN | exit 0; **38 tools / 9 groups**; per-tool split 4/5/6/4/4/6/3/1/5 |
| 4.13 | `make validate` equivalent (gofumpt + golangci-lint + prettier + eslint) | 🟢 GREEN | all 4 exit 0 |

> **Important note on the MCP tool count:** the change ships **5 tools** in `mcp-code-health` (`analyze_repo`, `get_analysis`, `get_repo_issues`, `generate_health_report`, `suppress_issue`), not the 4 declared in the original spec + design + proposal drafts. The 5th tool (`suppress_issue`) was added per the resolved **suppression-feedback open question** during the `ask-on-risk` user gate (2026-10-11). This is the most material spec-vs-impl drift to reconcile at archive (see Drift #1 below).

## Manual Smokes (Deferred to User) — 4.14–4.16

| # | Smoke | Why deferred |
|---|-------|--------------|
| 4.14 | E2E: `analyze_repo(owner,name,"gogs")` → poll `get_analysis` → verify `health_score` + `top_issues` render | Needs live backend (Coolify deploy or `make dev` local) + live Gogs access + reachable LLM (OPENROUTER_API_KEY) |
| 4.15 | Generate-from-issue: `generate_challenge({sourceIssueId, language})` → verify `origin="code_health"` + `source_issue_id` row | Needs live backend + LLM for the inner `GenerateChallenge` (S3 cascade) |
| 4.16 | Dashboard UI: navigate `/repo/:owner/:name/code-health`, import a fixture repo, verify score + issues render; 6th analysis rejected by daily cap | Needs browser + live backend |

> These 3 smokes mirror the S3 free-practice-mode archive's deferral pattern (`4.12`, `4.13` in that change's tasks.md). They are NOT blockers for archive; they land in a post-archive user validation.

## Spec Reconciliation

### `code-health` (13 requirements — `specs/code-health/spec.md`)

| Req | Status | Evidence |
|-----|--------|----------|
| 001 — Gogs import | ✅ satisfied | `gogs_client.ListFiles` (PR 1 task 1.3) + analyzer filter for `node_modules/` / `.git/` / `dist/` / `vendor/` / binaries / >1 MB; `driven/gogs/gogs_client_test.go` |
| 002 — GitHub import | ✅ satisfied | `github_client.go` (PR 1 task 1.2) with PAT via `GITHUB_TOKEN` env, 30s timeout, 404 → `ErrGitHubNotFound`, 403 + `x-ratelimit-remaining:0` → `ErrGitHubRateLimited`; `httptest` coverage |
| 003 — Async analysis job | ✅ satisfied | `service.go` goroutine `pending → running → completed \| failed` with INSERT-then-respond <500ms pattern; 24h `expires_at` TTL → `failed`; `service_test.go` coverage |
| 004 — Health score formula | ✅ satisfied | `health_score.go` weights `{critical:15, high:8, medium:3, low:1}`, clamp 0–100, formula `100 − Σ(weight)`; table-driven tests including the worked example (2crit+4high+6med+10low → 10) |
| 005 — Ranked issues | ✅ satisfied | `RankIssues(issues)` orders by `(severity_weight DESC, line_number ASC, category ASC)`; tests |
| 006 — Issue categories (5) | ✅ satisfied | `Registry.Categories()` returns security / performance / refactor / style / architecture; unknown → `ErrCodeHealthInvalidCategory`; tests |
| 007 — 6 seeded rules | ✅ satisfied | `rules/secret.go`, `injection.go`, `n_plus_one.go`, `god_method.go`, `magic_number.go`, `leaky_abstraction.go`; ≥5 true-positive + ≥5 true-negative per rule |
| 008 — LLM verification + suppression | ✅ satisfied | `analyzer.go` regex fast-path first, Ollama `qwen2.5-coder:3b` only on ambiguous (>50KB), DeepSeek V3 reserved for "deep" opt-in; suppression via `code_health_suppressions` table (PR 1 tasks 1.18-1.20); round-trip test |
| 009 — Analysis cache (SHA-keyed, 7d TTL) | ✅ satisfied | `analysis_cache_repository.go` key `analysis:<file_sha>` namespaced by `user_id`, 7d TTL, read filters `expires_at > NOW()`; sqlmock tests |
| 010 — Generate challenges from issues | ✅ satisfied | `GenerateChallengeHint.SourceIssueID` additive field (PR 2 task 2.7) + `mcp-challenges.generate_challenge` wire arg `sourceIssueId` (PR 2 task 2.9); `origin="code_health"`, `generatedBy="code-health-v1"`, `sourcePath=<issue.file_path>`, persists `source_issue_id` |
| 011 — Daily cap (5/day UTC) | ✅ satisfied | Server-side `CountSince(userID, midnightUTC)` returns MCP `-32609` "Daily limit reached, resets at 00:00 UTC" (PR 2 task 2.11); client-side localStorage cap mirrors S3 practice.use-case pattern |
| 012 — Health-score history (last 10) | ✅ satisfied | `ListHistory(userID, owner, name, limit=10)` append-only ordering by `created_at DESC` |
| 013 — Auth on every endpoint | ⚠️ **partial → deferred to S5** | MCP `-32602` "user_id required" ✅ satisfied + tested; **REST `/api/v1/code-health/*` deferred to S5** per the user gate (MCP-only is acceptable for S4) |

### `challenges` (3 MODIFIED requirements — `specs/challenges/spec.md`)

| Req | Status | Evidence |
|-----|--------|----------|
| REQ-CHALLENGES-CODE-HEALTH-001 `SourceIssueID` on `GenerateChallengeHint` | ✅ satisfied | `challenge_models_source_issue_id_test.go` |
| REQ-CHALLENGES-CODE-HEALTH-002 `origin="code_health"` + `generatedBy="code-health-v1"` + `sourcePath` persisted | ✅ satisfied | `challenge_service.go` branch in `GenerateChallenge` |
| REQ-CHALLENGES-CODE-HEALTH-003 reuse S3 orchestrator (no new orchestrator) | ✅ satisfied | The MCP `mcp-code-health.generate_health_report` (md only) is a separate path; the actual challenge generation reuses `ChallengeService.GenerateChallenge` end-to-end with the additive `SourceIssueID` field |

### `mcp` (5 MODIFIED requirements — `specs/mcp/spec.md`)

| Req | Status | Evidence |
|-----|--------|----------|
| REQ-MCP-CODE-HEALTH-001 9th group `mcp-code-health` registered | ⚠️ **drift** | Spec declared 4 tools; impl ships **5** (`suppress_issue` added per the suppression gate). All 4 originally-spec'd tools present + 1 new tool. **Archive MUST update the mcp delta to 5 tools / 38 total.** |
| REQ-MCP-CODE-HEALTH-002 hex purity in `infrastructure/mcp/code_health/` | ✅ satisfied | `hex_purity_test.go` asserts 0 `database/sql` + 0 `net/http` imports |
| REQ-MCP-CODE-HEALTH-003 tool test coverage | ✅ satisfied | 5 `_test.go` files (analyze_repo, get_analysis, get_repo_issues, generate_health_report, suppress_issue) |
| Tool Registry MODIFIED (count) | ⚠️ **drift** | Spec → 33 + 4 = 37; impl → 33 + 5 = **38** |
| S2 Tool Inventory MODIFIED (table) | ⚠️ **drift** | Spec → 37 total / 9 groups; impl → **38 total / 9 groups** |

## Drift (spec vs implementation)

These are the 6 items the archive sub-agent MUST reconcile:

1. **mcp tool-count drift (WARNING)** — `specs/mcp/spec.md`, `design.md`, `proposal.md` all declare **4 tools / 37 total**; implementation ships **5 tools / 38** (`suppress_issue` added per resolved suppression-feedback gate, task 2.12). Reconcile to 5 / 38.
2. **challenges spec migration note (SUGGESTION)** — `specs/challenges/spec.md` says `source_issue_id` column is "out of scope for S4", but migration `010_create_code_health.sql` actually adds it (`ALTER TABLE public.challenges ADD COLUMN source_issue_id TEXT`). Reconcile: this column is IN scope and shipped in migration 010.
3. **tasks.md stale counts (WARNING)** — task 2.6 and 4.12 references say "37 tools / 4 tools"; the 5th tool (`suppress_issue`, task 2.12) makes them stale. Update to 38 / 5.
4. **code-health spec Out of Scope omits REST (WARNING)** — REQ-CODE-HEALTH-013 requires `/api/v1/code-health/*` auth, but REST is deferred (MCP-only) without being listed in the spec's Out of Scope. Add "REST `/api/v1/code-health/*`" to code-health Out of Scope.
5. **AGENTS.md (repo root) stale (SUGGESTION)** — still documents pre-S4 state (33 tools / 8 groups, S4 "scaffold vacío"). Update to 38 / 9 / S4 done.
6. **tasks.md checkboxes still `[ ]` (SUGGESTION)** — apply tracked completion in `apply-progress.md` rather than flipping boxes; archive should flip all 60 boxes to `[x]` (the work was done; the boxes reflect the original planning, not the post-apply state).

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| `gentle-ai sdd-verify-validate` unavailable in this environment (gentle-ai 4.0.0 has no `sdd-*` command family) → no canonical validator admission | high | This report is the orchestrator-persisted fallback. The next `sdd-verify-validate` binary in the environment can admit it. |
| mcp spec drift (4→5 tools / 37→38) un-reconciled | medium | Archive updates the mcp delta before syncing canonical `openspec/specs/mcp/spec.md` |
| REQ-013 REST deferral un-acknowledged in spec | medium | Add "REST `/api/v1/code-health/*`" to code-health Out of Scope at archive |
| Manual smokes (4.14–4.16) unverified live | medium | User runs E2E after Coolify deploy / `make dev` with `OPENROUTER_API_KEY` + browser |
| Pre-existing `auth.service.spec.ts` flake (1 skipped file with `describe.skip` + 40-line TODO) | low | Out of scope for S4; tracked for follow-up PR (vitest setupFiles / `pool: 'forks'` to re-enable) |
| SDD planning artifacts not yet committed on the merge commit (`openspec/changes/code-health-dashboard/{explore,proposal,design,specs,tasks}`) | low | The `docs(sdd): add S4 code-health-dashboard planning artifacts` commit (e5225fc) on the chain carries them; archive will move them to `archive/2026-10-11-code-health-dashboard/` |
| Stacked chain is 35 implementation commits + 4 merge commits = 39 commits total since `9af1407` (98 files, +15,369/-254 LOC) — far above the 400-line review budget | low (orchestrator pre-cleared) | `ask-on-risk` gate pre-cleared on 2026-10-11; chain is well-organized into 3 PRs with clean rollback boundaries; per-PR review budget ≤ 6,340 LOC (still over but individually scoped) |

## Recommendation

**Verdict: PASS WITH WARNINGS.**

Archive-ready on the merits: **YES**, conditioned on the archive sub-agent reconciling the 6 drift items above. `next_recommended` = **archive**.

After archive: user runs the 3 deferred manual smokes against a live Coolify deploy (or `make dev` local stack with `OPENROUTER_API_KEY` set + a real `GOGS_TOKEN` or `GITHUB_TOKEN` to import a fixture repo). The daily cap (5/day) can be exercised by importing 6 repos; the generate-from-issue smoke verifies the killer feature end-to-end.
