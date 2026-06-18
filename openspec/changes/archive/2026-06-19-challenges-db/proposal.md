# Proposal: Migrate Challenges from Mock Data to PostgreSQL

## Intent
Replace 8 hardcoded frontend challenges with a real PostgreSQL-backed repository. Enables dynamic challenge management and eliminates mock data maintenance.

## Scope

### In Scope
- Backend: challenge model, service, handler, routes, migrations + seed
- Frontend: HTTP repository, service wiring, JWT header auth
- Seed existing 8 challenges into `public.challenges`

### Out of Scope
- Persisting `tempChallenges` (imported from Gogs)
- Admin CRUD for challenges
- PostgREST direct access from frontend

## Capabilities

### New Capabilities
- `challenge-management`: retrieving challenges from the database via authenticated API

### Modified Capabilities
- None

## Approach
Follow existing Go hexagonal pattern: raw SQL via `database/sql` + `lib/pq`, service receives `*sql.DB`, handler uses constructor injection. Frontend adds `HttpChallengeRepository` using `fetch` with `Authorization` header (consistent with `AuthService`), replacing `MockChallengeRepository` in `ChallengeService` while preserving `tempChallenges` in-memory map.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/core/domain/models/challenge_models.go` | New | Go domain model |
| `backend/internal/core/services/challenge_service.go` | New | Business logic |
| `backend/internal/infrastructure/driving/handlers/challenge_handler.go` | New | HTTP handler |
| `backend/cmd/api/main.go` | Modified | Register `/api/v1/challenges` routes |
| `backend/internal/infrastructure/driven/supabase/migrations/003_create_challenges.sql` | New | Table schema |
| `backend/internal/infrastructure/driven/supabase/migrations/003_seed_challenges.sql` | New | Seed data |
| `frontend/.../repositories/http-challenge.repository.ts` | New | HTTP repo |
| `frontend/.../services/challenge.service.ts` | Modified | Inject HTTP repo, keep tempChallenges |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Auth header gap in frontend HTTP clients | Med | Use `fetch` with manual `Authorization` header |
| Multiline SQL seed syntax errors | Low | Use `$$` delimiters for code fields |
| Manual migration execution missed | Med | Document execution steps in migration files |

## Rollback Plan
1. Revert `challenge.service.ts` to instantiate `MockChallengeRepository`
2. Drop `public.challenges` table
3. Remove backend routes and files

## Dependencies
- Supabase PostgreSQL access
- Existing JWT middleware

## Success Criteria
- [ ] `GET /api/v1/challenges` returns all 8 seeded challenges from DB
- [ ] Frontend displays challenges fetched via HTTP instead of mock
- [ ] `tempChallenges` functionality remains unchanged
