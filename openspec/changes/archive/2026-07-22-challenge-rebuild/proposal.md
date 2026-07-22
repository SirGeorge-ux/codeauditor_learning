# Proposal: challenge-rebuild

## What

Migrate the 8 curated challenges from the deprecated `MockChallengeRepository` to a persistent v2 Challenge model in PostgreSQL. Rewrite every description to provide context instead of spoiling the answer. Add objective rubric data (`expected_findings`, `test_cases`, `linter_rules`, `hints`, `solution_code`) and a reproducible scoring engine. Delete `mock-challenge.repository.ts` permanently.

## Why

Current descriptions in `MockChallengeRepository` literally name the smell (e.g., ch-sqli says "Este endpoint es vulnerable a SQL Injection..."). Users read the description and already know the answer — this is a quiz, not a dojo. The v1 model lacks `expected_findings`, `hints`, and `test_cases`, so scoring is subjective and non-reproducible. S1 must fix this pedagogical and technical debt before Free Practice (S3) can reuse the same model.

## Scope

### In Scope
- DB migration: add v2 columns (`learning_objectives`, `hints`, `expected_findings`, `test_cases`, `linter_rules`, `solution_code`, `solution_explanation`, `base_points`, `bonus_points`, `penalty_per_hint`, `time_bonus`, `origin`, `created_by`) to `public.challenges`
- Rewrite 8 curated challenges (ch-sqli, ch-xss, ch-god, ch-callback, ch-mutation, ch-dead, ch-errors, ch-naming) with context-only descriptions, 1-3 expected findings, 2-4 test cases, 1-3 linter rules, 3 progressive hints, and solution code
- Delete `frontend/codeauditor/src/app/infrastructure/repositories/mock-challenge.repository.ts`
- Backend: update `models.Challenge` and `CreateChallengeInput` in `backend/internal/core/domain/models/challenge_models.go`
- Backend: implement objective scoring engine in `backend/internal/core/services/` (hexagonal, pure Go)
- Frontend: update `Challenge` domain model in `frontend/codeauditor/src/app/domain/models/challenge.ts`
- Frontend: update `HttpChallengeRepository` and `ChallengeService` to consume v2 API
- Frontend: display hints with cost in Dojo, show scoring breakdown after audit

### Out of Scope
- Free practice mode challenge generation (S3)
- Code health dashboard (S4)
- Tutor chat integration with hints (S2)
- Import from markdown file

### Deferred
- More than 8 curated challenges
- Hint reveal animation / UX polish
- Difficulty auto-adjustment

## Capabilities

### New Capabilities
- `challenge-scoring`: objective score calculation (`base + tests×weight×30 + lint×20 + findings×10 − hints×cost + time_bonus`)

### Modified Capabilities
- `challenges`: v2 model migration — new fields (`expected_findings`, `hints`, `test_cases`, `linter_rules`, `solution_code`, etc.), delete mock repository, rewrite 8 seeds

## Approach

**Backend:** extend `public.challenges` with JSONB columns for array/object fields to avoid schema explosion. Update Go domain model and `ChallengeService`. Implement a pure `ScoringService` in `core/services/` that takes an `AuditSession` + `Challenge` and returns a score breakdown — zero infrastructure imports, fully testable.

**Frontend:** update the `Challenge` interface to match v2, then update `HttpChallengeRepository` to parse new JSONB fields. Delete `MockChallengeRepository` and wire `ChallengeService` to HTTP only. In `dojo-page.component.ts`, add a hints panel and a scoring-breakdown overlay after audit submission.

**Data migration:** write an idempotent SQL seed file (`005_seed_challenges_v2.sql`) that replaces the 8 curated challenges with v2 data using `ON CONFLICT (id) DO UPDATE`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `backend/internal/core/domain/models/challenge_models.go` | Modified | Add v2 fields (Hints, ExpectedFindings, TestCases, LinterRules, SolutionCode, etc.) |
| `backend/internal/core/services/challenge_service.go` | Modified | Parse JSONB fields; validate v2 data |
| `backend/internal/core/services/` | New | `scoring_service.go` — hexagonal, pure Go |
| `public.challenges` (PostgreSQL) | Modified | Migration adds JSONB columns for v2 arrays |
| `frontend/.../domain/models/challenge.ts` | Modified | Extend interface with v2 fields |
| `frontend/.../repositories/mock-challenge.repository.ts` | Deleted | Replaced by HTTP repository + DB seeds |
| `frontend/.../repositories/http-challenge.repository.ts` | Modified | Parse v2 response fields |
| `frontend/.../components/dojo/dojo-page.component.ts` | Modified | Add hints UI, scoring breakdown display |
| `openspec/specs/challenges/spec.md` | Modified | Update to v2 requirements |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| JSONB column performance on large challenge lists | Low | 8 rows is trivial; add GIN index if query patterns change |
| Rewriting 8 descriptions without spoilers is harder than expected | Med | Review each with the "would I know the answer?" test from `codeauditor-add-challenge` skill |
| Frontend crashes parsing new JSONB fields | Low | Add validation in repository layer; never pass `any` to components |
| Scoring formula feels unfair to users | Med | Calibrate `base_points` so `solution_code` yields satisfying max score |

## Rollback Plan

1. Revert DB migration with `ALTER TABLE` to drop new v2 columns (all nullable).
2. Restore `MockChallengeRepository` from git history if frontend must fall back.
3. Re-deploy previous Docker image. Old v1 schema remains compatible.

## Dependencies

- `openspec/changes/challenge-rebuild/specs/challenges/spec.md` (v2 delta spec)
- PostgreSQL 15+ (JSONB, `gen_random_uuid()`)
- Existing `ChallengeService` and `HttpChallengeRepository` from archived `2026-06-19-challenges-db` spec

## Success Criteria

- [ ] `mock-challenge.repository.ts` is deleted and no import references it
- [ ] All 8 challenges return from `GET /api/v1/challenges` with full v2 fields
- [ ] Submitting `solution_code` yields maximum score (all tests pass, all findings matched, no hints, time bonus)
- [ ] Using all 3 hints and submitting correct solution yields score < max (penalty applied)
- [ ] Every description passes the "no spoiler" test: a developer cannot name the smell from the description alone
- [ ] `make validate` passes (Go + Angular tests + lint)
