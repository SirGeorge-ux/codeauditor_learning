# Delta for Challenges (S4: code-health-dashboard)

> **Capability:** `challenges` (MODIFIED)
> **Change:** `code-health-dashboard` (S4)
> **Depends on:** `free-practice-mode` (S3, archived 2026-10-10), `challenge-rebuild` (S1, archived 2026-07-22).
> **This delta, on archive, lands at:** `openspec/specs/challenges/spec.md` (additive; existing v2 fields and orchestration unchanged).

## Purpose

The `challenges` capability gains two additive fields so the S4 code-health flow can persist challenges generated from real-repo issues: `GenerateChallengeHint.SourceIssueID` and the persisted `Challenge.origin="code_health"`. Both changes are backward-compatible — existing callers that omit the new fields continue to compile and behave exactly as in S3. No DB migration is required (the `origin` column was introduced in S1's `004_add_challenge_ownership.sql` and already accepts arbitrary values; `source_path` is the existing `source_repo` column reused for file-path semantics in this context).

## ADDED Requirements

### Requirement: REQ-CHALLENGES-CODE-HEALTH-001 — SourceIssueID on GenerateChallengeHint

The system SHALL extend `GenerateChallengeHint` with an optional `SourceIssueID string` field. When present, it MUST be persisted on the resulting `Challenge` row as `sourceIssueID`. When absent (zero value), behavior MUST be identical to S3.

#### Scenario: SourceIssueID is persisted when provided

- GIVEN the S4 code-health flow calls `ChallengeService.GenerateChallenge(ctx, userID, GenerateChallengeHint{SourceIssueID: "issue-3", Language: "typescript", Difficulty: "mid"})`
- WHEN the system persists the resulting row
- THEN the row carries `sourceIssueID="issue-3"`, `origin="code_health"`, `language="typescript"`, `difficulty="mid"`
- AND the new `challenge_id` is returned to the code-health flow

#### Scenario: SourceIssueID empty preserves S3 behavior

- GIVEN the S3 free-practice flow calls `ChallengeService.GenerateChallenge(ctx, userID, GenerateChallengeHint{TopicID: "inheritance", Level: "D"})` (no `SourceIssueID`)
- WHEN the system persists
- THEN `sourceIssueID` is empty (`""`), `origin="generated"`, and behavior is identical to S3

### Requirement: REQ-CHALLENGES-CODE-HEALTH-002 — Origin="code_health" Persistence

The system SHALL persist challenges generated from code-health issues with `origin="code_health"` and `GeneratedBy="code-health-v1"`. The `sourcePath` column SHALL equal the issue's `file_path` (from the parent analysis row). The dojo MUST open these challenges identically to other origins.

#### Scenario: Origin and sourcePath set from issue

- GIVEN a code-health issue on file `src/api/auth.ts` line `14` with `rule="RULE-SEC-SECRET-001"`
- WHEN the system persists the generated challenge
- THEN `origin="code_health"`, `GeneratedBy="code-health-v1"`, `sourcePath="src/api/auth.ts"`, `createdBy=<authenticated user id>`

#### Scenario: Code-health challenges list in the dojo

- GIVEN 3 challenges with `origin="code_health"` exist for the user
- WHEN the user navigates to `/dojo`
- THEN the dojo listing includes the 3 challenges tagged with `language`, `category` (`security` in this case), and a "from your code" badge

#### Scenario: Sandbox gate still runs

- GIVEN the LLM emits a `solutionCode` that fails the sandbox gate on the first 2 attempts
- WHEN the 3rd attempt passes after the prompt is augmented with prior errors
- THEN the challenge is persisted (same as S3 free-practice); `sourcePath` and `origin` are set before persist

#### Scenario: Sandbox gate fails 3 times

- GIVEN three consecutive sandbox runs fail
- WHEN the system reaches the retry ceiling
- THEN `ErrChallengeGenerationFailedSandbox` is returned and no row is persisted (same as S3)

### Requirement: REQ-CHALLENGES-CODE-HEALTH-003 — Reuse of S3 Generation Pipeline

The system SHALL NOT introduce a new orchestrator for code-health-generated challenges. The S3 un-stubbed `ChallengeService.GenerateChallenge` orchestration (curriculum → pedagogical → LLM cascade → sandbox gate → persist) SHALL handle both `origin="generated"` and `origin="code_health"` flows identically, with the only divergence being the persistence payload (`SourceIssueID`, `sourcePath`, `origin`).

#### Scenario: Same orchestrator handles both origins

- GIVEN two calls to `GenerateChallenge`: one with `TopicID="inheritance", Level="D"` (S3) and one with `SourceIssueID="issue-3"` (S4)
- WHEN the system runs both
- THEN both share the same LLM + sandbox + validation pipeline
- AND the only delta is the persisted `sourceIssueID`, `sourcePath`, and `origin` fields

## Migration Notes

- **No DB migration required.** The `origin` and `source_repo` columns already exist (S1 migration `004_add_challenge_ownership.sql`); both accept arbitrary values. `sourcePath` reuses the `source_repo` column with file-path semantics. A new optional column `source_issue_id TEXT` MAY be added in a follow-up migration if the analytics layer needs to index code-health challenges by their parent issue — out of scope for S4.
- **Backward compatibility:** all callers that omit `SourceIssueID` continue to compile (zero-value default) and behave as in S3.
- **Rollback:** removing the S4 wiring in `challenge_service.go` (drop the `SourceIssueID` mapping in the persist step) reverts this delta without affecting S3.