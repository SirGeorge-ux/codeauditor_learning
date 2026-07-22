# Proposal: language-progress

## What

Persist per-language mastery (`LanguageProgress`) and learning preferences (`LearningProfile`) so the platform can track F-S and Junior-Architect ranges, drive the tutor's socratic level, and render a `/profile` dashboard.

## Why

The current `users` table only stores auth fields. S1 introduces the new challenge scoring engine (base + tests + lint + findings − hints + time bonus). We need a progress ledger to consume those scores. `LearningProfile` is the memory layer the tutor chat (S2) needs to personalize prompts.

## Scope

### In Scope
- `user_language_progress` table (composite PK `user_id, language`).
- `user_learning_profile` table (PK `user_id`).
- Go domain models, service (`UserProgressService`), port, and Supabase adapter.
- Audit-session hook: `AuditService` → `RecordAuditCompletion` on `completed`.
- Angular `/profile` route, `ProfileComponent`, use cases, and repositories.
- Anti-gaming dedup: same `challenge_id` counts once per language.

### Out of Scope
- Radar chart visualization (deferred to S2 UI polish).
- Automated `LearningProfile` inference from chat history (S2).
- Code-health repo aggregation (S4).
- Leaderboard (rejected per product philosophy).
- Historical audit backfill (start from zero).

## Capabilities

### New
- `language-progress`: scoring, ranges, persistence, anti-gaming.
- `learning-profile`: preferences and learning-style storage.
- `user-profile-page`: `/profile` UI and route.

### Modified
- `user`: extends domain model with progress/learning-profile relations; adds API sub-resources.
- `audit`: adds progress-update side-effect on session completion.

## Approach

1. **DB migration:** create `user_language_progress` and `user_learning_profile` with FK to `users(id)`.
2. **Backend:** `internal/core/domain/models/language_progress.go` + `learning_profile.go`. `UserProgressService` in `core/services/` calculates ranges (pure Go), dedup logic, and `tasa_exito`. `SupabaseUserProgressRepository` in `infrastructure/driven/supabase/` implements the port. Extend `UserHandler` in `infrastructure/driving/handlers/` with GET/PUT endpoints. Inject `UserProgressService` into `AuditService` to call `RecordAuditCompletion` after saving a completed session.
3. **Frontend:** pure TS models in `domain/models/`. Use cases in `application/`. Repositories in `infrastructure/repositories/`. Standalone `ProfileComponent` in `infrastructure/components/profile/` wired via signals.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Existing users lack progress rows | High | Upsert default F/Junior row on first read |
| `tasa_exito` division by zero | Low | Return 0 when `intentados == 0` |
| Dedup array grows unbounded | Low | Cap `completed_challenge_ids` JSONB at 500 entries |
| Challenge-rebuild delay blocks e2e testing | Med | Mock challenges in unit tests; integrate after rebuild merge |

## Rollback Plan

1. Drop `user_language_progress` and `user_learning_profile` tables.
2. Remove new handlers/routes from `UserHandler` and `app.routes.ts`.
3. Remove `UserProgressService` injection from `AuditService`.
4. Delete `profile` component and related use cases. No `users` data is lost.

## Dependencies

- `challenge-rebuild` (S1): new `Challenge` model with `learning_objectives`, `base_points`, `hints`.
- `supabase-auth`: existing `users` table and JWT middleware.

## Success Criteria

- [ ] `user_language_progress` and `user_learning_profile` tables created.
- [ ] Completing an audit updates the correct language row (puntos, rango, tasa_exito, topics_dominados).
- [ ] Same challenge completed twice does NOT double-count puntos.
- [ ] `/profile` renders language cards with correct F-S and Junior-Architect labels.
- [ ] `make test-backend` passes for `UserProgressService` (table-driven range tests).
- [ ] `make test-frontend` passes for `ProfileComponent` and use cases.
- [ ] Zero `core/*` → `infrastructure/*` imports in Go or TS.
