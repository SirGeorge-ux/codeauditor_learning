# Design: language-progress

## Technical Approach

Introduce two new tracking layers—`LanguageProgress` (F-S ranges for free practice, Junior-Architect for code health) and `LearningProfile` (tutor chat preferences). We will refactor the existing `UserProgressService` to strictly follow Hexagonal Architecture (replacing the direct `sql.DB` dependency with a pure `UserProgressRepository` port). The service will handle range calculation (pure Go), global rank determination (median of language ranks), and anti-gaming (JSONB ID deduplication). `AuditService` will trigger `RecordAuditCompletion` after each completed session. The frontend will consume this via a new `/profile` dashboard route.

## Architecture Decisions

### Decision: Hexagonal Refactor of UserProgressService

**Choice**: Extract `sql.DB` from `UserProgressService` into a `UserProgressRepository` port, implemented by `infrastructure/driven/supabase`.
**Alternatives considered**: Leave `sql.DB` and just add new queries.
**Rationale**: Strict hexagonal architecture is non-negotiable in this project. `core/services` cannot know about SQL logic directly.

### Decision: Global Rank Calculation

**Choice**: Pure Go logic in `UserProgressService` computes the global rank as the median of all active language ranks.
**Alternatives considered**: Compute on-the-fly via a PostgreSQL database view.
**Rationale**: Keeping the calculation in the domain layer ensures testability via pure Go tests and avoids logic leaking into the DB.

### Decision: Anti-Gaming Dedup Mechanism

**Choice**: Maintain a `completed_challenge_ids JSONB` array in `user_language_progress`, capped at 500 entries.
**Alternatives considered**: A separate mapping table (`user_challenge_completions`).
**Rationale**: Simpler structure avoiding N+1 row lookups per update, easily handled in code. Cap ensures it doesn't grow unboundedly.

### Decision: Audit Hook Integration

**Choice**: Synchronous call `UserProgressService.RecordAuditCompletion` at the end of `AuditService.FinishSession`.
**Alternatives considered**: Async Event-driven (channel/queue).
**Rationale**: The monolith design allows safe synchronous orchestration. If the DB fails, it's easier to surface the error directly on session completion.

## Data Flow

    AuditService (FinishSession)
         │
         └─→ UserProgressService.RecordAuditCompletion(userID, challengeID, session)
                 │
                 ├── 1. Fetches current LanguageProgress (Port)
                 ├── 2. Anti-gaming: Ignores points if challengeID in dedup array
                 ├── 3. Calculates new Score, TasaExito, and Range (F-S / Jr-Arch)
                 ├── 4. Calculates Global Rank (median)
                 └── 5. Saves Progress & Global Rank (Port) ──→ Supabase

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/core/domain/models/language_progress.go` | Create | Go structs for `LanguageProgress` and `LearningProfile`. |
| `backend/internal/ports/user_progress.go` | Create | Define `UserProgressRepository` port interface. |
| `backend/internal/core/services/user_progress_service.go` | Modify | Remove `sql.DB`, use port. Add RecordAuditCompletion & calculation logic. |
| `backend/internal/core/services/audit_service.go` | Modify | Inject `UserProgressService`, invoke on audit completion. |
| `backend/internal/infrastructure/driven/supabase/progress_repository.go`| Create | PostgreSQL implementation of the progress port. |
| `backend/internal/infrastructure/driving/handlers/user_handler.go` | Modify | Add `GET /api/v1/users/:id/profile` endpoint. |
| `frontend/codeauditor/src/app/domain/models/user.ts` | Modify | Add TS `LanguageProgress` and `LearningProfile` definitions. |
| `frontend/codeauditor/src/app/application/profile.use-case.ts` | Create | Use case to fetch full user profile. |
| `frontend/codeauditor/src/app/infrastructure/components/profile/` | Create | Standalone Angular component with signals and `@for` control flow. |
| `db/migrations/XXX_user_language_progress.sql` | Create | DDL for new tables with `user_id, language` composite PK. |

## Interfaces / Contracts

```go
// internal/core/domain/models/language_progress.go
type LanguageProgress struct {
    UserID                string    `json:"user_id"`
    Language              string    `json:"language"`
    Rango                 string    `json:"rango"` // F-S
    Puntos                int       `json:"puntos"`
    RangoCodeHealth       string    `json:"rango_code_health"` // Junior-Architect
    PuntosCodeHealth      int       `json:"puntos_code_health"`
    ChallengesCompletados int       `json:"challenges_completados"`
    ChallengesIntentados  int       `json:"challenges_intentados"`
    TasaExito             float64   `json:"tasa_exito"`
    TopicsDominados       []string  `json:"topics_dominados"`
    CompletedChallengeIDs []string  `json:"-"` // Internal for dedup
}

// internal/ports/user_progress.go
type UserProgressRepository interface {
    GetLanguageProgress(ctx context.Context, userID, lang string) (*models.LanguageProgress, error)
    GetAllLanguageProgress(ctx context.Context, userID string) ([]models.LanguageProgress, error)
    UpdateLanguageProgress(ctx context.Context, progress *models.LanguageProgress) error
    UpdateGlobalProfile(ctx context.Context, profile *models.UserProfile) error
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Go) | Range Calculation | Table-driven tests validating F-S and Junior-Architect point thresholds. |
| Unit (Go) | Global Rank | Table-driven tests validating median rank calculation. |
| Unit (Go) | Anti-gaming Dedup | Mock repo; assert second completion of same `challengeID` adds no points. |
| Unit (TS) | Profile Component | `HttpClientTestingController` to mock `/profile`, assert DOM renders cards. |
| E2E | Progress Flow | Complete an audit via Dojo, visit `/profile`, assert points increased. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

New tables `user_language_progress` and `user_learning_profile`. A default row (F / Junior) will be upserted when reading progress for a user/language combination that doesn't exist yet, avoiding backfill scripts.

## Open Questions

- [ ] Does `tasa_exito` (success rate) divide by zero if `intentados` is reset or corrupted? (Need safeguard returning 0).
- [ ] How exactly will `mcp-pedagogical` update `LearningProfile` incrementally without overwriting concurrent user updates? (Deferred to S2 tutor-chat).
