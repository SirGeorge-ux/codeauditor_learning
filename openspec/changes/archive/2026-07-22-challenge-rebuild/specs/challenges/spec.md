# Delta for Challenges

> **Change:** challenge-rebuild (S1)
> **Modifies:** `openspec/specs/challenges/spec.md`

## ADDED Requirements

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

## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Backward Compatibility — sourceRepo optional field

(Reason: The v2 model replaces the entire Challenge interface; the sourceRepo optional field from the v1 migration is subsumed by the full v2 model which includes origin, source_repo, and source_path as first-class fields.)
(Migration: The `sourceRepo?: string` field is replaced by `origin`, `sourceRepo`, and `sourcePath` in the v2 Challenge interface.)
