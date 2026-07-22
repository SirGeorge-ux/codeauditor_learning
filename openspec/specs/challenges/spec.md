# Challenge Management Specification

## Purpose

Retrieve code-audit challenges from a PostgreSQL database via authenticated REST API, using a v2 Challenge model with JSONB fields for hints, expected findings, test cases, linter rules, and scoring metadata. Replaces hardcoded mock data and the v1 schema. Supports progressive hint disclosure, automated scoring, and context-only descriptions that avoid spoilers.

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

The system MUST provide an idempotent SQL seed file (`005_seed_challenges_v2.sql`) that replaces the 8 curated challenges with v2 data. The file MUST use `ON CONFLICT (id) DO UPDATE` for idempotency. Each challenge MUST include: full v2 description (context-only, no spoilers), 1-3 expected_findings, 2-4 test_cases, 1-3 linter_rules, 3 progressive hints, solution_code, and solution_explanation.
(Previously: seeded v1 challenges from MockChallengeRepository with code_smell field only)

#### Scenario: v2 seed replaces all 8 challenges

- GIVEN the `public.challenges` table has 8 v1 challenges
- WHEN `005_seed_challenges_v2.sql` is executed
- THEN all 8 challenges MUST be updated with v2 fields
- AND no duplicate rows are created

#### Scenario: v2 seed is idempotent

- GIVEN all 8 challenges already have v2 data
- WHEN `005_seed_challenges_v2.sql` is executed again
- THEN the data MUST remain unchanged (ON CONFLICT DO UPDATE with same values)

### Requirement: Backend Challenge Service

The system MUST update `ChallengeService` in `backend/internal/core/services/challenge_service.go` to parse v2 JSONB fields when reading challenges from PostgreSQL. The `GetByID` method MUST exclude `solution_code` and `solution_explanation` from the response unless explicitly requested with a `reveal_solution` flag (for tutor use only). The `Create` method MUST validate v2 model constraints (expected_findings 1-3, test_cases 2-4, exactly 3 hints).
(Previously: returned raw v1 columns without JSONB parsing or solution filtering)

#### Scenario: GetByID excludes solution by default

- GIVEN a v2 challenge exists with solution_code and solution_explanation
- WHEN `GetByID(ctx, "ch-sqli")` is called without reveal_solution
- THEN the returned challenge MUST have empty solution_code and solution_explanation

#### Scenario: Create validates v2 constraints

- GIVEN a challenge payload with 0 expected_findings
- WHEN `Create` is called
- THEN it MUST return a validation error

### Requirement: Backend Challenge Handler

The system MUST update `ChallengeHandler` responses to include v2 fields (`hints`, `expected_findings`, `test_cases`, `linter_rules`, `base_points`, etc.) in JSON responses. The `GET /api/v1/challenges/{id}` endpoint MUST accept an optional query parameter `?reveal_solution=true` that, when present AND the requesting user has Senior+ rank or is using the tutor context, includes `solution_code` and `solution_explanation` in the response.
(Previously: returned only v1 fields — title, description, difficulty, category, language, repo_url, code, code_smell)

#### Scenario: List returns v2 fields

- GIVEN 8 v2 challenges exist in the database
- WHEN `GET /api/v1/challenges` is called
- THEN the response MUST include hints, expected_findings, test_cases, linter_rules, base_points for each challenge
- AND solution_code and solution_explanation MUST be absent

#### Scenario: Reveal solution with valid permission

- GIVEN a Senior+ user requests `GET /api/v1/challenges/ch-sqli?reveal_solution=true`
- WHEN the request is processed
- THEN the response MUST include solution_code and solution_explanation

### Requirement: Route Registration

The system MUST register challenge routes under `/api/v1/challenges` in the Chi router within `main.go`, protected by `authmiddleware.AuthMiddleware`. The `ChallengeService` MUST be instantiated with the shared `*sql.DB` connection.

#### Scenario: Routes registered

- GIVEN the API server starts
- WHEN `GET /api/v1/challenges` is called with valid JWT
- THEN the request MUST reach the ChallengeHandler

### Requirement: Frontend HTTP Repository

The system MUST update `HttpChallengeRepository` to parse v2 JSONB response fields into the updated `Challenge` domain model. The repository MUST map backend snake_case fields (`expected_findings`, `test_cases`, `linter_rules`, `base_points`, `time_bonus`) to frontend camelCase (`expectedFindings`, `testCases`, `linterRules`, `basePoints`, `timeBonus`). The `getAll()` and `getById()` methods MUST handle null/missing v2 fields gracefully (default to empty arrays or zero values).
(Previously: mapped only v1 fields — codeSmell, repoUrl, sourceRepo)

#### Scenario: Parse v2 response with all fields

- GIVEN the backend returns a full v2 challenge JSON
- WHEN `getById("ch-sqli")` is called
- THEN the returned Challenge MUST have populated expectedFindings, testCases, linterRules, hints, basePoints

#### Scenario: Handle missing v2 fields gracefully

- GIVEN the backend returns a challenge with null test_cases
- WHEN the repository parses the response
- THEN testCases MUST default to an empty array `[]` (not null or undefined)

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
- WHEN `GetChallenge(id)` is called
- THEN it MUST fetch from the repository only (no Map lookup)

#### Scenario: Challenges reload after import

- GIVEN `importChallenge()` succeeds
- WHEN the import completes
- THEN the challenges list MUST be reloaded from the repository

### Requirement: Challenge model v2

The system MUST persist challenges with the v2 model, replacing the v1 schema. The `Challenge` struct/interface SHALL include: `id` (string), `title` (string), `description` (string — context only, no spoilers), `difficulty` ('junior'|'mid'|'senior'|'architect'), `language` (string), `category` ('security'|'performance'|'refactor'|'style'|'concurrency'|'architecture'), `learning_objectives` ([]string), `hints` ([]Hint), `common_mistakes` ([]string), `estimated_time_minutes` (int), `code` (string — vulnerable code), `expected_findings` ([]ExpectedFinding, 1-3 items), `test_cases` ([]TestCase, 2-4 items), `linter_rules` ([]LinterRule), `solution_code` (string), `solution_explanation` (string), `base_points` (int), `bonus_points` (int), `penalty_per_hint` (int), `time_bonus` (bool), `origin` ('curated'|'generated'|'imported'|'gogs'|'github'), `created_at` (timestamp), `created_by` (string).

#### Scenario: Import a v2 challenge

- GIVEN a valid v2 challenge payload with all required fields
- WHEN the challenge is persisted to PostgreSQL
- THEN all v2 fields are stored including JSONB arrays (hints, expected_findings, test_cases, linter_rules)
- AND solution_code and solution_explanation are persisted but NEVER returned to the frontend

#### Scenario: v2 model validation — expected_findings count

- GIVEN a challenge with 0 expected_findings
- WHEN the challenge is validated
- THEN validation MUST fail with "expected_findings must have 1-3 items"

#### Scenario: v2 model validation — test_cases count

- GIVEN a challenge with 1 test_case
- WHEN the challenge is validated
- THEN validation MUST fail with "test_cases must have 2-4 items"

#### Scenario: v2 model validation — hints count

- GIVEN a challenge with 4 hints
- WHEN the challenge is validated
- THEN validation MUST fail with "exactly 3 hints required"

### Requirement: Scoring engine

The system MUST implement a pure Go `ScoringService` in `backend/internal/core/services/scoring_service.go` with zero infrastructure imports. The score formula SHALL be: `score = base_points + (tests_passed × weight × 30) + (lint_clean × 20) + (findings_matched × 10) - (hints_used × cost) + time_bonus`. The service MUST accept an `AuditSession` and `Challenge` and return a `ScoreBreakdown` struct. Score MUST NOT be negative (floor at 0).

#### Scenario: Perfect solution scores maximum

- GIVEN a challenge with base_points=100, 2 test_cases (weight=5 each), 1 linter_rule, 2 expected_findings, time_bonus=true, no hints used
- WHEN the user passes all tests, all lint rules, matches all findings, and finishes under estimated_time
- THEN score = 100 + (2×5×30) + (1×20) + (2×10) + time_bonus = 470 + time_bonus

#### Scenario: Partial findings yield partial points

- GIVEN a challenge with 3 expected_findings
- WHEN the user's audit detects only 1 of 3 findings
- THEN findings_matched points = 1 × 10 = 10

#### Scenario: Zero tests yields zero test points

- GIVEN a challenge with 3 test_cases but the user's code passes 0
- WHEN score is calculated
- THEN test_points = 0 (not negative)

#### Scenario: All 3 hints used applies full penalty

- GIVEN a challenge with hints costing [0, 10, 25] points
- WHEN the user uses all 3 hints
- THEN hints_penalty = 0 + 10 + 25 = 35 points deducted

#### Scenario: Score floors at zero

- GIVEN base_points=50 and hints_penalty=80
- WHEN score is calculated
- THEN final score = 0 (not -30)

### Requirement: Description anti-spoiler rule

The system SHALL enforce that every challenge `description` provides context about the code's purpose and environment WITHOUT naming the code smell, vulnerability, or fix. The `description` MUST NOT contain: the smell name (e.g., "SQL injection", "XSS", "God Class"), the fix technique (e.g., "parametrized queries", "sanitize input"), or line numbers pointing to the issue.

#### Scenario: Description passes no-spoiler test

- GIVEN a challenge description for ch-sqli
- WHEN a developer reads only the description (not the code)
- THEN they cannot determine the vulnerability type from the description alone

#### Scenario: Spoiler-containing description rejected

- GIVEN a challenge description contains "SQL injection" or "parametrized queries"
- WHEN the challenge is validated
- THEN validation MUST fail with "description contains spoiler content"

### Requirement: Hint system

The system SHALL provide exactly 3 progressive hints per challenge. Hint level 1 SHALL be free (cost=0), level 2 SHALL cost 10 points, level 3 SHALL cost 25 points. Each hint MUST be progressively more revealing without giving away the solution. The `Hint` struct SHALL include: `level` (1|2|3), `content` (string), `cost_points` (int). Hints are revealed sequentially — level N+1 is unavailable until level N is consumed.

#### Scenario: First hint is free

- GIVEN a user requests hint level 1 on any challenge
- WHEN the hint is revealed
- THEN cost_points = 0 and no penalty is applied

#### Scenario: Sequential hint unlock

- GIVEN a user has NOT consumed hint level 1
- WHEN the user requests hint level 2
- THEN the system MUST reject the request with "hint 1 must be consumed first"

#### Scenario: Hint penalty applied to score

- GIVEN a user consumes hint level 2 (cost=10) and completes the audit
- WHEN the score is calculated
- THEN hints_penalty includes 10 points deducted

### Requirement: Delete MockChallengeRepository

The system MUST permanently delete `frontend/codeauditor/src/app/infrastructure/repositories/mock-challenge.repository.ts`. No file in the codebase SHALL import from this module. All challenge data MUST come from the HTTP API backed by PostgreSQL.

#### Scenario: Mock repository deleted

- GIVEN the codebase is built
- WHEN `mock-challenge.repository.ts` is searched in the repository
- THEN the file MUST NOT exist
- AND no import statement references it

### Requirement: DB migration — v2 JSONB columns

The system MUST add JSONB columns to `public.challenges` via migration: `learning_objectives` (JSONB array), `hints` (JSONB array), `common_mistakes` (JSONB array), `estimated_time_minutes` (INTEGER), `expected_findings` (JSONB array), `test_cases` (JSONB array), `linter_rules` (JSONB array), `solution_code` (TEXT), `solution_explanation` (TEXT), `base_points` (INTEGER DEFAULT 100), `bonus_points` (INTEGER DEFAULT 50), `penalty_per_hint` (INTEGER DEFAULT 10), `time_bonus` (BOOLEAN DEFAULT false), `origin` (TEXT DEFAULT 'curated'), `created_by` (TEXT). All new columns SHALL be nullable to support backward compatibility during migration.

#### Scenario: Migration adds v2 columns

- GIVEN the `public.challenges` table exists with v1 columns
- WHEN migration `005_add_v2_challenge_fields.sql` is executed
- THEN all v2 JSONB and scalar columns MUST exist
- AND existing v1 rows MUST have NULL for new columns (no data loss)
