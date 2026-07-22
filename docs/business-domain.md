# Modelo de Negocio (CodeAuditor)

> **Audiencia:** PMs, devs, agentes que toquen el dominio o los use cases.
> **Lee esto antes de:** cambiar el modelo de datos, añadir gamificación, tocar la lógica de "audit" o el flujo del Dojo.

---

## 1. Qué es CodeAuditor (una sola frase)

Un **dojo de auditoría de código** donde un desarrollador **practica detectar, entender y corregir code smells y vulnerabilidades** sobre código real, en un entorno gamificado, con un **tutor de IA socrático** siempre presente.

**Hay 3 modos de práctica** (ver §3):

1. **Curated** — 8 challenges clásicos con code smells conocidos (SQLi, XSS, God Class, etc.).
2. **Free practice** — la IA genera ejercicios on-demand según el lenguaje y nivel del user.
3. **Code health** — el user importa un repo real (Gogs o GitHub) y la IA analiza code smells, refactors, complejidad y propone prácticas.

---

## 2. Las entidades principales

```
Usuario
  ↓ practica
Challenge (Reto)
  ↓ genera
AuditSession (Sesión de auditoría)
  ↓ produce
Finding (Hallazgo)
```

Y las nuevas entidades del roadmap:

```
UserProfile
  ↓ tiene N
LanguageProgress    (F-S para free practice, Junior-Architect para code health)
LearningProfile     (lo que el "chat me conoce")

Challenge
  ↓ tiene N
Hint               (3 niveles, con coste)
ExpectedFinding    (para rúbrica)
```

### 2.1 User (Usuario)

**Entidad:** `UserProfile` (Go) / `User` (TS).

```typescript
interface UserProfile {
  id: string;            // UUID de Supabase
  email: string;
  username: string;

  // Global
  rango_global: 'Junior' | 'Mid' | 'Senior' | 'Architect';  // mediana de languageProgress
  puntos_maestria: number;                                    // suma
  racha_dias: number;
  ultima_sesion?: Date;

  // Por lenguaje (NUEVO)
  language_progress: Record<string, LanguageProgress>;

  // Perfil de aprendizaje (NUEVO, para el tutor chat)
  learning_profile: LearningProfile;

  // Premium opt-in
  use_minimax: boolean;  // si el user quiere respuestas con M3
}
```

**`LanguageProgress`:**

```typescript
interface LanguageProgress {
  language: string;                    // 'typescript', 'rust', etc.
  rango: 'F' | 'E' | 'D' | 'C' | 'B' | 'A' | 'S';  // free practice
  rango_code_health: 'Junior' | 'Mid' | 'Senior' | 'Architect';  // code health
  puntos: number;
  challenges_completados: number;
  challenges_intentados: number;
  tasa_exito: number;                  // 0..1
  topics_dominados: string[];           // ['classes', 'inheritance', ...]
  ultimo_completado?: Date;
}
```

**`LearningProfile` (NUEVO, para el tutor chat):**

```typescript
interface LearningProfile {
  user_id: string;
  preferencias: {
    idioma: 'es' | 'en' | 'fr' | 'de';
    nivel_socratismo: 0 | 1 | 2 | 3;   // default 2
    longitud_maxima_msg: number;       // palabras
  };
  estilo_aprendizaje: {
    aprende_mejor_con: 'ejemplos' | 'explicaciones' | 'preguntas' | 'diagramas';
    bloqueos_tipicos: string[];        // ['genéricos', 'asincronía', ...]
    topics_que_le_cuestan: string[];   // detectados automáticamente
  };
  resumen: string;                     // "Sabe TypeScript junior, le cuesta genéricos, prefiere ejemplos"
  ultima_actualizacion: Date;
}
```

**Rangos free practice (F-S):**

| Rango | Puntos | Significado |
|---|---|---|
| F | 0-49 | Primer contacto |
| E | 50-149 | Entiende la sintaxis |
| D | 150-399 | Resuelve con guidance |
| C | 400-899 | Resuelve solo |
| B | 900-1999 | Resuelve complejos |
| A | 2000-3999 | Diseña sistemas |
| S | 4000+ | Master |

**Rangos code health (Junior-Architect):**

| Rango | Puntos | Significado |
|---|---|---|
| Junior | 0-99 | Encuentra issues básicos |
| Mid | 100-499 | Encuentra issues medios |
| Senior | 500-1999 | Encuentra issues sutiles |
| Architect | 2000+ | Detecta problemas arquitectónicos |

**El rango global = mediana de los rangos por lenguaje** (más representativo que la media).

### 2.2 Challenge (Reto) — NUEVO MODELO

```typescript
interface Challenge {
  id: string;
  title: string;
  description: string;                  // CONTEXTO, no respuesta
  difficulty: 'junior' | 'mid' | 'senior' | 'architect';
  language: string;
  category: 'security' | 'performance' | 'refactor' | 'style' | 'concurrency' | 'architecture';

  // Pedagogy (NUEVO)
  learning_objectives: string[];
  hints: Hint[];                        // 3 niveles, con coste
  common_mistakes: string[];
  estimated_time_minutes: number;

  // El código
  code: string;                         // código vulnerable
  expected_findings: ExpectedFinding[]; // para rúbrica
  test_cases: TestCase[];               // tests funcionales
  linter_rules: LinterRule[];           // reglas objetivas
  solution_code: string;                // solución canónica
  solution_explanation: string;         // por qué funciona

  // Gamificación
  base_points: number;
  bonus_points: number;                 // por encontrar TODOS los findings
  penalty_per_hint: number;             // coste de usar una pista
  time_bonus: boolean;

  // Origen
  origin: 'curated' | 'generated' | 'imported' | 'gogs' | 'github';
  source_repo?: string;                 // 'owner/repo'
  source_path?: string;                 // 'src/auth/login.ts'
  generated_by?: 'ollama' | 'openrouter' | 'human';
  created_at: Date;
  created_by: string;                   // user_id
}

interface Hint {
  level: 1 | 2 | 3;                     // 1 = gratis, 2 = 10pts, 3 = 25pts
  content: string;                      // pista, sin spoiler
  cost_points: number;
}

interface ExpectedFinding {
  severity: 'low' | 'medium' | 'high' | 'critical';
  category: 'security' | 'performance' | 'refactor' | 'style';
  message: string;                      // "Posible SQL injection en línea 3"
  evidence: string;                     // snippet
  suggested_fix: string;                // "Usa queries parametrizadas"
}

interface TestCase {
  name: string;
  input: any;
  expected_output: any;
  weight: number;                       // 1-10, para el scoring
}

interface LinterRule {
  type: 'no-eval' | 'no-direct-sql' | 'complexity-max' | 'no-magic-numbers' | ...;
  config: Record<string, any>;
}
```

**El `Challenge` se persiste en PostgreSQL** (no en el `mock-challenge.repository.ts` que está deprecated).

### 2.3 AuditSession (Sesión de auditoría)

**Entidad:** `AuditSession` (TS) / `audit_history` (Postgres).

```typescript
interface AuditSession {
  id: string;                  // UUID
  user_id: string;
  challenge_id: string;
  started_at: Date;
  finished_at?: Date;
  status: 'running' | 'completed' | 'failed';
  exit_code?: number;
  duration_ms?: number;
  sandbox_mode: 'docker' | 'local';
  language: string;

  // Scoring (NUEVO)
  score: number;
  score_breakdown: {
    base_points: number;
    test_points: number;            // Σ(test_passed × weight × 30)
    lint_points: number;            // Σ(lint_clean × 20)
    findings_matched: number;      // Σ(expected_finding_matched × 10)
    hints_penalty: number;          // -Σ(hint_used × cost)
    time_bonus: number;             // si resolvió en < estimated_time
  };

  events: AuditEvent[];            // streamed durante la sesión
  findings: Finding[];             // extraídos al final
  hints_used: number[];            // [1, 2] = usó hint 1 y 2
  test_results: TestResult[];
  lint_results: LintResult[];
}
```

**Servicio responsable:** `AuditService` (Go) + `AuditHistoryService` (Go).

### 2.4 Finding (Hallazgo)

```typescript
interface Finding {
  id: string;
  session_id: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  category: 'security' | 'performance' | 'refactor' | 'style';
  message: string;
  evidence?: string;
  source: 'sandbox' | 'ollama' | 'openrouter' | 'tests' | 'linter' | 'user';
  detected_at: Date;
}
```

**Conceptos clave:**

- Un `Finding` es una **observación** sobre el código. No es una corrección, es una pista.
- Lo detecta el sandbox (errores en runtime), el linter (reglas objetivas), los tests (qué falla), Ollama/OpenRouter (análisis IA), o el user (cuando marca algo).
- Se agrupan en el `AuditSession.findings`.
- Se comparan con los `expected_findings` del challenge para calcular el score.

---

## 3. Los 3 modos de práctica

### 3.1 Curated mode (`/dashboard`)

El modo clásico. 8 challenges curados por el equipo. El user los ve en una grid, los abre en el Dojo, los resuelve.

**Flujo:**

```
User en /dashboard
   ↓ click en un challenge
User en /dojo/:id
   ↓ lee descripción (sin spoiler)
   ↓ puede usar hints (con coste)
   ↓ edita el código en Monaco
User click "Run Audit"
   ↓ backend ejecuta:
   │  ├─ Sandbox: corre el código
   │  ├─ Tests: corre los test_cases del challenge
   │  ├─ Linter: ejecuta las linter_rules
   │  └─ IA: Ollama/OpenRouter analiza con rúbrica
   ↓ backend compara findings del user con expected_findings
   ↓ score objetivo: base + tests + lint + findings - hints + time bonus
User ve el score + feedback formativo
   ↓ "Volver al dashboard"
```

### 3.2 Free practice mode (`/practice/free`)

El user elige un lenguaje, la IA genera un challenge on-demand.

**Flujo:**

```
User en /practice/free
   ↓ click en un lenguaje
User en /practice/free/:lang
   ↓ elige nivel (F-S) o "adaptativo"
   ↓ o escribe: "Quiero aprender sobre Herencias"
   ↓ mcp-pedagogical.get_learning_objective(lang, topic, level)
   ↓ mcp-pedagogical.generate_story_context(lang, topic)
   ↓ mcp-challenges.generate_challenge(lang, topic, level, context)
   ↓ mcp-pedagogical.validate_solution(lang, code, expected) en bucle
User recibe un challenge generado
   ↓ sigue el mismo flujo que Curated
```

**El chat del tutor está siempre visible** (a la derecha o como FAB en móvil).

### 3.3 Code health mode (`/repo/:owner/:name/code-health`)

El user importa un repo real y la IA lo analiza.

**Flujo:**

```
User en /repo/:owner/:name/code-health
   ↓ mcp-code-health.analyze_repo(owner, name, source)
   ↓ mcp-external-docs.get_library_docs para librerías usadas
   ↓ mcp-sandbox.run_code para validar issues
   ↓ mcp-pedagogical.validate_solution para feedback
   ↓ Ollama/DeepSeek analiza cada archivo (con caché)
   ↓ ranking de issues por severity
User ve:
   ├─ Health score (0-100)
   ├─ Top 10 issues (con link al archivo)
   ├─ Languages breakdown
   ├─ "Generate challenges from these" (NUEVO)
   └─ "Drill into issue #N"
```

**Cuando el user hace "Generate challenges from these":**

- Para cada issue top, `mcp-challenges.generate_challenge` crea un challenge basado en el código real.
- Los challenges se guardan en `challenges` table con `origin='code_health'`.
- El user puede resolverlos en el Dojo (modo Curated con su código).

---

## 4. El tutor con IA (socrático)

### 4.1 Características

- **Siempre visible** cuando el user está en `/dojo/*` o `/practice/*`.
- **Conoce el contexto actual** del user (chat context):
  - Lenguaje y nivel.
  - Challenge actual (si está en uno).
  - Código que tiene abierto.
  - Output del sandbox.
  - Historial reciente de challenges.
  - Preferencias (idioma, socratismo, longitud).
- **Tiene memoria persistente** (`LearningProfile`):
  - Lo que sabe el user.
  - Lo que le cuesta.
  - Cómo aprende mejor.
  - Resumen de la historia.

### 4.2 Niveles de socratismo (4)

| Nivel | Comportamiento | Cuándo se usa |
|---|---|---|
| 0 (Directa) | "Hay un SQL injection en línea 3, usa queries parametrizadas" | Rango Senior+ que ya falló 2 veces, o modo opt-in |
| 1 (Guiada) | "Mira la línea 3. ¿Qué pasa si `username = \"' OR 1=1--\"`?" | Default para rango Mid |
| 2 (Socrática pura) | "¿Qué tipo de dato es `username`? ¿Y si el usuario lo controla? ¿Cómo construyes una query SQL con datos del usuario de forma segura?" | Default para rango Junior |
| 3 (Descubrimiento) | La IA NO responde. Espera a que el user escriba su hipótesis. | Modo avanzado opt-in |

**Nivel por defecto según rango:**

| Rango free practice | Rango code health | Socratismo default |
|---|---|---|
| F-E | Junior | 2 (pura) |
| D-C | Mid | 1 (guiada) |
| B-A | Senior | 0 (directa) si falla 2 veces |
| S | Architect | 0 (directa) |

### 4.3 Capacidades del chat (vía MCP tools)

| User dice | Chat usa | Tool MCP |
|---|---|---|
| "Dame un ejercicio" | Lista topics del nivel del user | `mcp-curriculum.get_topics` |
| "Dame uno sobre Herencias" | Genera el challenge | `mcp-pedagogical.get_learning_objective` + `mcp-challenges.generate_challenge` |
| "Otro más difícil" | Sube nivel y regenera | `mcp-pedagogical.get_learning_objective(lang, topic, level+1)` |
| "No entiendo por qué falla" | Análisis socrático del código | `mcp-pedagogical.get_socratic_prompt` + análisis del código |
| "¿Qué es Herencia?" | Mini-lección + ejercicio corto | `mcp-curriculum.get_topic` + `mcp-pedagogical.generate_story_context` |
| "¿Cómo se usa X función de Y librería?" | Docs oficiales actualizadas | `mcp-external-docs.get_library_docs` |
| "Muéstrame la solución" | Revela `solutionCode` con penalización | `mcp-challenges.get_challenge` (con `reveal_solution=true`) |
| "¿Cómo voy?" | Resumen del `LanguageProgress` | `mcp-user.get_language_progress` |
| "Estoy aburrido" | Sugiere otro smell o cambio de lenguaje | `mcp-user.get_learning_profile` + `mcp-curriculum.get_topics` |

### 4.4 Restricciones del chat (importante)

- **NO** resuelve el challenge por el user. Si pide "dame el código de la solución", el chat le da pistas graduadas, no la respuesta directa.
- **NO** habla de otros users. Single-tenant.
- **NO** cambia de tema si el user está en pleno challenge. Sugiere "termina este y luego vemos".
- **SÍ** puede usar todas las MCP tools.
- **SÍ** mantiene memoria de la conversación actual (ventana de ~10 mensajes) + resumen persistente.

---

## 5. Gamificación (core, no nice-to-have)

**Filosofía:** el objetivo es **practicar**, no competir. No hay leaderboard, no hay "ganar".

**Mecánica por modo:**

### 5.1 Curated mode

- `score = base_points + (test_passed × weight × 30) + (lint_clean × 20) + (expected_finding_matched × 10) - (hint_used × hint_cost) + (time_bonus si aplica)`
- Bonus por encontrar TODOS los findings.
- Penalty por usar hints.

### 5.2 Free practice mode

- Mismo scoring que Curated.
- Bonus por explorar topics nuevos.
- Rango F-S por lenguaje.

### 5.3 Code health mode

- Score por cada issue encontrado (vs esperado).
- Rango Junior-Architect por lenguaje.
- Bonus por análisis completo del repo (no solo 1 archivo).

### 5.4 Rango global

- `rango_global = MEDIANA(rango por lenguaje)` (más robusto que la media).
- Se actualiza al cruzar umbral.
- Single source of truth: `language_progress`.

### 5.5 Racha

- +1 día si completas ≥1 audit en el día.
- Reset si no auditas en 24h.

### 5.6 Lo que NO hay (y no debería haber)

- ❌ Leaderboard global.
- ❌ Logros/badges cosméticos.
- ❌ Compra de puntos.
- ❌ Tiempo límite (el audit es a tu ritmo, salvo time_bonus opcional).
- ❌ "Desbloquea el siguiente nivel al terminar el anterior" (ansiedad artificial).

---

## 6. El sistema de currículas (`.atl/curricula/`)

**Single source of truth:** `.atl/curricula/<lang>.md` (markdown versionado).

**Estructura:** cada topic tiene slug, difficulty (F-S), prerequisites, category, description, exercises (3+), common_mistakes, resources.

**Quién lo llena:** el user manualmente al principio, luego con peer review de la IA.

**Quién lo lee:** `mcp-curriculum` (devuelve topics al LLM) + `mcp-pedagogical.get_learning_objective` (filtra por nivel).

Ver `docs/curricula-format.md` para el formato completo.

---

## 7. Roadmap (estado)

| Feature | Estado | Spec |
|---|---|---|
| Auth Supabase | ✅ | `2026-06-10-auth-supabase` |
| Sandbox multi-lenguaje | ✅ | `2026-06-22-multi-lang-sandbox-oleada1..7` |
| Integración Gogs (mcp-integration) | ✅ | archivada |
| Vault persistido | ✅ | — |
| Análisis IA con Ollama | ✅ | — |
| 8 challenges curados (viejo modelo) | ✅ | deprecado, refactor en S1 |
| Gamificación (rangos, racha) | ✅ | básico, refactor en S1 |
| **Challenge rebuild (nuevo modelo)** | 🔲 S1 | `openspec/changes/challenge-rebuild/` |
| **Language Progress (F-S + Junior-Architect)** | 🔲 S1 | `openspec/changes/language-progress/` |
| **Dictionary (glosario personal)** | 🔲 S2 | `openspec/changes/dictionary/` |
| **Tutor Chat (socrático + context)** | 🔲 S2 | `openspec/changes/tutor-chat/` |
| **MCP Monolith (9 grupos de tools)** | 🔲 S2 | `openspec/changes/mcp-monolith/` |
| **Free Practice Mode (generador on-demand)** | 🔲 S3 | `openspec/changes/free-practice-mode/` |
| **Code Health Dashboard (análisis de repo)** | 🔲 S4 | `openspec/changes/code-health-dashboard/` |
| **LLM Cascade (OpenRouter + Ollama + M3)** | 🔲 S5 | `openspec/changes/llm-cascade/` |
| Persist challenges importados | 🔲 | (parte de free-practice-mode) |
| Detección auto de categoría al importar | 🔲 | (parte de code-health-dashboard) |
| Leaderboard | ❌ NO | (decidido NO hacer por filosofía) |
| Herramientas avanzadas (semgrep, trivy) | 🔲 | post-S6 |
| OAuth (GitHub, Google) | 🔲 | post-S6 |
| Challenges generados por IA | 🔲 S3 | (parte de free-practice-mode) |

---

## 8. Anti-patrones de producto que el agente debe evitar

| Anti-patrón | Por qué evitarlo |
|---|---|
| Spoilear la respuesta en la descripción | Mata el aprendizaje. La descripción da CONTEXTO, no la respuesta. |
| "Gamificar" con puntos triviales | Vacía la motivación intrínseca. |
| Castigar al user por fallar | Un dojo es para equivocarse. |
| Mostrar la solución en el primer fallo | El user tiene 3 hints antes. |
| Limitar a "X minutos" el audit | El aprendizaje no tiene cronómetro. |
| "Compite con tus amigos" | Falsa motivación. La motivación real es la maestría. |
| "Desbloquea el siguiente nivel al terminar el anterior" | Crea ansiedad artificial. |
| Que la IA resuelva el challenge por el user | Anula el propósito. La IA es socrática, noResolvedora. |
| Que la currícula la genere la IA sin revisión del user | El user es el dueño de su aprendizaje. La IA sugiere, el user decide. |
| Hardcodear 8 challenges para siempre | Stagnation. El catálogo debe crecer con la comunidad + generación. |

---

## 9. Recursos

- **Spec activo de challenges:** `openspec/specs/challenges/spec.md`
- **Spec archivado de challenges reales:** `openspec/changes/archive/2026-06-12-real-challenges/`
- **Spec archivado de challenges en DB:** `openspec/changes/archive/2026-06-19-challenges-db/`
- **Currícula:** `.atl/curricula/<lang>.md`
- **LLM Strategy:** `docs/llm-strategy.md`
- **MCP Tools:** `docs/mcp-tools.md`
- **Skill:** `.atl/skills/codeauditor-add-challenge/SKILL.md`
- **Skill:** `.atl/skills/codeauditor-tutor-chat/SKILL.md`
- **Skill:** `.atl/skills/codeauditor-free-practice-generator/SKILL.md`
- **Harness:** `.atl/harnesses/add-challenge.harness.md`
- **Harness:** `.atl/harnesses/add-curriculum-topic.harness.md`
