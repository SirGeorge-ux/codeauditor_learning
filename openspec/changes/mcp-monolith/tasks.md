# Tasks: MCP Monolith (S2)

> **Reconciled 2026-10-08.** The shared MCP runtime (tool.go, registry.go, dispatcher.go, server.go, ratelimiter.go, mcpctx) moved to `backend/internal/mcpkit/` via the `mcpkit` change (archived `2026-10-08-mcpkit`). The dictionary group (task 4.2) shipped via the `dictionary` change (archived `2026-10-08-dictionary`) and is registered in `cmd/api/main.go`. Phase 1 services (1.9) and adapters (1.10) exist in the working tree but remain uncommitted until they ship with their tool groups in PR 4. The architecture stays hexagonal: single in-process MCP server, tool groups as driving adapters, runtime as a library — no re-architecture needed.

## Review Workload Forecast

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

| Work Unit | Goal | PR | Focused test command | Runtime harness | Rollback boundary |
|-----------|------|----|---------------------|-----------------|-------------------|
| 1 | MCP infra + core services | PR 1 | `go test ./internal/infrastructure/mcp/... -run Registry\|Dispatcher` | `curl /mcp/tools/list` returns empty list | Revert mcp/, services/, driven/ |
| 2 | Sandbox + Challenges tools | PR 2 | `go test ./mcp/sandbox/... ./mcp/challenges/...` | `POST /mcp/tools/call` mcp-sandbox.list_languages | Revert mcp/sandbox/, mcp/challenges/ |
| 3 | User + Curriculum tools | PR 3 | `go test ./mcp/user/... ./mcp/curriculum/...` | `POST /mcp/tools/call` mcp-user.get_profile | Revert mcp/user/, mcp/curriculum/ |
| 4 | Dict + Ped + ExtDocs tools | PR 4 | `go test ./mcp/dictionary/... ./mcp/pedagogical/... ./mcp/external_docs/...` | `POST /mcp/tools/call` mcp-dictionary.search_term | Revert those 3 dirs |
| 5 | Frontend + wiring | PR 5 | `make test-frontend && make test-backend` | Frontend MCPClientAdapter.listTools() succeeds | Revert adapter.ts, main.go wiring |

## Phase 1: MCP Core + Services (PR 1)

- [x] 1.1 RED: dispatcher enforces 30s timeout and 50KB cap
- [x] 1.2 RED: rate limiter returns `-32609` at 11th call/min
- [x] 1.3 `tool.go` — MCPTool interface
- [x] 1.4 `registry.go` — thread-safe Register/Get/List
- [x] 1.5 `dispatcher.go` — JSON-RPC parsing, timeout, truncation, error mapping
- [x] 1.6 `server.go` — Chi: `/mcp/tools/list`, `/mcp/tools/call`, `/mcp/events` SSE
- [x] 1.7 Rate-limiter middleware (per-user, per-tool, 10 req/min)
- [x] 1.8 RED: response sanitization (no secrets leaked)
- [x] 1.9 CurriculumService + DictionaryService + PedagogicalService + ExternalDocsService
- [x] 1.10 `context7/adapter.go` + `brave/adapter.go`

## Phase 2: Sandbox + Challenges (PR 2)

- [ ] 2.1 RED: `mcp-sandbox.run_code` enforces `--cap-drop=ALL --network=none`
- [ ] 2.2 `mcp/sandbox/` — `run_code`, `list_languages`, `healthcheck`, `get_install_hint`
- [ ] 2.3 `mcp/challenges/` — `list_challenges`, `get_challenge`, `create_temp_challenge`, `generate_challenge`, `validate_challenge`

## Phase 3: User + Curriculum (PR 3)

- [ ] 3.1 RED: `mcp-curriculum.get_topics` rejects `../` path traversal
- [ ] 3.2 `mcp/user/` — 6 tools (profile, progress, history, learning_profile, audit, update)
- [ ] 3.3 `mcp/curriculum/` — `get_topics`, `get_topic`, `suggest_topic`, `review_topic`

## Phase 4: Dict + Pedagogical + ExtDocs (PR 4)

- [ ] 4.1 RED: `mcp-pedagogical.get_socratic_prompt` resists prompt injection
- [x] 4.2 `mcp/dictionary/` — `search_term`, `get_term`, `add_term`, `list_terms` (shipped via the `dictionary` change, archived 2026-10-08; 4 tools registered in `cmd/api/main.go`)
- [ ] 4.3 `mcp/pedagogical/` — 6 tools (objective, story, scaffolding, validate, socratic, peer_review)
- [ ] 4.4 `mcp/external_docs/` — `get_library_docs`, `search_documentation`, `get_real_example`

## Phase 5: Frontend + Wiring (PR 5)

- [ ] 5.1 `domain/ports/mcp-client.port.ts` — MCPClientPort with listTools/callTool/streamTool
- [ ] 5.2 `infrastructure/adapters/mcp-client.adapter.ts` — HTTP + SSE implementation
- [ ] 5.3 Wire `cmd/api/main.go` — init all services, registry, register 32 tools, mount routes
- [ ] 5.4 Verify: `make test-backend && make test-frontend` pass
- [ ] 5.5 Verify: `grep -r "sql.DB\|sql.Open" mcp/` empty (hexagonal purity)
- [ ] 5.6 Verify: `GET /mcp/tools/list` returns exactly 32 tools, no deferred groups
