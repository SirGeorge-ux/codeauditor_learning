# Contributing Guide (CodeAuditor)

> **Audiencia:** contribuidores nuevos, juniors, agentes IA.
> **Lee esto antes de:** tu primer PR.

---

## 1. Setup local (10 minutos)

### 1.1 Prereqs

- **Go 1.23+**
- **Node.js 20+** + **pnpm 9+**
- **Docker** + **Docker Compose**
- **golangci-lint** (https://golangci-lint.run)
- **gofumpt** (`go install mvdan.cc/gofumpt@latest`)
- **Ollama** (opcional, para análisis IA local)

### 1.2 Clonar e instalar

```bash
git clone https://github.com/SirGeorge-ux/codeauditor_learning.git
cd codeauditor_learning

cp .env.example .env
docker compose up -d          # postgres + kong + studio + ollama

cd backend
cp .env.example .env          # ⚠️ ojo, hay .env.example también aquí
go mod download

cd ../frontend/codeauditor
pnpm install
```

### 1.3 Arrancar dev stack

```bash
# Terminal 1 — backend
cd backend && go run cmd/api/main.go
# → http://localhost:8080

# Terminal 2 — frontend
cd frontend/codeauditor && pnpm start
# → http://localhost:4200
```

Si todo va bien:
- Frontend en :4200 carga.
- Backend en :8080 responde a `GET /health` con JSON.
- Ollama en :11434 (si la levantaste con `docker compose`).

---

## 2. Workflow diario

### 2.1 Antes de empezar una tarea

1. **Lee el AGENTS.md** (raíz). Es el system prompt para humanos y agentes.
2. **Lee los docs relevantes** (`docs/`):
   - ¿Vas a tocar arquitectura? → `hexagonal-architecture.md`
   - ¿Vas a tocar UI? → `ui-and-stack.md`
   - ¿Vas a tocar el dominio? → `business-domain.md`
   - ¿Vas a añadir un lenguaje? → `sandbox-providers.md`
   - ¿Vas a abrir un spec? → `openspec-workflow.md`
3. **Busca si ya hay un spec abierto** en `openspec/changes/`.
4. Si es un cambio material, **abre un spec** (ver `openspec-workflow.md`).

### 2.2 Mientras trabajas

- **Commits frecuentes y pequeños** (no un mega-commit al final).
- Mensajes en conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, `style:`.
- **TDD cuando tenga sentido** (servicios puros, adapters). No para UI.
- Si encuentras un code smell, **créale un issue** o coméntalo en el PR.

### 2.3 Antes de hacer push

```bash
# 1. Format
make fix

# 2. Quality gate
make validate

# 3. Tests
make test

# 4. Build (sanity)
make build
```

Si `make validate` falla, **no pushees**. Arregla primero.

### 2.4 PR

- **Un PR por task del spec** (si el spec recomienda chained PRs, síguelo).
- Título: `[spec-name] <descripción>` (ej: `[mcp-integration] Add GogsClient proxy`).
- Descripción: link al spec + checklist del `tasks.md`.
- Si cierra un issue, `Closes #N`.
- Si el cambio es visual, **adjunta screenshot o GIF**.

---

## 3. Cómo añadir las 3 cosas más comunes

### 3.1 Añadir un lenguaje al sandbox

→ **`.atl/harnesses/add-language.harness.md`** (paso a paso, ~15 min)

Resumen:
- 1 archivo `.go` (~30 líneas) en `backend/internal/infrastructure/driven/sandbox/providers/`.
- 1 archivo `_test.go` (~20 líneas).
- 1 línea en `NewDefaultRegistry()`.
- 1 mapeo en `inferLanguage()` (si es un lenguaje que viene de Gogs).
- 0 cambios en frontend (auto-acepta el lenguaje).

### 3.2 Añadir un challenge

→ **`.atl/harnesses/add-challenge.harness.md`** (paso a paso, ~30 min)

Resumen:
- Definir el `Challenge` (id, título, descripción, difficulty, language, code, codeSmell).
- Persistirlo en PostgreSQL (idealmente con un seed SQL en `db/seed/`).
- (Opcional) Marcarlo como featured en el dashboard.

### 3.3 Añadir un endpoint API

→ **`.atl/harnesses/add-api-endpoint.harness.md`** (paso a paso, ~1h)

Resumen:
1. Definir el método en el servicio (`core/services/`).
2. Si es nuevo flujo, definir el caso de uso (`application/` en frontend).
3. Crear handler en `backend/internal/infrastructure/driving/handlers/`.
4. Registrar ruta en `main.go` bajo el middleware de auth.
5. Crear/actualizar adapter en `frontend/.../services/`.
6. Tests: handler (`httptest`), service, adapter (`HttpClientTestingController`).
7. Documentar en el spec correspondiente (`openspec/specs/<cap>/spec.md`).

### 3.4 Añadir un tool al MCP monolítico (S2+)

→ **`.atl/harnesses/add-mcp-tool.harness.md`** (paso a paso, ~30 min)

Resumen:
1. Definir el tool en `infrastructure/mcp/<categoría>/<tool_name>.go` implementando `MCPTool`.
2. Definir el puerto correspondiente en `ports/` (si no existe).
3. Añadir el tool al `Registry` en `infrastructure/mcp/registry.go`.
4. Tests: ejecutar el tool con un mock service.
5. Documentar en `docs/mcp-tools.md`.

**Trampa común:** hacer IO directo en el tool (acceder a la DB, llamar a un SDK externo) en vez de pasar por un servicio de `core/services/`. **Regla:** los tools son thin wrappers sobre los servicios existentes.

### 3.5 Añadir un topic a la currícula de un lenguaje (S1+)

→ **`.atl/harnesses/add-curriculum-topic.harness.md`** (paso a paso, ~15 min)

Resumen:
1. Editar `.atl/curricula/<lang>.md`.
2. Añadir el bloque `### <topic-slug>` siguiendo la estructura canónica (slug, difficulty, prerequisites, description, exercises, common_mistakes, resources).
3. Verificar con `.atl/curricula/lint.sh` (script de validación).
4. Commit con `docs(curriculum): add <topic> to <lang>`.

**Trampa común:** poner spoilers en la `description`. La descripción es la LECCIÓN (concepto), no el EJERCICIO (reto).

### 3.6 Añadir un prompt del tutor (S2+)

→ **`.atl/harnesses/add-tutor-prompt.harness.md`** (paso a paso, ~30 min)

Resumen:
1. Crear/editar `.atl/prompts/tutor-socratic-<N>.md` (4 niveles: 0, 1, 2, 3).
2. Definir el system prompt con instrucciones claras + 2-3 ejemplos de input/output.
3. Validar con 5-10 tests: el prompt debe producir respuestas coherentes.
4. Commit con `feat(tutor): add socratic level <N> prompt`.

**Trampa común:** que el prompt spoilee en niveles altos de socratismo. Validar que el nivel 2 (socrática pura) NO contiene respuestas directas.

### 3.7 Añadir un término al diccionario (S2+)

→ **`.atl/harnesses/add-dictionary-term.harness.md`** (paso a paso, ~5 min)

Resumen:
1. Abrir `/dictionary` en el frontend.
2. Click "Add".
3. Rellenar: term, explanation, category, example (opcional), tags.
4. Submit.

(O vía API: `POST /api/v1/dictionary` con el body JSON.)

### 3.8 Añadir una regla de code health (S4+)

→ **`.atl/harnesses/add-code-health-rule.harness.md`** (paso a paso, ~1h)

Resumen:
1. Crear/editar el archivo de reglas en `backend/internal/core/services/code_health/rules/<regla>.go`.
2. Implementar la función `Detect(fileContent string) []Issue`.
3. Añadir la regla al registry de reglas.
4. Tests: tabla-driven con N ejemplos (positivo, negativo, edge cases).
5. Commit con `feat(code-health): add <regla> rule`.

### 3.9 Añadir un evento que actualiza LanguageProgress (S1+)

→ **`.atl/harnesses/add-language-progress-event.harness.md`** (paso a paso, ~30 min)

Resumen:
1. Identificar el evento: ¿qué acción del user debe actualizar el progreso? (e.g. "completó un challenge", "usó un hint", "falló un test").
2. Crear/editar el handler del evento en `core/services/user_progress_service.go`.
3. Actualizar la query SQL que recalcula `puntos`, `rango`, `tasa_exito`.
4. Tests: el evento actualiza correctamente, y el rango se recalcula al cruzar umbral.
5. Commit con `feat(progress): add <evento> handler`.

---

## 4. Convenciones de código

### 4.1 Backend Go

- **Errores con `fmt.Errorf("...: %w", err)`** siempre (wrapped).
- **Errores centinela** en `var ErrXxx = errors.New(...)` cuando se exponen al dominio.
- **Context propagation:** el primer arg de toda función que hace IO es `ctx context.Context`.
- **Nombres de archivos:** lowercase, snake_case (no kebab, no camel). `audit_service.go`, `gogs_client.go`.
- **Paquetes:** lowercase, una palabra si es posible. `services`, `handlers`, `ports`, `models`.
- **Interfaces:** terminan en sustantivo abstracto, no en "Interface" ni "IFace". `SandboxExecutor`, `AuthValidator`, `SSEStreamer`, `LanguageProvider`.
- **No** uses `interface{}`; usa `any` (Go 1.18+).
- **Test files:** `<archivo>_test.go` con `package <mismo>` (white-box) o `package <mismo>_test` (black-box). Preferir black-box cuando es posible.
- **Mocks:** no generes con mockery. Usa `httptest`, `testify/mock` o fakes manuales de 10 líneas.

### 4.2 Frontend TypeScript

- **Tipos puros en `domain/`**, cero imports de `@angular/*`.
- **Standalone components** siempre (`standalone: true`).
- **`inject()`** sobre constructor DI.
- **Signals** para estado local, `computed()` para derivados.
- **RxJS** solo para HTTP / SSE / WebSocket. Convertir a signals en el componente.
- **Template literals** sobre concatenación.
- **`?.` y `??`** en vez de `&&` y `||` para nullish.
- **No** `any`. Si no sabes el tipo, `unknown` y luego valida.
- **No** `[innerHTML]` sin sanitizar. Preferir `{{ }}` o `[textContent]`.
- **No** `console.log` en commits. Usar logger si hace falta.
- **No** comentarios obvios. `// i++` no aporta. Comentar el **por qué**, no el **qué**.

### 4.3 Commits

- Conventional commits: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`, `style:`, `perf:`, `build:`, `ci:`.
- Imperativo en presente: "Add" no "Added".
- Primera línea ≤ 72 chars.
- Si el commit cierra un issue: `Closes #123` en el body.

### 4.4 Tests

- **Tabla-driven en Go** cuando hay N variantes (lenguajes, errores HTTP).
- **Un assert por test** cuando sea posible (no obligatorio).
- **Nombres descriptivos:** `TestPythonProvider_DockerCommand` no `TestProvider1`.
- **Coverage no es meta:** 100% coverage con tests inútiles es peor que 70% con tests significativos.

---

## 5. Cómo reportar bugs

Abre un issue con:

1. **Qué pasó** (síntoma, no causa).
2. **Qué esperabas** que pasara.
3. **Cómo reproducirlo** (pasos, código, comando).
4. **Entorno** (OS, Go version, Node version, branch, commit SHA).
5. **Logs relevantes** (recorta lo sensible: tokens, passwords).
6. **Screenshot/video** si es UI.

---

## 6. Cómo proponer features

1. **Abre un spec en `openspec/changes/<nombre>/`** (ver `openspec-workflow.md`).
2. **Empieza con `proposal.md`** — explica el "qué" y "por qué" sin código todavía.
3. **Pide review** a otro dev o agente antes de pasar a `design.md`.
4. **Implementa con el spec abierto como guía**, marcando tasks `[x]`.

---

## 7. Estructura de archivos: dónde va cada cosa

```
¿Quiero...                          → va en...
─────────────────────────────────────────────────────────
Definir un tipo del dominio         → domain/models/  (TS) | core/domain/models/  (Go)
Definir un contrato (interface)     → domain/ports/   (TS) | ports/  (Go)
Orquestar lógica de negocio         → application/    (TS) | core/services/  (Go)
Llamar a Supabase / Gogs / Ollama   → infrastructure/services/   (TS) | infrastructure/driven/   (Go)
Implementar un puerto TS            → infrastructure/repositories/  (TS)
Exponer HTTP                        → infrastructure/driving/handlers/  (Go)
Crear una página UI                 → infrastructure/components/<carpeta-página>/  (TS)
Reusar un componente                → infrastructure/components/shared/  (TS)
Crear un provider de lenguaje       → infrastructure/driven/sandbox/providers/<lenguaje>.go  (Go)
Definir un test                     → <junto al archivo que prueba, mismo nombre + _test>  (ambos)
```

---

## 8. Onboarding para juniors (7 días) — actualizado S0+

**Día 1:** Setup local, leer AGENTS.md + `hexagonal-architecture.md` + `business-domain.md`. Levantar el stack. Loguearte.

**Día 2:** Explorar el dashboard. Abrir un challenge. Hacer un audit. Ver cómo llega al backend. Mirar los logs del sandbox. Abrir el chat del tutor (si está en S2) y preguntarle algo.

**Día 3:** Leer `sandbox-providers.md` + `curricula-format.md`. Añadir un provider de un lenguaje que falte o mejorar uno existente. Crear 3 topics de currícula para ese lenguaje. PR.

**Día 4:** Leer `ui-and-stack.md` + `llm-strategy.md`. Hacer un cambio cosmético en una página (color, padding). O configurar el LLM cascade. PR.

**Día 5:** Leer `openspec-workflow.md` + `mcp-tools.md`. Leer 3 specs archivados. Abrir un spec para una feature que te gustaría (de las 8 del roadmap S0-S6).

**Día 6:** Implementar el spec (las 2-3 tasks más pequeñas). Marcar `[x]`. PR con chained PRs.

**Día 7:** Presentar al equipo lo aprendido, pedir feedback, refinar el spec si hace falta.

---

## 9. Recursos

- **System prompt del agente:** `AGENTS.md` (raíz)
- **Arquitectura:** `docs/hexagonal-architecture.md`
- **UI/UX:** `docs/ui-and-stack.md`
- **Negocio:** `docs/business-domain.md`
- **OpenSpec:** `docs/openspec-workflow.md`
- **Sandbox providers:** `docs/sandbox-providers.md`
- **LLM Strategy:** `docs/llm-strategy.md`
- **Curricula Format:** `docs/curricula-format.md`
- **MCP Tools:** `docs/mcp-tools.md`
- **Harnesses paso a paso:** `.atl/harnesses/`
- **Skills del proyecto:** `.atl/skills/`
