# Design: Challenge Rebuild

## Architecture Overview

The system transitions from a static mock repository to a fully dynamic, objective scoring engine powered by PostgreSQL JSONB.
- **Database**: `public.challenges` stores challenge metadata and arrays (`hints`, `test_cases`, etc.) in JSONB columns.
- **Backend API**: `ChallengeService` loads challenges from the DB. `ScoringService` runs pure-Go logic comparing `AuditSession` output against `Challenge` v2 objective fields.
- **Frontend**: The `ChallengeService` exclusively uses `HttpChallengeRepository`. The `DojoPageComponent` receives the new fields to render hints progressively and display a score breakdown overlay.

## Architecture Decisions

### Decision: JSONB vs Normalized Tables
**Choice**: Store `hints`, `expected_findings`, `test_cases`, and `linter_rules` as JSONB columns in `public.challenges`.
**Alternatives considered**: Create separate normalized tables (`challenge_hints`, `challenge_test_cases`, etc.) with foreign keys.
**Rationale**: Challenges are document-like. Normalizing these arrays would require 4+ JOINS per fetch and complex insert logic, while JSONB maps cleanly to Go structs and TypeScript interfaces without any querying downside (we only read/write challenges as whole entities).

### Decision: Scoring Engine Design
**Choice**: `ScoringService` as a pure, stateless component in `backend/internal/core/services/`.
**Alternatives considered**: Calculate the score directly inside `AuditService`, or inside the Supabase adapter.
**Rationale**: By passing `Challenge` and `AuditSession` models into a pure function, we can use table-driven tests for every edge case of scoring without needing database mocks or sandbox environments.

### Decision: Frontend Repository Layer
**Choice**: Delete `mock-challenge.repository.ts` and rely strictly on HTTP + Database Seed.
**Alternatives considered**: Update the mock repository to match the new v2 schema.
**Rationale**: The mock repository adds maintenance overhead, code duplication, and masks API integration bugs. Seeding the local PostgreSQL is the standard approach for real-world scenarios.

### Decision: Hint Reveal UX
**Choice**: Progressive unlock.
**Alternatives considered**: All hints available immediately, or timed reveals.
**Rationale**: Progressive unlock ensures users only incur the penalty for hints they actively request, matching the pedagogical goal of trying without help first.

## Domain Model

### Go Domain Model (`backend/internal/core/domain/models/challenge_models.go`)
```go
package models
import "time"

type Hint struct {
	Level      int    `json:"level"`
	Content    string `json:"content"`
	CostPoints int    `json:"cost_points"`
}

type ExpectedFinding struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Message      string `json:"message"`
	Evidence     string `json:"evidence"`
	SuggestedFix string `json:"suggested_fix"`
}

type TestCase struct {
	Name           string `json:"name"`
	Input          any    `json:"input"`
	ExpectedOutput any    `json:"expected_output"`
	Weight         int    `json:"weight"`
}

type LinterRule struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// Challenge represents a code-audit challenge (v2).
type Challenge struct {
	ID                   string            `json:"id"`
	Title                string            `json:"title"`
	Description          string            `json:"description"`
	Difficulty           string            `json:"difficulty"`
	Category             string            `json:"category"`
	Language             string            `json:"language"`
	LearningObjectives   []string          `json:"learning_objectives"`
	Hints                []Hint            `json:"hints"`
	CommonMistakes       []string          `json:"common_mistakes"`
	EstimatedTimeMinutes int               `json:"estimated_time_minutes"`
	Code                 string            `json:"code"`
	ExpectedFindings     []ExpectedFinding `json:"expected_findings"`
	TestCases            []TestCase        `json:"test_cases"`
	LinterRules          []LinterRule      `json:"linter_rules"`
	SolutionCode         string            `json:"solution_code"`
	SolutionExplanation  string            `json:"solution_explanation"`
	BasePoints           int               `json:"base_points"`
	BonusPoints          int               `json:"bonus_points"`
	PenaltyPerHint       int               `json:"penalty_per_hint"`
	TimeBonus            bool              `json:"time_bonus"`
	Origin               string            `json:"origin"`
	SourceRepo           string            `json:"source_repo,omitempty"`
	SourcePath           string            `json:"source_path,omitempty"`
	GeneratedBy          string            `json:"generated_by,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	CreatedBy            string            `json:"created_by"`
}
```

### TypeScript Domain Model (`frontend/.../domain/models/challenge.ts`)
Matches the Go structs exactly. Removes deprecated `codeSmell` and `repoUrl`.

## Database Design

Migration (`004_add_challenge_v2_columns.sql`) using incremental columns:
```sql
ALTER TABLE public.challenges
  DROP COLUMN IF EXISTS code_smell,
  DROP COLUMN IF EXISTS repo_url,
  ADD COLUMN learning_objectives JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN hints JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN common_mistakes JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN estimated_time_minutes INT DEFAULT 15,
  ADD COLUMN expected_findings JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN test_cases JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN linter_rules JSONB DEFAULT '[]'::jsonb,
  ADD COLUMN solution_code TEXT DEFAULT '',
  ADD COLUMN solution_explanation TEXT DEFAULT '',
  ADD COLUMN base_points INT DEFAULT 100,
  ADD COLUMN bonus_points INT DEFAULT 50,
  ADD COLUMN penalty_per_hint INT DEFAULT 15,
  ADD COLUMN time_bonus BOOLEAN DEFAULT false,
  ADD COLUMN origin VARCHAR(50) DEFAULT 'curated',
  ADD COLUMN source_path VARCHAR(255),
  ADD COLUMN generated_by VARCHAR(50);
```

## Data Flow

    [Frontend Dojo] ──(submit code)──→ [AuditService]
          │                                  │
          │                            [Sandbox/Linter]
          │                                  │
          │                            [ScoringService]
          │                            (calculates score)
          │                                  │
          └─────(returns ScoreBreakdown)─────┘

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/.../models/challenge_models.go` | Modify | Update `Challenge` struct with JSONB nested structures. |
| `backend/.../services/scoring_service.go` | Create | Pure Go implementation of the score algorithm. |
| `backend/.../services/scoring_service_test.go` | Create | Table-driven tests covering scoring edges. |
| `frontend/.../domain/models/challenge.ts` | Modify | Align TS interface with the new v2 schema. |
| `frontend/.../repositories/mock-challenge.repository.ts` | Delete | Remove offline mock entirely. |
| `frontend/.../repositories/http-challenge.repository.ts` | Modify | Adapt mapping for new JSON properties. |
| `frontend/.../components/dojo/dojo-page.component.ts` | Modify | Integrate hints unlocking and breakdown modal overlay. |
| `db/migrations/004_add_challenge_v2_columns.sql` | Create | Schema definition for v2 properties. |
| `db/migrations/005_seed_challenges_v2.sql` | Create | UPSERT 8 curated challenges (spoilers removed). |

## Interfaces / Contracts

**Score Breakdown**:
```go
type ScoreBreakdown struct {
	BasePoints      int `json:"base_points"`
	TestPoints      int `json:"test_points"`      // Σ(test_passed × weight × 30)
	LintPoints      int `json:"lint_points"`      // Σ(lint_clean × 20)
	FindingsMatched int `json:"findings_matched"` // Σ(expected_finding_matched × 10)
	HintsPenalty    int `json:"hints_penalty"`    // -Σ(hint_used × cost)
	TimeBonus       int `json:"time_bonus"`
	Total           int `json:"total"`
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `ScoringService` | Table-driven tests feeding mock `AuditSession` and `Challenge` inputs. |
| Unit | `HttpChallengeRepository` | Check that v2 fields parse correctly from mock HTTP responses. |
| Integration | Postgres JSONB | Insert and read a full v2 Challenge through `ChallengeService`. |
| E2E | Dojo Flow | Complete a challenge, unlock a hint, submit, and assert the returned ScoreBreakdown in the UI. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

A phased approach during local deploy:
1. Run `004` ALTER TABLE migration.
2. Run `005` data seed `ON CONFLICT DO UPDATE`.
3. Drop `mock-challenge.repository.ts` dependencies from frontend DI providers.

## Open Questions

- [ ] How exactly do we match a user finding to an `ExpectedFinding`? String similarity, regex, or category/severity exact match?
