# Proposal: MCP Monolith (S2)

## Intent

Build the **MCP (Model Context Protocol) monolith**: a single Go binary exposing 32 tools across 7 groups via HTTP+SSE (JSON-RPC 2.0). These tools are the backend capabilities consumed by the **tutor chat** and the **Angular frontend** via `MCPClientPort`. Every tool delegates to a service in `core/services/` — NEVER accesses DB/IO directly. This is the backbone that makes the tutor "context-aware" and pedagogically competent.

## Scope

### In Scope (S2 — "MCP Base")
- **mcp-sandbox** (4 tools): `run_code`, `list_languages`, `healthcheck`, `get_install_hint`
- **mcp-challenges** (5 tools): `list_challenges`, `get_challenge`, `create_temp_challenge`, `generate_challenge`, `validate_challenge`
- **mcp-user** (6 tools): `get_profile`, `get_language_progress`, `get_history`, `get_learning_profile`, `record_audit_attempt`, `update_learning_profile`
- **mcp-curriculum** (4 tools): `get_topics`, `get_topic`, `suggest_topic`, `review_topic`
- **mcp-dictionary** (4 tools): `search_term`, `get_term`, `add_term`, `list_terms`
- **mcp-pedagogical** ⭐ (6 tools): `get_learning_objective`, `generate_story_context`, `create_scaffolded_steps`, `validate_solution`, `get_socratic_prompt`, `peer_review_topic`
- **mcp-external_docs** (3 tools): `get_library_docs` (Context7), `search_documentation` (Brave), `get_real_example`

### Out of Scope / Deferred
- **mcp-inspiration** (3 tools) → **S3** (requires Exercism wrapper + GitHub education examples)
- **mcp-code_health** (4 tools) → **S4** (requires Code Health Dashboard infrastructure)

## Capabilities

### New Capabilities
- `mcp-sandbox`: Sandbox execution via MCP tools
- `mcp-challenges`: Challenge management via MCP tools
- `mcp-user`: User profile/progress via MCP tools
- `mcp-curriculum`: Curriculum reading via MCP tools
- `mcp-dictionary`: Personal glossary via MCP tools
- `mcp-pedagogical`: Pedagogical engine (learning objectives, socratic prompts, validation)
- `mcp-external-docs`: Library documentation lookup (Context7 + Brave)

### Modified Capabilities
- `challenge-service`: Add `GenerateChallenge` + `ValidateChallenge` methods to support `mcp-challenges.generate_challenge` / `validate_challenge`
- `user-progress-service`: Already has methods; expose via MCP wrapper tools
- `tutor-chat`: Frontend use-case will consume `MCPClientPort` instead of direct HTTP

## Approach

1. **Create missing services** in `core/services/`: `CurriculumService` (reads `.atl/curricula/*.md`), `DictionaryService` (CRUD for glossary — **shipped**, see reconciled state), `TutorService` / `PedagogicalService` (socratic prompts, scaffolding, story context), `ExternalDocsService` (Context7 + Brave adapters).
2. **Create adapters** in `infrastructure/driven/`: `context7/`, `brave/`, `github/`, `exercism/` (S3). Each implements a port defined in `ports/`.
3. **MCP runtime** lives in `internal/mcpkit/` (**shipped**, see reconciled state): `tool.go` (`MCPTool` interface), `registry.go` (`Registry`), `server.go` (Chi router mounting `POST /mcp/tools/call`, `GET /mcp/events` SSE, `GET /mcp/tools/list`), `dispatcher.go` (JSON-RPC 2.0), `ratelimiter.go`, `mcpctx/`. **Tool groups** stay in `infrastructure/mcp/`, one sub-package per group (`sandbox/`, `challenges/`, `user/`, `curriculum/`, `dictionary/`, `pedagogical/`, `external_docs/`), importing `mcpkit` for runtime symbols.
4. **Wire in `cmd/api/main.go`**: Instantiate all services, instantiate `mcpkit.NewRegistry(...)` with every tool, mount MCP routes under the same Chi router (protected by `AuthMiddleware`).
5. **Frontend adapter**: `infrastructure/adapters/mcp-client.adapter.ts` implementing `domain/ports/MCPClientPort`.

## Reconciled State (2026-10-08)

- **`dictionary` change (archived `2026-10-08-dictionary`)**: `DictionaryService`, `DictionaryRepositoryPort`, Supabase adapter + RLS, `/api/v1/dictionary` handler, and the `mcp-dictionary` group (4 tools) are shipped and registered. Frontend `/dictionary` page shipped.
- **`mcpkit` change (archived `2026-10-08-mcpkit`)**: the shared runtime was extracted from `infrastructure/mcp/` to `internal/mcpkit/` as a reusable library. Tool groups stay in `infrastructure/mcp/<group>/`. 5 tools are currently registered (4 dictionary + 1 tutor lookup); the remaining ~27 land with PRs 2-4.
- **Architecture stance**: confirmed hexagonal — no re-architecture required. The MCP monolith is a single in-process server; tool groups are driving adapters over `mcpkit`, delegating exclusively to `core/services/` via `ports/`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/core/services/` | New | Add `CurriculumService`, `PedagogicalService`, `ExternalDocsService` (`DictionaryService` shipped) |
| `backend/internal/ports/` | New | Add `LLMClient`, `CurriculumPort`, `PedagogicalPort`, `ExternalDocsPort` (`DictionaryRepositoryPort` shipped) |
| `backend/internal/infrastructure/driven/` | New | Add `context7/`, `brave/`, `github/` adapters |
| `backend/internal/mcpkit/` | Shipped | Shared MCP runtime (tool, registry, dispatcher, server, ratelimiter, mcpctx) — `mcpkit` change |
| `backend/internal/infrastructure/mcp/<group>/` | New | 7 group sub-packages (dictionary shipped; sandbox, challenges, user, curriculum, pedagogical, external_docs pending) |
| `backend/cmd/api/main.go` | Modified | Wire all new services + MCP registry + routes (5 tools registered; 32 by PR 5) |
| `frontend/src/app/domain/ports/` | New | `MCPClientPort` interface |
| `frontend/src/app/infrastructure/adapters/` | New | `MCPClientAdapter` (HTTP+SSE) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| LLM hallucinates tool names | Med | Registry validates `tool_call.name` before dispatch; return `jsonrpc` error `-32601` |
| Tool returns >200K tokens | Med | Hard cap at 50K chars per response; truncate + summarize |
| Tool timeout (e.g. sandbox) | Med | 30s timeout per `Execute()`; SSE stream sends `{"status":"timeout"}` |
| Direct DB access in tools | Low | Code review check: `grep -r "sql.DB\|sql.Open" backend/internal/infrastructure/mcp/` must be empty |
| Circular dependency service→tool | Low | Tools only import `core/services/` and `ports/`; services do NOT import `mcp/` |

## Rollback Plan

1. Revert `cmd/api/main.go` to remove MCP route registration.
2. Remove the tool-group directories under `backend/internal/infrastructure/mcp/` (runtime `mcpkit` is a shipped library — kept unless the whole S2 MCP front is reverted).
3. Remove new driven adapters (`context7/`, `brave/`, `github/`) if they cause build issues.
4. Frontend: switch `TutorChatUseCase` back to direct `HttpClient` calls to existing `/api/v1/*` endpoints.
5. The existing REST API (`/api/v1/challenges`, `/api/v1/users/.../progress`) remains untouched and functional throughout.

## Dependencies

- **S1 services**: `ChallengeService`, `UserProgressService`, `AuditService` must be stable (they exist and are tested).
- **Dictionary S2**: `DictionaryService` schema + repo must exist (separate `openspec/changes/dictionary/` spec).
- **Tutor-chat S2**: Frontend `TutorChatUseCase` consumes this MCP monolith (coordinated delivery).
- **External APIs**: Context7 API key, Brave Search API key (backend `.env` only).
- **Ollama/Groq**: `PedagogicalService` needs an `LLMClient` port; S5 cascade refines the concrete adapter.

## Success Criteria

- [ ] `GET /mcp/tools/list` returns 32 tools (all groups except deferred) with valid OpenAI-compatible schemas (4 dictionary tools already registered)
- [ ] `POST /mcp/tools/call` executes any tool with JSON-RPC 2.0 and returns a result or structured error
- [ ] `GET /mcp/events` streams long-running tool results via SSE
- [ ] Every tool in `infrastructure/mcp/<group>/` delegates to a `core/services/` method (zero direct DB/IO); the runtime lives in `internal/mcpkit/`
- [ ] `make test-backend` passes for all new MCP packages (≥1 test per tool)
- [ ] Frontend `MCPClientAdapter` successfully calls `mcp-pedagogical.get_socratic_prompt` and receives a prompt
- [ ] No import from `core/` to `infrastructure/mcp/` or `mcpkit/`, and no import from tool groups to DB/HTTP SDKs (hexagonal purity)
