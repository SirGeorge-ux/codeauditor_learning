# Proposal: code-health-dashboard (S4)

> **Capabilities:** `code-health` (NEW), `challenges` (MODIFIED), `mcp` (MODIFIED)
> **Depends on:** `mcp-monolith`, `tutor-chat`, `free-practice-mode` (all archived 2026-10-10)
> **Shape:** 3 stacked-to-main PRs, ~2400 lines, `ask-on-risk` gate before apply (matches S3).

## Intent

**What:** the third practice mode — the user imports a real repo (Gogs or GitHub), and the system produces a health dashboard: a 0–100 health score, ranked issues across 5 categories (security / performance / refactor / style / architecture), a language breakdown, and a "generate challenges from these" CTA that turns detected issues into dojo challenges with `origin="code_health"`.

**Why:** curated (8 static) and free-practice (generated) both depend on content we author. Code health is the first mode that consumes *the user's own code* — the highest-leverage learning loop, and the only one of the 3 modes AGENTS.md §0 promises that still has zero implementation.

**For whom:** single-user dev (`ggogsmic`) analyzing their own repos, ranked Junior–Mid–Senior–Architect.

## Scope

### In Scope (MVP)
- Backend: GitHub source adapter (`driven/github/` — does not exist today), TTL-aware `AnalysisCache` (key `analysis:<sha>`, 7d), `code_health` core package (`service.go`, `health_score.go`, `analyzer.go`, `rules/` + registry), Supabase migration `010_create_code_health.sql` + `CodeHealthRepository`.
- Backend: `mcp-code-health` group — `analyze_repo`, `get_analysis`, `get_repo_issues`, `generate_health_report` (9th group, completing the mcp spec's "9 groups").
- Backend: extend `GenerateChallengeHint` with `SourceIssueID` (additive), `GenerateChallenge` accepts `Origin="code_health"`.
- Frontend: `/repo/:owner/:name/code-health` route + 5 dumb components + `code-health.use-case.ts` + dashboard tile.
- Health-score formula exactly per skill (`100 − Σ(weight)`, weights `{critical:15, high:8, medium:3, low:1}`, clamp 0–100).
- 6 seeded rules across the 5 categories; per-user daily cap 5 analyses.

### Out of Scope
- GitHub **OAuth flow** — MVP uses app-wide `GITHUB_TOKEN` (PAT) env var; per-user OAuth deferred to S6+.
- `mcp-inspiration` (Exercism/GitHub education) — deferred since S2, remains S3+.
- Server-side daily cap — S5 (`llm-cascade`) generalizes the cascade and owns the server-side cap; S4 enforces client-side.
- Multi-user sharing, curriculum authoring UI, gamification tweaks, inline rule rendering in dojo.

## Capabilities

### New Capabilities
- `code-health`: repo analysis (Gogs/GitHub) → async job → health score + ranked issues + language breakdown + generate challenges from issues. Delta lands at `openspec/specs/code-health/spec.md`.

### Modified Capabilities
- `challenges`: `GenerateChallengeHint` gains `SourceIssueID`; `GenerateChallenge` persists `Origin="code_health"` (additive, zero-value default = no-op). Delta lands at `openspec/specs/challenges/spec.md`.
- `mcp`: register the 9th group `mcp-code-health` (4 tools) → 37 tools / 9 groups (spec currently says 33 / 8). Delta lands at `openspec/specs/mcp/spec.md`.

## Approach

Mirror S3's stacked-to-main cadence (3 PRs, ask-on-risk gate):

- **PR 1 (backend foundation, ~900 lines):** `github_client.go` + `AnalysisCache` + migration `010_*` + `CodeHealthRepository` + `code_health` service (`AnalyzeRepo`/`GetAnalysis`/`RankIssues` async job) + 6 seeded rules.
- **PR 2 (MCP + challenges wiring, ~700 lines):** `mcp-code-health` group (4 tools) + registry/wiring + `SourceIssueID` on `GenerateChallengeHint` + mapping tests. Smallest proof of value: `curl mcp-code-health.analyze_repo(...)` → `get_analysis` → valid `health_score` + `top_issues`.
- **PR 3 (frontend, ~800 lines):** models + port + use case + 5 components + route + dashboard tile.

**Resolved decisions (from explore open questions):**

1. **Async lifecycle** — goroutine + DB status enum (`pending|running|completed|failed`), `analysis_id` returned immediately, polled via `get_analysis`. No worker queue (overkill for single-user).
2. **GitHub auth** — app-wide `GITHUB_TOKEN` env (MVP); OAuth roadmap note S6+. Clean `GITHUB_RATE_LIMITED` error on quota.
3. **Cache** — new `AnalysisCache` driven adapter (7d TTL, `analysis:<sha>`). Do **not** extend `ports.CachePort` (avoids touching the 10-message chat ring buffer).
4. **Daily cap** — 5 analyses/day (client-side now; server-side moves to S5).
5. **Rule seed** — 6 rules: hardcoded-secret, eval-injection, god-method, leaky-abstraction, magic-number, console-log-leak.

**Key tradeoff:** regex rules run first (fast path, low false-positive), LLM (Ollama `qwen2.5-coder:3b` per-file; DeepSeek V3 for >50KB files) verifies ambiguous cases. `ExpectedFinding` shape is reused 1:1 for `CodeHealthIssue`.

### Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~2400 |
| 400-line budget risk | **High** |
| Chained PRs recommended | **Yes** (3 stacked-to-main) |

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `backend/internal/infrastructure/driven/github/` | New | `github_client.go` mirroring `gogs_client.go` (ListRepos/ListFiles/GetFileContents/ListRepoTree) |
| `backend/internal/infrastructure/driven/gogs/` | Modified | add `ListFiles` (tree fetch) |
| `backend/internal/core/domain/models/` | New | `code_health_analysis.go`, `code_health_issue.go`, `health_score.go` |
| `backend/internal/core/services/code_health/` | New | service + health_score + analyzer + `rules/` (6 seeds) + registry |
| `backend/internal/ports/` | New | `code_health.go` (Repository + Analyzer) |
| `backend/internal/infrastructure/driven/supabase/` | New | migration `010_*` + `code_health_repository.go` |
| `backend/internal/infrastructure/driven/cache/` | New | `analysis_cache.go` (TTL, `analysis:<sha>`) |
| `backend/internal/infrastructure/mcp/code_health/` | New | 4 tools + 4 tests |
| `backend/internal/infrastructure/mcp/registry.go` | Modified | register 4 tools |
| `backend/cmd/api/wiring.go` + `main.go` | Modified | wire service + GitHub client + cache + env vars |
| `backend/internal/core/services/challenge_service.go` | Modified | `GenerateChallengeHint.SourceIssueID` + `Origin="code_health"` |
| `frontend/.../domain/` | New | models + `code-health-repository.port.ts` |
| `frontend/.../application/code-health.use-case.ts` | New | signals + polling (exponential backoff) |
| `frontend/.../components/code-health/` | New | 5 dumb components + generate-challenges CTA |
| `frontend/.../route config` | Modified | `/repo/:owner/:name/code-health` |
| `frontend/.../dashboard/` | Modified | code-health tile |
| `docs/` (business-domain §3.3, mcp-tools §3.7, hexagonal-architecture, new `code-health-rules.md`) | Modified/New | shipped pointers + rule catalogue |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| GitHub auth model (anonymous vs token vs org) | Med | MVP PAT env var; per-user OAuth S6+; clean rate-limit error |
| Repo size blows budget (>300 files / >100MB) | High | Hard cap 300 files + 1MB/file; user-facing warning; explicit "Analyze full repo" confirm |
| Ollama `qwen2.5-coder:3b` false-positive rate | High | Regex rules first (fast path); LLM only for ambiguous; confidence score; "not an issue" trains cache |
| Cache invalidation on push | Med | SHA-based key + 7d TTL + "Re-analyze" CTA bypassing cache |
| Async job lifecycle | Med | S3 status enum pattern; `analysis_id` returned immediately; 24h job TTL in DB |
| Stacked chain size (~2400 lines) | High | `ask-on-risk` gate before PR 1; each PR ≤1000 lines; clean revert boundaries |
| `GenerateChallenge` extension breaks callers | Low | Additive `SourceIssueID,omitempty`; empty default = no-op |
| Schema migration safety | Med | New `010_*` tables only, no backfill; rollback drops both tables |
| Hexagonal purity violation | Low | grep gate: 0 `database/sql` in `core/`; 0 `@angular/*` in `domain/` |

## Rollback Plan

- **Per-PR revert boundaries:** PR 3 reverts frontend (route + components + use case) — no backend regression. PR 2 reverts the `mcp-code-health` group + `SourceIssueID` field (tools unregister, `GenerateChallenge` returns to prior behavior). PR 1 reverts service + cache + adapter + migration — drops `code_health_analyses`/`code_health_issues` tables.
- **PR 2 alone landing (MCP without foundation):** remove the 4 tool registrations from `registry.go`; the `mcp-code-health/` package is deleted or left unregistered (no compile error if the group dir is removed cleanly).
- **Cache invalidation:** cache is keyed `analysis:<file_sha>` — a bad cached analysis is cleared by deleting the key or bumping TTL to 0; "Re-analyze" CTA bypasses cache per-repo.
- **DB:** migration `010_*` is additive; rollback `DROP TABLE code_health_issues; DROP TABLE code_health_analyses;`. No existing rows touched.

## Dependencies

- `mcp-monolith` (S2) ✅ archived — `mcpkit` runtime + `MCPClientAdapter` + registry pattern.
- `tutor-chat` (S2) ✅ archived — `LLMClientPort` + `llm_cascade` (Groq→DeepSeek→MiniMax) + `OllamaClient`.
- `free-practice-mode` (S3) ✅ archived — `GenerateChallenge` un-stub + sandbox gate `ExecuteWithTests` + `GenerateChallengeHint` additive-field pattern (`TopicID`/`Level`).
- `challenge-rebuild` (S1) ✅ archived — v2 `Challenge` model + `ExpectedFinding` shape (reused 1:1 for issues).
- **Nothing else blocking.** `llm-cascade` (S5) is a sibling, not a prerequisite.

## Success Criteria

- [ ] Each PR's gates green on master: `make validate` (gofumpt + golangci-lint + Prettier + ESLint) + unit tests per touched area.
- [ ] `GET /mcp/tools/list` returns 37 tools / 9 groups (incl. `mcp-code-health`).
- [ ] E2E smoke against a fixture repo (<50 files): `analyze_repo` → poll `get_analysis` → `health_score` + `top_issues` returned.
- [ ] "Generate challenges from issues" smoke: `generate_challenge({SourceIssueID, Language})` persists `origin="code_health"` row.
- [ ] Cache hit rate >70% on second visit to the same fixture repo (SHA-stable files skip LLM).
- [ ] Health-score formula matches skill spec exactly (`100 − Σ(weight)`, clamp 0–100).
- [ ] Hexagonal purity: 0 `database/sql` in `backend/internal/core/`; 0 `@angular/*` in `frontend/.../domain/`.
- [ ] `/repo/:owner/:name/code-health` renders real dashboard (not STUB); daily cap enforced (≥6 requests rejected).
