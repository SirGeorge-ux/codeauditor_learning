# Tasks: Challenge Rebuild (S1)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~950 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Backend foundation → PR 2: Scoring + Wiring → PR 3: Frontend |
| Delivery strategy | auto-forecast |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | DB schema + Go model + seeds | PR 1 | `go test ./internal/core/domain/models/` | `make dev-backend` then `GET /api/v1/challenges` | Revert `005_`/`006_` migrations + model rollback |
| 2 | ScoringService + ChallengeService v2 + handler | PR 2 | `go test ./internal/core/services/ -run TestScoring` | Sending audit with known findings and checking score | Revert ScoringService.go + handler changes |
| 3 | Frontend TS model + HttpRepo + delete mock + Dojo UX | PR 3 | `pnpm test -- --run` | Load dojo page, verify hints panel renders | Restore mock-repo from git, revert component changes |

## Phase 1: DB Schema + Domain Model

- [x] 1.1 Create `backend/.../migrations/005_add_challenge_v2_columns.sql` — ALTER TABLE with JSONB columns (hints, expected_findings, test_cases, linter_rules, etc.) and DROP code_smell, repo_url
- [x] 1.2 Extend `backend/.../models/challenge_models.go` — add Hint, ExpectedFinding, TestCase, LinterRule structs; add v2 fields to Challenge; update CreateChallengeInput
- [x] 1.3 Add v2 validation to Challenge (expected_findings 1-3, test_cases 2-4, exactly 3 hints, no-spoiler description check)

## Phase 2: Scoring Engine

- [x] 2.1 Create `backend/.../services/scoring_service.go` — pure Go `ScoringService.Calculate(challenge, AuditInput) ScoreBreakdown` with formula: base + tests×weight×30 + lint×20 + findings×10 − hints_penalty + time_bonus, floor at 0
- [x] 2.2 Create `backend/.../services/scoring_service_test.go` — table-driven tests covering perfect score, partial findings, zero tests, all hints, score floor, time bonus

## Phase 3: Backend Wiring + Seeds

- [x] 3.1 Update `ChallengeService.GetAll`/`GetByID` — SELECT v2 columns, parse JSONB fields with `json.Unmarshal`, exclude solution_code/solution_explanation from response (use `challengeResponse` DTO)
- [x] 3.2 Update `ChallengeService.Create` — validate v2 constraints, INSERT v2 columns
- [x] 3.3 Add `GetByID` `reveal_solution` support — include solution fields when flag + user rank allow
- [x] 3.4 Update `ChallengeHandler` — serialize v2 fields in JSON, accept `?reveal_solution=true` query param
- [x] 3.5 Update `challenge_handler_test.go` — add v2 field expectations in mock rows, test solution exclusion
- [x] 3.6 Create `backend/.../migrations/006_seed_challenges_v2.sql` — idempotent UPSERT replacing 8 curated challenges with v2 context-only descriptions, 1-3 expected_findings, 2-4 test_cases, 1-3 linter_rules, 3 progressive hints, solution_code, solution_explanation

## Phase 4: Frontend Model + Repository

- [x] 4.1 Update `frontend/.../domain/models/challenge.ts` — replace v1 fields with v2: remove codeSmell/repoUrl, add hints/expectedFindings/testCases/linterRules/basePoints/bonusPoints/penaltyPerHint/timeBonus/learningObjectives/commonMistakes/estimatedTimeMinutes/sourceRepo/sourcePath/origin/solutionCode/solutionExplanation
- [x] 4.2 Update `frontend/.../repositories/http-challenge.repository.ts` — map snake_case v2 JSONB fields to camelCase, default null arrays to `[]`, parse Date fields
- [x] 4.3 Delete `frontend/.../repositories/mock-challenge.repository.ts` — remove file and update spec file that references `MockChallengeRepository` (inline mock in `challenge.service.spec.ts`)
- [x] 4.4 Verify no imports reference `mock-challenge.repository` — grep and remove any stale DI wiring

## Phase 5: Dojo UX — Hints + Scoring

- [x] 5.1 Add hints panel to `dojo-page.component.ts` — progressive unlock buttons (level 1 free, level 2→3 cost), show hint content when revealed, track consumed hints in state
- [x] 5.2 Add scoring breakdown overlay — display ScoreBreakdown (base, test, lint, findings, hints penalty, time bonus, total) after audit completes
- [x] 5.3 Update dojo description display to use context-only `description` field, remove prominent code_smell badge (context-panel and dashboard updated)
- [x] 5.4 Run `pnpm test` — 50 tests pass, 6 spec files green

## Phase 6: Verify

- [x] 6.1 `make test-backend` passes — all existing + new backend tests green (reconciled: `go test ./internal/...` 8/8 PASS, `make` unavailable in verification env)
- [x] 6.2 `make test-frontend` passes — all existing + new Angular tests green (reconciled: `pnpm test` 50/50 PASS, `make` unavailable in verification env)
- [x] 6.3 `make validate` passes — lint + format + build + test across stack (reconciled: `go vet ./...` PASS, underlying commands pass, `make` unavailable in verification env)
