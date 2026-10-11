# Exploration: code-health-dashboard (S4)

> **Change:** `code-health-dashboard`
> **Sprint:** S4 (scaffold ready, intent declared in `proposal.md`)
> **Date:** 2026-10-10
> **Status:** explore complete — ready for proposal
> **Author:** sdd-explore (delegated)

---

## Current State

### What already exists for code health (docs + skills, zero code)

The S4 intent is fully **specified upstream** but **not yet implemented**. There is consistent pre-design across six docs, one skill, one harness, and zero Go/TS files:

| Artifact | Location | What it pre-decides |
|---|---|---|
| Business flow | `docs/business-domain.md` §3.3 | `/repo/:owner/:name/code-health` page flow; "Generate challenges from these" creates `origin='code_health'` rows via `mcp-challenges.generate_challenge` |
| MCP tool catalogue | `docs/mcp-tools.md` §3.7 | 4 tools fully designed: `analyze_repo`, `get_analysis`, `get_repo_issues`, `generate_health_report` (group `mcp-code-health`) |
| MCP spec snippet | `docs/openspec-workflow.md` §8.3 | Sample requirements + scenario for Gogs source (Ollama local, cache by SHA, security/perf/ret style categories, severity ranking) |
| Hexagonal layers | `docs/hexagonal-architecture.md` L47/L88/L151/L158 | `CodeHealthService` listed in `core/services/`; `mcp-code-health/` group; `code-health.use-case.ts`; `code-health/` component folder |
| LLM strategy | `docs/llm-strategy.md` §4 | Two cascade entries: `code_health_local` (Ollama `qwen2.5-coder:3b`) and `code_health_repo` (OpenRouter DeepSeek V3 → t3 GPT-4o-mini opt-in) |
| UI route | `docs/ui-and-stack.md` L113 | Route `/repo/:owner/:name/code-health` → `CodeHealthPageComponent` |
| Contributing | `docs/contributing.md` L188 | Cross-link to `add-code-health-rule.harness.md` for rule additions |
| Skill (full) | `.atl/skills/codeauditor-code-health/SKILL.md` | End-to-end flow; health-score formula (`100 − Σ(weight_per_issue)`, weights `{critical:15, high:8, medium:3, low:1}`); 5 categories; LLM prompt template; cache key `analysis:<sha>` 7d TTL; "generate challenge from issue" wiring |
| Harness | `.atl/harnesses/add-code-health-rule.harness.md` | 8-step rule authoring: design → categorize → file in `backend/internal/core/services/code_health/rules/<slug>.go` → 5+/5+ tests → register in `registry.go` → commit |

### What is shipped today (zero code-health-specific artifacts)

- **8/9 MCP groups registered** in `backend/internal/infrastructure/mcp/` (`challenges`, `curriculum`, `dictionary`, `external_docs`, `pedagogical`, `sandbox`, `tutor`, `user`); `code_health/` and `inspiration/` folders do **not** exist (S2 archive deferred both).
- **33 MCP tools** registered via `cmd/api/wiring.go` (per AGENTS.md §13).
- **Gogs adapter exists** (`backend/internal/infrastructure/driven/gogs/gogs_client.go` 181 lines + handler + tests; 1 MB per-file cap already enforced; `token <value>` auth header).
- **GitHub adapter is MISSING.** No `*github*.go` files anywhere in `backend/internal/infrastructure/`; no `GitHubClient` reference; no `api.github.com` URL anywhere in `backend/`. The S4 proposal intent of "Gogs + GitHub" has only one half implemented.
- **No `code_health` package, port, model, repository, migration, use case, component, or route** anywhere in backend or frontend.
- **Cascade wiring exists** — `LLMClientPort.Complete` (S3) and `OllamaClient` (`backend/internal/ports/ollama.go` + `driven/ollama/`) can be reused for the file-level and repo-level lanes.
- **In-memory cache infrastructure exists** but for a different shape: `backend/internal/infrastructure/driven/session_cache.go` is a 10-message ring buffer for chat (not TTL-aware). S4 will need either a new TTL-aware `AnalysisCache` or extend `ports.CachePort` with a TTL primitive.
- **Schema reuse opportunity** — `ExpectedFinding` in `domain/models/challenge_models.go` (Severity/Category/Message/Evidence/SuggestedFix) maps 1:1 onto a code-health `Issue` shape; the S3 generate-challenge pipeline (`ChallengeService.GenerateChallenge` un-stub + sandbox gate `ExecuteWithTests`) is exactly the "generate challenge from issue" path.

### Reference archive for the pattern (free-practice-mode, archived 2026-10-10)

`openspec/changes/archive/2026-10-10-free-practice-mode/` is the canonical example. S3 shipped as **3 stacked-to-main PRs, ~2400 lines, ask-on-risk**, with the work-unit table format already adopted by `tasks.md`. S4 is comparable in surface area and should follow the same conventions.

---

## Affected Areas

### Backend (Go)

| Path | Status | Why |
|---|---|---|
| `backend/internal/core/domain/models/` | **NEW** | `code_health_analysis.go`, `code_health_issue.go`, `health_score.go` (3 files). Reuse `ExpectedFinding` shape for issues. |
| `backend/internal/core/services/code_health/` | **NEW** | `service.go` (orchestrator), `health_score.go` (formula), `analyzer.go` (per-file pipeline), `rules/<slug>.go` (5–10 seeded rules), `rules/registry.go` |
| `backend/internal/ports/` | **NEW** | `code_health.go` (Repository + Analyzer interface), extend `cache.go` with TTL primitive if reusing `CachePort` |
| `backend/internal/infrastructure/driven/github/` | **NEW** | `github_client.go` + tests; mirror `gogs_client.go` shape (`ListRepos`, `ListFiles`, `GetFileContents`, `ListRepoTree`); add GitHub REST-specific fields (rate-limit handling, `installation_token` for orgs, optional `Accept: application/vnd.github+json`) |
| `backend/internal/infrastructure/driven/gogs/` | **REUSE** | Already implements the `gogs_client.go` contract; need to add `ListFiles(owner, repo, branch, path?)` returning `[]FileEntry{Path, Size, SHA}` (currently only `GetFileContents` exists) |
| `backend/internal/infrastructure/driven/ollama/` | **REUSE** | Existing `OllamaClient` used by `AuditService`; add a JSON-mode `GenerateStructured(prompt, schema)` method or route file-level analysis through `LLMClientPort.Complete` (S3) |
| `backend/internal/infrastructure/driven/llm_cascade/` | **REUSE** | Existing cascade (Groq → DeepSeek → MiniMax); add a `code_health_repo` lane per `docs/llm-strategy.md` §4 |
| `backend/internal/infrastructure/driven/supabase/migrations/` | **NEW** | `010_create_code_health.sql` — `code_health_analyses` (id, user_id, source, owner, name, branch, status, health_score, top_issues JSONB, languages JSONB, created_at, completed_at) + `code_health_issues` (id, analysis_id, severity, category, file_path, line, message, evidence, suggested_fix, rule, sha) |
| `backend/internal/infrastructure/driven/supabase/code_health_repository.go` | **NEW** | Implements `ports.CodeHealthRepository` |
| `backend/internal/infrastructure/driven/cache/analysis_cache.go` (NEW) OR extend `session_cache.go` | **NEW/MODIFY** | TTL-aware file-analysis cache; key `analysis:<file_sha>`, 7d TTL per skill spec |
| `backend/internal/infrastructure/mcp/code_health/` | **NEW** | `analyze_repo.go` + `get_analysis.go` + `get_repo_issues.go` + `generate_health_report.go` + 4 `_test.go` files. Each implements `MCPTool` interface (Name/Description/Schema/Execute). |
| `backend/internal/infrastructure/mcp/registry.go` (or equivalent) | **MODIFY** | Register the 4 new tools in `NewRegistry` |
| `backend/internal/infrastructure/driving/handlers/` | **NEW** | `code_health_handler.go` for async job submit + report fetch endpoints (or all traffic via MCP — design choice) |
| `backend/cmd/api/wiring.go` + `wiring_test.go` + `main.go` | **MODIFY** | Wire `CodeHealthService` + GitHub client + analysis cache + 4 MCP tools + repo token env vars (`GITHUB_TOKEN`, optional per-user override) |
| `backend/internal/core/services/challenge_service.go` | **MODIFY** | Extend `GenerateChallengeHint` with `SourceIssueID string` (mirrors S3's `TopicID`/`Level` additivity); pass `Origin="code_health"` and `GeneratedBy="code-health-v1"` in the persistence step |
| `backend/internal/infrastructure/mcp/challenges/adapter.go` | **MODIFY** | Map `SourceIssueID` into `GenerateChallengeHint`; surface `Origin` on the wire |
| `backend/internal/core/services/audit_history_service.go` | **REUSE** | Persist code-health analyses alongside audit sessions for the history page (optional) |

### Frontend (Angular)

| Path | Status | Why |
|---|---|---|
| `frontend/codeauditor/src/app/domain/models/` | **NEW** | `code-health-analysis.ts`, `code-health-issue.ts`, `health-score.ts` (re-export of `Severity`/`Category` enums to keep parity with backend) |
| `frontend/codeauditor/src/app/domain/ports/` | **NEW** | `code-health-repository.port.ts` (`analyzeRepo`, `getAnalysis`, `getRepoIssues`, `generateHealthReport`) |
| `frontend/codeauditor/src/app/application/` | **NEW** | `code-health.use-case.ts` (signals for `loading`, `analysis`, `error`, `topIssues`, `languages`, `remainingToday`; command flow: `submitAnalysis`, `pollAnalysis`, `generateChallenges`); polling loop with exponential backoff |
| `frontend/codeauditor/src/app/infrastructure/components/code-health/` | **NEW** | 5 dumb standalone components: `code-health-page`, `health-score-card`, `top-issues-list`, `languages-breakdown`, `drill-into-issue-modal`. Plus `generate-challenges-cta.component.ts`. |
| `frontend/codeauditor/src/app/infrastructure/components/codehealth/new-empty.yml` (route config) | **MODIFY** | Register `/repo/:owner/:name/code-health` → `CodeHealthPageComponent` |
| `frontend/codeauditor/src/app/infrastructure/repositories/http-code-health.repository.ts` | **NEW** | Implements `CodeHealthRepositoryPort`; uses `MCPClientPort.callTool` (existing) — no new HTTP wiring |
| `frontend/codeauditor/src/app/infrastructure/components/dashboard/dashboard-page.component.ts` | **MODIFY** | Add a "Code Health" tile linking to a repo-picker modal (per `docs/business-domain.md` §3.3) |
| `frontend/codeauditor/src/app/infrastructure/components/practice/practice-page.component.ts` (and free-practice variants) | **REUSE** | "Generate challenges from these" CTA navigates to the existing dojo flow with `origin=code_health` challenges |

### Docs / skills / harnesses

| Path | Status | Why |
|---|---|---|
| `.atl/harnesses/add-code-health-rule.harness.md` | **REUSE** | Already complete; references `backend/internal/core/services/code_health/rules/<slug>.go` path which doesn't exist yet — first S4 PR creates the dir |
| `docs/mcp-tools.md` §3.7 | **MODIFY** | Already documents the 4 tools; just add `Status: ✅ shipped 2026-XX-XX` once registered |
| `docs/business-domain.md` §3.3 | **MODIFY** | Add notes on which PR landed each piece (Gogs support first, GitHub second) |
| `docs/hexagonal-architecture.md` | **MODIFY** | Replace the planned/aspirational entries with shipped pointers |
| `docs/code-health-rules.md` | **NEW** | Catalogue of seeded rules (rule name, category, severity, languages, detection, false positives) — referenced by harness §Step 7 but does not exist |
| `.atl/curricula/<lang>.md` | **REUSE** | Existing TypeScript curriculum can drive the "generate challenges from issues" path |

### Out of scope (deferred)

- `mcp-inspiration` (Exercism + GitHub education repos) — was deferred in S2 and remains S3+ follow-up, not S4.
- Server-side daily cap on analyses — S5 (`llm-cascade`) generalizes the cascade and can move the cap server-side.
- Curriculum authoring UI for code-health-specific topics — S6+.
- Inline/snapshot rules (markdown / hint rendering improvements in the dojo) — independent of S4.

---

## Approaches

### Option A — Single full-vertical PR (one-shot)

Build the entire S4 surface in one PR: GitHub adapter + analysis cache + 5 seeded rules + service + repository + 4 MCP tools + frontend route + components + dashboard link.

- **Pros:** one review, atomic from the product decision ("code health mode ships"), shortest calendar time.
- **Cons:** likely 3000–4500 changed lines (S3 was ~2400 lines across 3 PRs); exceeds the 400-line review budget by ~10×; one reviewer cannot meaningfully audit it in one pass; merge conflicts with concurrent S5 cascade work; risks the canonical S3 "stacked-to-main" cadence that the project relies on.
- **Effort:** High (3–4 weeks solo, ~2 weeks stacked).
- **Verdict:** ❌ rejected. Breaks the project's stacked-to-main cadence and the chained-PR guidance in AGENTS.md §13.

### Option B — Stacked-to-main, 3 PRs following the S3 pattern (recommended)

Mirror S3's structure exactly: **3 stacked-to-main PRs, ~2400 lines total, ask-on-risk gatekeeper before apply**.

| PR | Goal | Lines (est.) | Depends on |
|---|---|---|---|
| PR 1: Backend foundation | GitHub client + TTL-aware `AnalysisCache` + DB migration `010_*` + `CodeHealthRepository` + 5 starter rules + `CodeHealthService.AnalyzeRepo/GetAnalysis/RankIssues` (async job) | ~900 | master |
| PR 2: Backend MCP + challenges wiring | `mcp-code-health/` group with 4 tools; registry + wiring; extend `GenerateChallengeHint` with `SourceIssueID`; `ChallengeService.GenerateChallenge` accepts `Origin="code_health"`; mapping tests | ~700 | PR 1 |
| PR 3: Frontend dashboard | `domain/models` + `ports` + `application/code-health.use-case.ts` + 5 dumb components + route registration + dashboard tile; specs + tests | ~800 | PR 2 |

- **Pros:** each PR is reviewable in isolation (≤1000 lines, <2× the budget but still within the chained-PR threshold per AGENTS.md §13); rollback boundaries are clean (no skill, no call, no UI regression on revert); reuses the S3 conventions the project team already knows; preserves the ask-on-risk gatekeeper check.
- **Cons:** 3 PR cadence adds 1–2 days of overhead vs. one-shot; the async-job model must be designed up front to avoid PR 2 having to refactor PR 1's sync path.
- **Effort:** Medium (2–3 weeks stacked).
- **Verdict:** ✅ recommended. Same shape as S3 and S2; project team has muscle memory for it.

### Option C — Stacked-to-main, 4 PRs (slim PR 1, expanded 2–4)

Split PR 1 in two: 1a GitHub-only, 1b cache + service + rules.

- **Pros:** each PR ≤600 lines; easier review.
- **Cons:** doubles the PR-count overhead; PR 1a ships the GitHub adapter in isolation, which is not independently useful (no service to bind to); violates the "reviewable work unit" principle in `work-unit-commits`.
- **Effort:** Medium-High (overhead > value).
- **Verdict:** ❌ rejected. PR 1a has no reviewable end-to-end value; the split creates merge-conflict surface for no benefit.

---

## Recommendation

**Adopt Option B (3 stacked-to-main PRs, ask-on-risk gate).**

Rationale:

1. **Pattern parity.** S3 shipped the same shape on 2026-10-10 and is the canonical reference. S4 should not invent a new cadence.
2. **Scope is comparable.** S3 touched `core/services/challenge_service.go` + `driven/llm_cascade/` (3 providers) + 5 new components. S4 touches `core/services/code_health/` (new package) + `driven/github/` (new) + 5 new components — same surface, different domain.
3. **Rollback boundaries are clean.** Each PR reverts independently with no functional regression on the existing 3 practice modes.
4. **Gatekeeper alignment.** The orchestrator's `ask-on-risk` strategy triggers naturally at PR 1 (highest risk: GitHub auth + cache + DB schema) and PR 2 (MCP wire + generate-challenge-from-issue validation).
5. **End-to-end vertical slice on PR 2.** After PR 2 lands, an operator can `curl` `mcp-code-health.analyze_repo(owner, name, "gogs")` and see a valid `health_score` + `top_issues` JSON. That's the smallest proof of value.
6. **PR 3 ships the UI.** The dashboard is the user-visible surface; without it, the backend is invisible.

### Smallest end-to-end vertical slice (proof of value after PR 2)

1. Operator picks `anomalyco/codeauditor` (any public repo) via a CLI script or `curl`.
2. `mcp-code-health.analyze_repo("anomalyco", "codeauditor", "gogs")` → returns `analysis_id`.
3. `mcp-code-health.get_analysis(analysis_id)` (polls every 2s up to 5min) → returns `{status:"completed", health_score: 78, top_issues:[...10], languages: {go: 0.6, ts: 0.3, ...}, summary: "..."}`.
4. `mcp-code-health.generate_health_report(analysis_id, "md")` → returns the report content.
5. (Optional) `mcp-challenges.generate_challenge({SourceIssueID: "issue-3", Language: "go"})` → returns a v2 `Challenge{Origin:"code_health"}`.

This slice proves Gogs-only works end-to-end; GitHub lands in PR 1 but is not on the critical path.

### MVP wire shape for `mcp-code-health.*` tools

```jsonc
// 1. analyze_repo — fire-and-forget, returns analysis_id
{ "name":"mcp-code-health.analyze_repo",
  "args":{ "owner":"anomalyco","name":"codeauditor","source":"gogs","branch":"master" } }

// 2. get_analysis — poll for completion
{ "name":"mcp-code-health.get_analysis", "args":{ "analysis_id":"a-..." } }

// 3. get_repo_issues — list ranked issues with filters
{ "name":"mcp-code-health.get_repo_issues",
  "args":{ "owner":"anomalyco","name":"codeauditor","source":"gogs",
           "severity":"critical|high|medium|low","limit":50 } }

// 4. generate_health_report — produce final artifact
{ "name":"mcp-code-health.generate_health_report",
  "args":{ "analysis_id":"a-...","format":"md|pdf" } }
```

All 4 are documented in `docs/mcp-tools.md` §3.7 — the wire shape is already canonical; PR 2 implements it.

### Existing ports/handlers reused vs. added

**Reused as-is (no changes):**

- `ports.LLMClientPort.Complete` (S3) — file-level LLM analysis (≤50KB files → Ollama route; >50KB → cascade via `code_health_repo` lane).
- `ports.OllamaClient` (existing) — primary file-level analyzer per skill spec; route through cascade.
- `backend/internal/infrastructure/driven/gogs/gogs_client.go` — extends with `ListFiles` (per-repo tree fetch); does not change auth or shape.
- `backend/internal/infrastructure/mcp/registry.go` — pattern only; adds 4 new registrations.
- `domain/models/challenge_models.go::ExpectedFinding` — shape reused for `CodeHealthIssue` (1:1 field mapping).
- `ChallengeService.GenerateChallenge` (S3 un-stubbed) — accepts `SourceIssueID` extension on `GenerateChallengeHint`; same validation gate (`validateChallengeV2` + sandbox `ExecuteWithTests` ≤3 retries).
- Frontend `MCPClientPort` (existing) — `callTool` carries the new tools without port changes.

**New (added in S4):**

- `ports.CodeHealthRepository` (Supabase-backed)
- `ports.CachePort` TTL extension OR new `AnalysisCache` driven adapter
- `domain/models/code_health_analysis.go`, `code_health_issue.go`, `health_score.go`
- `core/services/code_health/{service.go, health_score.go, analyzer.go, rules/registry.go, rules/<slug>.go × 5–10}`
- `driven/github/github_client.go` (+ tests)
- `driven/supabase/migrations/010_create_code_health.sql`
- `driven/supabase/code_health_repository.go`
- `infrastructure/mcp/code_health/{analyze_repo.go, get_analysis.go, get_repo_issues.go, generate_health_report.go}` (+ 4 tests)
- Frontend `domain/models/code-health*.ts` + `domain/ports/code-health-repository.port.ts`
- Frontend `application/code-health.use-case.ts` + `infrastructure/components/code-health/` (5 dumb components) + route registration

---

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| GitHub adapter auth model (anonymous vs token vs org installation) | Medium | MVP ships with PAT (`GITHUB_TOKEN` env); document per-user override later; rate-limit handling returns clean `GITHUB_RATE_LIMITED` error |
| Repository size (>300 files, >100 MB) blows analysis budget | High | Hard cap 300 files/repo + per-file 1 MB (already enforced in `gogs_client.go`); user-facing warning when cap is hit; "Analyze full repo" CTA gated behind explicit confirmation |
| Ollama qwen2.5-coder:3b produces high false-positive rate | High | Layer regex rules FIRST (fast path), LLM only for verification/ambiguous; expose confidence score; let user mark "not an issue" to train cache invalidation |
| Cache invalidation on repo push | Medium | SHA-based cache key (per skill spec); 7d TTL; expose "Re-analyze" CTA that bypasses cache for that repo; document that re-analysis after a force-push is needed for changed files |
| Async job lifecycle (background goroutine + status tracking) | Medium | Use the S3 pattern: status enum `pending|running|completed|failed` + analysis_id returned immediately; status polled via `get_analysis`; job TTL 24h in DB |
| LLM cost (DeepSeek V3 for >50KB files) | Low | Free tier today (per `docs/llm-strategy.md`); per-user daily cap (default 5 analyses/day, like S3's 20 generations/day); hard cap on file size; "Use local Ollama only" toggle for cost-sensitive users |
| Stacked-to-main chain size (~2400 lines) | High | ask-on-risk gatekeeper before apply of PR 1 (matches S3); each PR is ≤1000 lines; rollback boundaries clean |
| `ChallengeService.GenerateChallenge` extension breaks existing callers | Low | Additive: `SourceIssueID string \`json:"sourceIssueID,omitempty"\`` on `GenerateChallengeHint` (same pattern as S3's `TopicID`/`Level` additivity); nil/empty default = no-op behavior |
| Frontend polling loop without SSE | Low | Polling is sufficient for 30s-2min analyses; SSE is overkill for this cadence; expose `?poll_interval_ms` query param to the use case |
| Spec scope creep (issues ranking algorithm, gamification tweaks) | Medium | Lock the ranking formula to the skill's `100 − Σ(weight)` formula; defer gamification tweaks to S6+ (per S3's "out of scope" pattern) |
| `mcp-inspiration` conflict (S3+ deferred; not in S4 scope) | Low | Hard out-of-scope; no cross-spec coupling |
| Hexagonal purity violation (analysis service imports `database/sql`) | Low | Run the canonical grep gate: 0 `database/sql` imports in `backend/internal/core/`; ports + driven adapters respect the direction; verified post-each-PR in CI |
| Frontend domain purity violation (component imports `@angular/*`) | Low | Run canonical grep gate: 0 `@angular/*` imports in `frontend/.../domain/`; use cases + signals + dumb components pattern |

---

## Ready for Proposal

**Yes.** The orchestrator can launch the **`propose`** phase next.

- Proposal can mirror the structure of `openspec/changes/archive/2026-10-10-free-practice-mode/proposal.md` (Intent / Scope / Capabilities / Approach / Affected Areas / Risks / Rollback / Dependencies / Success Criteria / Open Questions).
- Open questions to resolve in the proposal phase:
  1. **Async job lifecycle model.** Goroutine + DB status enum (recommended, matches S3's Socratic orchestration) vs. in-process worker queue (overkill for single-user).
  2. **GitHub auth scope. App-wide `GITHUB_TOKEN` env (MVP) vs. per-user OAuth (S6+). Recommend MVP ships env token + a roadmap note for OAuth in S6+.
  3. **Cache primitive.** New `AnalysisCache` driven adapter (recommended; 7d TTL, key `analysis:<sha>`) vs. extending `ports.CachePort` with `GetWithTTL`/`SetWithTTL` (changes the existing 10-message ring buffer port surface).
  4. **Daily cap.** 5 analyses/day (recommended; matches S3's 20 generations/day) vs. 3 (conservative) vs. no cap (defer to S5 server-side cap).
  5. **Rule seed size.** 5 starter rules (MVP: hardcoded-secret, eval-injection, console-log-leak, magic-number, god-method, leaky-abstraction = 6 across the 5 categories) vs. 10 (better coverage, more PR 1 work).

- Forecast: ~2400 lines total across 3 PRs. **`ask-on-risk` gatekeeper will trigger before apply of PR 1** (matches orchestrator's PR strategy).

### Persistence gates to run on completion

- [ ] `go test -count=1 ./backend/...` — all pass
- [ ] `golangci-lint run ./backend/...` — 0 findings
- [ ] `gofumpt -l ./backend` — empty
- [ ] `go vet ./backend/...` — exit 0
- [ ] `go build ./...` — exit 0
- [ ] `pnpm --dir frontend/codeauditor install` — exit 0
- [ ] `pnpm --dir frontend/codeauditor lint` — 0 errors, 0 warnings
- [ ] `pnpm --dir frontend/codeauditor prettier --check` — 0 issues
- [ ] `npx ng test --watch=false` (×3 consecutive) — all pass
- [ ] `npx ng build` — exit 0
- [ ] Hexagonal purity regression: 0 `database/sql` imports in `backend/internal/core/`; 0 `@angular/*` imports in `frontend/.../domain/`
- [ ] End-to-end smoke: `curl` `mcp-code-health.analyze_repo(owner, name, "gogs")` → poll `get_analysis` → verify `health_score` + `top_issues` returned
- [ ] "Generate challenges from issues" smoke: `curl` `mcp-challenges.generate_challenge({SourceIssueID, Language})` → verify `origin="code_health"` row in `challenges` table

### Migration notes

- **DB:** New migration `010_create_code_health.sql` adds 2 tables. No backfill required (existing rows untouched). Rollback drops both tables.
- **LLM cascade:** New `code_health_repo` lane added to `docs/llm-strategy.md` cascade; no breaking change to existing lanes.
- **MCP monolith:** 4 new tools added to `mcp-code-health` group (the 9th group, completing `docs/mcp-tools.md` §3.7); no breaking change to existing 32 tools.
- **MCP transport:** No protocol change; tools are registered the same way as S2/S3.
- **Frontend routes:** New route `/repo/:owner/:name/code-health` registered; no existing route changed.
- **Backward compatibility:** All new domain fields (`SourceIssueID`, `Origin="code_health"`) are additive with zero-value defaults; existing callers continue to compile and behave as before (same pattern as S3's `TopicID`/`Level`).

---

## Discoveries saved to Engram

- Topic key: `sdd/code-health-dashboard/explore`
- Captures: current code state, GitHub adapter gap, schema reuse opportunity (`ExpectedFinding` 1:1 mapping), S3 stacked-to-main pattern reuse, async-job lifecycle decision deferred to propose, MVP wire shape from `docs/mcp-tools.md` §3.7.