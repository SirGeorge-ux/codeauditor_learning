# Tasks: Language Progress (S1)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1000–1400 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Backend core) → PR 2 (API + hook) → PR 3 (Frontend) |
| Delivery strategy | auto-forecast |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Backend models + service + repo | PR 1 | `go test ./internal/core/services/...` | `go run ./cmd/api` + `curl /health` | Revert migration, drop tables |
| 2 | API endpoints + audit hook | PR 2 | `go test ./internal/infrastructure/driving/...` | `curl /api/v1/users/:id/progress` | Revert handler & AuditService changes |
| 3 | Frontend profile page | PR 3 | `pnpm test -- --include=**/profile/**` | `pnpm start`, navigate to `/profile` | Revert component, route, use case |

## Phase 1: DB & Domain Foundation

- [x] 1.1 Create `db/migrations/XXX_user_language_progress.sql` — `user_language_progress` table (composite PK `user_id, language`)
- [x] 1.2 Create `db/migrations/XXX_user_learning_profile.sql` — `user_learning_profile` table (PK `user_id`)
- [x] 1.3 Create `backend/internal/core/domain/models/language_progress.go` — `LanguageProgress` and `LearningProfile` structs

## Phase 2: Port, Service & Adapter

- [x] 2.1 Create `backend/internal/ports/user_progress.go` — `UserProgressRepository` interface (Upsert, GetAll, Update)
- [x] 2.2 Refactor `backend/internal/core/services/user_progress_service.go` — replace `sql.DB` with port; add range calc (F-S, Jr-Arch), global rank median, `tasa_exito`
- [x] 2.3 Create `backend/internal/infrastructure/driven/supabase/progress_repository.go` — Supabase adapter implementing `UserProgressRepository`
- [x] 2.4 Add anti-gaming dedup in service: `completed_challenge_ids` capped at 500, FIFO eviction, skip duplicate points

## Phase 3: API & Audit Hook

- [x] 3.1 Extend `backend/internal/infrastructure/driving/handlers/user_handler.go` — GET/PUT `/progress/:lang`, GET/PUT `/learning-profile`, GET all progress with `rango_global`
- [x] 3.2 Inject `UserProgressService` into `AuditService` — call `RecordAuditCompletion` in `FinishSession` after session persist

## Phase 4: Frontend Profile Page

- [x] 4.1 Add TS `LanguageProgress` and `LearningProfile` types to `frontend/codeauditor/src/app/domain/models/user.ts`
- [x] 4.2 Create `frontend/codeauditor/src/app/application/profile.use-case.ts` — fetch progress + learning-profile
- [x] 4.3 Create `frontend/codeauditor/src/app/infrastructure/repositories/profile.repository.ts` — HTTP calls via `ProfileRepositoryPort`
- [x] 4.4 Create `frontend/codeauditor/src/app/domain/ports/profile-repository.port.ts` — interface for profile data access
- [x] 4.5 Create `frontend/codeauditor/src/app/infrastructure/components/profile/` — standalone `ProfileComponent` with signals, language cards, F-S badges, Jr-Arch badges, settings form
- [x] 4.6 Add `/profile` route in `frontend/codeauditor/src/app/app.routes.ts`

## Phase 5: Tests

- [x] 5.1 Table-driven test for F-S range calculation (7 thresholds: F→S)
- [x] 5.2 Table-driven test for Jr-Arch range calculation (4 thresholds)
- [x] 5.3 Table-driven test for global rank median (odd, even, empty → Junior default)
- [x] 5.4 Unit test for anti-gaming dedup — same challenge_id skips puntos
- [x] 5.5 Unit test for `tasa_exito` division by zero safeguard
- [x] 5.6 Unit test for `completed_challenge_ids` FIFO cap at 500
- [x] 5.7 Vitest test for `ProfileComponent` — renders language cards with correct rango badges
- [x] 5.8 Run `make test-backend` and `make test-frontend` — all pass
