# Proposal: Personal Tech Dictionary

## Intent

Give the user a persistent, private tech glossary to collect, classify, and recall terms they encounter while practicing. Solves the "I saw this concept before but forgot what it means" friction that breaks flow during audits and tutor chat sessions.

## Scope

### In Scope
- `dictionary_terms` table with FTS index, 6 categories, synonyms, tags.
- REST API: CRUD + full-text search + category/tag filters.
- MCP group `mcp-dictionary`: `search_term`, `get_term`, `add_term`, `list_terms`.
- Angular `/dictionary` page: search, A-Z grid, filters, add/edit modal, detail view.
- Tutor chat auto-lookup integration ("qué es X").

### Out of Scope
- Multi-user/shared terms (single-tenant only).
- Auto-suggest categories or auto-generate definitions from LLM.
- Public dictionaries or import from external glossaries.
- Advanced NLP (stemming beyond Postgres `to_tsvector`).

## Capabilities

### New Capabilities
- `dictionary`: CRUD, full-text search, and MCP tools for the personal glossary.

### Modified Capabilities
- None

## Approach

Backend: add `Term` domain model, `DictionaryService`, `DictionaryRepositoryPort`, Supabase adapter with RLS, Chi handler at `/api/v1/dictionary`, and 4 MCP tools in `internal/infrastructure/mcp/dictionary/`. Use Postgres `to_tsvector` + GIN index for ranked FTS. Frontend: add `Term` model, `DictionaryRepositoryPort`, `DictionaryUseCase`, HTTP adapter, and standalone `dictionary/` components with signals. Wire a nav link in layout.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `backend/internal/core/domain/models/term.go` | New | Domain model |
| `backend/internal/core/services/dictionary_service.go` | New | Business logic |
| `backend/internal/ports/dictionary_repository_port.go` | New | Repository interface |
| `backend/internal/infrastructure/driven/supabase/dictionary_repository.go` | New | Supabase adapter |
| `backend/internal/infrastructure/driving/dictionary_handler.go` | New | HTTP handler |
| `backend/internal/infrastructure/mcp/dictionary/` | New | 4 MCP tools |
| `frontend/.../domain/models/term.ts` | New | Frontend model |
| `frontend/.../application/dictionary.use-case.ts` | New | Use case |
| `frontend/.../repositories/http-dictionary.repository.ts` | New | HTTP adapter |
| `frontend/.../components/dictionary/` | New | List, detail, modal |
| `frontend/.../components/layout/` | Modified | Add /dictionary nav link |
| `backend/migrations/` | New | Table + FTS index + RLS |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| FTS query slows with large arrays | Low | GIN index on `to_tsvector`; cap results at 20 |
| `add_term` MCP tool writes without explicit user consent | Med | Require frontend confirmation before calling tool; validate `user_id` from JWT |
| Later multi-user migration breaks single-tenant assumption | Med | `user_id` column + RLS from day one |

## Rollback Plan

1. Revert DB: drop `dictionary_terms` table and related migrations.
2. Revert backend: remove handler registration in `main.go`, delete service/handler/MCP files.
3. Revert frontend: remove `/dictionary` route, nav link, components, use case.
4. Revert Engram: delete `sdd/dictionary/proposal` topic.

## Dependencies
- `auth` spec (JWT middleware) — already in place.
- `mcp-monolith` (S2) — registry and dispatcher pattern must exist; dictionary tools register into it.

## Success Criteria

- [ ] User can add, edit, delete, and search terms via `/dictionary` UI.
- [ ] Full-text search returns ranked results in <200 ms for 1K terms.
- [ ] MCP tools `mcp-dictionary.*` respond correctly via `POST /mcp/tools/call`.
- [ ] `make test-backend` and `make test-frontend` pass for new code.
- [ ] RLS prevents reading another user's terms (tested).
