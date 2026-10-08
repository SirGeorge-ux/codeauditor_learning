# Design: Personal Tech Dictionary

## Technical Approach

The Personal Tech Dictionary is a full-stack vertical slice built with a strict Hexagonal Architecture in Go, and Angular 21 with Signals on the frontend. The feature introduces a private glossary for users to store technical terms. The primary data store is PostgreSQL (via Supabase), which handles Row-Level Security (RLS) and Full-Text Search (FTS).

The backend will expose both a REST API for the Angular frontend and 4 new MCP tools under the `mcp-dictionary` group. The tutor chat (via LLM) will consume these MCP tools to automatically retrieve definitions when a user asks conceptual questions.

## Architecture Decisions

### Decision: Postgres FTS (ts_vector) vs LIKE/ILIKE

**Choice**: `ts_vector` + `GIN` index.
**Alternatives considered**: Standard `ILIKE '%query%'`.
**Rationale**: `ILIKE` requires a full table scan and does not scale or rank (`ts_rank`). Arrays (synonyms, tags) cannot be easily pattern-matched in a single performant `ILIKE` query without joins or unnesting. `ts_vector` guarantees fast lookup (<200ms) and relevance-based ranking, which is critical when the MCP tutor queries the dictionary.

### Decision: Separate table vs JSONB array on User

**Choice**: Separate `dictionary_terms` table.
**Alternatives considered**: A `terms` JSONB array column on the `users` table.
**Rationale**: JSONB updates are atomic overwrites. A growing dictionary (1K+ terms) would result in massive JSON block overwrites on every update, causing DB bloat. A separate table allows pagination, GIN indexing for search, and robust Row-Level Security.

### Decision: Categories - Enum vs Check Constraint

**Choice**: Standard `VARCHAR` with a `CHECK` constraint.
**Alternatives considered**: Native Postgres `ENUM` type.
**Rationale**: Managing custom `ENUM` types in PostgreSQL migrations can be brittle, especially if categories need to change or expand later. A `CHECK` constraint achieves the exact same domain safety while keeping schema operations straightforward.

### Decision: Frontend - Modal vs Separate Page for Add/Edit

**Choice**: Modal for Add/Edit; Separate page for Detail view (`/dictionary/:term`).
**Alternatives considered**: Separate routes for `/dictionary/new` and `/dictionary/:term/edit`.
**Rationale**: Adding terms frequently breaks flow. A modal allows the user (or tutor) to quickly add a term while preserving their current search/list context. The Detail view remains a separate route so terms can be shareable/linkable and provide deep focus, but it reuses the same Modal component for editing.

## Data Flow

    [Tutor Chat / User] ──→ [Angular UI / MCP Client]
                                │
                                ▼
                       [HTTP API /api/v1/dictionary]
                                │
                                ▼
                      [Core: DictionaryService]
                                │
                                ▼
                    [Port: DictionaryRepositoryPort]
                                │
                                ▼
                 [Adapter: Supabase DictionaryRepository]
                                │
                                ▼
                  [PostgreSQL: dictionary_terms Table]

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/core/domain/models/term.go` | Create | Domain model for Term |
| `backend/internal/core/services/dictionary_service.go` | Create | Business logic for term management |
| `backend/internal/ports/dictionary_repository_port.go` | Create | Interface for storage adapter |
| `backend/internal/infrastructure/driven/supabase/dictionary_repository.go` | Create | Supabase implementation for PG |
| `backend/internal/infrastructure/driving/dictionary_handler.go` | Create | REST API endpoints |
| `backend/internal/infrastructure/mcp/dictionary/` | Create | 4 MCP tools integration |
| `backend/migrations/..._create_dictionary.sql` | Create | DDL, FTS index, and RLS policies |
| `frontend/codeauditor/src/app/domain/models/term.ts` | Create | Frontend Term interfaces |
| `frontend/codeauditor/src/app/domain/ports/dictionary-repository.port.ts` | Create | Frontend repository port |
| `frontend/codeauditor/src/app/application/dictionary.use-case.ts` | Create | Signals-based use case |
| `frontend/codeauditor/src/app/infrastructure/repositories/http-dictionary.repository.ts` | Create | HTTP Angular adapter |
| `frontend/codeauditor/src/app/infrastructure/components/dictionary/` | Create | Page, modal, list, and detail components |
| `frontend/codeauditor/src/app/infrastructure/components/layout/` | Modify | Add `/dictionary` navigation link |

## Interfaces / Contracts

**Go Model:**
```go
package models

import "time"

type Term struct {
	ID          string    `json:"id"`
	UserID      string    `json:"-"`
	Term        string    `json:"term"`
	Explanation string    `json:"explanation"`
	Category    string    `json:"category"`
	Example     *string   `json:"example,omitempty"`
	Synonyms    []string  `json:"synonyms"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
```

**TypeScript Model:**
```typescript
export type TermCategory = 'Concepto' | 'Patrón' | 'Herramienta' | 'Abreviatura' | 'Anti-patrón' | 'Otro';

export interface Term {
  id: string;
  user_id: string;
  term: string;
  explanation: string;
  category: TermCategory;
  example?: string;
  synonyms: string[];
  tags: string[];
  created_at: Date;
  updated_at: Date;
}
```

**MCP Dictionary Tools:**
- `mcp-dictionary.search_term`: `{ query: string }`
- `mcp-dictionary.get_term`: `{ id: string }`
- `mcp-dictionary.add_term`: `{ term, explanation, category, example?, synonyms?, tags? }`
- `mcp-dictionary.list_terms`: `{ category?, tag? }`

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | DictionaryService | Go `testify` mocks for repository port testing CRUD logic |
| Unit | DictionaryHandler | Go table-driven tests verifying HTTP 400s, 201s, 200s, 404s |
| Unit | Angular DictionaryUseCase | Verify signals update correctly with `HttpTestingController` |
| Integration | Postgres `tsvector` FTS | DB-level execution of FTS against actual arrays/strings |
| Integration | MCP Tools | Ensure `mcp-dictionary` calls correctly map to underlying service |
| E2E | Dictionary UI & Tutor Chat | Cypress/Playwright flow adding a term and retrieving it |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

A single new SQL migration script will be introduced:
1. Creates `dictionary_terms` table with generated `search_vector`.
2. Creates unique index and GIN index.
3. Enables RLS and configures `Users can manage their own terms` policy.

No data migration required as this is a new feature.

## Open Questions

- None
