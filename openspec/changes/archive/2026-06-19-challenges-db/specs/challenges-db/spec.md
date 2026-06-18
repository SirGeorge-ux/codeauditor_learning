# Challenge Management Specification

## Purpose

Retrieve code-audit challenges from a PostgreSQL database via authenticated REST API, replacing hardcoded mock data. Enables dynamic challenge management while preserving backward compatibility with the frontend domain model and port interfaces.

## Requirements

### Requirement: Database Schema

The system MUST define a `public.challenges` table matching the `Challenge` domain model with the following columns: `id` (UUID PK, default `gen_random_uuid()`), `title` (TEXT NOT NULL), `description` (TEXT NOT NULL), `difficulty` (TEXT NOT NULL, CHECK IN ('junior','mid','senior','architect')), `category` (TEXT NOT NULL), `language` (TEXT NOT NULL), `repo_url` (TEXT NOT NULL), `code` (TEXT NOT NULL), `code_smell` (TEXT NOT NULL), `status` (TEXT DEFAULT 'available', CHECK IN ('available')), `created_at` (TIMESTAMPTZ DEFAULT NOW()). Row Level Security MUST be enabled. Authenticated users SHALL SELECT all rows. No INSERT/UPDATE/DELETE policies for this phase.

#### Scenario: Table creation

- GIVEN a PostgreSQL 15 database with Supabase
- WHEN migration `003_create_challenges.sql` is executed
- THEN the `public.challenges` table MUST exist with all specified columns and constraints
- AND RLS MUST be enabled with a SELECT policy for authenticated users

#### Scenario: Invalid difficulty rejected

- GIVEN the challenges table exists
- WHEN an INSERT attempts difficulty='expert'
- THEN the database MUST reject the row with a CHECK constraint violation

#### Scenario: Authenticated user can read

- GIVEN a user is authenticated via Supabase Auth
- WHEN the user queries `public.challenges`
- THEN the user MUST receive all rows with status='available'

### Requirement: Seed Data

The system MUST provide an idempotent SQL seed file (`003_seed_challenges.sql`) that inserts all 8 challenges from `MockChallengeRepository`. The file MUST use `DO $$ ... END $$` blocks with existence checks (`WHERE NOT EXISTS`) to ensure idempotency. Each challenge MUST use its original mock ID (e.g., `ch-sqli`, `ch-xss`).

#### Scenario: First-time seed

- GIVEN an empty `public.challenges` table
- WHEN `003_seed_challenges.sql` is executed
- THEN all 8 challenges MUST be inserted with correct data

#### Scenario: Re-run seed is idempotent

- GIVEN all 8 challenges already exist in the table
- WHEN `003_seed_challenges.sql` is executed again
- THEN no duplicate rows MUST be created
- AND the total row count MUST remain 8

### Requirement: Backend Challenge Service

The system MUST implement `ChallengeService` in `backend/internal/core/services/challenge_service.go` with a `*sql.DB` dependency via constructor injection. It MUST expose `GetAll(ctx context.Context) ([]Challenge, error)` returning all challenges with status='available', and `GetByID(ctx context.Context, id string) (Challenge, error)` returning a single challenge or error if not found. The pattern MUST follow `AuditHistoryService`.

#### Scenario: Get all challenges

- GIVEN the database contains 8 seeded challenges with status='available'
- WHEN `GetAll(ctx)` is called
- THEN it MUST return all 8 challenges ordered by `created_at`
- AND no error MUST be returned

#### Scenario: Get challenge by valid ID

- GIVEN a challenge exists with id='ch-sqli'
- WHEN `GetByID(ctx, "ch-sqli")` is called
- THEN it MUST return that challenge with all fields populated

#### Scenario: Get challenge by non-existent ID

- GIVEN no challenge exists with id='ch-nonexistent'
- WHEN `GetByID(ctx, "ch-nonexistent")` is called
- THEN it MUST return an error (sql.ErrNoRows or wrapped equivalent)

### Requirement: Backend Challenge Handler

The system MUST implement `ChallengeHandler` in `backend/internal/infrastructure/driving/handlers/challenge_handler.go` with `ChallengeService` dependency via constructor injection. It MUST expose `ListChallenges(w, r)` for `GET /api/v1/challenges` and `GetChallenge(w, r)` for `GET /api/v1/challenges/{id}`. Responses MUST use `json.NewEncoder(w).Encode()`. Empty results MUST return `[]` (not null). Not-found MUST return HTTP 404 with JSON error body.

#### Scenario: List all challenges

- GIVEN 8 challenges exist in the database
- WHEN an authenticated user sends `GET /api/v1/challenges`
- THEN the response MUST be HTTP 200 with `Content-Type: application/json`
- AND the body MUST be a JSON array of 8 challenge objects

#### Scenario: Get single challenge

- GIVEN challenge 'ch-sqli' exists
- WHEN an authenticated user sends `GET /api/v1/challenges/ch-sqli`
- THEN the response MUST be HTTP 200 with the challenge as a JSON object

#### Scenario: Challenge not found

- WHEN an authenticated user sends `GET /api/v1/challenges/ch-nonexistent`
- THEN the response MUST be HTTP 404 with a JSON error body

#### Scenario: Unauthenticated request

- WHEN a request lacks a valid JWT
- THEN the endpoint MUST return HTTP 401 (enforced by `AuthMiddleware`)

### Requirement: Route Registration

The system MUST register challenge routes under `/api/v1/challenges` in the Chi router within `main.go`, protected by `authmiddleware.AuthMiddleware`. The `ChallengeService` MUST be instantiated with the shared `*sql.DB` connection.

#### Scenario: Routes registered

- GIVEN the API server starts
- WHEN `GET /api/v1/challenges` is called with valid JWT
- THEN the request MUST reach the ChallengeHandler

### Requirement: Frontend HTTP Repository

The system MUST implement `HttpChallengeRepository` in `frontend/.../repositories/http-challenge.repository.ts` implementing the `ChallengeRepository` port. It MUST use the native `fetch` API with manual `Authorization: Bearer <token>` header (NOT HttpClient). The token MUST be retrieved from `AuthService`. `getAll()` MUST return `[]` on network error. `getById(id)` MUST return `null` on 404 or network error.

#### Scenario: Fetch all challenges successfully

- GIVEN the backend returns 8 challenges
- WHEN `getAll()` is called with a valid auth token
- THEN it MUST return an array of 8 Challenge objects

#### Scenario: Network error on getAll

- GIVEN the backend is unreachable
- WHEN `getAll()` is called
- THEN it MUST return an empty array `[]` (not throw)

#### Scenario: Get challenge by ID — not found

- GIVEN the backend returns HTTP 404
- WHEN `getById("ch-nonexistent")` is called
- THEN it MUST return `null`

#### Scenario: Network error on getById

- GIVEN the backend is unreachable
- WHEN `getById("ch-sqli")` is called
- THEN it MUST return `null`

### Requirement: Frontend Service Wiring

The system MUST update `ChallengeService` to instantiate `HttpChallengeRepository` instead of `MockChallengeRepository`. The `tempChallenges` Map lookup in `getChallenge(id)` MUST be preserved — temp challenges are checked before delegating to the repository. The `ChallengeRepository` port interface MUST NOT change.

#### Scenario: Load challenges from HTTP

- GIVEN `ChallengeService` is initialized with `HttpChallengeRepository`
- WHEN `loadChallenges()` is called
- THEN it MUST fetch challenges from the backend API
- AND `challengesSignal` MUST be updated with the results

#### Scenario: Temp challenge takes priority

- GIVEN a temp challenge with id='temp-123' exists in the Map
- WHEN `getChallenge('temp-123')` is called
- THEN it MUST return the temp challenge WITHOUT calling the HTTP repository

#### Scenario: Regular challenge from HTTP

- GIVEN no temp challenge exists for id='ch-sqli'
- WHEN `getChallenge('ch-sqli')` is called
- THEN it MUST delegate to the HTTP repository

### Requirement: Backward Compatibility

The `Challenge` domain model in the frontend MUST NOT change. The `ChallengeRepository` port interface MUST NOT change (methods: `getAll()`, `getById(id)`). Existing tests for `ChallengeService` and `ChallengeUseCase` MUST still pass after the migration without modification.

#### Scenario: Domain model unchanged

- GIVEN the existing `Challenge` interface
- WHEN the migration is complete
- THEN all fields and types MUST remain identical

#### Scenario: Port interface unchanged

- GIVEN the existing `ChallengeRepository` interface
- WHEN `HttpChallengeRepository` is implemented
- THEN it MUST satisfy the interface without any interface changes
