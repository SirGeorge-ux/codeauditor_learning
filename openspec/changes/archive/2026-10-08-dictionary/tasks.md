# Tasks: Personal Tech Dictionary

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2,300 (actual: see apply-progress) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes (originally) |
| Suggested split | PR 1 (Backend) → PR 2 (MCP) → PR 3 (Frontend) |
| Delivery strategy | size:exception-accepted (single PR per maintainer override) |
| Chain strategy | single-pr |

### Work Unit (this PR = all 3)

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| single-pr | Backend + MCP + Frontend | 1 | `go test -count=1 ./internal/...` + `pnpm test` | `make dev-backend` + curl /api/v1/dictionary (JWT) + /dictionary browse | Revert migration + delete new files |

## Phase 1: Migration & Domain Models

- [x] 1.1 Create `backend/migrations/..._create_dictionary.sql` — table, FTS vector, GIN index, unique(user_id, term), RLS policies
- [x] 1.2 Create `backend/internal/core/domain/models/term.go` — `Term` struct with json tags
- [x] 1.3 Create `frontend/codeauditor/src/app/domain/models/term.ts` — `Term` interface + `TermCategory` union

## Phase 2: Backend Ports & Adapters

- [x] 2.1 Create `backend/internal/ports/dictionary_repository_port.go` — CRUD + FTS search interface
- [x] 2.2 Create `backend/internal/infrastructure/driven/supabase/dictionary_repository.go` — Supabase adapter implementing the port

## Phase 3: Core Backend Service

- [x] 3.1 Create `backend/internal/core/services/dictionary_service.go` — CRUD, duplicate check, input validation
- [x] 3.2 Create `backend/internal/infrastructure/driving/handlers/dictionary_handler.go` — 6 REST endpoints (POST, GET list, GET :id, PUT, DELETE, search)
- [x] 3.3 Register `/api/v1/dictionary` route in `backend/cmd/api/main.go`

## Phase 4: MCP Dictionary Tools

- [x] 4.1 Create `backend/internal/infrastructure/mcp/dictionary/` — 4 tools: `search_term`, `get_term`, `add_term`, `list_terms`
- [x] 4.2 Wire `mcp-dictionary` group into MCP registry in `backend/cmd/api/main.go` (HTTP /mcp routes still deferred to mcp-monolith S2)

## Phase 5: Frontend Adapter & Use Case

- [x] 5.1 Create `frontend/codeauditor/src/app/domain/ports/dictionary-repository.port.ts`
- [x] 5.2 Create `frontend/codeauditor/src/app/infrastructure/repositories/http-dictionary.repository.ts`
- [x] 5.3 Create `frontend/codeauditor/src/app/application/dictionary.use-case.ts` — signals-based state

## Phase 6: Frontend UI Components

- [x] 6.1 Create list/search (in-page) — A-Z nav + category chips + debounced search input
- [x] 6.2 Create add/edit modal component with inline form validation
- [x] 6.3 Create term detail view component (`/dictionary/:id` route)
- [x] 6.4 Create `/dictionary` page component orchestrating list/modal/detail
- [x] 6.5 Add `/dictionary` route + nav link in layout component

## Phase 7: Tests & Verification

- [x] 7.1 Tests for DictionaryService (mock repository port) — 21 cases across validation, CRUD, search
- [x] 7.2 Table-driven tests for DictionaryHandler (HTTP codes: 201, 400, 401, 404, 409, 204) — 17 cases
- [x] 7.3 Tests for MCP dictionary tools (search, add, get, list) — 15 cases
- [x] 7.4 Tests for Angular DictionaryUseCase — 15 cases
- [x] 7.5 Tests for Angular DictionaryPageComponent — 14 cases
- [x] 7.6 Verify `make test-backend` + `make test-frontend` + `make validate` pass (see apply-progress)
