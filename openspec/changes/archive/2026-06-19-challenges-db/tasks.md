# Tasks: Migrate Challenges from Mock Data to PostgreSQL

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~535 (500–600) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

## Phase 1: Database Migrations

- [x] 1.1 Create `003_create_challenges.sql` — `public.challenges` table with TEXT PK, CHECK constraints, index, RLS SELECT policy for authenticated
- [x] 1.2 Create `003_seed_challenges.sql` — idempotent DO block inserting 8 challenges with explicit IDs, `ON CONFLICT DO NOTHING`

## Phase 2: Backend Domain & Service

- [x] 2.1 Create `challenge_models.go` — `Challenge` struct with `json:"camelCase"` tags matching spec columns
- [x] 2.2 Create `challenge_service.go` — `ChallengeService` with `GetAll(ctx)` and `GetByID(ctx,id)`, `ErrChallengeNotFound` sentinel, raw SQL matching `AuditHistoryService` pattern
- [x] 2.3 Create `challenge_service_test.go` — sqlmock: GetAll returns sorted challenges, GetByID returns challenge for valid ID, GetByID returns `ErrChallengeNotFound` for missing

## Phase 3: Backend Handler & Routes

- [x] 3.1 Create `challenge_handler.go` — `ChallengeHandler` with `ListChallenges` (200 JSON array, nil→`[]`) and `GetChallenge` (200/404 JSON error)
- [x] 3.2 Create `challenge_handler_test.go` — httptest: 200 with array, 404 JSON error, nil→`[]`
- [x] 3.3 Modify `main.go` — wire `ChallengeService` + `ChallengeHandler`, register `GET /api/v1/challenges` and `GET /api/v1/challenges/{id}` under auth middleware group

## Phase 4: Frontend HTTP Repository

- [x] 4.1 Create `http-challenge.repository.ts` — `HttpChallengeRepository` implementing `ChallengeRepository` via `fetch`, `Authorization: Bearer` header, `TokenProvider` interface, returns `[]`/`null` on error/404
- [x] 4.2 Create `http-challenge.repository.spec.ts` — tests: getAll returns [] on network error, getById returns null on 404/error, auth header present

## Phase 5: Frontend Service Wiring

- [x] 5.1 Modify `challenge.service.ts` — replace `MockChallengeRepository` with `HttpChallengeRepository`, inject `TokenProvider` via inline adapter `{ getToken: () => authService.getAccessToken() }`, preserve `tempChallenges` Map priority in `getChallenge(id)`
- [x] 5.2 Update `challenge.service.spec.ts` — verify loadChallenges delegates to HTTP repo, tempChallenge takes priority over HTTP

## Phase 6: Verification

- [x] 6.1 Run `go test ./...` — all backend tests pass ✅
- [x] 6.2 Run `ng test` — all frontend challenge tests pass ✅ (pre-existing gogs.service.spec.ts failure unrelated)
- [x] 6.3 Manual: seed migration SQL syntactically valid (parentheses balanced, 8 challenges, idempotent ON CONFLICT)