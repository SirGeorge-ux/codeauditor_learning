# Tasks: Free Practice Mode (S3)

> **Estado:** planning completo, listo para apply.
> **Strategy:** stacked-to-main, 3 PRs, como `tutor-chat` (referencia: `archive/2026-10-10-tutor-chat/tasks.md`).
> **Skills requeridos:** `work-unit-commits`, `chained-pr`, `codeauditor-free-practice-generator`.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2400 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Backend cascade `Complete`) → PR 2 (Backend un-stub + content seed) → PR 3 (Frontend) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Backend cascade: `LLMClientPort.Complete` + 3 provider impls + 5s timeout | PR 1 | `go test ./backend/internal/infrastructure/driven/llm_cascade/... ./backend/internal/ports/...` | `go build ./backend/cmd/api` | Revert PR 1 — no caller invokes `Complete` yet; `StreamChat` path is untouched |
| 2 | Backend un-stub: `GenerateChallenge` orchestration + `GenerateChallengeHint` extension + option funcs + `CURRICULA_BASE_DIR` env + MCP tool adapter + typescript curriculum seed | PR 2 | `go test ./backend/internal/core/services/... ./backend/internal/infrastructure/mcp/challenges/...` | `curl -X POST $BASE/api/v1/mcp/tools/call -d '{"name":"mcp-challenges.generate_challenge","args":{"language":"typescript","topicId":"inheritance","level":"D"}}'` | Revert PR 2 — `GenerateChallenge` returns `ErrChallengeGenerationNotWired` (the pre-PR-1 stub error), no row written |
| 3 | Frontend: `domain/models/curriculum.ts` + `PracticeUseCase` (signals, daily cap, localStorage) + 4 dumb components + `practice-page` STUB replacement | PR 3 | `pnpm test -- --include '**/practice/**'` | Navigate `/practice/free` and pick `typescript` / `inheritance` / `D` — verify challenge opens in dojo | Revert PR 3 — `practice-page` falls back to STUB; backend PR 2 is unaffected |

## Phase 1: Backend Cascade JSON-Mode (PR 1 of 3)

> **Branch:** `feature/free-practice-pr1` (base: `master`)
> **Stacked-to-main chain, PR 1.** Phases 2 and 3 land in PR 2 and PR 3.

- [ ] 1.1 Add `Complete`, `CompleteRequest`, `CompleteResponse`, `JSONSchema` to `LLMClientPort` in `backend/internal/ports/tutor.go` (no breaking change to `StreamChat`)
- [ ] 1.2 Add `DefaultCompleteTimeout = 5 * time.Second` and `ErrLLMCompleteTimeout`
- [ ] 1.3 Add `Complete(ctx, CompleteRequest) (CompleteResponse, error)` to cascade orchestrator with provider fallthrough Groq → DeepSeek → MiniMax
- [ ] 1.4 Implement Groq provider `Complete` using `response_format: {type:"json_object"}` (or schema-specific request body shape)
- [ ] 1.5 Implement DeepSeek provider `Complete` (OpenRouter-compat JSON mode)
- [ ] 1.6 Implement MiniMax provider `Complete` (text mode + JSON parser; **NOT** a JSON-mode provider)
- [ ] 1.7 Go unit tests (`ports/tutor_test.go` add): compile-time `Complete` present on `LLMClientPort`
- [ ] 1.8 Go unit tests (`llm_cascade/complete_test.go`): provider fallthrough + JSON-mode request body shape + MiniMax text fallback + 5s timeout
- [ ] 1.9 Go quality gates: `go test`, `golangci-lint`, `gofumpt`, `go vet`, `go build` all pass
- [ ] 1.10 Hexagonal purity check: 0 `database/sql` imports in `backend/internal/core/`

### PR 1 outcome (planned)

Phase 1 delivered as **stacked-to-main** chain, PR 1 of 3. Branch `feature/free-practice-pr1`, base `master`. The cascade gains a non-streaming JSON-mode path; `StreamChat` is untouched. No HTTP handler or MCP tool calls `Complete` yet — it is wired only through the cascade adapter, ready for PR 2 to consume.

## Phase 2: Backend Un-stub + Curricula (PR 2 of 3)

> **Branch:** `feature/free-practice-pr2` (base: `master` after PR 1 merged)
> **Depends on:** PR 1 (`Complete` available on cascade)

- [ ] 2.1 Extend `GenerateChallengeHint` in `backend/internal/core/domain/models/challenge_models.go` with `TopicID string` and `Level string` (F..S)
- [ ] 2.2 Add 4 option funcs to `ChallengeService`: `WithCurriculumService`, `WithPedagogicalService`, `WithLLMClient`, `WithSandboxExecutor` (additive, nil-safe)
- [ ] 2.3 Add `ErrChallengeGenerationNotWired` (returned when `llm` is nil)
- [ ] 2.4 Implement `ChallengeService.GenerateChallenge` orchestration: read topic → fetch learning objective → build story context → assemble system+prompt → call `llm.Complete` → parse JSON draft → loop ≤3 sandbox runs → `validateChallengeV2` → map F-S to difficulty → build `Challenge{Origin:"generated", GeneratedBy:"free-practice-v1", CreatedBy:userID}` → `repo.Insert` → return
- [ ] 2.5 Add helper `difficultyForLevel(level string) string` (`F,E→junior`, `D,C→mid`, `B,A→senior`, `S→architect`)
- [ ] 2.6 Add `ErrChallengeGenerationInvalidJSON`, `ErrChallengeGenerationInvalid`, `ErrChallengeGenerationFailedSandbox`
- [ ] 2.7 Extend `mcp-challenges.generate_challenge` wire shape in `infrastructure/mcp/challenges/challenges_tool.go` and `adapter.go` to accept `topicId`, `level`; map into domain `GenerateChallengeHint`
- [ ] 2.8 Add `CURRICULA_BASE_DIR` env override in `cmd/api/main.go`; default `.atl/curricula`
- [ ] 2.9 Wire new deps into `ChallengeService` construction in `main.go` (option funcs)
- [ ] 2.10 Create `.atl/curricula/typescript.md` — 10–15 topics, F–S ordered, format per `docs/curricula-format.md` §2 (header `# TypeScript Curriculum`, section `## Topics`, each topic `### <slug>` with description)
- [ ] 2.11 Go unit tests (`challenge_service_generate_test.go`, table-driven): prompt assembly, level mapping, retry loop, sandbox gate, `origin/generatedBy/createdBy` fields, persist-via-Insert, unwired error, JSON parse error, validateChallengeV2 error, sandbox-fail-3-times error
- [ ] 2.12 Go unit tests (`mcp/challenges/generate_challenge_adapter_test.go`): wire-shape mapping (topicId, level)
- [ ] 2.13 Go quality gates: `go test`, `golangci-lint`, `gofumpt`, `go vet`, `go build` all pass
- [ ] 2.14 Hexagonal purity check: 0 `database/sql` imports in `backend/internal/core/`

### PR 2 outcome (planned)

Phase 2 delivered as **stacked-to-main** chain, PR 2 of 3. Branch `feature/free-practice-pr2`, base `master` after PR 1. The un-stubbed `GenerateChallenge` is reachable end-to-end through `mcp-challenges.generate_challenge`. The typescript curriculum is the canonical example; other languages arrive via `add-curriculum-topic.harness.md` follow-ups.

## Phase 3: Frontend Domain & Application (PR 3 of 3)

> **Branch:** `feature/free-practice-pr3` (base: `master` after PR 2 merged)
> **Depends on:** PR 2 (`mcp-challenges.generate_challenge` end-to-end)

- [ ] 3.1 Create `domain/models/curriculum.ts` — `CurriculumTopic`, `GenerateChallengeArgs` types
- [ ] 3.2 Create `application/practice.use-case.ts` — signals for `language`, `topic`, `level`, `loading`, `error`, `remainingToday`; `generateChallenge()` command flow; daily-cap enforcement (≥20 rejected); localStorage persistence of daily count
- [ ] 3.3 Implement localStorage daily count key (`practice.dailyCount.<UTC-date>`) with UTC-midnight reset
- [ ] 3.4 Create 4 dumb standalone components: `language-selector`, `level-selector` (F–S), `topic-list` (filtered by language), `generated-challenge` (renders the result + opens dojo route)
- [ ] 3.5 Replace `practice-page.component.ts` STUB with real selector + generated-challenge layout (uses new control flow `@if`/`@for`; **no** `*ngIf`/`*ngFor`)
- [ ] 3.6 Angular tests (`practice.use-case.spec.ts`): signal transitions, daily cap enforcement, localStorage reset
- [ ] 3.7 Angular tests (4 component specs): `input.required` + `output` emit behaviors + render
- [ ] 3.8 Frontend quality gates: `pnpm lint`, `pnpm prettier --check`, `npx ng test --watch=false`, `npx ng build`
- [ ] 3.9 Hexagonal purity: 0 `@angular/*` imports in `frontend/.../domain/`

### PR 3 outcome (planned)

Phase 3 delivered as **stacked-to-main** chain, PR 3 of 3. Branch `feature/free-practice-pr3`, base `master` after PR 2. The `/practice/free` page becomes functional: language → topic → F–S level → generated challenge → dojo. Daily cap is enforced client-side (20 UTC). Frontend is 100% hexagonal: use case + signals + dumb components, port untouched (`MCPClientPort.callTool` is reused).

## Phase 4: End-to-End Verification (closes the chain)

> Verification runs on `master` after PR 3 merges; the per-PR quality gates above are the first line of defense.

- [ ] 4.1 `go test -count=1 ./backend/...` → ALL PASS, 0 FAIL
- [ ] 4.2 `golangci-lint run ./backend/...` → 0 findings
- [ ] 4.3 `gofumpt -l ./backend` → empty
- [ ] 4.4 `go vet ./backend/...` → exit 0
- [ ] 4.5 `go build ./...` → exit 0
- [ ] 4.6 `pnpm --dir frontend/codeauditor install` → exit 0
- [ ] 4.7 `pnpm --dir frontend/codeauditor lint` → 0 errors, 0 warnings
- [ ] 4.8 `pnpm --dir frontend/codeauditor prettier --check` → 0 issues
- [ ] 4.9 `npx ng test --watch=false` (in `frontend/codeauditor`) → all tests pass, 0 FAIL
- [ ] 4.10 `npx ng build` → exit 0
- [ ] 4.11 Hexagonal purity (regression): 0 `database/sql` imports in `backend/internal/core/`; 0 `@angular/*` imports in `frontend/.../domain/`
- [ ] 4.12 End-to-end smoke (manual or Playwright): navigate `/practice/free`, select `typescript` / `inheritance` / `D`, generate, verify challenge opens in dojo and runs in sandbox
- [ ] 4.13 Daily cap smoke: trigger 20 generations, verify 21st is rejected client-side and the UI shows "Daily limit reached"

## Dependencies

- `mcp-monolith` (S2, archived 2026-10-10): `mcp-curriculum`, `mcp-pedagogical`, `mcp-challenges`, `MCPClientAdapter` exist and are stable.
- `tutor-chat` (S2, archived 2026-10-10): `LLMClientPort` (chat-only `StreamChat`) exists; S3 *adds* `Complete` without breaking `StreamChat`. Cascade (Groq → DeepSeek → MiniMax) is already wired.
- `challenge-rebuild` (S1, archived 2026-07-22): v2 `Challenge` model (`expectedFindings`, `testCases`, `hints`) is stable; S3 reuses it.
- `language-progress` (S1, archived 2026-07-22): F–S rank model is stable; S3 reads it through the cascade prompt only (does not modify it).

### Internal dependencies (PR ordering)

- PR 2 depends on PR 1 (`Complete` on cascade).
- PR 3 depends on PR 2 (`mcp-challenges.generate_challenge` end-to-end).
- All three merge to `master` via `--no-ff` after `make validate` passes per PR.

## Out of Scope (deferred)

- Other languages' curricula beyond TypeScript — follow-up via `add-curriculum-topic.harness.md` + the harness's reference PR.
- Curriculum authoring UI (deferred to S6+).
- Server-side daily cap (S5 generalizes the cascade; daily cap moves server-side there).
- `mcp-inspiration` (Exercism + GitHub) — S3+ add-on; not a blocker for S3.
- LLM cascade *generalization* (config-driven multi-provider routing) — S5.
- Updating `codeauditor-free-practice-generator/SKILL.md` to reflect the new flow — small follow-up PR.
- Markdown / hint rendering improvements in the dojo — independent of S3.

## Open Questions

- [ ] Open: confirm the route is `/practice/free` (proposal) vs `/practice` (current stub). Resolve in PR 3 by updating the existing route or aliasing; no spec change required.
- [ ] Open: confirm the canonical `CurriculumTopic.Difficulty` field is a single F–S letter (per `docs/curricula-format.md` §3) and not a 4-bucket string; spec assumes F–S throughout, mapping to 4-bucket only at `Challenge` persistence time. No code change required; docs/curricula-format.md already aligns.
