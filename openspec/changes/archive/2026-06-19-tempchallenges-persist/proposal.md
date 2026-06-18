# Proposal: Persist tempChallenges (imports) to Database

## Intent

Imported challenges from Gogs/GitHub via MCP currently live only in an in-memory `Map<string, Challenge>` in the frontend. We need to persist them in PostgreSQL so they survive browser reloads and users can resume exercise sessions.

## Scope

### In Scope
- Add nullable `user_id` to `public.challenges` via new migration
- `POST /api/v1/challenges` with deduplication (`source_repo` + `user_id`) and new `source_repo` field
- Ownership-aware reads: seeds (NULL) + user-owned rows
- Frontend: replace `addTempChallenge`/`tempChallenges` Map with async `importChallenge` via HTTP POST
- Add `create()` to `ChallengeRepository` port and `HttpChallengeRepository`

### Out of Scope
- Exercise progress persistence
- Findings `.json`/`.md` persistence
- URL normalization library (manual strip for now)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `challenges`:
  - ADD ownership (`user_id` nullable, seeds = NULL)
  - ADD `POST /api/v1/challenges` with deduplication
  - MODIFY `GetAll`/`GetByID` to filter by current user or NULL
  - MODIFY frontend `ChallengeRepository` port: add `create(challenge)`
  - REMOVE frontend `tempChallenges` in-memory Map

## Approach

1. **Schema**: `ALTER TABLE public.challenges ADD COLUMN user_id UUID REFERENCES public.usuarios(id)` in `004_add_challenge_ownership.sql`. Do not modify `003` (deployed).
2. **Deduplication**: `source_repo` field (e.g., `GgogsMIC/academy-mic`, lowercased + trimmed) identifies the source repo. `POST` checks `source_repo + user_id`; returns existing (200) or inserts (201). `repo_url` keeps full URL (e.g., `https://gogs.madeincode.online/GgogsMIC/academy-mic`).
3. **Backend**: `ChallengeService.Create(ctx, challenge, userID)` with dedup logic. `GetAll`/`GetByID` add `WHERE user_id IS NULL OR user_id = $currentUser`. Backend generates UUID v4.
4. **Frontend**: `ChallengeService.importChallenge(repoUrl, metadata)` async POST (returns `Promise<string>`). Only 2 callers to update (mcp-page.component.ts:193, vault-page.component.ts:148), both follow same pattern: `id = await importChallenge()` → `router.navigate()`. `addTempChallenge()` and `tempChallenges` Map removed.
4. **RLS**: Keep ownership filters in Go; add INSERT policy as defense-in-depth.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/.../migrations/004_add_challenge_ownership.sql` | New | Migration + RLS INSERT policy |
| `backend/.../models/challenge_models.go` | Modified | Add `UserID *string` |
| `backend/.../challenge_service.go` | Modified | Add `Create`, ownership filters |
| `backend/.../challenge_handler.go` | Modified | Add `CreateChallenge` handler |
| `backend/cmd/api/main.go` | Modified | Register POST route |
| `frontend/.../challenge-repository.port.ts` | Modified | Add `create()` |
| `frontend/.../http-challenge.repository.ts` | Modified | Implement `create()` |
| `frontend/.../challenge.use-case.ts` | Modified | Add `createChallenge()` |
| `frontend/.../challenge.service.ts` | Modified | Replace Map with async import |

## Risks

| Risk | Likelihood | Mitigation | Resolution |
|------|------------|------------|-----------|
| Callers of sync `addTempChallenge` break | High | Only 2 callers found (mcp-page:193, vault-page:148). Both follow same pattern. | ✅ Resolved — `importChallenge()` returns `Promise<string>`, callers use `await` |
| Deduplication misses on URL variants (http vs https) | Low | New `source_repo` field (`owner/name`, lowercase+trim) as dedup key instead of `repo_url`. | ✅ Resolved — `source_repo` is stable identifier |
| `repo_url` misused as filepath or empty string | High | Current imports set `repoUrl` to `response.path` (file path) or `''`. `source_repo` fixes this. | ✅ Resolved — separate fields for source (`source_repo`) and URL (`repo_url`) |
| Migration on table with existing data | Low | `user_id` nullable; seeds remain NULL | ✅ Acceptable |

## Rollback Plan

1. Revert `004` migration (`ALTER TABLE ... DROP COLUMN user_id`).
2. Restore `tempChallenges` Map and `addTempChallenge()` in frontend.
3. Remove POST route and handler.

## Dependencies

None.

## Success Criteria

- [ ] `POST /api/v1/challenges` persists challenge with `user_id`
- [ ] Duplicate import by same user returns existing row (no dup)
- [ ] `GET /api/v1/challenges` returns seeds + user-owned challenges
- [ ] Frontend `tempChallenges` Map removed; imports survive page reload
- [ ] Existing seed challenges remain readable by all authenticated users
