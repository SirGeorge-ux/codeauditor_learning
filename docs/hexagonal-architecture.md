# Hexagonal Architecture (CodeAuditor)

> **Audiencia:** devs nuevos, juniors, agentes IA.
> **Lee esto antes de:** proponer cambios estructurales, crear nuevos adapters, o tocar boundaries entre capas.

---

## 1. Por qué hexagonal

CodeAuditor tiene **dos stacks** (Go + Angular) y **5 servicios externos** (Supabase, Gogs, Ollama, Docker daemon, PostgreSQL). Sin una regla arquitectónica clara:

- El dominio termina dependiendo del SDK de Supabase.
- Cambiar de auth provider requiere reescribir medio backend.
- Testear un caso de uso obliga a levantar Docker + Postgres + Kong.
- Un junior no sabe dónde va cada cosa.

La arquitectura hexagonal (puertos y adaptadores) resuelve los 4 problemas: **el dominio no sabe nada del mundo exterior**, y todo lo que sale o entra lo hace a través de un puerto.

---

## 2. La regla física

```
core/domain/    ← solo stdlib (Go) o TS puro (frontend)
core/services/  ← importa de core/domain/ y de ports/
ports/          ← solo interfaces, cero implementaciones
infrastructure/driven/    ← implementa ports/, importa SDKs externos
infrastructure/driving/   ← llama a core/services/, importa de core/domain/
```

**Línea roja:** `core/*` NUNCA importa de `infrastructure/*`. En ningún sentido, en ningún archivo, "solo para una cosita". Es una regla física, no lógica. La rompes una vez, la rompes para siempre.

---

## 3. Backend Go (hexagonal simétrico)

### 3.1 Mapa

```
backend/
├── cmd/api/main.go                          # Composition root
└── internal/
    ├── core/                               # "El hexágono" — depende solo de stdlib
    │   ├── domain/
    │   │   └── models/                      # AuditRequest, Challenge, UserProfile, LanguageProgress, LearningProfile, AuditSession
    │   │                                   # (tipos puros, sin tags de JSON si no son del dominio)
    │   └── services/                       # AuditService, UserProgressService, AuditHistoryService, ChallengeService, TutorService, CodeHealthService, DictionaryService, CurriculumService
    │                                       # (orquestan, no hacen IO directo)
    ├── ports/                              # Interfaces (driven + driving)
    │   ├── auth.go                         # AuthValidator
    │   ├── sandbox.go                      # SandboxExecutor
    │   ├── sse.go                          # SSEStreamer
    │   ├── provider.go                     # LanguageProvider
    │   ├── llm.go                          # LLMClient (OpenRouter/Ollama/M3)
    │   ├── mcp.go                          # MCPChallengePort, MCPUserPort, MCPCurriculumPort, MCPPedagogicalPort
    │   └── ...
    └── infrastructure/
        ├── driven/                         # Adaptadores secundarios (llamados por core)
        │   ├── gogs/                       # GogsClient
        │   ├── github/                     # GitHubClient (S2)
        │   ├── ollama/                     # OllamaClient (local)
        │   ├── openrouter/                 # OpenRouterClient (cascade, S5)
        │   ├── minimax/                    # MiniMaxClient (opt-in, S5)
        │   ├── context7/                   # Context7Client (S2)
        │   ├── brave/                      # BraveSearchClient (S2)
        │   ├── exercism/                   # ExercismClient (S3)
        │   ├── sandbox/
        │   │   ├── localsandbox.go
        │   │   ├── dockersandbox.go
        │   │   └── providers/              # 50+ LanguageProvider
        │   └── supabase/
        │       ├── supabase_client.go
        │       └── supabase_auth.go
        ├── driving/                        # Adaptadores primarios
        │   ├── authmiddleware/             # JWT middleware
        │   └── handlers/                   # 6+ handlers HTTP
        └── mcp/                            # ⭐ MCP monolítico (S2)
            ├── server.go                   # HTTP + SSE
            ├── registry.go                 # registro de tools
            ├── discovery.go                # GET /mcp/tools/list
            ├── dispatcher.go               # router de tool calls
            ├── sandbox/                    # mcp-sandbox tools
            ├── challenges/                 # mcp-challenges tools
            ├── user/                       # mcp-user tools
            ├── curriculum/                 # mcp-curriculum tools
            ├── dictionary/                 # mcp-dictionary tools
            ├── pedagogical/                # ⭐ mcp-pedagogical tools (LA PIEZA CENTRAL)
            ├── code_health/                # mcp-code-health tools (S4)
            ├── external_docs/              # Context7 + Brave (S2)
            └── inspiration/                # Exercism + GitHub (S3)
```

### 3.2 Ejemplo: ¿dónde va la lógica de "ejecutar audit"?

```go
// core/services/audit_service.go
type AuditService struct {
    sandbox      ports.SandboxExecutor     // puerto, no implementación
    ollamaClient *ollamadriven.Client       // ⚠️ ESTO ROMPE HEXAGONAL
    progress     *UserProgressService
    history      *AuditHistoryService
}
```

**Code smell:** `AuditService` recibe `*ollamadriven.Client` directamente. Lo correcto sería definir un `ports.LLMClient` y que `ollamadriven.Client` lo implemente. Está en el roadmap (H-? de la auditoría).

**Regla de diagnóstico rápido:**
- `grep -r "infrastructure" backend/internal/core/` → si hay resultados, hay violación.
- `grep -r "core/" backend/internal/infrastructure/` → debe haber resultados SOLO en `driving/`.

### 3.3 El composition root

`cmd/api/main.go` es el **único** lugar donde se conoce todo:

```go
// Pseudocódigo
gogsClient := gogs.NewGogsClient(env.GOGS_BASE_URL, env.GOGS_TOKEN)
ollamaClient := ollama.NewClient(env.OLLAMA_BASE_URL, env.OLLAMA_MODEL)
registry := providers.NewDefaultRegistry()
sandboxExec := sandboxpkg.NewDockerSandbox(registry)  // o NewLocalSandbox(registry)

auditService := services.NewAuditService(sandboxExec).
    WithOllama(ollamaClient).
    WithProgress(userProgress).
    WithHistory(history)

auditHandler := handlers.NewAuditHandler(auditService)
```

Si necesitas un nuevo adapter:
1. Define el puerto en `ports/`.
2. Implementa en `infrastructure/driven/<nombre>/`.
3. Cablea en `main.go`.

---

## 4. Frontend Angular (hexagonal paralelo)

### 4.1 Mapa

```
frontend/codeauditor/src/app/
├── domain/                                 # Cero imports de @angular/*
│   ├── models/                             # Challenge, User, LanguageProgress, LearningProfile, AuditEvent, AuditSession, Finding
│   └── ports/                              # AuthPort, ChallengeRepositoryPort, AuditRepositoryPort, LLMPort, MCPClientPort, TutorChatPort
├── application/                            # Use cases
│   ├── audit.use-case.ts
│   ├── challenge.use-case.ts
│   ├── tutor-chat.use-case.ts              # ⭐ S2
│   ├── free-practice.use-case.ts           # ⭐ S3
│   ├── code-health.use-case.ts             # ⭐ S4
│   ├── dictionary.use-case.ts              # ⭐ S2
│   └── ...
└── infrastructure/                         # Angular
    ├── components/                         # Standalone, solo UI
    │   ├── dashboard/, dojo/, mcp/, vault/, layout/, shared/
    │   ├── practice/                       # ⭐ S2-S3
    │   ├── code-health/                    # ⭐ S4
    │   ├── profile/                        # ⭐ S1
    │   ├── dictionary/                     # ⭐ S2
    │   └── tutor-chat/                     # ⭐ S2 (panel lateral)
    ├── guards/                             # authGuard
    ├── repositories/                       # http-challenge (mock-challenge deprecated S1)
    ├── services/                           # auth, audit, challenge, gogs, vault, github (S2)
    ├── adapters/                           # ⭐ NUEVO S2
    │   ├── mcp-client.adapter.ts           # HTTP+SSE al monolito
    │   ├── llm.adapter.ts                  # OpenRouter + Ollama + M3
    │   └── ...
    ├── ollama.adapter.ts                   # legacy, refactor a llm.adapter.ts
    └── supabase.adapter.ts
```

### 4.2 Reglas de oro

| Regla | Por qué |
|---|---|
| `domain/*` no puede importar de `@angular/*` | Para poder testear el dominio con `tsc --noEmit` y sin Angular. Para poder compartir el dominio con otro frontend (Next, Svelte, lo que sea). |
| `application/*` solo importa de `domain/*` | Los use cases son lógica de negocio pura, no saben de HTTP ni de Angular. |
| `infrastructure/components/*` no llama a `HttpClient` directamente | Pasa por un use case. Si un componente hace `http.get(...)` directo, **es code smell**. |
| `infrastructure/services/*` implementa `domain/ports/*` con `inject()` | Inyección de dependencias limpia, fácil de mockear en tests. |
| `application/index.ts` y `domain/index.ts` reexportan | Para que los componentes importen `@app/application` y no la ruta interna. |

### 4.3 Caso especial: `mock-challenge.repository.ts`

Existe como **fallback offline** para desarrollo sin backend. Tiene 8 challenges hardcodeados con código vulnerable realista (SQL injection, XSS, race conditions, etc.).

**Estado actual:** huérfano. El frontend ya consume del backend HTTP por defecto. La spec `2026-06-12-real-challenges` lo creó pero la spec `2026-06-19-challenges-db` lo jubiló.

**Decisión pendiente:**
- Opción A: borrarlo y crear `db/seed/01_challenges.sql` con los 8.
- Opción B: documentarlo como fallback y registrar el repo mock solo en `app.config.ts` cuando `environment.useMockData === true`.

Recomendación: **A**. El repositorio mock añade 200 líneas de código muerto y un import confuso. Los 8 challenges deben vivir donde corresponde: en la DB.

### 4.4 ¿Por qué el frontend necesita `domain/` separado si Angular ya tiene DI?

Porque la promesa de hexagonal no es "tener interfaces", es **poder cambiar de framework sin reescribir el dominio**. Si mañana CodeAuditor migra a Svelte o Solid, el `domain/` queda intacto. Si hoy depende de `@angular/common/http`, queda atado.

Además: el dominio TS es **compartible con el backend** (los `Challenge`, `Finding` etc. son los mismos conceptos).

---

## 5. La regla de los providers (sandbox)

`LanguageProvider` es la pieza más elegante del proyecto. Sirve como ejemplo de cómo hacer hexagonal bien:

1. **Puerto** (`ports/provider.go`): 6 métodos puros.
2. **Registry** (`infrastructure/driven/sandbox/providers/registry.go`): un map `language → provider`.
3. **Implementaciones** (`providers/<lenguaje>.go`): un archivo de ~30 líneas por lenguaje.
4. **Consumidores** (`localsandbox.go`, `dockersandbox.go`): una sola línea `registry.Get(lang)` en lugar de un `switch` de 50 cases.

**El test:** si quieres añadir un lenguaje, ¿cuánto código nuevo necesitas?
- Backend: 1 archivo `.go` (~30 líneas) + 1 archivo `_test.go` (~20 líneas) + 1 línea en `NewDefaultRegistry()`. **Total: 2 archivos + 1 línea.**
- Frontend: 0 (solo el lenguaje aparece en el dropdown del Dojo).

Si necesitas tocar más de eso, **algo va mal**.

---

## 6. Anti-patrones que el agente debe detectar y señalar

| Anti-patrón | Cómo detectarlo | Solución |
|---|---|---|
| God service | `services/foo.go` > 300 líneas | Partir en sub-servicios. |
| Leaky abstraction | `core/services/` importa de `infrastructure/` | Definir el puerto que falta. |
| Anemic domain | `domain/models/` tiene solo DTOs sin métodos ni invariantes | Añadir factory methods (`NewChallenge()` con validación) y métodos de negocio. |
| Switch sobre tipo | `switch lang { case "go": ... }` en `infrastructure/driving/` o en `core/services/` | Usar registry / strategy / map. |
| Inyección de concreción | `Service` recibe `*ollamadriven.Client` en vez de `ports.LLMClient` | Definir el puerto. |
| Side effects en domain | `domain/models/foo.go` hace IO o tiene timestamps `time.Now()` | Mover a `services/`. |
| Tests con red real | `services_test.go` que llama a `httpbin.org` | Mock con `httptest` o `t.Skip()`. |
| Componente acoplado a HttpClient | `grep -r "HttpClient" infrastructure/components/` | Pasar por use case. |

---

## 7. ¿Cómo sé que mi cambio respeta hexagonal?

Checklist de PR:

- [ ] Ningún import nuevo cruza `core/` ↔ `infrastructure/`.
- [ ] Si tocas un adapter existente, no se filtra al dominio.
- [ ] Si añades un servicio externo, defines un puerto antes de implementar.
- [ ] Los tests del servicio nuevo mockean el puerto, no la implementación concreta.
- [ ] El `main.go` (o `app.config.ts` en frontend) es el único que conoce a todos.
- [ ] Si añades una tool MCP, va en `infrastructure/mcp/<categoría>/` e implementa un puerto definido en `ports/`.
- [ ] Si añades un adapter de LLM, va en `infrastructure/driven/<proveedor>/` e implementa `ports.LLMClient`.

Si el checklist falla, **el PR no se mergea**.

---

## 8. El MCP monolítico como nueva capa (S2)

El monolito añade una nueva "rama" en `infrastructure/mcp/`. Es **driven** (los tools son llamados por el chat del frontend, que actúa como cliente MCP), pero implementa puertos del dominio:

```
core/services/TutorService  ← usa
ports.MCPPedagogicalPort     ← interfaz
infrastructure/mcp/pedagogical/  ← implementación
```

**Regla:** los tools del MCP **nunca** hacen IO directo. Siempre pasan por un servicio de `core/services/`.

```go
// ✅ CORRECTO
type GetLearningObjectiveTool struct {
    curriculum *services.CurriculumService  // puerto via servicio
    llm        ports.LLMClient
}

func (t *GetLearningObjectiveTool) Execute(...) {
    topic, err := t.curriculum.GetTopic(...)  // pasa por el servicio
    // ...
}

// ❌ INCORRECTO
type GetLearningObjectiveTool struct {
    db *sql.DB  // acceso directo a la DB
}

func (t *GetLearningObjectiveTool) Execute(...) {
    topic := t.db.QueryRow("SELECT * FROM curriculum WHERE ...")  // rompe hexagonal
    // ...
}
```

Ver `docs/mcp-tools.md` para el catálogo completo de los 9 grupos de tools.

---

## 9. Recursos

- **Spec activo que define los providers:** `openspec/specs/sandbox-provider-registry/spec.md`
- **Spec archivado del pattern original:** `openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/design.md`
- **Cómo añadir un lenguaje (paso a paso):** `.atl/harnesses/add-language.harness.md`
- **Cómo añadir un tool MCP:** `.atl/harnesses/add-mcp-tool.harness.md`
- **Catálogo completo de tools:** `docs/mcp-tools.md`
