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

- [x] 3.1 RED: `mcp-curriculum.get_topics` rejects `../` path traversal
- [x] 3.2 `mcp/user/` — 6 tools (profile, progress, history, learning_profile, audit, update)
- [x] 3.3 `mcp/curriculum/` — `get_topics`, `get_topic`, `suggest_topic`, `review_topic`

## Phase 4: Dict + Pedagogical + ExtDocs (PR 4)

- [x] 4.1 RED: `mcp-pedagogical.get_socratic_prompt` resists prompt injection
- [x] 4.2 `mcp/dictionary/` — `search_term`, `get_term`, `add_term`, `list_terms` (shipped via the `dictionary` change, archived 2026-10-08; 4 tools registered in `cmd/api/main.go`)
- [x] 4.3 `mcp/pedagogical/` — 6 tools (objective, story, scaffolding, validate, socratic, peer_review)
- [x] 4.4 `mcp/external_docs/` — `get_library_docs`, `search_documentation`, `get_real_example`

## Phase 5: Frontend + Wiring (PR 5)

- [x] 5.1 `domain/ports/mcp-client.port.ts` — MCPClientPort with listTools/callTool/streamTool
- [x] 5.2 `infrastructure/adapters/mcp-client.adapter.ts` — HTTP + SSE implementation
- [x] 5.3 Wire `cmd/api/main.go` — init all services, registry, register 33 tools (4 dictionary + 1 tutor + 4 sandbox + 5 challenges + 6 user + 4 curriculum + 6 pedagogical + 3 external_docs), mount routes
- [x] 5.4 Verify: `go test -count=1 ./...` (backend) and `npx ng test --watch=false` (frontend) pass
- [x] 5.5 Verify: `grep -rE "database/sql|sql\.DB|sql\.Open" backend/internal/infrastructure/mcp/ backend/internal/mcpkit/ backend/internal/core/services/` empty for files created in PR 5 (hexagonal purity holds)
- [x] 5.6 Verify: `GET /mcp/tools/list` returns exactly 33 tools, no deferred groups

### Phase 5 reconciliation (count = 33, not 32)

The spec headline reads "exactly 32 tools across 7 groups"; the
canonical wiring ships 33 across 8 groups. The 33rd is
`mcp-tutor.tutor_lookup`, registered by the archived
2026-10-08-dictionary change. Removing it would regress the tutor
LLM (which uses it to ground term-lookup questions in the user's
personal glossary); the smaller blast radius is to keep the tool
and acknowledge 33 as the canonical count. The
`TestBuildMCPRegistry_HTTP_GET_MCPToolsList_Reports33` integration
test (backend/cmd/api/wiring_test.go) pins the live count so any
regression in the wiring surfaces at CI.
