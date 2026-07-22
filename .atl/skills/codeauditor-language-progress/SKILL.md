---
name: codeauditor-language-progress
description: Use this skill when an agent needs to calculate, update, or display per-language progress. Trigger phrases include "lenguaje progress", "language progress", "mi nivel en X", "rango F-S", "Junior-Architect", "tasa de éxito", "puntos por lenguaje", "actualizar progreso", "perfil del usuario".
---

# Language Progress (S1+)

> **Outcome:** the user's per-language progress is correctly calculated, persisted, and displayed.
> **Two range systems:**
> - **F-S** (free practice mode): F, E, D, C, B, A, S
> - **Junior-Architect** (code health mode): Junior, Mid, Senior, Architect
> **Storage:** `user_language_progress` table.

## When to load

Load this skill when:
- The user completes an audit session (need to update progress).
- The user opens `/profile` (need to display progress).
- The agent is designing the gamification rules.
- The user asks "¿cómo voy en TypeScript?" or "mi nivel en Rust".

## Pre-flight

1. Read `AGENTS.md` (root).
2. Read `docs/business-domain.md` §2.1 (UserProfile + LanguageProgress).
3. Read `docs/business-domain.md` §5 (gamification rules).
4. Read `openspec/changes/language-progress/` (the spec).

## The data model

```typescript
interface LanguageProgress {
  user_id: string;
  language: string;                                    // 'typescript', 'rust', etc.

  // Free practice (F-S)
  rango: 'F' | 'E' | 'D' | 'C' | 'B' | 'A' | 'S';
  puntos: number;
  challenges_completados: number;
  challenges_intentados: number;
  tasa_exito: number;                                  // 0..1
  topics_dominados: string[];                          // ['classes', 'inheritance', ...]

  // Code health (Junior-Architect)
  rango_code_health: 'Junior' | 'Mid' | 'Senior' | 'Architect';
  puntos_code_health: number;
  repos_analizados: number;
  issues_encontrados: number;

  // Común
  ultimo_completado?: Date;
  ultima_actualizacion: Date;
}
```

## Range systems

### Free practice: F-S

| Rango | Puntos (acumulados en free practice) |
|---|---|
| F | 0-49 |
| E | 50-149 |
| D | 150-399 |
| C | 400-899 |
| B | 900-1999 |
| A | 2000-3999 |
| S | 4000+ |

### Code health: Junior-Architect

| Rango | Puntos (acumulados en code health) |
|---|---|
| Junior | 0-99 |
| Mid | 100-499 |
| Senior | 500-1999 |
| Architect | 2000+ |

### Global: mediana de los rangos por lenguaje

```python
def rango_global(language_progress):
    rangos_free = [lp.rango for lp in language_progress.values()]
    if not rangos_free:
        return "Junior"
    return mediana(rangos_free)  # 'F'..'S' → mapeado a Junior..Architect
```

## How progress is updated

### Trigger: completed audit session

```go
func (s *UserProgressService) RecordAuditCompletion(ctx context.Context, userID, challengeID string, session AuditSession) error {
    // 1. Determine the language
    language := session.Language

    // 2. Get current progress
    progress, err := s.repo.GetLanguageProgress(ctx, userID, language)
    if err != nil {
        return err
    }

    // 3. Update fields
    if session.Mode == "free_practice" || session.Mode == "curated" {
        progress.Puntos += session.Score
        progress.ChallengesCompletados += 1
    } else if session.Mode == "code_health" {
        progress.PuntosCodeHealth += session.Score
        progress.IssuesEncontrados += session.FindingsMatched
    }
    progress.ChallengesIntentados += 1
    progress.TasaExito = float64(progress.ChallengesCompletados) / float64(progress.ChallengesIntentados)
    progress.UltimoCompletado = time.Now()

    // 4. Recalculate ranges
    progress.Rango = rangoFromPuntosFree(progress.Puntos)
    progress.RangoCodeHealth = rangoFromPuntosCodeHealth(progress.PuntosCodeHealth)

    // 5. Update topics_dominados if the challenge had learning_objectives
    if challenge.LearningObjectives != nil {
        for _, topic := range challenge.LearningObjectives {
            if !contains(progress.TopicsDominados, topic) {
                progress.TopicsDominados = append(progress.TopicsDominados, topic)
            }
        }
    }

    // 6. Persist
    return s.repo.UpdateLanguageProgress(ctx, progress)
}
```

### Helper: rango from puntos

```go
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

## Display in `/profile`

```
┌────────────────────────────────────────────────────┐
│ Avatar: mic                                         │
│ Email: ggogsmic@madeincode.online                   │
│ Rango global: Senior (mediana de lenguajes)        │
│ Racha: 12 días 🔥                                   │
├────────────────────────────────────────────────────┤
│ Por lenguaje:                                       │
│                                                    │
│ TypeScript  [D] 150 pts  (12/20 challenges)        │
│   topics: classes, inheritance, types              │
│                                                    │
│ Python      [E]  80 pts  (5/15)                    │
│   topics: variables, types, lists                  │
│                                                    │
│ Rust        [F]  20 pts  (2/10)                    │
│                                                    │
│ Go          [C] 600 pts  (18/22)                   │
│   topics: goroutines, channels, interfaces         │
│                                                    │
│ Haskell     [F]   0 pts  (0/8)                     │
│                                                    │
├────────────────────────────────────────────────────┤
│ [Gráfico radar — TypeScript, Python, Rust, Go, +]  │
├────────────────────────────────────────────────────┤
│ Code health:                                        │
│ academy-mic   [Mid] 250 pts   12 issues            │
│ todo-app      [Junior] 50 pts  3 issues            │
├────────────────────────────────────────────────────┤
│ ⚙️ Settings:                                        │
│ Idioma: Español                                     │
│ Socratismo: Nivel 2 (puro)                          │
│ Premium: [ ] Usar M3 para el chat                  │
└────────────────────────────────────────────────────┘
```

## Anti-gaming rules

| Comportamiento | Penalización |
|---|---|
| Completar el mismo challenge 10 veces para subir puntos | Solo cuenta 1 vez. El segundo intento no suma puntos. |
| Usar todas las pistas siempre | El `tasa_exito` baja (pista = -X puntos). No abuses. |
| Generar 100 challenges en 1 día | Rate limit: 5 challenges generados/hora, 20/día. |
| "Generate challenges from these" en code health + completar todos | Bonus, pero no infinito. Cap a 5 issues → 5 challenges. |
| Forzar fallos para "practicar el error" | Solo cuentan los exitos. Los fallos no suman ni restan (solo `intentados` sube). |

## Persistence

**Tabla:** `user_language_progress`

```sql
CREATE TABLE user_language_progress (
  user_id UUID NOT NULL REFERENCES users(id),
  language TEXT NOT NULL,

  -- Free practice
  rango TEXT NOT NULL DEFAULT 'F',
  puntos INTEGER NOT NULL DEFAULT 0,
  challenges_completados INTEGER NOT NULL DEFAULT 0,
  challenges_intentados INTEGER NOT NULL DEFAULT 0,
  tasa_exito REAL NOT NULL DEFAULT 0,
  topics_dominados JSONB NOT NULL DEFAULT '[]',

  -- Code health
  rango_code_health TEXT NOT NULL DEFAULT 'Junior',
  puntos_code_health INTEGER NOT NULL DEFAULT 0,
  repos_analizados INTEGER NOT NULL DEFAULT 0,
  issues_encontrados INTEGER NOT NULL DEFAULT 0,

  -- Común
  ultimo_completado TIMESTAMP,
  ultima_actualizacion TIMESTAMP NOT NULL DEFAULT NOW(),

  PRIMARY KEY (user_id, language)
);

CREATE INDEX idx_ulp_user ON user_language_progress(user_id);
CREATE INDEX idx_ulp_language ON user_language_progress(language);
```

## Migration

```sql
-- db/migrations/NN_user_language_progress.sql
CREATE TABLE user_language_progress (
  -- ... (definición arriba)
);
```

## Don't do

- ❌ Don't update progress on every keystroke. Only on audit completion.
- ❌ Don't use mean for global range. Use median.
- ❌ Don't let the user game the system (same challenge 10x).
- ❌ Don't show the progress to other users (single-tenant).
- ❌ Don't cap the user at a max range. Let them go to S+ or Architect+.
- ❌ Don't penalize the user for failing (only count completions, not failures).

## Resources

- **Spec:** `openspec/changes/language-progress/`
- **Doc:** `docs/business-domain.md` §2.1, §5
- **Skill:** `codeauditor-tutor-chat` (how the chat uses `LearningProfile` and `LanguageProgress`)
- **Harness:** `add-language-progress-event.harness.md`
