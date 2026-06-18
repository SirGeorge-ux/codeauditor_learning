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

The system MUST implement `ChallengeService` in `backend/internal/core/services/challenge_service.go` with a `*sql.DB` dependency via constructor injection. It MUST expose `GetAll(ctx context.Context) ([]Challenge, error)` returning challenges WHERE `user_id IS NULL OR user_id = $currentUser` with status='available', ordered by `created_at DESC`. It MUST expose `GetByID(ctx context.Context, id string) (Challenge, error)` returning a single challenge or 404 error if not found. If the challenge exists but `user_id` belongs to another user (and is not NULL), MUST return not-found error. The pattern MUST follow `AuditHistoryService`.
(Previously: returned all challenges and any by ID without ownership filtering)

#### Scenario: Get all challenges — seeds + owned

- GIVEN the database contains 8 seeded challenges (user_id=NULL) and 2 user-owned challenges
- WHEN `GetAll(ctx)` is called with `currentUser='u1'`
- THEN it MUST return all 10 challenges ordered by `created_at DESC`

#### Scenario: Get all challenges — excludes other users

- GIVEN challenge A has `user_id='u2'` and challenge B has `user_id=NULL`
- WHEN `GetAll(ctx)` is called with `currentUser='u1'`
- THEN it MUST return challenge B but NOT challenge A

#### Scenario: Get challenge by valid ID — owned or public

- GIVEN a challenge exists with id='ch-sqli' and `user_id=NULL`
- WHEN `GetByID(ctx, "ch-sqli")` is called by any authenticated user
- THEN it MUST return that challenge

#### Scenario: Get challenge by ID — other user's private

- GIVEN a challenge exists with id='ch-private' and `user_id='u2'`
- WHEN `GetByID(ctx, "ch-private")` is called with `currentUser='u1'`
- THEN it MUST return a not-found error

#### Scenario: Get challenge by non-existent ID

- GIVEN no challenge exists with id='ch-nonexistent'
- WHEN `GetByID(ctx, "ch-nonexistent")` is called
- THEN it MUST return an error (sql.ErrNoRows or wrapped equivalent)

### Requirement: Backend Challenge Handler

The system MUST implement `ChallengeHandler` in `backend/internal/infrastructure/driving/handlers/challenge_handler.go` with `ChallengeService` dependency via constructor injection. It MUST expose `ListChallenges(w, r)` for `GET /api/v1/challenges` and `GetChallenge(w, r)` for `GET /api/v1/challenges/{id}` and `CreateChallenge(w, r)` for `POST /api/v1/challenges`. Responses MUST use `json.NewEncoder(w).Encode()`. Empty results MUST return `[]` (not null). Not-found MUST return HTTP 404 with JSON error body. `GetChallenge` MUST return 404 if challenge belongs to another user.
(Previously: only exposed ListChallenges and GetChallenge, no CreateChallenge)

#### Scenario: List all challenges — includes seeds + owned

- GIVEN 8 seeded challenges (user_id=NULL) and 2 user-owned challenges exist
- WHEN an authenticated user sends `GET /api/v1/challenges`
- THEN the response MUST be HTTP 200 with a JSON array of 10 challenges

#### Scenario: Get single challenge — public seed

- GIVEN seeded challenge 'ch-sqli' exists with `user_id=NULL`
- WHEN an authenticated user sends `GET /api/v1/challenges/ch-sqli`
- THEN the response MUST be HTTP 200 with the challenge as JSON

#### Scenario: Get challenge — other user's private returns 404

- GIVEN challenge 'ch-private' exists with `user_id='u2'`
- WHEN user 'u1' sends `GET /api/v1/challenges/ch-private`
- THEN the response MUST be HTTP 404

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

The system MUST update `ChallengeService` to instantiate `HttpChallengeRepository` instead of `MockChallengeRepository`. The `tempChallenges` Map MUST be removed entirely. `getChallenge(id)` MUST delegate directly to the repository without any Map lookup. `importChallenge(repoUrl, metadata)` MUST replace `addTempChallenge()` as the async import mechanism.
(Previously: preserved tempChallenges Map lookup before delegating to repository)

#### Scenario: Load challenges from HTTP

- GIVEN `ChallengeService` is initialized with `HttpChallengeRepository`
- WHEN `loadChallenges()` is called
- THEN it MUST fetch challenges from the backend API
- AND `challengesSignal` MUST be updated with the results

#### Scenario: All challenges from repository

- GIVEN no tempChallenges Map exists
- WHEN `getChallenge('ch-sqli')` is called
- THEN it MUST delegate to the HTTP repository

#### Scenario: Import replaces addTempChallenge

- GIVEN a user imports a challenge from Gogs
- WHEN `importChallenge()` completes
- THEN the challenge MUST be persisted via backend POST
- AND the challenges list MUST be reloaded

### Requirement: Backward Compatibility

The `Challenge` domain model in the frontend MAY add `sourceRepo?: string` optional field. The `ChallengeRepository` port interface MUST NOT change existing method signatures (`getAll()`, `getById(id)`). Existing 8 seed challenges MUST remain accessible to all authenticated users.
(Previously: Challenge model had no sourceRepo field)

#### Scenario: Domain model gains optional sourceRepo

- GIVEN the existing `Challenge` interface
- WHEN the migration is complete
- THEN `sourceRepo?: string` MUST be an optional field
- AND all existing fields and types MUST remain identical

#### Scenario: Port interface unchanged

- GIVEN the existing `ChallengeRepository` interface
- WHEN `HttpChallengeRepository` is implemented with `create()`
- THEN existing methods `getAll()` and `getById(id)` MUST satisfy the interface without signature changes

#### Scenario: Seed challenges remain accessible

- GIVEN 8 seed challenges exist with `user_id=NULL`
- WHEN any authenticated user calls `getAll()`
- THEN all 8 seed challenges MUST be included in the results

### Requirement: Database Schema — Ownership Columns

The system SHALL add `user_id UUID REFERENCES public.usuarios(id)` nullable and `source_repo TEXT` columns to `public.challenges` via migration `004_add_challenge_ownership.sql`. The system SHALL add an RLS INSERT policy for defense-in-depth. Existing 8 seed challenges SHALL remain with `user_id = NULL` and `source_repo = NULL`.

#### Scenario: Migration adds ownership columns

- GIVEN the `public.challenges` table exists with 8 seeded rows
- WHEN `004_add_challenge_ownership.sql` is executed
- THEN `user_id` and `source_repo` columns MUST exist
- AND all 8 existing rows MUST have `user_id = NULL` and `source_repo = NULL`

#### Scenario: RLS INSERT policy allows authenticated insert

- GIVEN a user is authenticated via JWT
- WHEN the user inserts a row into `public.challenges` with their `user_id`
- THEN the insert MUST succeed

#### Scenario: RLS INSERT policy rejects mismatched user_id

- GIVEN a user is authenticated with JWT claiming user_id='A'
- WHEN the user attempts to insert a row with `user_id = 'B'`
- THEN the insert MUST be rejected by RLS

### Requirement: Backend API — Create Challenge

The system MUST expose `POST /api/v1/challenges` that accepts JSON with: `title`, `description`, `difficulty`, `category`, `language`, `repo_url`, `source_repo`, `code`, `code_smell`. Status defaults to `'available'`. `user_id` MUST be extracted from JWT. If `source_repo + user_id` already exists, MUST return 200 with existing challenge. If new, MUST insert with UUID v4 and return 201. Without valid JWT, MUST return 401.

#### Scenario: Create new challenge

- GIVEN an authenticated user with valid JWT
- WHEN they POST to `/api/v1/challenges` with a unique `source_repo`
- THEN the response MUST be HTTP 201 with the created challenge JSON
- AND the challenge MUST have a backend-generated UUID v4 as `id`

#### Scenario: Duplicate import returns existing

- GIVEN a challenge already exists with `source_repo='ggogsmic/academy-mic'` and `user_id='user-1'`
- WHEN the same user POSTs with the same `source_repo`
- THEN the response MUST be HTTP 200 with the existing challenge
- AND no new row MUST be inserted

#### Scenario: Unauthenticated create rejected

- WHEN a request to `POST /api/v1/challenges` lacks a valid JWT
- THEN the response MUST be HTTP 401

#### Scenario: Invalid difficulty rejected

- WHEN a POST includes `difficulty='expert'`
- THEN the response MUST be HTTP 400 with `{"error": "message"}`

### Requirement: Backend Service — Create with Dedup

The system MUST implement `ChallengeService.Create(ctx, challenge, userID)` that normalizes `source_repo` (lowercase + trim) before comparison. If a match is found on `source_repo + user_id`, MUST return the existing challenge and `nil` error. If no match, MUST insert with `user_id` and `source_repo`.

#### Scenario: Dedup finds existing challenge

- GIVEN a challenge exists with `source_repo='GgogsMIC/academy-mic'` and `user_id='u1'`
- WHEN `Create` is called with `source_repo='  GgogsMIC/academy-mic  '` and `userID='u1'`
- THEN it MUST return the existing challenge and `nil` error

#### Scenario: Create inserts new challenge

- GIVEN no challenge exists with `source_repo='owner/repo'` and `user_id='u1'`
- WHEN `Create` is called with those values
- THEN it MUST insert a new row with `user_id` and `source_repo`
- AND return the created challenge with generated UUID

### Requirement: Backend Handler — CreateChallenge

The system MUST implement `CreateChallenge(w, r)` that decodes JSON body, calls `service.Create` with `userID` from context. On success (201), MUST encode the created challenge as JSON. On dedup (200), MUST encode the existing challenge as JSON. On validation error, MUST return 400 with `{"error": "message"}`.

#### Scenario: Handler returns 201 on new challenge

- GIVEN `service.Create` returns a new challenge with no error
- WHEN `CreateChallenge` is called
- THEN the response MUST be HTTP 201 with the challenge JSON

#### Scenario: Handler returns 200 on dedup

- GIVEN `service.Create` returns an existing challenge (dedup hit)
- WHEN `CreateChallenge` is called
- THEN the response MUST be HTTP 200 with the existing challenge JSON

#### Scenario: Handler returns 400 on invalid body

- GIVEN the request body is missing required field `title`
- WHEN `CreateChallenge` is called
- THEN the response MUST be HTTP 400 with `{"error": "message"}`

### Requirement: Frontend Repository Port — create()

The `ChallengeRepository` port MUST gain `create(challenge: CreateChallengeInput): Promise<Challenge>`. `CreateChallengeInput` is `Omit<Challenge, 'id' | 'createdAt' | 'status'>`.

#### Scenario: Port interface includes create

- GIVEN the `ChallengeRepository` interface
- WHEN a caller invokes `create(input)`
- THEN it MUST return `Promise<Challenge>`

### Requirement: Frontend HTTP Repository — create()

The system MUST implement `HttpChallengeRepository.create()` that POSTs to `/api/v1/challenges` with `Authorization: Bearer` header. MUST map frontend camelCase to backend snake_case in request body. MUST map backend snake_case response to camelCase Challenge. On network error, MUST throw.

#### Scenario: Create maps camelCase to snake_case

- GIVEN a `CreateChallengeInput` with `sourceRepo='owner/repo'`
- WHEN `create()` is called
- THEN the POST body MUST contain `source_repo: 'owner/repo'`

#### Scenario: Create maps response to camelCase

- GIVEN the backend returns `{user_id: 'u1', source_repo: 'owner/repo'}`
- WHEN `create()` receives the response
- THEN it MUST return a Challenge with `sourceRepo: 'owner/repo'`

#### Scenario: Network error throws

- GIVEN the backend is unreachable
- WHEN `create()` is called
- THEN it MUST throw an error

### Requirement: Frontend Service — importChallenge()

The system MUST implement `ChallengeService.importChallenge()` as async, returning `Promise<string>` (the challenge ID). This MUST replace `addTempChallenge()` and the `tempChallenges` Map MUST be removed entirely. `getChallenge(id)` MUST NOT check the Map anymore — only calls the repository. After successful import, MUST reload challenges list.

#### Scenario: Import returns challenge ID

- GIVEN a valid repo URL and metadata
- WHEN `importChallenge()` is called
- THEN it MUST POST to the backend and return the challenge ID string

#### Scenario: getChallenge no longer checks Map

- GIVEN `importChallenge()` has persisted a challenge
- WHEN `getChallenge(id)` is called
- THEN it MUST fetch from the repository only (no Map lookup)

#### Scenario: Challenges reload after import

- GIVEN `importChallenge()` succeeds
- WHEN the import completes
- THEN the challenges list MUST be reloaded from the repository
