# Tasks: Code Health Dashboard (S4)

> **Estado:** planning completo, listo para apply (gate `ask-on-risk` antes de PR 1).
> **Strategy:** stacked-to-main, 3 PRs, mirror de `free-practice-mode` (S3, archivado 2026-10-10).
> **Skills:** codeauditor-code-health, work-unit-commits, chained-pr.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2400 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Backend foundation) → PR 2 (MCP group + S3 wiring) → PR 3 (Frontend dashboard) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |
| Decision needed before apply | Yes |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Backend foundation: source clients + DB cache + migration + 6 rules + `CodeHealthService` | PR 1 | `go test ./backend/internal/core/services/code_health/... ./backend/internal/infrastructure/driven/github/... ./backend/internal/infrastructure/driven/supabase/...` | `go build ./backend/cmd/api` (service constructed, no transport calls it) | Revert PR 1 — drops the 3 tables + `source_issue_id`; nothing calls the service yet |
| 2 | `mcp-code-health` group (4 tools) + registry + `SourceIssueID` wiring | PR 2 | `go test ./backend/internal/infrastructure/mcp/code_health/... ./backend/internal/core/services/...` | `curl -X POST $BASE/api/v1/mcp/tools/call -d '{"name":"mcp-code-health.analyze_repo","args":{"owner":"ggogsmic","name":"academy-mic","source":"gogs"}}'` | Revert PR 2 — unregister 4 tools (33/8); `GenerateChallenge` returns to S3 behavior |
| 3 | Frontend dashboard: models + port + use case + 6 components + route + tile | PR 3 | `pnpm --dir frontend/codeauditor test -- --include '**/code-health/**'` | Navigate `/repo/:owner/:name/code-health`, import a fixture repo → score + issues render | Revert PR 3 — route + components removed; backend unaffected |

## Phase 1: Backend foundation (PR 1 of 3)

> **Branch:** `feature/code-health-pr1` (base: `master`)
> **Stacked-to-main chain, PR 1.** Phases 2–3 land in PR 2 and PR 3.

- [ ] 1.1 Create `backend/internal/ports/code_health.go` — `SourceClient` (ListFiles, GetFileContent), `CodeHealthRepository` (InsertAnalysis, UpdateStatus, Complete, FindByID, ListHistory, CountSince, InsertIssue, ListIssues), `AnalysisCache` (Get/Set with TTL), `OllamaGenerator` (Generate); structs `FileEntry`, `FileContent`; compile-time guards.
- [ ] 1.2 Create `backend/internal/infrastructure/driven/github/github_client.go` — `NewGitHubClient(baseURL, token)`, `ListFiles` (recursive git tree, filter blobs), `GetFileContent` (base64-decode); PAT from `GITHUB_TOKEN`; 30s timeout; translate 404→`ErrGitHubNotFound`, 403+`x-ratelimit-remaining:0`→`ErrGitHubRateLimited`. + `httptest` tests (REQ-CODE-HEALTH-002).
- [ ] 1.3 Extend `backend/internal/infrastructure/driven/gogs/gogs_client.go` with `ListFiles(ctx, owner, repo, branch) ([]FileEntry, error)` (tree fetch); + test (REQ-CODE-HEALTH-001).
- [ ] 1.4 Create `backend/internal/core/domain/models/code_health_issue.go` (`CodeHealthIssue` mirroring `ExpectedFinding` + `Rule`/`Confidence`/`SHA`/`Line`, `Severity`, `IssueCategory`) and `.../code_health_analysis.go` (`CodeHealthAnalysis`, `CodeHealthStatus`).
- [ ] 1.5 Create `backend/internal/core/domain/models/health_score.go` — `Weight(sev)`, `HealthScore(issues)` (`100 − Σ(weight)`, weights `{critical:15,high:8,medium:3,low:1}`, clamp 0–100), `RankIssues` (weight DESC, line ASC, category ASC); + table-driven tests (REQ-CODE-HEALTH-004/005: zero→100, mixed→11, clamp→0).
- [ ] 1.6 Create `backend/internal/core/services/code_health/rules/registry.go` — `Rule` (ID/Category/Severity/Weight/Detect), `Registry` (Register/Rules/Categories), `ErrCodeHealthInvalidCategory`; + tests (REQ-CODE-HEALTH-006: 5 categories covered, unknown category rejected).
- [ ] 1.7 Seed 6 rules (REQ-CODE-HEALTH-007): `rules/secret.go` (`RULE-SEC-SECRET-001`), `rules/injection.go` (`RULE-SEC-INJECT-001`), `rules/n_plus_one.go` (`RULE-PERF-N1-001`), `rules/god_method.go` (`RULE-REFAC-GOD-001`), `rules/magic_number.go` (`RULE-STYLE-MAGIC-001`), `rules/leaky_abstraction.go` (`RULE-ARCH-LEAK-001`); each with ≥5 true-positive + ≥5 true-negative tests.
- [ ] 1.8 Create `backend/internal/infrastructure/driven/supabase/migrations/010_create_code_health.sql` — tables `code_health_analyses`, `code_health_issues`, `analysis_cache` + indexes + RLS mirroring `009_`; **resolved open question: `ALTER TABLE public.challenges ADD COLUMN IF NOT EXISTS source_issue_id TEXT`** (nullable; persists `SourceIssueID` per REQ-CHALLENGES-CODE-HEALTH-001). Down migration drops the 3 tables (issues→analyses order) + drops `source_issue_id`.
- [ ] 1.9 Create `backend/internal/infrastructure/driven/supabase/code_health_repository.go` — `SupabaseCodeHealthRepository` implements `ports.CodeHealthRepository`; every read scopes `user_id`; + `sqlmock` tests.
- [ ] 1.10 Create `backend/internal/infrastructure/driven/supabase/analysis_cache_repository.go` — `SupabaseAnalysisCache` implements `ports.AnalysisCache`; key `analysis:<file_sha>` namespaced by `user_id`; 7d TTL on write, read filters `expires_at > NOW()`; + tests (REQ-CODE-HEALTH-009).
- [ ] 1.11 Create `backend/internal/core/services/code_health/analyzer.go` — per-file pipeline: filter (`node_modules/`, `.git/`, `dist/`, `vendor/`, binaries, >1 MB) → cache lookup → regex rules → LLM only on ambiguous (≤50 KB `OllamaGenerator`; >50 KB `LLMClientPort.Complete(PreferredProvider:"deepseek")`) → cache write; + tests (REQ-CODE-HEALTH-008).
- [ ] 1.12 RED threat tests for 1.11: (a) `analyzer` on a file containing `ignore previous instructions` still yields JSON-schema-conforming findings only; (b) grep gate asserts `GITHUB_TOKEN`/`GOGS_TOKEN` never enters the prompt builder (design threat matrix).
- [ ] 1.13 Create `backend/internal/core/services/code_health/service.go` — `CodeHealthService` + option funcs; `AnalyzeRepo` async (INSERT `pending` → return `analysis_id` <500 ms → goroutine `pending→running→completed|failed`); sync `analyze(ctx, id, repo, force)`; `GetAnalysis`, `GetRepoIssues`, `GenerateHealthReport`; 300-file cap; 24h `expires_at` expiry → `failed`; + service tests (REQ-CODE-HEALTH-003/012/013).
- [ ] 1.14 Wire `GitHubClient` + `AnalysisCache` + `CodeHealthRepository` + `CodeHealthService` construction in `backend/cmd/api/main.go`; read `GITHUB_TOKEN` env (nil-safe when unset).
- [ ] 1.15 Go quality gates green: `go test ./...`, `golangci-lint run ./...`, `gofumpt -l .`, `go vet ./...`, `go build ./...`.
- [ ] 1.16 Hexagonal purity: 0 `database/sql` imports in `backend/internal/core/` (read-only).
- [ ] 1.17 Re-read `backend/internal/infrastructure/driven/supabase/migrations/010_create_code_health.sql` (read-only); PR description documents the 3-table drop + `source_issue_id` drop.
- [ ] 1.18 Create `backend/internal/infrastructure/driven/supabase/migrations/010b_create_suppressions.sql` — table `code_health_suppressions` (PK `(user_id, file_path, rule_id, line_number)`, `created_at`, optional `reason TEXT`); indexes for `(user_id, file_path)`; RLS enabled mirroring `009_`; down migration drops the table. (Resolved per user gate: persistence required, NOT client-side only.)
- [ ] 1.19 Extend `backend/internal/ports/code_health.go` `CodeHealthRepository` with `InsertSuppression` + `IsSuppressed(user_id, file, rule, line)`; extend `SupabaseCodeHealthRepository` accordingly; + `sqlmock` tests (insert dedup, lookup miss/hit).
- [ ] 1.20 Modify `backend/internal/core/services/code_health/analyzer.go` — skip suppressed `(file_path, rule_id, line_number)` combinations after rule detection; + test (REQ-CODE-HEALTH-008 round-trip: insert suppression → re-analyze same file → that issue is not re-emitted).

## Phase 2: MCP group + S3 wiring (PR 2 of 3)

> **Branch:** `feature/code-health-pr2` (base: `master` after PR 1)
> **Depends on:** PR 1 (`CodeHealthService` constructed).

- [ ] 2.1 Create `backend/internal/infrastructure/mcp/code_health/adapter.go` — local `Service` interface + `NewServiceAdapter(*services.CodeHealthService)`.
- [ ] 2.2 Create `backend/internal/infrastructure/mcp/code_health/analyze_repo.go` — `mcp-code-health.analyze_repo` (`owner`,`name`,`source`,`branch`,`force`) async → `{analysis_id,status:"pending"}`; validate `user_id`. RED threat test: `owner="../../evil"` → `-32602` before any outbound call (SSRF).
- [ ] 2.3 Create `backend/internal/infrastructure/mcp/code_health/get_analysis.go` — `mcp-code-health.get_analysis` (`analysis_id`) → `{status, health_score, top_issues, languages, summary, created_at, completed_at, error_message?}`.
- [ ] 2.4 Create `backend/internal/infrastructure/mcp/code_health/get_repo_issues.go` — `mcp-code-health.get_repo_issues` (`owner`,`name`,`source`,`severity?`,`limit?`) → `{issues:[...]}`.
- [ ] 2.5 Create `backend/internal/infrastructure/mcp/code_health/generate_health_report.go` — `mcp-code-health.generate_health_report` (`analysis_id`,`format=md|pdf`) → markdown `{report_id,content,format}`; `pdf` → `-32603` not implemented until S5.
- [ ] 2.6 Modify `backend/cmd/api/wiring.go` — add `CodeHealthService` to `WiringDeps`, register the 4 tools in `BuildMCPRegistry`; + `wiring_test.go` count assertion (37 tools / 9 groups).
- [ ] 2.7 Modify `backend/internal/core/domain/models/challenge_models.go` — add `SourceIssueID string \`json:"sourceIssueId,omitempty"\`` to `GenerateChallengeHint` (additive, zero-value = no-op).
- [ ] 2.8 Modify `backend/internal/core/services/challenge_service.go` — `GenerateChallenge` branches on `SourceIssueID`: `Origin="code_health"`, `GeneratedBy="code-health-v1"`, `SourcePath=<issue.file_path>`, persist `source_issue_id`; orchestration (LLM → sandbox ≤3 → validate → persist) unchanged; + tests (both origins, REQ-CHALLENGES-CODE-HEALTH-001/002/003).
- [ ] 2.9 Modify `backend/internal/infrastructure/mcp/challenges/adapter.go` + `challenges_tool.go` — accept optional `sourceIssueId` wire arg → `GenerateChallengeHint.SourceIssueID`; + mapping test.
- [ ] 2.11 Daily-cap defense-in-depth: `analyze_repo` calls `CountSince(userID, midnightUTC)`; ≥5 → MCP error `-32609` "Daily limit reached, resets at 00:00 UTC"; + test (REQ-CODE-HEALTH-011).
- [ ] 2.12 Add `mcp-code-health.suppress_issue` (`issue_id`, `reason?`) tool — marks the issue as suppressed for the authenticated user (writes to `code_health_suppressions`); returns `{suppression_id, suppressed_at}`; + tests (auth missing → -32602, unknown issue → -32603, happy path).
- [ ] 2.13 Go tests for the 5 new tools (analyze_repo / get_analysis / get_repo_issues / generate_health_report / suppress_issue): happy path + arg parsing + `-32602` on missing `user_id` (REQ-MCP-CODE-HEALTH-003); mock the service, no network.
- [ ] 2.14 Go quality gates green + hexagonal purity: 0 `database/sql`/`net/http` imports in `backend/internal/infrastructure/mcp/code_health/` (read-only).

## Phase 3: Frontend dashboard (PR 3 of 3)

> **Branch:** `feature/code-health-pr3` (base: `master` after PR 2)
> **Depends on:** PR 2 (`mcp-code-health.*` end-to-end).

- [ ] 3.1 Create `frontend/codeauditor/src/app/domain/models/code-health.ts` — `CodeHealthAnalysis`, `CodeHealthIssue`, severity/category/status/source unions; + spec.
- [ ] 3.2 Create `frontend/codeauditor/src/app/domain/ports/code-health.port.ts` — `CodeHealthPort` (analyzeRepo/getAnalysis/getRepoIssues/generateHealthReport) + `AnalyzeRepoArgs`; + spec.
- [ ] 3.3 Create `frontend/codeauditor/src/app/infrastructure/repositories/code-health.repository.ts` — impl delegating to `MCPClientPort.callTool` (4 tools); + spec.
- [ ] 3.4 Create `frontend/codeauditor/src/app/application/code-health.use-case.ts` — signals (`analysis`,`loading`,`error`,`topIssues`,`languages`,`remainingToday`); `submitAnalysis` → `pollAnalysis` (2s→30s backoff until `completed|failed`); 5/day cap (`codeHealth.dailyCount.<UTC-date>` in localStorage, injected `Storage`/`now()`); + spec (transitions, backoff, ≥6 rejected, UTC reset).
- [ ] 3.5 Create 5 dumb components in `frontend/codeauditor/src/app/infrastructure/components/code-health/`: `repo-url-bar`, `import-button`, `health-gauge` (SVG 0–100), `issues-table` (sortable + severity filter), `category-breakdown`; + 5 specs.
- [ ] 3.6 Create `.../components/code-health/code-health-page.component.ts` — composes the 5 + the CTA; + spec.
- [ ] 3.7 Create `.../components/code-health/generate-challenges-cta.component.ts` — calls `mcp-challenges.generate_challenge({sourceIssueId, language})`; + spec.
- [ ] 3.8 Modify `frontend/codeauditor/src/app/app.routes.ts` — register `/repo/:owner/:name/code-health`.
- [ ] 3.9 Modify `frontend/codeauditor/src/app/infrastructure/components/dashboard/dashboard-page.component.ts` — add the code-health tile; + spec.
- [ ] 3.10 Frontend quality gates green: `pnpm --dir frontend/codeauditor lint`, `prettier --check`, `npx ng test --watch=false`, `npx ng build`.
- [ ] 3.11 Hexagonal purity: 0 `@angular/*` imports in `frontend/codeauditor/src/app/domain/` (read-only).

## Phase 4: End-to-end verification (closes the chain)

> Verification runs on `master` after PR 3 merges; per-PR gates above are the first line of defense.

- [ ] 4.1 `go test -count=1 ./backend/...` → ALL PASS, 0 FAIL
- [ ] 4.2 `golangci-lint run ./backend/...` → 0 findings
- [ ] 4.3 `gofumpt -l ./backend` → empty
- [ ] 4.4 `go vet ./backend/...` → exit 0
- [ ] 4.5 `go build ./...` → exit 0
- [ ] 4.6 `pnpm --dir frontend/codeauditor install` → exit 0
- [ ] 4.7 `pnpm --dir frontend/codeauditor lint` → 0 errors, 0 warnings
- [ ] 4.8 `pnpm --dir frontend/codeauditor prettier --check` → 0 issues
- [ ] 4.9 `npx ng test --watch=false` (in `frontend/codeauditor`) → all pass, 0 FAIL
- [ ] 4.10 `npx ng build` → exit 0
- [ ] 4.11 Hexagonal purity regression: 0 `database/sql` in `backend/internal/core/`; 0 `@angular/*` in `frontend/codeauditor/src/app/domain/` (read-only)
- [ ] 4.12 `GET /mcp/tools/list` → 37 tools / 9 groups incl. `mcp-code-health.*` (4) (read-only)
- [ ] 4.13 `make validate` → exit 0
- [ ] 4.14 E2E smoke (manual): `analyze_repo(owner,name,"gogs")` → poll `get_analysis` → verify `health_score` + `top_issues`
- [ ] 4.15 Generate-from-issue smoke (manual): `generate_challenge({sourceIssueId, language})` → verify `origin="code_health"` + `source_issue_id` row
- [ ] 4.16 Dashboard smoke (manual): navigate `/repo/:owner/:name/code-health`, import a fixture repo, verify score + issues render; 6th analysis rejected by daily cap

## Dependencies

- `mcp-monolith` (S2, archived 2026-10-10): `mcpkit` runtime + `MCPClientAdapter` + registry pattern.
- `tutor-chat` (S2, archived 2026-10-10): `LLMClientPort.Complete` + `llm_cascade` (Groq→DeepSeek→MiniMax) + `OllamaClient`.
- `free-practice-mode` (S3, archived 2026-10-10): `GenerateChallenge` un-stub + sandbox gate `ExecuteWithTests` + `GenerateChallengeHint` additive-field pattern.
- `challenge-rebuild` (S1, archived 2026-07-22): v2 `Challenge` model + `ExpectedFinding` shape (reused 1:1).

### Internal dependencies (PR ordering)

- PR 2 depends on PR 1 (`CodeHealthService` constructed).
- PR 3 depends on PR 2 (`mcp-code-health.*` end-to-end).
- All three merge to `master` via `--no-ff` after `make validate` passes per PR.

## Out of Scope (deferred)

- **GitHub OAuth flow** — MVP uses app-wide `GITHUB_TOKEN`; per-user OAuth is S6+.
- **Server-side daily cap** — S4 ships client cap + count-query (`-32609`); full server cap moves to S5.
- **`generate_health_report(format:"pdf")`** — returns `-32603` until S5.
- **`mcp-inspiration`** (Exercism + GitHub education) — deferred since S2.
- **Multi-user sharing / leaderboards / public reports** — single-user dev.
- **Curriculum authoring UI for code-health topics** — S6+.
- **Inline / snapshot rule rendering in the dojo** — independent of S4.
- **Auto-fix / patch generation** — read-only analysis mode.
- **Real-time push (SSE/WebSocket)** — polling suffices for the 30s–2min cadence.
- **New rules beyond the 6 seeds** — follow-up via `add-code-health-rule.harness.md`.

## Open Questions

> **Resolved in this phase via the `ask-on-risk` user gate (2026-10-11):**

- [x] **Suppression feedback (REQ-CODE-HEALTH-008 "not an issue" scenario):** **resolved — implement now.** Tasks `1.18` (migration `010b_create_suppressions.sql`), `1.19` (port + repository extension), `1.20` (analyzer skip), `2.12` (`mcp-code-health.suppress_issue` tool) cover end-to-end persistence. NOT client-side only.
- [x] **REST endpoints (REQ-CODE-HEALTH-013 `/api/v1/code-health/*`):** **resolved — MCP-only is acceptable for S4.** Frontend consumes exclusively via `MCPClientAdapter`. REST path remains a S5+ candidate.

> **Resolved in this phase:** `source_issue_id TEXT` (nullable) lands in migration `010_` on the `challenges` table (1.8) — the `challenges` delta persists `SourceIssueID` on the resulting `Challenge` row. `generate_health_report(format:"pdf")` returns `-32603` until S5 (accepted).
