# User Progress Specification

## Purpose

Persist per-language mastery (`LanguageProgress`) with dual range systems (F-S for free practice, Junior-Architect for code health), learning preferences (`LearningProfile`), and wire audit completion to progress updates. Provide `/profile` UI for display.

## ADDED Requirements

### Requirement: LanguageProgress persistence

The system MUST persist a `LanguageProgress` row per `(user_id, language)` pair in the `user_language_progress` table with composite primary key. On first read for an unknown pair, the system MUST upsert a default row (rango=`F`, rango_code_health=`Junior`, puntos=0).

| Column | Type | Default |
|---|---|---|
| user_id | UUID | — (FK users.id) |
| language | TEXT | — |
| rango | TEXT | `F` |
| puntos | INTEGER | `0` |
| challenges_completados | INTEGER | `0` |
| challenges_intentados | INTEGER | `0` |
| tasa_exito | REAL | `0` |
| topics_dominados | JSONB | `[]` |
| completed_challenge_ids | JSONB | `[]` |
| rango_code_health | TEXT | `Junior` |
| puntos_code_health | INTEGER | `0` |
| repos_analizados | INTEGER | `0` |
| issues_encontrados | INTEGER | `0` |
| ultimo_completado | TIMESTAMP | NULL |
| ultima_actualizacion | TIMESTAMP | `NOW()` |

#### Scenario: First progress read creates default row

- GIVEN a user with no `LanguageProgress` for language "typescript"
- WHEN the system reads progress for that pair
- THEN a new row is upserted with rango=`F`, puntos=0, challenges_completados=0

#### Scenario: Existing row is returned unchanged

- GIVEN a user with existing `LanguageProgress` for "python" (puntos=150, rango=`D`)
- WHEN the system reads progress for that pair
- THEN the existing row is returned without modification

### Requirement: LanguageProgress range calculation

The system MUST calculate `rango` from `puntos` using the F-S thresholds and `rango_code_health` from `puntos_code_health` using the Junior-Architect thresholds. Calculation MUST be a pure function with no side effects.

| F-S Rango | Puntos | Code Health Rango | Puntos |
|---|---|---|---|
| F | 0-49 | Junior | 0-99 |
| E | 50-149 | Mid | 100-499 |
| D | 150-399 | Senior | 500-1999 |
| C | 400-899 | Architect | 2000+ |
| B | 900-1999 | | |
| A | 2000-3999 | | |
| S | 4000+ | | |

#### Scenario: Puntos cross threshold updates rango

- GIVEN a user with puntos=399 (rango=`D`)
- WHEN they earn 1 more point (puntos=400)
- THEN rango MUST be recalculated to `C`

#### Scenario: Puntos below minimum stays at F

- GIVEN a user with puntos=0
- WHEN range is calculated
- THEN rango MUST be `F`

### Requirement: LearningProfile persistence

The system MUST persist a `LearningProfile` per `user_id` in the `user_learning_profile` table. On first access, the system MUST create a default profile with idioma=`es`, nivel_socratismo=2, longitud_maxima_msg=200, aprende_mejor_con=`ejemplos`.

| Column | Type | Default |
|---|---|---|
| user_id | UUID | — (PK, FK users.id) |
| idioma | TEXT | `es` |
| nivel_socratismo | INTEGER | `2` |
| longitud_maxima_msg | INTEGER | `200` |
| aprende_mejor_con | TEXT | `ejemplos` |
| bloqueos_tipicos | JSONB | `[]` |
| topics_que_le_cuestan | JSONB | `[]` |
| resumen | TEXT | `''` |
| ultima_actualizacion | TIMESTAMP | `NOW()` |

#### Scenario: First profile access creates defaults

- GIVEN a user with no `LearningProfile`
- WHEN the system reads their profile
- THEN a row is created with idioma=`es`, nivel_socratismo=2, aprende_mejor_con=`ejemplos`

#### Scenario: User updates preferences

- GIVEN a user with idioma=`es`
- WHEN they PUT `/api/v1/users/:id/learning-profile` with `{idioma: "en"}`
- THEN idioma is updated to `en` and ultima_actualizacion is set to NOW()

### Requirement: Progress update on audit completion

The system MUST update `LanguageProgress` when an `AuditSession` reaches status `completed`. The update MUST be triggered by `AuditService` calling `UserProgressService.RecordAuditCompletion` after persisting the session.

For curated/free_practice modes: `puntos += session.score`, `challenges_completados += 1`.
For code_health mode: `puntos_code_health += session.score`, `issues_encontrados += findings_matched`.
Always: `challenges_intentados += 1`, `tasa_exito = completados / intentados`, `ultimo_completado = NOW()`.

#### Scenario: Successful curated audit updates progress

- GIVEN a completed audit for language "typescript" with score=45
- WHEN `RecordAuditCompletion` is called
- THEN puntos increases by 45, challenges_completados increments by 1, challenges_intentados increments by 1, tasa_exito is recalculated, ultimo_completado is set to NOW()

#### Scenario: Failed audit only increments intentados

- GIVEN a failed audit session (status=`failed`)
- WHEN the session is recorded
- THEN challenges_intentados increments by 1, puntos does NOT change, challenges_completados does NOT change

#### Scenario: Topics are added to dominados

- GIVEN a completed audit for a challenge with learning_objectives=["generics", "traits"]
- WHEN progress is updated
- THEN "generics" and "traits" are appended to topics_dominados if not already present

### Requirement: Anti-gaming dedup

The system MUST NOT double-count puntos for the same `challenge_id` in the same language. Deduplication is tracked via the `completed_challenge_ids` JSONB array (capped at 500 entries). Only the first successful completion of a challenge_id contributes puntos.

#### Scenario: Same challenge completed twice

- GIVEN a user already has "challenge-abc" in completed_challenge_ids for "typescript"
- WHEN they complete "challenge-abc" again with score=50
- THEN challenges_intentados increments by 1, puntos does NOT increase, challenges_completados does NOT increase

#### Scenario: First completion counts

- GIVEN a user does NOT have "challenge-xyz" in completed_challenge_ids for "rust"
- WHEN they complete "challenge-xyz" with score=30
- THEN puntos increases by 30, challenges_completados increments by 1, "challenge-xyz" is appended to completed_challenge_ids

#### Scenario: Completed_challenge_ids cap

- GIVEN a user has 500 entries in completed_challenge_ids for a language
- WHEN a new challenge is completed
- THEN the oldest entry is removed before appending the new one (FIFO, max 500)

### Requirement: Global range calculation

The system MUST calculate `rango_global` as the MEDIAN of all per-language `rango` values (F-S scale). The median MUST be computed by converting letter ranks to ordinal positions (F=0, E=1, D=2, C=3, B=4, A=5, S=6), finding the median ordinal, and converting back. If no languages have progress, global range defaults to `Junior`.

#### Scenario: Median of 3 languages

- GIVEN language_progress: typescript=`D`(2), python=`B`(4), rust=`F`(0)
- WHEN global range is calculated
- THEN median ordinal is 2 → rango_global=`D`

#### Scenario: Even number of languages

- GIVEN language_progress: go=`C`(3), typescript=`B`(4)
- WHEN global range is calculated
- THEN median ordinal is 3.5 → rounded down to 3 → rango_global=`C`

#### Scenario: No progress defaults to Junior

- GIVEN a user with zero language_progress rows
- WHEN global range is calculated
- THEN rango_global MUST be `Junior`

### Requirement: User Progress API endpoints

The system MUST expose REST endpoints under `/api/v1/users/:user_id/` for progress and learning-profile management. All endpoints MUST require valid JWT authentication.

| Method | Path | Action |
|---|---|---|
| GET | `/progress` | Return all LanguageProgress rows + rango_global |
| GET | `/progress/:language` | Return single LanguageProgress row |
| PUT | `/progress` | Update LearningProfile preferences |
| GET | `/learning-profile` | Return LearningProfile |
| PUT | `/learning-profile` | Update LearningProfile fields |

#### Scenario: GET all progress

- GIVEN an authenticated user with progress in 3 languages
- WHEN they GET `/api/v1/users/:id/progress`
- THEN the response contains 3 LanguageProgress objects and rango_global

#### Scenario: GET single language progress

- GIVEN an authenticated user with progress in "typescript"
- WHEN they GET `/api/v1/users/:id/progress/typescript`
- THEN the response contains the LanguageProgress for typescript only

#### Scenario: GET progress for unknown language

- GIVEN an authenticated user with no progress for "kotlin"
- WHEN they GET `/api/v1/users/:id/progress/kotlin`
- THEN a default row is upserted and returned (rango=`F`, puntos=0)

#### Scenario: Unauthenticated access rejected

- GIVEN a request without valid JWT
- WHEN any progress endpoint is called
- THEN the system MUST return 401 Unauthorized

### Requirement: Frontend /profile page

The system MUST provide a `/profile` route with a standalone `ProfileComponent` that displays: user avatar and email, rango_global, racha_dias, per-language progress cards (language name, rango badge, puntos, challenges completados/intentados, topics_dominados), LearningProfile settings (idioma selector, socratismo level, premium toggle). The component MUST use signals for state and inject use cases from `application/`, not services directly.

#### Scenario: Profile page loads with data

- GIVEN an authenticated user navigates to `/profile`
- WHEN the component initializes
- THEN it fetches progress and learning-profile via use cases and renders language cards with correct rango badges

#### Scenario: Language card shows stats

- GIVEN a language with puntos=150, rango=`D`, 12/20 challenges
- WHEN the profile renders
- THEN the card displays "TypeScript [D] 150 pts (12/20 challenges)" with topics listed below

#### Scenario: Settings update triggers PUT

- GIVEN the user changes idioma from "es" to "en" in the settings section
- WHEN they save
- THEN the use case calls PUT `/api/v1/users/:id/learning-profile` with the updated preferences
