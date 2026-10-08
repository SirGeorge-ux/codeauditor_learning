# Dictionary Specification

## Purpose

Provides a persistent, private tech glossary for users to collect, classify, and recall terms encountered during practice. Supports CRUD operations, full-text search via Postgres `tsvector`, MCP tools, and tutor chat auto-lookup.

## Requirements

### Requirement: Term Model

The system SHALL define a `Term` entity with the following fields:

| Field | Type | Constraint |
|---|---|---|
| `id` | UUID | Primary key, auto-generated |
| `user_id` | string (UUID) | NOT NULL; owner isolation |
| `term` | string | Required, max 50 chars, stored lowercase |
| `explanation` | string | Required, min 10 chars, max 500 chars |
| `category` | enum | Required; one of: `Concepto`, `Patrón`, `Herramienta`, `Abreviatura`, `Anti-patrón`, `Otro` |
| `example` | string | Optional, max 1000 chars |
| `synonyms` | string[] | Optional, each entry max 50 chars |
| `tags` | string[] | Optional, each entry max 30 chars |
| `created_at` | timestamp | Auto-set on insert |
| `updated_at` | timestamp | Auto-set on insert and update |

#### Scenario: Valid term creation

- GIVEN a user submits a term with all required fields
- WHEN the system validates the input
- THEN the term is persisted with `user_id` from JWT, `created_at`, and `updated_at` set

#### Scenario: Duplicate term rejection

- GIVEN a term with the same lowercase name already exists for the user
- WHEN the user attempts to create it again
- THEN the system returns a 409 Conflict error

#### Scenario: Invalid category rejection

- GIVEN a term submission with category "Design Pattern"
- WHEN the system validates the category
- THEN the system returns a 400 Bad Request with field-level error

#### Scenario: Explanation too short

- GIVEN a term with explanation of 5 characters
- WHEN the system validates the input
- THEN the system returns a 400 Bad Request

### Requirement: CRUD API

The system SHALL expose REST endpoints under `/api/v1/dictionary`, all protected by JWT middleware.

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/dictionary` | Create term |
| `GET` | `/api/v1/dictionary?q=<query>` | Full-text search (returns up to 20 ranked results) |
| `GET` | `/api/v1/dictionary?category=<cat>&tag=<tag>` | Filtered list |
| `GET` | `/api/v1/dictionary/:id` | Get term by ID |
| `PUT` | `/api/v1/dictionary/:id` | Update term |
| `DELETE` | `/api/v1/dictionary/:id` | Delete term |

#### Scenario: Create term via POST

- GIVEN a valid JSON body with `term`, `explanation`, and `category`
- WHEN the user sends `POST /api/v1/dictionary` with a valid JWT
- THEN the system returns 201 Created with the persisted `Term` including `id`, `created_at`, `updated_at`

#### Scenario: Search returns ranked results

- GIVEN 5 terms exist matching the query "closure"
- WHEN the user sends `GET /api/v1/dictionary?q=closure`
- THEN the system returns up to 20 results ordered by `ts_rank` descending

#### Scenario: Filter by category

- GIVEN terms exist across multiple categories
- WHEN the user sends `GET /api/v1/dictionary?category=Patrón`
- THEN only terms with category `Patrón` are returned

#### Scenario: Unauthenticated request rejected

- GIVEN no JWT token in the request
- WHEN the user sends any request to `/api/v1/dictionary`
- THEN the system returns 401 Unauthorized

#### Scenario: Delete term

- GIVEN a term exists owned by the authenticated user
- WHEN the user sends `DELETE /api/v1/dictionary/:id`
- THEN the term is removed and the system returns 204 No Content

#### Scenario: Delete term owned by another user

- GIVEN a term exists owned by a different `user_id`
- WHEN the user sends `DELETE /api/v1/dictionary/:id`
- THEN the system returns 404 Not Found (no information leak)

### Requirement: Full-Text Search

The system SHALL use Postgres `to_tsvector` with GIN index for ranked full-text search across `term`, `explanation`, `synonyms`, and `tags`.

#### Scenario: Search matches term name

- GIVEN a term "hardcode" exists
- WHEN the user searches for "hard"
- THEN the term appears in results with high rank

#### Scenario: Search matches explanation

- GIVEN a term "closure" with explanation mentioning "lexical scope"
- WHEN the user searches for "lexical"
- THEN the term appears in results

#### Scenario: Search matches synonyms

- GIVEN a term "PR" with synonym "pull request"
- WHEN the user searches for "pull request"
- THEN the term appears in results

#### Scenario: Search matches tags

- GIVEN a term tagged "javascript"
- WHEN the user searches for "javascript"
- THEN the term appears in results

#### Scenario: Empty query returns all terms

- GIVEN terms exist for the user
- WHEN the user sends `GET /api/v1/dictionary?q=`
- THEN all terms are returned (paginated or capped at 100)

#### Scenario: Search performance under load

- GIVEN 1,000 terms exist for a user
- WHEN the user searches for a common term
- THEN results return in under 200 ms

### Requirement: MCP Dictionary Tools

The system SHALL expose 4 MCP tools under the `mcp-dictionary` group: `search_term`, `get_term`, `add_term`, `list_terms`.

| Tool | Input | Output |
|---|---|---|
| `search_term` | `query: string` | Ranked list of matching `Term` (max 20) |
| `get_term` | `id: string` | Single `Term` or null |
| `add_term` | `term, explanation, category, example?, synonyms?, tags?` | Created `Term` |
| `list_terms` | `category?, tag?` | List of `Term` (all or filtered) |

#### Scenario: MCP search_term returns results

- GIVEN terms exist in the dictionary
- WHEN `search_term` is called with query "closure"
- THEN matching terms are returned ranked by relevance

#### Scenario: MCP add_term validates input

- GIVEN `add_term` is called with `explanation` of 3 characters
- WHEN the tool executes
- THEN it returns an error indicating minimum length violation

#### Scenario: MCP get_term returns null for missing ID

- GIVEN no term exists with the given ID
- WHEN `get_term` is called
- THEN it returns null (not an error)

#### Scenario: MCP list_terms filters by category

- GIVEN terms exist across categories
- WHEN `list_terms` is called with `category: "Anti-patrón"`
- THEN only anti-pattern terms are returned

### Requirement: Frontend Dictionary Page

The system SHALL provide an Angular standalone `/dictionary` page with search, A-Z grid, category filters, and add/edit modals, following the Dark IDE / Cyber-Minimalist style.

#### Scenario: Search bar filters results in real-time

- GIVEN the user is on `/dictionary` with terms loaded
- WHEN the user types in the search input
- THEN the displayed list updates to show matching terms (debounced at 300 ms)

#### Scenario: A-Z letter navigation

- GIVEN terms exist starting with various letters
- WHEN the user clicks letter "H" in the A-Z grid
- THEN only terms starting with "H" are displayed

#### Scenario: Category filter chip selection

- GIVEN the user is viewing all terms
- WHEN the user clicks the "Patrón" filter chip
- THEN only terms with category `Patrón` are shown

#### Scenario: Add term modal submission

- GIVEN the user clicks "Add new" and fills required fields
- WHEN the user clicks "Save"
- THEN the term is created, the modal closes, and the new term appears in the list

#### Scenario: Add term modal validation feedback

- GIVEN the user clicks "Save" with an empty explanation
- WHEN the form validates
- THEN the explanation field shows an inline error ("min 10 characters")

#### Scenario: Edit term updates display

- GIVEN the user opens the edit modal on an existing term
- WHEN the user changes the explanation and saves
- THEN the term updates and the list reflects the change

#### Scenario: Delete term with confirmation

- GIVEN the user clicks "Delete" on a term
- WHEN the confirmation dialog appears and the user confirms
- THEN the term is deleted and removed from the list

### Requirement: Tutor Chat Auto-Lookup

The system SHALL enable the tutor to search the dictionary automatically when a user asks "qué es X" or equivalent definition queries.

#### Scenario: Tutor finds term and uses it

- GIVEN a term "PR" exists with explanation "Pull Request"
- WHEN the user asks "Qué es PR?"
- THEN the tutor calls `search_term("PR")`, finds the term, and incorporates its explanation into the response

#### Scenario: Tutor finds no matching term

- GIVEN no term matches the user's query
- WHEN the user asks "Qué es monad?"
- THEN the tutor falls back to its general knowledge without exposing the dictionary absence

#### Scenario: Tutor respects socratic level

- GIVEN a term exists and the user's socratic level is 2
- WHEN the user asks about the term
- THEN the tutor uses the dictionary definition as context but guides with questions rather than giving the full answer directly

### Requirement: Row-Level Security

The system SHALL enforce Postgres RLS on `dictionary_terms` so each user can only access their own terms.

#### Scenario: RLS prevents cross-user reads

- GIVEN user A creates a term
- WHEN user B queries the dictionary API
- THEN user B cannot see user A's term

#### Scenario: RLS prevents cross-user writes

- GIVEN a JWT belonging to user A
- WHEN a request attempts to insert a term with `user_id` set to user B
- THEN the system overrides `user_id` from the JWT or rejects the request
