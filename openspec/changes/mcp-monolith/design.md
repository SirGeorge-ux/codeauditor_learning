# Design: MCP Monolith (S2)

> **Reconciled 2026-10-08.** The shared runtime described below now lives in `backend/internal/mcpkit/` (package `mcpkit`, plus the `mcpctx` leaf package) after the `mcpkit` extraction. Tool groups remain in `backend/internal/infrastructure/mcp/<group>/`. References to `infrastructure/mcp/tool.go` etc. read as `internal/mcpkit/*.go`.

## Technical Approach

The MCP Monolith integrates a Model Context Protocol server directly into the CodeAuditor Go backend. Instead of running a separate Node/Python MCP process, we expose HTTP and SSE endpoints from our existing Chi router. This allows the tutor LLM to execute 32 tools across 7 groups by delegating directly to our existing Hexagonal architecture `core/services/` layer without network hops. 

This approach minimizes deployment complexity while strictly preserving Hexagonal purity: MCP tools are driving adapters that translate JSON-RPC 2.0 into Go service method calls. 

## Architecture Decisions

### Decision: Runtime as a Shared Library (`mcpkit`)

**Choice**: Extract the MCP runtime (interface, registry, dispatcher, server, rate limiter, auth context keys) into `internal/mcpkit/`, a sibling library that tool groups import. **Shipped** (change `mcpkit`, archived 2026-10-08).
**Alternatives considered**: Keeping the runtime inside `infrastructure/mcp/` alongside tool groups; splitting each tool group into its own process.
**Rationale**: A single in-process monolith stays correct for a single-user app (see next decision), but the runtime and the tool groups have different reuse horizons: the runtime is protocol infrastructure that the future Gogs-fork integration and external clients will consume, while tool groups are academy-specific adapters. The library boundary enforces the dependency direction `tool-group → mcpkit → core/services` and prevents tool groups from importing each other's runtime helpers. This is structural (no behavior change): same wire protocol, same 32-tool registry, zero new dependencies.

### Decision: In-Process Monolith vs Separate Service

**Choice**: In-Process Monolith (Go).
**Alternatives considered**: Separate Node.js or Python MCP Server connecting to our REST API.
**Rationale**: CodeAuditor is a single-user application. A separate MCP process would require replicating our domain models, creating a secondary auth boundary, and making network calls for every tool. Keeping it in-process shares the DB pool, `core/services/`, and auth context, ensuring sub-millisecond execution overhead per tool.

### Decision: Tool Argument Validation

**Choice**: Per-tool unmarshaling using Go structs in `Execute()`.
**Alternatives considered**: Centralized JSON Schema validator middleware before reaching the tool.
**Rationale**: While a centralized JSON Schema validator avoids boilerplate, unmarshaling raw JSON into a strongly typed Go struct inside each tool's `Execute()` method provides absolute type safety, guarantees compilation checks against service inputs, and matches standard Go idioms. The boilerplate is minimal.

### Decision: Registry Thread-Safety

**Choice**: `sync.RWMutex` with standard Go `map[string]MCPTool`.
**Alternatives considered**: `sync.Map`.
**Rationale**: Tools are registered entirely at startup (write once) and read continuously thereafter. `sync.RWMutex` is perfectly suited for this read-heavy workload and provides a simpler API than `sync.Map`.

### Decision: SSE Connection Management

**Choice**: Ephemeral, per-request SSE channels.
**Alternatives considered**: Global broadcaster / persistent WebSocket.
**Rationale**: Long-running tool executions (e.g., `sandbox.run_code`) are request-scoped. When a client calls `streamTool()`, a dedicated SSE connection is opened for that specific execution and closed when the tool finishes, avoiding complex pub-sub state management.

## Architecture Overview

```text
    [Frontend / Tutor Chat]
           │
      (HTTP + SSE)
           │
    ┌──────▼────────────────────────────────────────┐
    │ internal/mcpkit/  (shared runtime library)    │
    │                                               │
    │  [Server: /mcp/tools/call, /mcp/events]       │
    │           │                                   │
    │  [Dispatcher] ────► [Registry (sync.RWMutex)] │
    │           │                                   │
    │           ▼                                   │
    │  [MCPTool: mcp-pedagogical.get_objective]     │
    │   ▲  (tool groups in infrastructure/mcp/)     │
    └───┼───────────────────────────────────────────┘
        │ (Method call)
    ┌───▼───────────────────────────────────────────┐
    │ infrastructure/mcp/<group>/ (Driving Adapters) │
    │  [dictionary/ (shipped)] [sandbox/] ...        │
    └───────┬───────────────────────────────────────┘
            │
    ┌───────▼───────────────────────────────────────┐
    │ core/services/ (Domain)                       │
    │                                               │
    │  [PedagogicalService / CurriculumService]     │
    └───────┬───────────────────────────────────────┘
            │
    ┌───────▼───────────────────────────────────────┐
    │ infrastructure/driven/ (Driven Adapters)      │
    │  [DB] [Ollama] [Context7] [Sandbox]           │
    └───────────────────────────────────────────────┘
```

## Contracts & Interfaces

### MCPTool Interface

Lives in `backend/internal/mcpkit/tool.go` (extracted from `infrastructure/mcp/`). Package `mcpkit`:

```go
package mcpkit

import (
    "context"
    "encoding/json"
    "github.com/sashabaranov/go-openai"
)

type MCPTool interface {
    Name() string
    Description() string
    Schema() openai.Tool
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}
```

### Registry Design

Lives in `backend/internal/mcpkit/registry.go`:

```go
package mcpkit

import (
    "fmt"
    "sync"
)

type Registry struct {
    mu    sync.RWMutex
    tools map[string]MCPTool
}

func NewRegistry() *Registry {
    return &Registry{tools: make(map[string]MCPTool)}
}

func (r *Registry) Register(tool MCPTool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.tools[tool.Name()]; exists {
        panic("tool already registered: " + tool.Name())
    }
    r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (MCPTool, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    tool, exists := r.tools[name]
    if !exists {
        return nil, fmt.Errorf("method not found: -32601")
    }
    return tool, nil
}

func (r *Registry) List() []MCPTool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var list []MCPTool
    for _, t := range r.tools {
        list = append(list, t)
    }
    return list
}
```

## Transport Format

The MCP monolith uses HTTP REST endpoints wrapping JSON-RPC 2.0 formats. 

**Request Format (`POST /mcp/tools/call`):**
```json
{
  "jsonrpc": "2.0",
  "id": "req-1",
  "method": "tools/call",
  "params": {
    "name": "mcp-sandbox.list_languages",
    "arguments": {}
  }
}
```

**Response Format:**
```json
{
  "jsonrpc": "2.0",
  "id": "req-1",
  "result": { ... }
}
```

**SSE Event Format (`GET /mcp/events?tool=...`):**
```text
event: message
data: {"status": "running", "output": "Compiling..."}

event: message
data: {"status": "completed", "result": {...}}
```

## Tool-per-Group Design (S2)

The system registers exactly 32 tools across 7 groups. Status at 2026-10-08: `mcp-dictionary` (4 tools) shipped and registered; `tutor lookup` (tutor-chat change) registered; remaining groups pending (PRs 2-4).

| Group | Tools | Dependencies |
|-------|-------|-------------|
| `mcp-sandbox` | `run_code`, `list_languages`, `healthcheck`, `get_install_hint` | `SandboxExecutor`, `ProviderRegistry` |
| `mcp-challenges` | `list_challenges`, `get_challenge`, `create_temp_challenge`, `generate_challenge`, `validate_challenge` | `ChallengeService` |
| `mcp-user` | `get_profile`, `get_language_progress`, `get_history`, `get_learning_profile`, `record_audit_attempt`, `update_learning_profile` | `UserProgressService`, `AuditHistoryService` |
| `mcp-curriculum` | `get_topics`, `get_topic`, `suggest_topic`, `review_topic` | `CurriculumService` |
| `mcp-dictionary` | `search_term`, `get_term`, `add_term`, `list_terms` | `DictionaryService` |
| `mcp-pedagogical` | `get_learning_objective`, `generate_story_context`, `create_scaffolded_steps`, `validate_solution`, `get_socratic_prompt`, `peer_review_topic` | `PedagogicalService`, `CurriculumService`, `LLMClient` |
| `mcp-external_docs`| `get_library_docs`, `search_documentation`, `get_real_example` | `ExternalDocsService` (Context7 + Brave) |

#### Why `run_code` depends on `*sandbox.DockerSandbox`, not the `SandboxExecutor` port

The Tool-per-Group table above lists `mcp-sandbox` as depending on `SandboxExecutor`, but the *production* wiring is intentionally narrower: `sandbox.NewServiceAdapter` takes the concrete `*sandbox.DockerSandbox`, not the port. The deviation is deliberate, security-motivated, and documented here so reviewers don't flag it as a hexagonal regression.

- The port `SandboxExecutor` covers both `LocalSandbox` and `DockerSandbox`. For `run_code` we want a **structural** (compile-time) guarantee that the spec's security flags (`--cap-drop=ALL`, `--network=none`, `--read-only`) are actually present — not just a runtime check the wiring might forget.
- `*sandbox.DockerSandbox` is the only production wiring that constructs a docker container with those flags. Because `NewServiceAdapter(sb *sandbox.DockerSandbox, ...)` constrains the type at the parameter, an unauthorized wiring (e.g. `LocalSandbox`, which provides none of those flags) would not compile and therefore cannot reach `main.go`.
- The runtime `SecurityProfile()` check inside `Execute()` (see `backend/internal/infrastructure/mcp/sandbox/sandbox_tool.go:234-242`, where `runCodeTool.Execute` consults `t.svc.SecurityProfile()` and iterates `requiredFlags`) is belt-and-suspenders: it catches test fakes that *do* match the concrete type but misconfigure the backend (e.g. omit a flag). Without the compile-time constraint on `NewServiceAdapter`, only the runtime check would remain — and a future refactor could drop the runtime check without the tool-layer tests catching it.
- The other `mcp-sandbox` tools (`list_languages`, `healthcheck`, `get_install_hint`) still use the broader port-shaped `Service` interface (see `sandbox.Service`) because they don't enforce execution-time security; they read registry metadata only and would gain nothing from the structural constraint.

## Auth Middleware & Rate Limiter

1. **Authentication**: All `/mcp/*` endpoints are protected by the same JWT middleware as `/api/v1/*`. The middleware injects `user_id` into the `context.Context`. Tools use `ctx.Value("user_id")` to read/write user-scoped data.
2. **Rate Limiting**: We enforce a per-user, per-tool limit of 10 requests per minute using an in-memory token bucket middleware executed before the `Dispatcher`.
3. **Execution Safety**: The dispatcher enforces a strict 30-second context timeout per call. Responses over 50KB are gracefully truncated with a `truncated: true` flag.

## Server Bootstrap Wiring

The composition root (`cmd/api/main.go`) wires dependencies in strict order:

1. Driven adapters initialized (Supabase, Context7, Brave, Ollama/LLMClient).
2. Core services initialized (`CurriculumService`, `PedagogicalService`, `DictionaryService`, etc.).
3. Instantiate `mcpkit.NewRegistry()`.
4. Register tools: `registry.Register(pedagogical.NewGetLearningObjectiveTool(curriculum, llm))`
5. Instantiate `mcpkit.NewServer(registry)`.
6. Mount `mcpServer.Router()` under the Chi router's `/mcp` group.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/mcpkit/{tool,registry,server,dispatcher,ratelimiter}.go` + `mcpctx/` | Shipped | Shared runtime library (`mcpkit` change) |
| `backend/internal/core/services/curriculum_service.go` | Create (WIP in tree) | Curriculum logic (reading `.atl/curricula/`) |
| `backend/internal/core/services/dictionary_service.go` | Shipped | Glossary CRUD (`dictionary` change) |
| `backend/internal/core/services/pedagogical_service.go` | Create (WIP in tree) | Tutor/Socratic logic |
| `backend/internal/core/services/external_docs_service.go` | Create (WIP in tree) | Context7/Brave interactions |
| `backend/internal/infrastructure/mcp/<groups>/*.go` | Create | Remaining tool groups (dictionary shipped); sandbox, challenges, user, curriculum, pedagogical, external_docs |
| `backend/internal/infrastructure/driven/{context7,brave}/*.go` | Create (WIP in tree) | External API adapters |
| `backend/cmd/api/main.go` | Modify | Wire up new services and MCP server (5 tools registered today) |
| `frontend/codeauditor/src/app/domain/ports/mcp-client.port.ts` | Create | Port definition |
| `frontend/codeauditor/src/app/infrastructure/adapters/mcp-client.adapter.ts` | Create | HTTP + SSE client to consume Go monolith |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Go) | Tool execution logic | One test per tool, passing mock services. Assert arg parsing and correct service calls. |
| Unit (Go) | Registry & Dispatcher | Test unknown tool returns `-32601`, duplicates panic, parsing JSON-RPC format, timeout enforcement. |
| Unit (TS) | `MCPClientAdapter` | Use `HttpClientTestingModule` to verify JSON-RPC payload shape. |
| Integration | End-to-end tool call | Boot test server with auth, `POST /mcp/tools/call`, assert correct output. |

## Threat Matrix

Applicable boundaries: Executable-file execution (Sandbox Tools), AI Generation (Pedagogical).

| Threat Case | Applicable? | Failure Mode | Test Strategy |
|-------------|-------------|--------------|---------------|
| RCE via `mcp-sandbox` | Yes | Unsafe container breakout. | Verify `DockerSandbox` runs with `--cap-drop=ALL` and `network=none`. Verify timeout cuts execution. |
| Path Traversal in Curricula | Yes | `get_topics` reads outside `.atl/` | Red test passing `../` in language or topic ID. |
| Prompt Injection | Yes | User inputs subvert socratic prompt. | Red test passing instructions in user chat. The LLM must follow system instructions first. |
| Tool Exhaustion (DDoS) | Yes | Excessive tool calls drain server. | Load test with 20 req/s to trigger `-32609` limit. |
| Data leakage | Yes | Tool exposes secrets. | Review tool responses; ensure services sanitize output before tool returns. |

## Open Questions

- [ ] Does `mcp-code-health` and `mcp-inspiration` have stub implementations in S2 to just return "Not implemented", or are they completely absent from the S2 build and registry? (Assuming absent).
- [ ] Will SSE `streamTool()` accept identical parameters to `callTool()`, or does it need a separate protocol like providing a job ID first? (Assuming direct call via Query Params/Body since it's `GET /mcp/events` vs `POST /mcp/tools/call`).
