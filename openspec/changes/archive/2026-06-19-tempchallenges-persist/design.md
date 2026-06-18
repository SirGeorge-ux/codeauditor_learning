# Design: Persist tempChallenges (imports) to Database

## Technical Approach

Replace the frontend `tempChallenges` in-memory Map with a persisted backend flow: `POST /api/v1/challenges` creates challenges in PostgreSQL with ownership (`user_id`) and source tracking (`source_repo`). Existing reads (`GetAll`, `GetByID`) become ownership-aware, returning seed challenges (NULL `user_id`) plus user-owned rows. Deduplication on `source_repo + user_id` prevents duplicate imports.

Maps to spec requirements: Database Schema (ADDED), Backend API Create (ADDED), Backend Service Create (ADDED), Backend Handler Create (ADDED), Frontend Repository Port (ADDED), Frontend HTTP Repository create (ADDED), Frontend Service importChallenge (MODIFIED), Backend Service ownership reads (MODIFIED), Backend Handler ownership reads (MODIFIED), Backward Compatibility (MODIFIED).

## Architecture Decisions

| Decision | Option A (chosen) | Option B (rejected) | Rationale |
|----------|-------------------|---------------------|-----------|
| Dedup key | `source_repo` (`owner/name`, lowercased+trimmed) | `repo_url` (full URL) | `repo_url` is misused — currently set to file paths or empty strings. `source_repo` is a stable identifier. |
| ID generation | PostgreSQL `gen_random_uuid()` via `RETURNING id` | Go `google/uuid` library | No UUID dependency in go.mod. DB already uses `gen_random_uuid()` in audit_sessions. Simpler, no new dependency. |
| Ownership filter location | Go service layer (WHERE clause) | Supabase RLS only | RLS is defense-in-depth. Go layer gives explicit control and consistent error handling. |
| Dedup return code | 200 with existing challenge | 209 or custom header | 200 is standard for "resource already exists, here it is". Client can distinguish from 201 (created). |
| Frontend temp ID removal | Backend returns real UUID, no temp IDs | Keep temp IDs, sync later | Temp IDs cause collision risk and require reconciliation. Backend-generated UUID is clean separation. |

## Data Flow

### Create Flow (new import)

```
MCP Page ──→ ChallengeService.importChallenge()
                  │
                  ▼
           ChallengeUseCase.createChallenge()
                  │
                  ▼
           HttpChallengeRepository.create()
                  │
                  ▼  POST /api/v1/challenges
           Chi Router ──→ JWT Middleware
                  │
                  ▼
           ChallengeHandler.CreateChallenge()
                  │
                  ▼
           ChallengeService.Create()
                  │
                  ├── Normalize source_repo (lowercase + trim)
                  ├── SELECT WHERE source_repo + user_id (dedup)
                  │     └── Found → return existing (200)
                  │     └── Not found → INSERT with gen_random_uuid() (201)
                  │
                  ▼
           PostgreSQL (public.challenges)
                  │
                  ▼
           201 Response ← Parse JSON ← Return Challenge
                  │
                  ▼
           loadChallenges() → Update signals → Navigate /dojo/:id
```

### Dedup Flow (same repo re-import)

```
MCP Page ──→ importChallenge()
                  │
                  ▼
           POST /api/v1/challenges (same source_repo)
                  │
                  ▼
           ChallengeService.Create()
                  │
                  ▼
           SELECT WHERE LOWER(source_repo) = $1 AND user_id = $2
                  │
                  ▼
           Found → return existing challenge
                  │
                  ▼
           200 Response (skip INSERT)
                  │
                  ▼
           loadChallenges() → Navigate /dojo/:id
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/driven/supabase/migrations/004_add_challenge_ownership.sql` | Create | ALTER TABLE add `user_id`, `source_repo`, index, RLS INSERT policy |
| `backend/internal/core/domain/models/challenge_models.go` | Modify | Add `UserID *string`, `SourceRepo string`; add `CreateChallengeInput` struct |
| `backend/internal/core/services/challenge_service.go` | Modify | Add `Create()` with dedup; modify `GetAll()`/`GetByID()` with ownership filters; add `ErrInvalidDifficulty` |
| `backend/internal/infrastructure/driving/handlers/challenge_handler.go` | Modify | Add `CreateChallenge()` handler |
| `backend/cmd/api/main.go` | Modify | Register `POST /api/v1/challenges` route in auth group |
| `frontend/codeauditor/src/app/domain/models/challenge.ts` | Modify | Add optional `sourceRepo?: string` |
| `frontend/codeauditor/src/app/domain/ports/challenge-repository.port.ts` | Modify | Add `create(input): Promise<Challenge>` |
| `frontend/codeauditor/src/app/infrastructure/repositories/http-challenge.repository.ts` | Modify | Implement `create()`; add `CreateChallengeInput` interface; update `mapSnakeToCamel` for new fields |
| `frontend/codeauditor/src/app/application/challenge.use-case.ts` | Modify | Add `createChallenge(input)` method |
| `frontend/codeauditor/src/app/infrastructure/services/challenge.service.ts` | Modify | Replace `addTempChallenge`/`tempChallenges` Map with `importChallenge()`; update `getChallenge()` to skip Map |
| `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.ts` | Modify | Replace `addTempChallenge()` call with `await importChallenge()`; make `fetchAndAudit()` async |
| `frontend/codeauditor/src/app/infrastructure/components/vault/vault-page.component.ts` | Modify | Replace `addTempChallenge()` call with `await importChallenge()`; make `reAudit()` async |

## Interfaces / Contracts

### Go — `CreateChallengeInput` (request body)

```go
type CreateChallengeInput struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    Difficulty  string `json:"difficulty"`
    Category    string `json:"category"`
    Language    string `json:"language"`
    RepoURL     string `json:"repo_url"`
    SourceRepo  string `json:"source_repo"`
    Code        string `json:"code"`
    CodeSmell   string `json:"code_smell"`
}
```

### Go — Extended `Challenge` model

```go
type Challenge struct {
    // ... existing fields ...
    UserID     *string    `json:"userId,omitempty"`
    SourceRepo string     `json:"sourceRepo,omitempty"`
}
```

### Go — `ChallengeService.Create` signature

```go
func (s *ChallengeService) Create(ctx context.Context, input CreateChallengeInput, userID string) (*models.Challenge, error)
```

Returns `(challenge, nil)` on both dedup hit and new insert. Caller distinguishes via HTTP status code set by handler.

### TypeScript — `CreateChallengeInput`

```typescript
export interface CreateChallengeInput {
  title: string;
  description: string;
  difficulty: ChallengeDifficulty;
  category: string;
  language: string;
  repoUrl: string;
  sourceRepo: string;
  code: string;
  codeSmell: string;
}
```

### TypeScript — Extended `ChallengeRepository` port

```typescript
export interface ChallengeRepository {
  getAll(): Promise<Challenge[]>;
  getById(id: string): Promise<Challenge | null>;
  create(challenge: CreateChallengeInput): Promise<Challenge>;
}
```

### TypeScript — Extended `Challenge` domain model

```typescript
export interface Challenge {
  // ... existing fields ...
  sourceRepo?: string;
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Go) | `Create()` dedup logic with varied `source_repo` casing/whitespace | Table-driven tests with `sqlmock` |
| Unit (Go) | `Create()` validates difficulty enum | Table-driven tests |
| Unit (Go) | `GetAll()` ownership filter — excludes other users' rows | `sqlmock` with seeded rows |
| Unit (Go) | `GetByID()` returns not-found for other user's private challenge | `sqlmock` |
| Unit (TS) | `HttpChallengeRepository.create()` maps camelCase → snake_case | Jest unit test with fetch mock |
| Unit (TS) | `ChallengeService.importChallenge()` calls useCase and reloads | Jest with mocked useCase |
| Integration | `POST /api/v1/challenges` → row in DB → `GET` returns it | Test container or Supabase test project |
| Integration | Duplicate POST returns 200 with same ID, no new row | Same as above |

## Migration / Rollout

1. Run `004_add_challenge_ownership.sql` — adds nullable columns, no data change needed.
2. Deploy backend with new `Create()` handler and ownership-aware reads.
3. Deploy frontend — `importChallenge()` replaces `addTempChallenge()`.
4. No feature flag needed. Rollback: drop columns, restore Map.

### Migration SQL

```sql
-- 004_add_challenge_ownership.sql
ALTER TABLE public.challenges ADD COLUMN user_id UUID REFERENCES public.usuarios(id);
ALTER TABLE public.challenges ADD COLUMN source_repo TEXT;
CREATE INDEX idx_challenges_source_repo_user ON public.challenges(source_repo, user_id);

-- RLS INSERT policy (defense-in-depth)
CREATE POLICY "Users can insert own challenges" ON public.challenges
    FOR INSERT WITH CHECK (auth.uid() = user_id);
```

## Open Questions

- [ ] Should `source_repo` have a `NOT NULL` constraint for user-created challenges? (Deferred — nullable allows gradual migration)
- [ ] Should the dedup query also filter by `status = 'available'`? (Yes — per spec, only available challenges are dedup candidates)
