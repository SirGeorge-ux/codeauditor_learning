# MCP Tools Catalogue (CodeAuditor Monolith)

> **Audiencia:** devs backend, agentes que vayan a añadir o modificar tools del MCP.
> **Lee esto antes de:** añadir una tool nueva, modificar el protocolo MCP, o tocar el monolito.

---

## 1. Visión general

**Decisión arquitectónica:** un único servidor MCP (monolito) que vive en el mismo binario Go del backend. Expone ~30 tools agrupadas en 9 categorías. El frontend (Angular) las consume vía `MCPClientPort` (un adapter que habla HTTP+SSE con el monolito).

**Por qué monolítico (no multi-server):**
- Más simple de operar (1 binario, 1 deploy, 1 log).
- El proyecto es single-user. No necesita escalado horizontal de MCPs.
- Multi-server se justifica cuando hay equipos independientes manteniendo cada MCP, o cuando necesitas escalar/aislar uno. No es el caso aquí.

**Por qué dentro del binario Go (no como servicio aparte):**
- Reutilizamos el `core/services/` y los `ports/` existentes.
- Mismo logging, mismas métricas, misma auth.
- Latencia más baja (in-process).

**Trampa futura:** si en S6+ el monolito crece demasiado (>50 tools, >1000 líneas de código MCP), considerar extraer a procesos separados. Por ahora, monolítico.

---

## 2. Protocolo de transporte

- **Transporte:** HTTP + Server-Sent Events (SSE).
- **Endpoint:** `POST /mcp/tools/call` (request/response para tool calls), `GET /mcp/events` (SSE para streaming de resultados largos).
- **Auth:** JWT (mismo middleware que `/api/v1/*`).
- **JSON-RPC 2.0** para el formato de mensajes (estándar MCP).
- **Tool discovery:** `GET /mcp/tools/list` devuelve todas las tools con su schema OpenAI-compatible.

### 2.1 Request ejemplo

```json
{
  "jsonrpc": "2.0",
  "id": "req-123",
  "method": "tools/call",
  "params": {
    "name": "mcp-pedagogical.get_learning_objective",
    "arguments": {
      "language": "typescript",
      "topic": "inheritance",
      "level": "D"
    }
  }
}
```

### 2.2 Response ejemplo

```json
{
  "jsonrpc": "2.0",
  "id": "req-123",
  "result": {
    "exercise": "Crea una clase `Animal` con un método `speak()`. Crea una subclase `Dog extends Animal` que override `speak()` para devolver 'Woof'.",
    "prerequisites": ["classes"],
    "restrictions": ["No uses generics todavía", "No uses decorators"],
    "common_mistakes": ["Olvidar `super()` en el constructor de Dog", "No llamar a `super.speak()` en el override"],
    "estimated_difficulty": "D"
  }
}
```

---

## 3. Las 9 categorías de tools

### 3.1 `mcp-sandbox` (ejecución de código)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `run_code` | `{language, code, tests?, timeout_seconds?}` | `{stdout, stderr, exit_code, test_results?}` | Wrapper sobre `LocalSandbox`/`DockerSandbox` |
| `list_languages` | `{}` | `{languages: [{key, display_name, docker_image, install_hint}]}` | De `ProviderRegistry` |
| `healthcheck` | `{language?}` | `{ok: bool, languages: {key: {available, hint}}}` | Check Ollama o `which` para local |
| `get_install_hint` | `{language}` | `{hint: string, url: string, command: string}` | De `LanguageProvider.InstallHint()` |

### 3.2 `mcp-challenges` (gestión de challenges)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `list_challenges` | `{filter?: {language?, difficulty?, category?, source?}}` | `{challenges: [...]}` | De DB (curated) + caché (generated) |
| `get_challenge` | `{id}` | `{challenge: {...}}` | Incluye `expectedFindings`, `hints`, `solutionCode` |
| `create_temp_challenge` | `{source: 'gogs'|'github', owner, repo, path, language}` | `{challenge: {...}}` | Importa un archivo como challenge temporal |
| `generate_challenge` | `{language, topic, level, story_context?}` | `{challenge: {...}}` | Llama a `mcp-pedagogical` + `mcp-sandbox` para validar |
| `validate_challenge` | `{challenge}` | `{valid: bool, errors: [...]}` | Ejecuta `solutionCode` y comprueba que los tests pasan |

### 3.3 `mcp-user` (perfil y progreso)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `get_profile` | `{user_id?}` (default: current) | `{profile: UserProfile}` | De DB |
| `get_language_progress` | `{language, user_id?}` | `{progress: LanguageProgress}` | F-S para free practice, Junior-Architect para code health |
| `get_history` | `{limit?, user_id?}` | `{sessions: [AuditSession]}` | De `audit_history` table |
| `get_learning_profile` | `{user_id?}` | `{profile: LearningProfile}` | El "perfil de aprendizaje" del user (lo que el chat usa) |
| `record_audit_attempt` | `{challenge_id, findings_matched, hints_used, duration_ms, success}` | `{}` | Actualiza `LanguageProgress` |
| `update_learning_profile` | `{profile: LearningProfile}` | `{}` | Para que el chat "recuerde" cosas del user |

### 3.4 `mcp-curriculum` (currícula versionada)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `get_topics` | `{language}` | `{topics: [{slug, difficulty, prerequisites, ...}]}` | Lee `.atl/curricula/<lang>.md` |
| `get_topic` | `{language, topic}` | `{topic: {...}}` | Detalle completo |
| `suggest_topic` | `{language, description}` | `{topic: {...}}` | IA propone (peer review) |
| `review_topic` | `{topic, feedback}` | `{topic: {...}}` | IA refina según feedback del user |

### 3.5 `mcp-dictionary` (glosario personal)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `search_term` | `{query}` | `{terms: [Term]}` | Búsqueda full-text en `dictionary_terms` |
| `get_term` | `{term}` | `{term: Term}` | Detalle |
| `add_term` | `{term, explanation, category, example?, tags?}` | `{term: Term}` | Inserta |
| `list_terms` | `{category?, tag?}` | `{terms: [Term]}` | Listado paginado |

### 3.6 `mcp-pedagogical` ⭐ LA PIEZA CENTRAL

Esta es la categoría que **diferencia CodeAuditor de "otro LLM con sandbox"**. Implementa la lógica pedagógica.

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `get_learning_objective` | `{language, topic, level}` | `{exercise, prerequisites, restrictions, common_mistakes, estimated_difficulty}` | Lee currícula + filtra |
| `generate_story_context` | `{language, topic}` | `{context: string}` | PBL: "Eres un dev en una startup..." |
| `create_scaffolded_steps` | `{challenge, num_steps?}` | `{steps: [Step]}` | Divide el ejercicio en 3-4 pasos |
| `validate_solution` | `{language, code, expected_solution, test_cases}` | `{valid: bool, feedback: string, run_output: string}` | Ejecuta + da feedback formativo |
| `get_socratic_prompt` | `{level: 0-3, language, topic, user_message}` | `{system_prompt, examples}` | 4 niveles de socratismo |
| `peer_review_topic` | `{topic, existing_topics}` | `{feedback: string, suggested_changes: [...]}` | IA revisa un topic nuevo |

**Sobre `get_socratic_prompt` (4 niveles):**

| Nivel | Comportamiento | Prompt base |
|---|---|---|
| 0 (Directa) | "Hay un SQL injection en línea 3, usa queries parametrizadas" | `.atl/prompts/tutor-socratic-0.md` |
| 1 (Guiada) | "Mira la línea 3. ¿Qué pasa si `username = \"' OR 1=1--\"`?" | `.atl/prompts/tutor-socratic-1.md` |
| 2 (Socrática pura) | "¿Qué tipo de dato es `username`? ¿Y si el usuario lo controla? ¿Cómo construyes una query SQL con datos del usuario de forma segura?" | `.atl/prompts/tutor-socratic-2.md` |
| 3 (Descubrimiento) | La IA NO responde. Espera a que el user escriba su hipótesis en un textarea. | `.atl/prompts/tutor-socratic-3.md` |

**Nivel por defecto por rango:**

| Rango free practice | Rango code health | Socratismo default |
|---|---|---|
| F-E | Junior | 2 (pura) |
| D-C | Mid | 1 (guiada) |
| B-A | Senior | 0 (directa) si falla 2 veces |
| S | Architect | 0 (directa) |

### 3.7 `mcp-code-health` (análisis de repo)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `analyze_repo` | `{owner, name, source: 'gogs'|'github'}` | `{analysis_id, status}` | Dispara análisis async (background job) |
| `get_analysis` | `{analysis_id}` | `{status, issues, health_score, summary}` | Polling del resultado |
| `get_repo_issues` | `{owner, name, source, severity?, limit?}` | `{issues: [Issue]}` | Issues rankeados por severity |
| `generate_health_report` | `{analysis_id, format: 'pdf'|'md'}` | `{report_url, expires_at}` | Genera el reporte final |

**Issues detectados:**

- `security`: SQLi, XSS, command injection, hardcoded secrets.
- `performance`: N+1, O(n²), memory leaks, inefficient loops.
- `refactor`: God classes, long methods, duplicated code.
- `style`: magic numbers, poor naming, dead code.
- `architecture`: leaky abstractions, tight coupling, circular deps.

### 3.8 `mcp-external-docs` (Context7 + Brave Search)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `get_library_docs` | `{library, topic?}` | `{docs: [Snippet]}` | Wrapper Context7 |
| `search_documentation` | `{query, language?}` | `{results: [SearchResult]}` | Wrapper Brave Search |
| `get_real_example` | `{library, function}` | `{example: string, source_url: string}` | Snippet oficial |

### 3.9 `mcp-inspiration` (Exercism + GitHub)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `get_exercism_exercise` | `{language, topic}` | `{exercise: ExercismExercise}` | API de Exercism |
| `get_exercism_hints` | `{exercise_id}` | `{hints: [string]}` | Hints pedagógicas |
| `get_github_education_example` | `{query, repos?: ['cs50', 'the-odin-project', ...]}` | `{examples: [CodeSnippet]}` | Few-shot desde repos educativos |

---

## 4. Implementación en el monolito

### 4.1 Estructura de archivos

```
backend/internal/infrastructure/mcp/
├── server.go                    # MCP server (HTTP + SSE)
├── registry.go                  # registro de todas las tools
├── discovery.go                 # GET /mcp/tools/list
├── dispatcher.go                # router de tool calls
├── sandbox/
│   ├── run_code.go
│   ├── list_languages.go
│   ├── healthcheck.go
│   └── get_install_hint.go
├── challenges/
│   ├── list_challenges.go
│   ├── get_challenge.go
│   ├── create_temp_challenge.go
│   ├── generate_challenge.go
│   └── validate_challenge.go
├── user/
│   ├── get_profile.go
│   ├── get_language_progress.go
│   ├── get_history.go
│   ├── get_learning_profile.go
│   ├── record_audit_attempt.go
│   └── update_learning_profile.go
├── curriculum/
│   ├── get_topics.go
│   ├── get_topic.go
│   ├── suggest_topic.go
│   └── review_topic.go
├── dictionary/
│   ├── search_term.go
│   ├── get_term.go
│   ├── add_term.go
│   └── list_terms.go
├── pedagogical/                 # ⭐
│   ├── get_learning_objective.go
│   ├── generate_story_context.go
│   ├── create_scaffolded_steps.go
│   ├── validate_solution.go
│   ├── get_socratic_prompt.go
│   └── peer_review_topic.go
├── code_health/
│   ├── analyze_repo.go
│   ├── get_analysis.go
│   ├── get_repo_issues.go
│   └── generate_health_report.go
├── external_docs/
│   ├── get_library_docs.go
│   ├── search_documentation.go
│   └── get_real_example.go
└── inspiration/
    ├── get_exercism_exercise.go
    ├── get_exercism_hints.go
    └── get_github_education_example.go
```

### 4.2 Patrón de una tool

Cada tool es una función Go que implementa la interfaz `MCPTool`:

```go
// internal/infrastructure/mcp/tool.go
type MCPTool interface {
    Name() string                                              // "mcp-pedagogical.get_learning_objective"
    Description() string                                        // para el LLM
    Schema() openai.Tool                                        // OpenAI-compatible schema
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}
```

Ejemplo de implementación:

```go
// internal/infrastructure/mcp/pedagogical/get_learning_objective.go
package pedagogical

import (
    "context"
    "encoding/json"
    "github.com/anomalyco/codeauditor/backend/internal/core/services"
)

type GetLearningObjectiveTool struct {
    curriculum *services.CurriculumService
    llm        ports.LLMClient
}

func NewGetLearningObjectiveTool(c *services.CurriculumService, l ports.LLMClient) *GetLearningObjectiveTool {
    return &GetLearningObjectiveTool{curriculum: c, llm: l}
}

func (t *GetLearningObjectiveTool) Name() string {
    return "mcp-pedagogical.get_learning_objective"
}

func (t *GetLearningObjectiveTool) Description() string {
    return `Returns the learning objective for a topic at a given level.
Use this to know what the student should learn, what they should NOT use yet,
and common mistakes to watch for.

Args:
  - language (string, required): the programming language (e.g. "typescript")
  - topic (string, required): the topic slug (e.g. "inheritance")
  - level (string, required): F, E, D, C, B, A, or S

Returns:
  - exercise (string): the exercise statement (no code yet, the IA generates it)
  - prerequisites (string[]): topics the user must know first
  - restrictions (string[]): what the user must NOT use at this level
  - common_mistakes (string[]): typical errors to watch for
  - estimated_difficulty (string): estimated difficulty for the user
`
}

func (t *GetLearningObjectiveTool) Schema() openai.Tool {
    return openai.Tool{
        Type: "function",
        Function: openai.FunctionDefinition{
            Name:        "mcp-pedagogical.get_learning_objective",
            Description: t.Description(),
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "language": {"type": "string", "description": "e.g. typescript"},
                    "topic":    {"type": "string", "description": "e.g. inheritance"},
                    "level":    {"type": "string", "enum": ["F","E","D","C","B","A","S"]}
                },
                "required": ["language", "topic", "level"]
            }`),
        },
    }
}

func (t *GetLearningObjectiveTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
    var input struct {
        Language string `json:"language"`
        Topic    string `json:"topic"`
        Level    string `json:"level"`
    }
    if err := json.Unmarshal(args, &input); err != nil {
        return nil, fmt.Errorf("invalid args: %w", err)
    }

    // 1. Get the topic from curriculum
    topic, err := t.curriculum.GetTopic(ctx, input.Language, input.Topic)
    if err != nil {
        return nil, err
    }

    // 2. Filter exercises by level
    exercise := pickExerciseByLevel(topic.Exercises, input.Level)

    // 3. Determine restrictions (what the user must NOT use)
    restrictions := calculateRestrictions(topic.Prerequisites, topic.Category)

    return map[string]any{
        "exercise":            exercise,
        "prerequisites":       topic.Prerequisites,
        "restrictions":        restrictions,
        "common_mistakes":     topic.CommonMistakes,
        "estimated_difficulty": input.Level,
    }, nil
}
```

### 4.3 Registro en el registry

```go
// internal/infrastructure/mcp/registry.go
func NewRegistry(curriculum *services.CurriculumService, llm ports.LLMClient) *Registry {
    r := &Registry{}

    // sandbox
    r.Register(sandbox.NewRunCodeTool(...))
    r.Register(sandbox.NewListLanguagesTool(...))
    r.Register(sandbox.NewHealthcheckTool(...))
    r.Register(sandbox.NewGetInstallHintTool(...))

    // challenges
    r.Register(challenges.NewListChallengesTool(...))
    // ... etc

    // pedagogical (la pieza central)
    r.Register(pedagogical.NewGetLearningObjectiveTool(curriculum, llm))
    r.Register(pedagogical.NewGenerateStoryContextTool(...))
    r.Register(pedagogical.NewCreateScaffoldedStepsTool(...))
    r.Register(pedagogical.NewValidateSolutionTool(...))
    r.Register(pedagogical.NewGetSocraticPromptTool(...))
    r.Register(pedagogical.NewPeerReviewTopicTool(...))

    // ... etc
    return r
}
```

---

## 5. Consumo desde el frontend

### 5.1 `MCPClientPort` (puerto en `domain/ports/`)

```typescript
// frontend/.../domain/ports/mcp-client.port.ts
export abstract class MCPClientPort {
  abstract listTools(): Observable<MCPTool[]>;
  abstract callTool<T = any>(name: string, args: Record<string, any>): Observable<T>;
  abstract streamTool<T = any>(name: string, args: Record<string, any>): Observable<T>;
}
```

### 5.2 Adapter HTTP+SSE

```typescript
// frontend/.../infrastructure/adapters/mcp-client.adapter.ts
@Injectable({ providedIn: 'root' })
export class MCPClientAdapter implements MCPClientPort {
  private http = inject(HttpClient);
  private sse = inject(SSEClient);

  listTools(): Observable<MCPTool[]> {
    return this.http.get<MCPTool[]>(`${environment.apiUrl}/mcp/tools/list`);
  }

  callTool<T>(name: string, args: Record<string, any>): Observable<T> {
    return this.http.post<T>(`${environment.apiUrl}/mcp/tools/call`, {
      jsonrpc: '2.0',
      id: crypto.randomUUID(),
      method: 'tools/call',
      params: { name, arguments: args },
    }).pipe(map(r => r.result));
  }

  streamTool<T>(name: string, args: Record<string, any>): Observable<T> {
    return this.sse.stream<T>(`/mcp/events`, {
      name, arguments: args,
    });
  }
}
```

### 5.3 Uso en el tutor chat

```typescript
// frontend/.../application/tutor-chat.use-case.ts
@Injectable({ providedIn: 'root' })
export class TutorChatUseCase {
  private mcp = inject(MCPClientPort);
  private llm = inject(LLMPort);

  async sendMessage(message: string, context: ChatContext): Promise<TutorResponse> {
    // 1. Get available tools
    const tools = await firstValueFrom(this.mcp.listTools());

    // 2. Call LLM with tools
    const response = await firstValueFrom(this.llm.chat({
      messages: [...],
      tools,
      system: context.socraticPrompt, // from mcp-pedagogical.get_socratic_prompt
    }));

    // 3. If LLM wants to call a tool, do it
    if (response.tool_call) {
      const result = await firstValueFrom(
        this.mcp.callTool(response.tool_call.name, response.tool_call.args)
      );
      // 4. Continue conversation with tool result
      return await this.continueWithToolResult(message, result, context);
    }

    return { text: response.text };
  }
}
```

---

## 6. Trampas y mitigaciones

| Trampa | Mitigación |
|---|---|
| El LLM alucina un tool name que no existe | Validar el tool_call.name contra el registry antes de ejecutar |
| Tool devuelve demasiado output (>200K tokens) | Truncar + resumir antes de pasar al LLM |
| Tool tarda demasiado | Timeout de 30s por tool call. Si excede, fallback |
| Tool escribe en la DB sin querer | Las tools son read-only por defecto. Las que escriben (`add_term`, `update_learning_profile`) requieren opt-in del user |
| Tool expone secrets al LLM | Sanitizar responses. El LLM nunca debe ver `OPENROUTER_API_KEY` ni similares |
| Cascada de tool calls (tool A llama a tool B llama a tool C) | Max depth: 3 niveles. Cortar y reportar al user |
| El LLM no usa las tools disponibles | Mejorar las descripciones. Si `mcp-pedagogical.get_learning_objective` tiene una descripción de 5 líneas, el LLM la usará |

---

## 7. Tests del MCP

Cada tool tiene su test:

```go
// internal/infrastructure/mcp/pedagogical/get_learning_objective_test.go
func TestGetLearningObjectiveTool_Execute(t *testing.T) {
    // Setup
    curriculum := &mockCurriculumService{
        topics: map[string]*Topic{
            "typescript/inheritance": {Slug: "inheritance", Difficulty: "D", ...},
        },
    }
    tool := NewGetLearningObjectiveTool(curriculum, nil)

    // Execute
    result, err := tool.Execute(context.Background(), json.RawMessage(`{
        "language": "typescript",
        "topic": "inheritance",
        "level": "D"
    }`))

    // Assert
    require.NoError(t, err)
    assert.Contains(t, result, "exercise")
    assert.Equal(t, []string{"classes"}, result["prerequisites"])
}
```

---

## 8. Recursos

- **Spec:** `openspec/changes/mcp-monolith/` (S2)
- **Skill:** `codeauditor-mcp-tool-design` (cómo diseñar una tool nueva)
- **Harness:** `add-mcp-tool.harness.md` (paso a paso)
- **MCP spec:** https://modelcontextprotocol.io/
- **MCP Go SDK:** https://github.com/modelcontextprotocol/go-sdk
- **MCP TypeScript SDK:** https://github.com/modelcontextprotocol/typescript-sdk
