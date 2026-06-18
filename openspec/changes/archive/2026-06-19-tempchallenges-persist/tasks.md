# Tasks: Persist tempChallenges (Imports) to PostgreSQL

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~450 |
| 800-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-forecast |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

## Phase 1: Database Migration

- [x] 1.1 Create `backend/.../migrations/004_add_challenge_ownership.sql` — ALTER TABLE add `user_id UUID`, `source_repo TEXT`; CREATE INDEX on `(source_repo, user_id)`; CREATE RLS INSERT policy

## Phase 2: Backend Domain Model

- [x] 2.1 Modify `challenge_models.go` — add `UserID *string`, `SourceRepo string` fields; add `CreateChallengeInput` struct for POST body (without ID, UserID, CreatedAt, Status)

## Phase 3: Backend Service

- [x] 3.1 Add `Create()` with dedup — normalize source_repo (lowercase+trim), SELECT dedup by `source_repo + user_id`, INSERT with `gen_random_uuid()` RETURNING
- [x] 3.2 Modify `GetAll()`/`GetByID()` — add `WHERE user_id IS NULL OR user_id = $currentUser` ownership filter; GetByID returns 404 for other user's private challenge

## Phase 4: Backend Handler & Routes

- [x] 4.1 Add `CreateChallenge` handler — decode `CreateChallengeInput`, extract userID from JWT context, call service.Create, return 201/200/400
- [x] 4.2 Update `ListChallenges`/`GetChallenge` signatures to pass userID from context
- [x] 4.3 Register `POST /api/v1/challenges` in auth group in `main.go`

## Phase 5: Backend Tests

- [x] 5.1 Write `challenge_service_test.go` — table-driven tests: Create dedup/insert/normalize, GetAll ownership filter, GetByID 404 for other user's private challenge

## Phase 6: Frontend Domain & Port

- [x] 6.1 Add `sourceRepo?: string` optional field to Challenge model in `challenge.ts`
- [x] 6.2 Add `create(input: CreateChallengeInput): Promise<Challenge>` to `ChallengeRepository` port interface

## Phase 7: Frontend HTTP Repository

- [x] 7.1 Implement `create()` in `http-challenge.repository.ts` — POST to `/api/v1/challenges` with Authorization header, camelCase↔snake_case mapping, throw on network error
- [x] 7.2 Write tests in `http-challenge.repository.spec.ts` — create success, field mapping, network error, auth header

## Phase 8: Frontend Service

- [x] 8.1 Replace `addTempChallenge()` and `tempChallenges` Map with async `importChallenge()` returning `Promise<string>`; update `getChallenge()` to skip Map, reload list after import
- [x] 8.2 Update `challenge.service.spec.ts` — remove tempChallenges tests, add importChallenge tests (calls create, reloads list, returns ID)

## Phase 9: Frontend Use Case & Callers

- [x] 9.1 Add `createChallenge(input)` delegating to repo.create in `challenge.use-case.ts`
- [x] 9.2 Update `mcp-page.component.ts` — replace `addTempChallenge({...})` with `await importChallenge({...})`, pass `sourceRepo` and `repoUrl`
- [x] 9.3 Update `vault-page.component.ts` — replace `addTempChallenge(challenge)` with `await importChallenge({...})`, make method async

## Phase 10: Verification

- [x] 10.1 Run `go test ./...` in backend — all tests pass
- [x] 10.2 Run `ng test` in frontend — all tests pass
- [x] 10.3 Run `go vet ./...` and `tsc --noEmit`
- [x] 10.4 SQL syntax check on migration file
