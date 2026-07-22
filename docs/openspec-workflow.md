# OpenSpec Workflow (CodeAuditor)

> **Audiencia:** devs, tech leads, agentes que vayan a hacer cambios materiales.
> **Lee esto antes de:** crear una feature, un refactor grande, un nuevo adapter, o cualquier cambio que toque >100 líneas o varias capas.

---

## 1. Qué es OpenSpec y por qué lo usamos

**OpenSpec** es el sistema de **Spec-Driven Development (SDD)** del proyecto CodeAuditor. Es la versión local de [Gentle-AI openspec](https://github.com/anomalyco/gentle-ai) (heredado del fork del usuario).

**Filosofía:** un cambio material se vive primero como **documento** y luego como **código**. El documento es la fuente de verdad sobre *qué* queremos y *por qué*. El código es la consecuencia.

**Por qué:**
- Obliga a pensar el alcance antes de tocar.
- Crea un historial de decisiones (no se pierde por qué elegimos X).
- Permite PRs más pequeños (cada task del spec es un commit o un PR).
- Da a los agentes IA un contexto preciso de qué hacer.

---

## 2. Estructura del repo `openspec/`

```
openspec/
├── config.yaml                          # Reglas globales (testing, formato, etc.)
├── specs/                               # Specs activos (el "deber ser")
│   ├── audit/spec.md
│   ├── auth/spec.md
│   ├── challenges/spec.md
│   ├── layout/spec.md
│   └── sandbox-provider-registry/spec.md
├── changes/                             # Cambios en curso (WIP)
│   ├── <nombre-del-cambio>/
│   │   ├── proposal.md
│   │   ├── design.md
│   │   ├── tasks.md
│   │   └── specs/
│   │       └── <cap-modificada>/spec.md       # delta spec
│   └── archive/                         # Cambios aplicados (histórico)
│       └── 2026-06-22-<nombre>/
│           ├── proposal.md
│           ├── design.md
│           ├── tasks.md
│           └── apply-progress.md
```

---

## 3. El ciclo de vida de un cambio

```
   ┌──────────────────────┐
   │ 1. Crear carpeta     │
   │    en changes/       │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 2. proposal.md       │  ← QUÉ y POR QUÉ
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 3. design.md         │  ← CÓMO
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 4. tasks.md          │  ← Lista de tareas
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 5. specs/<cap>/      │  ← Delta de la spec
   │    spec.md           │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 6. Implementar       │
   │    tasks [x]         │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 7. Archivar en       │
   │    archive/          │
   └──────────┬───────────┘
              │
              ▼
   ┌──────────────────────┐
   │ 8. Merge delta spec  │
   │    a specs/<cap>/    │
   └──────────────────────┘
```

---

## 4. Anatomía de los 4 archivos

### 4.1 `proposal.md`

**Responde:** ¿qué y por qué?

```markdown
# Proposal: <nombre corto del cambio>

## Intent
<1-2 párrafos: qué problema resolvemos y para quién>

## Scope
### In Scope
- Bullet 1
- Bullet 2
### Out of Scope
- Bullet 1 (defer a fase 2)

## Capabilities
### New Capabilities
- `<nombre-cap>`: <descripción de 1 línea>
### Modified Capabilities
- `<cap-existente>`: <qué cambia>

## Approach
<2-3 párrafos: cómo lo vamos a hacer, a alto nivel>

## Affected Areas
| Area | Impact | Description |
|---|---|---|
| `path/a/` | New | ... |
| `path/b/foo.go` | Modified | ... |

## Risks
| Risk | Likelihood | Mitigation |
|---|---|---|
| ... | Med | ... |

## Rollback Plan
<Pasos concretos para revertir si sale mal>

## Dependencies
- <Lista de servicios, libs, specs previas que se necesitan>

## Success Criteria
- [ ] <Criterio medible 1>
- [ ] <Criterio medible 2>
```

### 4.2 `design.md`

**Responde:** ¿cómo?

```markdown
# Design: <nombre del cambio>

## Technical Approach
<Párrafo principal con la estrategia>

## Architecture Decisions
| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| ... | A / B / C | ... | **B** |

## Data Flow
<Diagrama ASCII o secuencia de llamadas>

## File Changes
| File | Action | Description |
|---|---|---|
| ... | Create / Modify / Delete | ... |

## Interfaces / Contracts
<Definiciones de tipos, funciones, signatures>

## Testing Strategy
| Layer | What to Test | Approach |
|---|---|---|
| ... | ... | ... |

## Migration / Rollout
<Si hay migración de datos, backwards compat, etc.>

## Open Questions
-<Cosas que se dejaron pendientes>
```

### 4.3 `tasks.md`

**Responde:** ¿qué hago paso a paso?

```markdown
# Tasks: <nombre del cambio>

## Review Workload Forecast
| Field | Value |
|---|---|
| Estimated changed lines | ~XXX |
| 400-line budget risk | High / Med / Low |
| Chained PRs recommended | Yes / No |

### Suggested Work Units
| Unit | Goal | Likely PR | Notes |
|---|---|---|---|
| 1 | ... | PR 1 | base: main |
| 2 | ... | PR 2 | depends on PR 1 |

## Phase 1: <Nombre>
- [ ] 1.1 <Tarea concreta>
- [ ] 1.2 <Tarea concreta>

## Phase 2: <Nombre>
- [ ] 2.1 <Tarea concreta>
- [ ] 2.2 <Tarea concreta>

## Phase N: Verification
- [ ] N.1 `go test ./...` pasa
- [ ] N.2 `pnpm test` pasa
- [ ] N.3 `make validate` pasa
```

**Reglas:**
- Cada tarea debe ser **completable en una sesión** (regla del `config.yaml`).
- Marca `[x]` cuando se complete (no al hacer commit, al verificar).
- Si una tarea es muy grande, partirla.

### 4.4 `specs/<cap>/spec.md` (delta)

**Responde:** ¿qué cambia en el "deber ser"?

```markdown
# <Nombre de la Capability> Specification (delta)

## Purpose
<1 párrafo: qué define esta capability>

## ADDED Requirements

### Requirement: <Nombre del nuevo requirement>
The system MUST / SHOULD / MAY ...

#### Scenario: <Nombre del escenario>
- GIVEN <estado inicial>
- WHEN <acción>
- THEN <resultado esperado>

## MODIFIED Requirements

### Requirement: <Nombre existente>
<Diff con ## antes / después>

## REMOVED Requirements

### Requirement: <Nombre eliminado>
<Razón>
```

**Convención:** `## ADDED Requirements` / `## MODIFIED Requirements` / `## REMOVED Requirements`. El merge posterior a `openspec/specs/<cap>/spec.md` los integra.

---

## 5. Cuándo abrir un spec

**SÍ abrir spec cuando:**
- La feature toca >100 líneas o varias capas.
- Introduce un nuevo adapter, puerto, o servicio externo.
- Cambia un endpoint HTTP público.
- Cambia la estructura de la DB.
- Refactoriza >2 archivos en hexagonal (cambia boundaries).
- Cualquier cosa que rompa compatibilidad con el frontend actual.

**NO abrir spec cuando:**
- Es un fix de typo, formato, o comentario.
- Es un test que añade cobertura sin cambiar comportamiento.
- Es un cambio cosmético de UI (color, padding, fuente).
- Es una mejora de docs.

**En esos casos, PR directo con mensaje `fix:`, `docs:`, `style:`, `test:`.**

---

## 6. Cómo crear un spec nuevo (paso a paso)

```bash
# 1. Crear carpeta
mkdir -p openspec/changes/<nombre-corto>/specs

# 2. Copiar la estructura de uno existente
cp -r openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/* \
      openspec/changes/<nombre-corto>/

# 3. Renombrar la spec si la affected capability es otra
mv openspec/changes/<nombre-corto>/specs/audit openspec/changes/<nombre-corto>/specs/<nueva-cap>

# 4. Editar los 4 archivos con tu contenido
$EDITOR openspec/changes/<nombre-corto>/proposal.md
$EDITOR openspec/changes/<nombre-corto>/design.md
$EDITOR openspec/changes/<nombre-corto>/tasks.md
$EDITOR openspec/changes/<nombre-corto>/specs/<cap>/spec.md

# 5. Implementar las tareas marcando [x]

# 6. Archivar cuando todo esté en [x]
# (preferentemente como parte del merge del último PR)
```

---

## 7. Archivado

Cuando todas las tareas están en `[x]` y el código está en master:

1. Crear carpeta en `openspec/changes/archive/<YYYY-MM-DD>-<nombre>/`.
2. Mover los 4 archivos ahí.
3. Crear `apply-progress.md` con 2 párrafos de qué se hizo realmente vs lo planeado.
4. Mergear el `specs/<cap>/spec.md` en `openspec/specs/<cap>/spec.md` (manualmente, con `## ADDED Requirements` integrados).
5. Commit con `chore: archive <nombre> spec`.

---

## 8. Estado actual del repo y roadmap S0-S6

### 8.1 Estado actual (2026-07-22)

| Spec | Estado | Acción recomendada |
|---|---|---|
| `mcp-integration` (en `changes/`) | 🟡 tareas completas, sin archivar | **Archivar** (mover a `archive/2026-07-22-mcp-integration/`) |
| `multi-lang-sandbox-oleada1` (en `changes/`) | 🟡 duplicado del archivado | **Eliminar** (es ruido, ya está aplicado) o re-versionar si es una nueva oleada |
| 14 specs en `archive/` | ✅ | — |
| 5 specs activas en `specs/` | ✅ | — |

### 8.2 Specs planificadas (roadmap S0-S6)

| Spec | Sprint | Prioridad | Capacidades afectadas |
|---|---|---|---|
| `challenge-rebuild` | S1 | 🔴 Alta | `challenges` (reescribir modelo) |
| `language-progress` | S1 | 🔴 Alta | `user` (nueva entidad) |
| `dictionary` | S2 | 🟡 Media | nueva capacidad |
| `tutor-chat` | S2 | 🟡 Media | `audit`, `user` (nueva entidad) |
| `mcp-monolith` | S2 | 🟡 Media | nueva capacidad (9 grupos de tools) |
| `free-practice-mode` | S3 | 🟡 Media | `challenges`, `curriculum` |
| `code-health-dashboard` | S4 | 🟢 Baja-Media | nueva capacidad |
| `llm-cascade` | S5 | 🟢 Baja | `llm` (nueva capacidad) |

### 8.3 Especificación de capacidad para cada spec nueva

#### `challenge-rebuild` (S1)

**Capacidad modificada:** `challenges`

**ADDED Requirements:**

```markdown
### Requirement: Challenge model v2

The system MUST persist challenges with the new model (expected_findings, hints, solution_code, test_cases, linter_rules).

#### Scenario: Import a v2 challenge
- GIVEN a markdown file with the new challenge format
- WHEN the user runs the import script
- THEN the challenge is persisted in `challenges` table with all new fields
- AND the old `mock-challenge.repository.ts` is deleted

#### Scenario: Score calculation
- GIVEN a completed audit session
- WHEN the session is saved
- THEN the score is calculated as: `base + (tests × weight × 30) + (lint × 20) + (findings_matched × 10) - (hints × cost) + (time_bonus)`
```

#### `language-progress` (S1)

**Capacidad modificada:** `user`

**ADDED Requirements:**

```markdown
### Requirement: LanguageProgress entity

The system MUST persist per-language progress with F-S range (free practice) and Junior-Architect range (code health).

#### Scenario: Update progress after audit
- GIVEN a completed audit session for language "typescript"
- WHEN the session is saved
- THEN the user's `language_progress["typescript"]` is updated:
  - `puntos += session.score`
  - `challenges_completados += 1`
  - `challenges_intentados += 1`
  - `tasa_exito = completed / attempted`
  - `rango = rango_from_puntos(puntos)` (F-S)
  - `ultimo_completado = NOW()`

### Requirement: LearningProfile entity

The system MUST persist a `learning_profile` per user with preferences, learning style, and summary.

#### Scenario: Update learning profile
- GIVEN a completed audit session where the user used 3 hints
- WHEN the session is saved
- THEN the tutor updates `learning_profile.bloqueos_tipicos` to include "hint_overuse" if the pattern repeats
```

#### `dictionary` (S2)

**Capacidad nueva:** `dictionary`

```markdown
### Requirement: Personal tech glossary

The system MUST allow users to add, search, and list personal tech terms.

#### Scenario: Add a term
- GIVEN the user is on /dictionary and clicks "Add"
- WHEN they fill in "term=hardcode", "explanation=...", "category=Práctica"
- THEN the term is persisted in `dictionary_terms` table
- AND appears in the list and search results
```

#### `tutor-chat` (S2)

**Capacidad nueva:** `tutor`

```markdown
### Requirement: Socratic tutor chat

The system MUST provide a chat panel always visible during practice.

#### Scenario: Socratic response at level 2
- GIVEN the user is in a TypeScript challenge with rango=D
- AND `learning_profile.nivel_socratismo = 2` (socrática pura)
- WHEN the user asks for help
- THEN the tutor responds with questions, not answers
- AND uses `mcp-pedagogical.get_socratic_prompt(level=2, ...)`
```

#### `mcp-monolith` (S2)

**Capacidad nueva:** `mcp`

```markdown
### Requirement: MCP monolith with 9 tool groups

The system MUST expose a single MCP server (monolith) with 9 tool groups: sandbox, challenges, user, curriculum, dictionary, pedagogical, code_health, external_docs, inspiration.

#### Scenario: Tool discovery
- GIVEN the frontend connects to /mcp/tools/list
- WHEN the request is authenticated
- THEN the response includes all 30+ tools with their OpenAI-compatible schemas
```

#### `free-practice-mode` (S3)

**Capacidad nueva:** `free-practice`

```markdown
### Requirement: On-demand challenge generation

The system MUST generate challenges on-demand based on language, topic, and level.

#### Scenario: Generate a challenge for Herencias
- GIVEN the user is on /practice/free/typescript and types "Herencias nivel D"
- WHEN the request is made
- THEN:
  1. `mcp-curriculum.get_topic("typescript", "inheritance")` returns the topic
  2. `mcp-pedagogical.get_learning_objective("typescript", "inheritance", "D")` returns the exercise + restrictions
  3. `mcp-pedagogical.generate_story_context("typescript", "inheritance")` returns the PBL context
  4. The LLM (Groq qwen-2.5-coder-32b) generates a challenge with the constraints
  5. `mcp-pedagogical.validate_solution` runs the solution in a loop until it works
  6. The challenge is returned to the user with all fields populated
```

#### `code-health-dashboard` (S4)

**Capacidad nueva:** `code-health`

```markdown
### Requirement: Repo analysis dashboard

The system MUST analyze a repo (Gogs or GitHub) and present a health dashboard with ranked issues.

#### Scenario: Analyze a Gogs repo
- GIVEN the user is on /repo/owner/name/code-health and clicks "Analyze now"
- WHEN the request is made
- THEN:
  1. `mcp-code-health.analyze_repo(owner, name, "gogs")` starts a background job
  2. The system uses Ollama local for file analysis (with cache by SHA)
  3. For each file, the system detects: security, performance, refactor, style issues
  4. The system ranks issues by severity
  5. The dashboard displays: health_score, top 10 issues, language breakdown
```

#### `llm-cascade` (S5)

**Capacidad nueva:** `llm`

```markdown
### Requirement: LLM cascade with cost control

The system MUST route LLM requests through a configurable cascade (Ollama local → OpenRouter free tier → paid fallback).

#### Scenario: Audit uses Ollama
- GIVEN a request to audit TypeScript code
- WHEN the request is made
- THEN the system uses `ports.LLMClient` which routes to Ollama local (qwen2.5-coder:3b)
- AND the cost is $0

#### Scenario: Tutor chat uses OpenRouter cascade
- GIVEN a request to the tutor chat
- WHEN the request is made
- THEN the system uses Groq llama-3.3-70b-versatile (free tier)
- IF that fails, Cerebras llama-3.3-70b
- IF that fails, DeepSeek V3 (free tier)
- IF all fail, GPT-4o-mini (paid, requires user opt-in)
```

---

## 9. Para agentes IA

**Si vas a proponer un cambio material:**

1. Lee `openspec/config.yaml` para conocer las reglas.
2. Lee `openspec/specs/<cap-relacionada>/spec.md` para conocer el "deber ser" actual.
3. Lee al menos 2 specs en `archive/` para ver el tono y la estructura esperada.
4. Crea tu cambio siguiendo este workflow.
5. **No implementes código sin spec previo** (a menos que sea trivial).

**Si encuentras un spec activo con todas las tareas en `[x]` y el código en master:**
- Sugiere al usuario archivarlo.
- No implementes nada nuevo basado en él, está cerrado.

---

## 10. Recursos

- **OpenSpec upstream:** https://github.com/anomalyco/gentle-ai
- **Plantilla recomendada:** `openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/`
- **Config del proyecto:** `openspec/config.yaml`
