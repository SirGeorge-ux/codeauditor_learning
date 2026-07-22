# Harness: Add an Event that Updates Language Progress

> **Step-by-step workflow** for adding a new event that triggers an update to `LanguageProgress`.
> **Audience:** devs, agents.
> **Estimated time:** 30-60 minutes.
> **Output:** a new event handler that updates the user's per-language progress when a specific action happens.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/business-domain.md` §2.1 (LanguageProgress model) and §5 (gamification).
- [ ] You've read `codeauditor-language-progress` skill.
- [ ] You know the event (what action triggers the update).
- [ ] The DB migration for `user_language_progress` is in place.

---

## Step 1 — Identify the event

Examples of events that update LanguageProgress:

| Event | Trigger | What to update |
|---|---|---|
| `audit.completed` | User finished an audit | `puntos`, `challenges_completados`, `rango` |
| `challenge.attempted` | User opened a challenge | `challenges_intentados` |
| `hint.used` | User used a hint | `hints_usados_total` (new field?) |
| `code_health.repo_analyzed` | User analyzed a repo | `repos_analizados`, `puntos_code_health` |
| `code_health.issue_found` | User detected an issue in code health | `issues_encontrados` |
| `challenge.solution_submitted` | User submitted a solution | (re-score if better than previous) |
| `challenge.perfect_score` | User got 100% on a challenge | Bonus points |
| `streak.day_completed` | User did ≥1 audit today | (handled by UserProgressService, not Language) |

Pick the event you're adding.

---

## Step 2 — Define the event payload

**File:** `backend/internal/core/events/<event_name>.go` (or extend existing)

```go
package events

import "time"

type AuditCompleted struct {
    UserID      string
    ChallengeID string
    Language    string
    Mode        string    // 'curated', 'free_practice', 'code_health'
    Score       int
    HintsUsed   int
    CompletedAt time.Time
}
```

---

## Step 3 — Add a method to `UserProgressService`

**File:** `backend/internal/core/services/user_progress_service.go` (append)

```go
// RecordAuditCompleted updates the LanguageProgress after an audit is completed.
func (s *UserProgressService) RecordAuditCompleted(ctx context.Context, event events.AuditCompleted) error {
    // 1. Get current progress for this language
    progress, err := s.repo.GetLanguageProgress(ctx, event.UserID, event.Language)
    if err != nil {
        return fmt.Errorf("get language progress: %w", err)
    }

    // 2. Update fields
    if event.Mode == "code_health" {
        progress.PuntosCodeHealth += event.Score
        progress.IssuesEncontrados += event.HintsUsed  // or however you count issues
    } else {
        // free_practice or curated
        progress.Puntos += event.Score
        progress.ChallengesCompletados += 1
    }
    progress.ChallengesIntentados += 1
    progress.TasaExito = float64(progress.ChallengesCompletados) / float64(progress.ChallengesIntentados)
    progress.UltimoCompletado = event.CompletedAt

    // 3. Recalculate ranges
    progress.Rango = rangoFromPuntosFree(progress.Puntos)
    progress.RangoCodeHealth = rangoFromPuntosCodeHealth(progress.PuntosCodeHealth)

    // 4. Persist
    if err := s.repo.UpdateLanguageProgress(ctx, progress); err != nil {
        return fmt.Errorf("update language progress: %w", err)
    }

    return nil
}

// Helper functions
func rangoFromPuntosFree(puntos int) string {
    switch {
    case puntos < 50:   return "F"
    case puntos < 150:  return "E"
    case puntos < 400:  return "D"
    case puntos < 900:  return "C"
    case puntos < 2000: return "B"
    case puntos < 4000: return "A"
    default:            return "S"
    }
}

func rangoFromPuntosCodeHealth(puntos int) string {
    switch {
    case puntos < 100:   return "Junior"
    case puntos < 500:   return "Mid"
    case puntos < 2000:  return "Senior"
    default:             return "Architect"
    }
}
```

---

## Step 4 — Add a test

**File:** `backend/internal/core/services/user_progress_service_test.go` (append)

```go
func TestUserProgressService_RecordAuditCompleted_FreePractice(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    repo := services.NewUserProgressRepository(db)
    service := services.NewUserProgressService(repo)

    userID := "test-user"
    language := "typescript"

    // Initial state: F, 0 puntos
    err := repo.CreateLanguageProgress(context.Background(), userID, language)
    require.NoError(t, err)

    // Trigger the event
    event := events.AuditCompleted{
        UserID:      userID,
        ChallengeID: "ch-test",
        Language:    language,
        Mode:        "free_practice",
        Score:       30,
        CompletedAt: time.Now(),
    }
    err = service.RecordAuditCompleted(context.Background(), event)
    require.NoError(t, err)

    // Assert
    progress, err := repo.GetLanguageProgress(context.Background(), userID, language)
    require.NoError(t, err)
    assert.Equal(t, 30, progress.Puntos)
    assert.Equal(t, 1, progress.ChallengesCompletados)
    assert.Equal(t, 1, progress.ChallengesIntentados)
    assert.Equal(t, 1.0, progress.TasaExito)
    assert.Equal(t, "F", progress.Rango)  // 30 < 50
}

func TestUserProgressService_RecordAuditCompleted_RangeUpgrade(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    repo := services.NewUserProgressRepository(db)
    service := services.NewUserProgressService(repo)

    userID := "test-user"
    language := "rust"

    // Initial state: 0 puntos
    err := repo.CreateLanguageProgress(context.Background(), userID, language)
    require.NoError(t, err)

    // Trigger 6 audits with score 30 each = 180 puntos → should be rango D
    for i := 0; i < 6; i++ {
        event := events.AuditCompleted{
            UserID:      userID,
            ChallengeID: fmt.Sprintf("ch-%d", i),
            Language:    language,
            Mode:        "free_practice",
            Score:       30,
            CompletedAt: time.Now(),
        }
        err := service.RecordAuditCompleted(context.Background(), event)
        require.NoError(t, err)
    }

    // Assert
    progress, err := repo.GetLanguageProgress(context.Background(), userID, language)
    require.NoError(t, err)
    assert.Equal(t, 180, progress.Puntos)
    assert.Equal(t, "D", progress.Rango)  // 150 <= 180 < 400
}

func TestUserProgressService_RecordAuditCompleted_CodeHealth(t *testing.T) {
    // Similar to above but with Mode="code_health"
    // Verifies that PuntosCodeHealth (not Puntos) is updated
}
```

---

## Step 5 — Wire the event in the calling service

**Wherever the event is triggered** (e.g. `AuditService.RunAudit` at the end):

```go
// At the end of AuditService.RunAudit
if s.progress != nil {
    event := events.AuditCompleted{
        UserID:      req.UserID,
        ChallengeID: req.ChallengeID,
        Language:    req.Language,
        Mode:        req.Mode,  // 'curated', 'free_practice', or 'code_health'
        Score:       session.Score,
        HintsUsed:   len(session.HintsUsed),
        CompletedAt: time.Now(),
    }
    if err := s.progress.RecordAuditCompleted(ctx, event); err != nil {
        log.Printf("failed to record progress: %v", err)
        // Don't fail the audit if the progress update fails
    }
}
```

---

## Step 6 — Test the integration

1. Start the backend + frontend.
2. Complete an audit.
3. Go to `/profile`.
4. Verify the `LanguageProgress` was updated:
   - `puntos` increased.
   - `challenges_completados` increased.
   - `rango` updated if the threshold was crossed.
5. Test a range upgrade:
   - Complete enough audits to cross a threshold.
   - Verify the rango changes.

---

## Step 7 — Document

Update `docs/business-domain.md` §5 (gamification) to mention the new event and what it updates.

---

## Step 8 — Commit

```bash
git add backend/internal/core/events/<event_name>.go \
        backend/internal/core/services/user_progress_service.go \
        backend/internal/core/services/user_progress_service_test.go \
        docs/business-domain.md

git commit -m "feat(progress): add <event_name> handler

- Updates LanguageProgress on <event>
- Tests: 3 unit tests (free practice, range upgrade, code health)
- Integration test: profile reflects the update"
```

---

## Checklist

- [ ] Event identified.
- [ ] Event payload defined.
- [ ] Service method added.
- [ ] Helper functions (rango) if needed.
- [ ] Tests: 3+ cases.
- [ ] Event wired in the calling service.
- [ ] Integration test passed.
- [ ] Documentation updated.
- [ ] Conventional commit.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Don't update progress on failures | Only on `audit.completed`, not on `audit.failed`. |
| Range thresholds are wrong | Check `codeauditor-language-progress` skill for the F-S and Junior-Architect tables. |
| Don't update `puntos` and `puntos_code_health` together | They're separate. Free practice → `puntos`. Code health → `puntos_code_health`. |
| Don't update `tasa_exito` if `challenges_intentados == 0` | Division by zero. Handle it explicitly. |
| Don't update `rango` if `puntos` didn't change | Only recalculate if there's a real change. |
| Forgot to add the event in the calling service | The progress won't update. Add a wiring test. |
| Forgot to add a migration if you add a new field | Add `db/migrations/NN_<new_field>.sql`. |

---

## Resources

- **Spec:** `openspec/changes/language-progress/`
- **Doc:** `docs/business-domain.md` §2.1, §5
- **Skill:** `codeauditor-language-progress`
