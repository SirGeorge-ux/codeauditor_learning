# AGENTS.md — CodeAuditor (academy-mic)

> **System prompt & contexto para agentes IA que trabajen en este repo.**
>
> **Última actualización:** 2026-07-22 (debate + auditoría Mavis)
> **Repo:** `github.com/SirGeorge-ux/codeauditor_learning` = `gogs.madeincode.online/GgogsMIC/academy-mic`

---

## 0. Lo que este proyecto ES (léelo primero, evita el 80% de errores)

**CodeAuditor** es un **Dojo de Auditoría** interactivo con **3 modos de práctica**:

1. **Curated mode** (8 challenges reescritos) — retos clásicos con code smells conocidos.
2. **Free practice mode** — la IA genera ejercicios on-demand según el lenguaje y nivel del user.
3. **Code health mode** — el user importa un repo real (Gogs o GitHub) y la IA analiza code smells, refactors, complejidad y propone prácticas.

**Hay un tutor con IA siempre visible** que:
- Conoce al user (perfil de aprendizaje persistente).
- Conoce el contexto actual (lenguaje, challenge, código, output del sandbox).
- Es **socrático** (0-3 niveles: hace preguntas, no da respuestas).
- Habla 4 idiomas (es, en, fr, de).

**El gamificación es core:**
- **Free practice** usa rangos **F-S** (más granulares).
- **Code health** usa rangos **Junior-Mid-Senior-Architect** (consistente con global).
- Racha, puntos por challenge, bonus por findings, penalty por hints.

**Stack multi-LLM:**
- **Ollama local** (`qwen2.5-coder:3b`) para sandbox y code health local.
- **OpenRouter cascade** (gratis) para generación y chat.
- **MiniMax M3** opt-in para respuestas premium (cuando el user lo activa).

**Esto NO es:**
- Un playground genérico de Monaco.
- Un simple linter online.
- Un SaaS multi-tenant (es single-user, `ggogsmic`).

---

## 1. Reglas de comportamiento del agente

* **Tono:** directo, quirúrgico, técnico. Sin rodeos. Si el humano pide algo vago, **pregunta una sola vez** lo que cambia el resultado.
* **Foco:** micro-hitos, feedback visual inmediato, evitar sobrecarga cognitiva. Una cosa bien hecha > tres a medias.
* **Código:** SOLID, hexagonal estricto, tests junto al código. Code smells se señalan con nombre.
* **Pedagogía:** la IA es **socrática** por defecto. Solo entrega respuestas directas si el user está bloqueado o tiene rango Senior+ en ese lenguaje.
* **Seguridad:** tokens de Gogs/GitHub/Supabase/Ollama NUNCA al frontend ni a logs. Sandbox `--cap-drop=ALL --network=none --read-only` por defecto.
* **Trazabilidad:** cada cambio material debe tener un **proposal en `openspec/changes/`** antes de tocar código. No improvisar.

---

## 2. Stack tecnológico (estado real)

### 2.1 Backend (Go)

- **Go 1.23**, **Chi v5** (router HTTP), `database/sql` + `lib/pq`.
- **Hexagonal estricto** en `backend/internal/{core,ports,infrastructure}`.
- **MCP monolítico** (1 binario, 9 grupos de tools, ~30 tools) — `mcp-sandbox`, `mcp-challenges`, `mcp-user`, `mcp-curriculum`, `mcp-dictionary`, `mcp-pedagogical` (la pieza central), `mcp-code-health`, `mcp-external-docs` (Context7 wrapper), `mcp-inspiration` (Exercism + GitHub).
- **Supabase** (PostgreSQL 15 + Auth) como persistencia y auth.
- **Sandbox** con dos modos: `LocalSandbox` (binarios en PATH) y `DockerSandbox` (contenedor Alpine efímero) + 50+ LanguageProvider.
- **SSE** para streaming de audit + tokens LLM.
- **Tests:** Go stdlib + `testify`.

### 2.2 Frontend (Angular)

- **Angular 21.2**, TypeScript 5.9, **Standalone Components** (cero `NgModules`).
- **`bootstrapApplication`** (no `platformBrowserDynamic`).
- **Signals** para estado, `computed()` y `effect()` cuando aplique.
- **Nuevo Control Flow** (`@if`, `@for`, `@switch`). Prohibido `*ngIf`, `*ngFor`, `*ngSwitch`.
- **RxJS 7** solo cuando hace falta combinar streams HTTP o SSE; en el resto, signals puros.
- **Tailwind CSS 4** con configuración **CSS-first** en `frontend/codeauditor/src/styles.css` (tema en `@theme {}`). **No** crear `tailwind.config.js`.
- **Monaco Editor 0.55** para el code panel.
- **xterm.js 6.0** para la terminal.
- **Lucide Angular** para iconos.
- **Tests:** Vitest 4 + jsdom 28.

### 2.3 LLM Strategy

| Caso | Modelo | Costo | Razón |
|---|---|---|---|
| Audit en sandbox (sandbox + análisis) | **Ollama local** `qwen2.5-coder:3b` | $0 | Privacidad, velocidad, sin rate limit |
| Code health de un repo (incremental) | **Ollama local** + caché | $0 | Privacidad |
| Generación de challenges (free practice) | **Groq** `qwen-2.5-coder-32b` | $0 (free tier) | Mejor en código que MiniMax M3 |
| Tutor chat (socrático) | **Groq** `llama-3.3-70b-versatile` | $0 (free tier) | Mejor en razonamiento |
| Análisis profundo de repo (raro) | **DeepSeek V3** vía OpenRouter | $0 (free tier) | Contexto largo |
| Fallback pago (casi nunca) | **GPT-4o-mini** | ~$0.15/1M | Último recurso |
| Premium opt-in (user lo activa) | **MiniMax M3** | plan MiniMAX | Cuando el user quiere lo mejor |

**Trampa:** **NO** exponer el token de OpenRouter/MiniMax al frontend. Siempre pasa por el backend.

**Documento detallado:** `docs/llm-strategy.md`.

### 2.4 Estructura del monorepo

```
academy-mic/
├── backend/                          # API Go (hexagonal)
│   ├── cmd/api/main.go
│   └── internal/
│       ├── core/                     # domain + application
│       │   ├── domain/models/        # audit, auth, challenge, language_progress
│       │   └── services/             # AuditService, UserProgressService, TutorService, ...
│       ├── ports/                    # Interfaces: SandboxExecutor, AuthValidator, SSEStreamer, LanguageProvider, LLMClient
│       └── infrastructure/
│           ├── driven/               # Adapters: gogs, ollama, sandbox(+providers), supabase, github
│           ├── driving/              # HTTP handlers + auth middleware
│           └── mcp/                  # MCP monolítico (9 grupos de tools)
│               ├── sandbox/
│               ├── challenges/
│               ├── user/
│               ├── curriculum/
│               ├── dictionary/
│               ├── pedagogical/      # ⭐ LA PIEZA CENTRAL
│               ├── code_health/
│               ├── external_docs/    # Context7 + Brave Search
│               └── inspiration/      # Exercism + GitHub
├── frontend/codeauditor/             # SPA Angular 21 (hexagonal)
│   └── src/app/
│       ├── domain/                   # Modelos + puertos
│       │   ├── models/               # Challenge, User, LanguageProgress, ChatContext, ...
│       │   └── ports/                # AuthPort, ChallengeRepositoryPort, MCPClientPort, ...
│       ├── application/              # Use cases
│       │   ├── audit.use-case.ts
│       │   ├── challenge.use-case.ts
│       │   ├── tutor-chat.use-case.ts
│       │   ├── code-health.use-case.ts
│       │   └── ...
│       └── infrastructure/           # Angular
│           ├── components/           # Standalone
│           │   ├── dashboard/
│           │   ├── dojo/
│           │   ├── practice/         # ⭐ NUEVO: free practice mode
│           │   ├── code-health/      # ⭐ NUEVO: dashboard de repo real
│           │   ├── profile/          # ⭐ NUEVO: language progress
│           │   ├── dictionary/       # ⭐ NUEVO: glosario personal
│           │   ├── tutor-chat/       # ⭐ NUEVO: panel lateral siempre visible
│           │   ├── mcp/              # legacy: file browser
│           │   ├── vault/
│           │   ├── layout/
│           │   └── shared/
│           ├── guards/
│           ├── repositories/
│           ├── services/
│           └── adapters/             # ⭐ NUEVO: MCP client adapter
├── openspec/                         # Spec-Driven Development
│   ├── changes/                      # Cambios en curso
│   │   ├── challenge-rebuild/        # ⭐ S1
│   │   ├── language-progress/        # ⭐ S1
│   │   ├── dictionary/               # ⭐ S2
│   │   ├── tutor-chat/               # ⭐ S2
│   │   ├── mcp-monolith/             # ⭐ S2
│   │   ├── free-practice-mode/       # ⭐ S3
│   │   ├── code-health-dashboard/    # ⭐ S4
│   │   └── llm-cascade/              # ⭐ S5
│   ├── archive/                      # Cambios ya aplicados (15+)
│   ├── specs/                        # Specs activas
│   └── config.yaml
├── .atl/                             # Harnesado del proyecto
│   ├── curricula/                    # ⭐ Currícula por lenguaje (.md)
│   │   ├── typescript.md
│   │   ├── python.md
│   │   ├── rust.md
│   │   └── ...
│   ├── prompts/                      # ⭐ Prompts del tutor (4 niveles de socratismo)
│   │   ├── tutor-system.md
│   │   ├── tutor-socratic-0.md       # directa
│   │   ├── tutor-socratic-1.md       # guiada
│   │   ├── tutor-socratic-2.md       # socrática pura
│   │   └── tutor-socratic-3.md       # descubrimiento
│   ├── skills/                       # 12 skills
│   └── harnesses/                    # 10 harnesses
├── docker-compose.yml                # Stack dev: postgres, kong, studio, ollama
├── docker-compose.prod.yml           # Stack prod: frontend + api
├── Makefile                          # Quality gates + tests
└── docs/                             # Documentación de referencia
    ├── hexagonal-architecture.md
    ├── ui-and-stack.md
    ├── business-domain.md
    ├── openspec-workflow.md
    ├── sandbox-providers.md
    ├── contributing.md
    ├── llm-strategy.md               # ⭐ NUEVO
    ├── curricula-format.md           # ⭐ NUEVO
    └── mcp-tools.md                  # ⭐ NUEVO
```

### 2.5 Gestor de paquetes y comandos

| Acción | Backend | Frontend |
|---|---|---|
| Instalar | `go mod download` | `pnpm install` (estricto, **nunca** `npm` ni `yarn`) |
| Test | `make test-backend` | `make test-frontend` |
| Lint | `make lint-backend` | `make lint-frontend` |
| Formato | `gofumpt -w .` (o `make fix`) | `pnpm format` (o `make fix`) |
| Build | `make build-backend` | `make build-frontend` |
| Dev | `make dev-backend` | `make dev-frontend` |
| E2E | — | `make e2e` |
| Pipeline | `make ci` (lint + test + build) | |
| Pre-deploy gate | `make validate` | |

---

## 3. Arquitectura hexagonal (regla de oro)

### 3.1 Principio

**Las capas son capas físicas, no lógicas.** Los imports van en una sola dirección:

```
core/domain/    ← solo importa de stdlib (Go) o de TS puro (frontend)
core/services/  ← importa de core/domain/ y de ports/
ports/          ← solo interfaces, cero implementaciones
infrastructure/driven/    ← implementa ports/, importa de SDKs externos
infrastructure/driving/   ← llama a core/services/, importa de core/domain/
infrastructure/mcp/       ← herramientas del MCP monolítico (S2)
```

**Línea roja:** `core/*` **nunca** importa de `infrastructure/*`.

### 3.2 Frontend hexagonal

- `domain/models/`: tipos TS puros. Cero `import` de `@angular/*` o librerías.
- `domain/ports/`: interfaces (`AuthPort`, `ChallengeRepositoryPort`, `AuditRepositoryPort`, `MCPClientPort`, `TutorChatPort`).
- `application/`: use cases (funciones o clases con un método `execute`).
- `infrastructure/services/`: implementaciones Angular (`@Injectable`).
- `infrastructure/repositories/`: implementaciones de los puertos de `domain/ports/`.
- `infrastructure/components/`: standalone components. Solo UI. Inyectan use cases, no servicios directamente.
- `infrastructure/adapters/`: cliente MCP (se conecta al monolito Go).

### 3.3 Backend hexagonal: simetría Go ↔ TS

| Frontend (TS) | Backend (Go) |
|---|---|
| `domain/models/` | `internal/core/domain/models/` |
| `domain/ports/` | `internal/ports/` |
| `application/` (use cases) | `internal/core/services/` |
| `infrastructure/services/` | `internal/infrastructure/driven/` (adapters secundarios) |
| `infrastructure/components/` | `internal/infrastructure/driving/` (adapters primarios) |
| `infrastructure/adapters/mcp-client` | `internal/infrastructure/mcp/` (MCP monolítico) |

### 3.4 MCP monolítico (S2)

Ver `docs/mcp-tools.md` para el catálogo completo de los 9 grupos de tools.

**Regla:** el MCP monolítico vive en `backend/internal/infrastructure/mcp/`. Implementa los puertos `MCPChallengePort`, `MCPUserPort`, `MCPCurriculumPort`, etc. Los use cases del frontend consumen el `MCPClientPort` que se conecta al monolito.

### 3.5 Proveedores de lenguaje (patrón importante)

Ver `docs/sandbox-providers.md` para detalle completo.

**Regla:** `LocalSandbox` y `DockerSandbox` **nunca** tienen un `switch language{}`. Siempre delegan al `ProviderRegistry`.

### 3.6 Detalle por capa: leer los docs

- **Arquitectura hexagonal profunda:** `docs/hexagonal-architecture.md`
- **UI/UX y stack frontend:** `docs/ui-and-stack.md`
- **Modelo de negocio (Reto, gamificación, modos de práctica):** `docs/business-domain.md`
- **Workflow de openspec:** `docs/openspec-workflow.md`
- **Cómo añadir un lenguaje al sandbox:** `docs/sandbox-providers.md`
- **Estrategia LLM (OpenRouter cascade + Ollama + M3):** `docs/llm-strategy.md`
- **Formato de la currícula por lenguaje:** `docs/curricula-format.md`
- **Catálogo de tools del MCP monolítico:** `docs/mcp-tools.md`
- **Cómo contribuir al proyecto:** `docs/contributing.md`

---

## 4. Estética visual (UI/UX)

- **Estilo:** Dark IDE / Cyber-Minimalista.
- **Paleta core** (declarada en `styles.css` con `@theme {}`):

| Token CSS | Hex | Uso |
|---|---|---|
| `--color-dojo-base` | `#0d1117` | Fondo |
| `--color-dojo-surface` | `#161b22` | Paneles, cards |
| `--color-dojo-text` | `#c9d1d9` | Texto |
| `--color-dojo-accent` | `#39d353` | Verde neón → tests OK, acierto |
| `--color-dojo-error` | `#f85149` | Rojo carmesí → error, vulnerabilidad |
| `--color-dojo-warning` | `#d29922` | Warnings, code smells |
| `--color-dojo-border` | `#30363d` | Bordes |

- **Tipografía:** `system-ui, -apple-system, ...` (declarada en `body`).
- **Iconos:** `@lucide/angular` (`<lucide-icon name="play">`).
- **Patrones visuales recurrentes:** bordes sutiles `border-[#30363d]`, glow neón en hover, números en verde `#39D353` para stats.

Detalles ampliados: `docs/ui-and-stack.md`.

---

## 5. Sistema de tests (estrategia pirámide)

| Nivel | Comando | Cubre |
|---|---|---|
| Unit Go | `make test-backend` | Servicios, adapters (sandbox, auth, ollama, gogs, github, mcp), handlers |
| Unit Angular | `make test-frontend` | Servicios (auth, gogs, vault), repos, guards, componentes |
| E2E | `make e2e` | Flujos completos: home, login, register, dashboard, dojo, practice, code-health, profile, dictionary |
| Quality gate | `make validate` | gofumpt + golangci-lint + Prettier + ESLint (pre-deploy) |

**Reglas:**
- Tabla-driven tests en Go cuando el caso tiene N variantes.
- En Angular, mockear `HttpClient` con `HttpClientTestingController`.
- Mockear el `MCPClientPort` con un fake (no HTTP real en unit tests).
- Un test que requiere red real o Docker se marca con `t.Skip()` o `-short`.
- No commitear sin que los unit tests del área tocada pasen.

---

## 6. Sistema de cambios: openspec (SDD)

**Regla:** todo cambio material toca código + toca un spec. El spec se vive en `openspec/changes/<nombre-cambio>/` y tiene 4 archivos:

1. `proposal.md` — intención, alcance, capacidades afectadas, riesgos, rollback.
2. `design.md` — enfoque técnico, decisiones con tradeoffs, contratos de interfaces, diagrama de flujo.
3. `tasks.md` — fases y tareas concretas, con `[ ]` / `[x]`.
4. `specs/<cap>/spec.md` — los nuevos requirements (Given/When/Then, RFC 2119).

Detalle: `docs/openspec-workflow.md`.

**Specs planificadas (roadmap S0-S6):**

| Spec | Sprint | Status |
|---|---|---|
| `challenge-rebuild` | S1 | pendiente |
| `language-progress` | S1 | pendiente |
| `dictionary` | S2 | pendiente |
| `tutor-chat` | S2 | pendiente |
| `mcp-monolith` | S2 | pendiente |
| `free-practice-mode` | S3 | pendiente |
| `code-health-dashboard` | S4 | pendiente |
| `llm-cascade` | S5 | pendiente |

---

## 7. Seguridad (no negociable)

- `GOGS_TOKEN`, `GITHUB_TOKEN`, `SUPABASE_JWT_SECRET`, `OPENROUTER_API_KEY`, `MINIMAX_API_KEY`, `CONTEXT7_API_KEY`, `BRAVE_API_KEY` viven en `.env` (backend) o en Coolify secrets (prod). **Nunca** al frontend.
- Sandbox Docker: `--cap-drop=ALL --network=none --read-only --tmpfs /tmp:rw,noexec,nosuid`.
- Sandbox Local: solo en dev (`SANDBOX_MODE=local`). Producción = `auto` (Docker).
- Validación JWT en TODA ruta `/api/v1/*` excepto `/health`.
- Logs: nunca loguear tokens ni passwords.
- Tamaño de archivo Gogs/GitHub: cap 1 MB.
- Tamaño de contexto al LLM: cap 200K tokens (medido antes de enviar).

---

## 8. Despliegue (Coolify)

- `docker-compose.prod.yml` define 2 servicios: `academy-mic-frontend` y `academy-mic-api`.
- 3 redes externas: `coolify`, `net-external`, `mic-supabase-access`.
- Coolify hace build + deploy automático al push a `master` si pasa `make validate`.
- Branch `master` = producción. Feature branches usan el prefijo `feature/`.
- Specs SDD: las tareas se cierran al mergear, no antes.

---

## 9. Cosas que el agente NO debe hacer

- ❌ Proponer NgModule, `*ngIf`, `*ngFor` aunque "sea más fácil".
- ❌ Sugerir `npm` o `yarn` — es `pnpm`.
- ❌ Crear `tailwind.config.js` — la config es CSS-first en `styles.css`.
- ❌ Tocar `core/*` desde `infrastructure/*` ni viceversa.
- ❌ Exponer `GOGS_TOKEN`, `GITHUB_TOKEN`, `OPENROUTER_API_KEY`, `MINIMAX_API_KEY` ni al frontend ni a logs.
- ❌ Hacer commit sin pasar `make validate`.
- ❌ Implementar un cambio material sin proposal en `openspec/changes/`.
- ❌ Usar `any` en TypeScript sin justificación.
- ❌ Hardcodear URLs: usar `environment.ts` o env vars.
- ❌ Generar ejercicios sin pasar por el currícula y el `mcp-pedagogical` (sin filtro pedagógico = ejercicios random).
- ❌ Resolver un challenge por el user. La IA es socrática, noResolvedora.

---

## 10. Cómo pedir ayuda al agente (para el humano)

- ✅ "Lee `docs/X.md` y propón un plan de 3 pasos."
- ✅ "Abre un spec en `openspec/changes/<nombre>/` para esta feature."
- ✅ "Usa la skill `codeauditor-add-language` para añadir soporte a Kotlin."
- ✅ "Genera un challenge on-demand para aprender Herencias en TypeScript."
- ✅ "Analiza el repo `academy-mic` y dame los 5 issues más importantes."
- ✅ "Dame 3 hints progresivos para este challenge sin spoileear la respuesta."

---

## 11. Skills y harnesses del proyecto (catálogo)

### 11.1 Skills (12 total)

| Skill | Trigger |
|---|---|
| `codeauditor-onboarding` | Primera vez, sin contexto |
| `codeauditor-add-language` | Añadir lenguaje al sandbox |
| `codeauditor-add-challenge` | Añadir challenge curado (nuevo modelo) |
| `codeauditor-add-api-endpoint` | Añadir endpoint HTTP |
| `codeauditor-run-quality-gate` | Run validate / pre-deploy |
| `codeauditor-practice-mode-selection` | Decidir entre 3 modos |
| `codeauditor-free-practice-generator` | Generar challenges on-demand |
| `codeauditor-tutor-chat` | Interactuar con el tutor |
| `codeauditor-code-health` | Analizar un repo |
| `codeauditor-dictionary` | Añadir término al glosario |
| `codeauditor-language-progress` | Calcular/mostrar LanguageProgress |
| `codeauditor-mcp-tool-design` | Diseñar nueva tool MCP |

### 11.2 Harnesses (10 total)

| Harness | Para qué |
|---|---|
| `add-language.harness.md` | Lenguaje nuevo en sandbox |
| `add-challenge.harness.md` | Challenge curado (nuevo modelo) |
| `add-api-endpoint.harness.md` | Endpoint HTTP nuevo |
| `fix-failing-test.harness.md` | Arreglar un test que falla |
| `add-mcp-tool.harness.md` | Tool nueva en el MCP monolítico |
| `add-curriculum-topic.harness.md` | Topic nuevo en `.atl/curricula/<lang>.md` |
| `add-tutor-prompt.harness.md` | Prompt nuevo del tutor (socrático) |
| `add-dictionary-term.harness.md` | Término nuevo en el glosario |
| `add-code-health-rule.harness.md` | Regla nueva de análisis de repo |
| `add-language-progress-event.harness.md` | Evento que actualiza LanguageProgress |

---

## 12. Roadmap de producto (snapshot 2026-07-22)

| Sprint | Foco | Entregables | Estado |
|---|---|---|---|
| **S0** | Debate + docs + specs vacías | Docs/skills/harnesses actualizados + 8 specs nuevas creadas | 🔄 en curso |
| **S1** | Challenge rebuild + Language Progress | 8 challenges nuevos + LanguageProgress + rango mixto | 🔲 |
| **S2** | Dictionary + Tutor Chat v1 + MCP base | Diccionario + chat con context awareness + MCP monolítico (sandbox, user, challenges) | 🔲 |
| **S3** | Free Practice Mode | Generador de challenges + currículas + validación + Exercism wrapper | 🔲 |
| **S4** | Code Health Dashboard | Análisis de repo + caché + ranking de issues + DeepWiki | 🔲 |
| **S5** | LLM Cascade completo | OpenRouter cascade + Ollama local + M3 opt-in | 🔲 |
| **S6+** | Iteración con datos reales | Mejoras de UX, socratismo fino, OAuth, multi-usuario (futuro) | 🔲 |

---

## 13. Anexo: estado actual del proyecto (snapshot 2026-07-22)

- **50+ lenguajes** en el sandbox.
- **6 handlers HTTP**: audit, auth, challenge, gogs, sse, vault.
- **5+ páginas Angular**: home, login, register, dashboard, dojo, mcp, vault.
- **~99 tests Go + 31 tests Angular + 6 tests E2E**.
- **2 specs activas** (una huérfana, debería archivarse) + 14 archivadas.
- **Pendiente cerrar:** `mcp-integration` (archivar), `multi-lang-sandbox-oleada1/` en `changes/` (eliminar).

Esto es un proyecto **maduro**, no un MVP. Trátalo como tal: con respeto por lo que ya está, y cuidado al añadir.
