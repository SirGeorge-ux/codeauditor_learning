# Code Health Specification

> **Capability:** `code-health` (NEW)
> **Change:** `code-health-dashboard` (S4)
> **Depends on:** `mcp-monolith` (S2), `tutor-chat` (S2), `free-practice-mode` (S3), `challenge-rebuild` (S1) — all archived.
> **This spec lands at:** `openspec/specs/code-health/spec.md` on archive.

## Purpose

The `code-health` capability analyzes a user-owned repository (Gogs or GitHub) and produces a ranked health dashboard: a 0–100 health score, detected issues across 5 categories (security / performance / refactor / style / architecture), a per-language breakdown, and a "generate challenges from these" CTA that converts issues into dojo challenges with `origin="code_health"`. The capability reuses the `ExpectedFinding` shape from `challenge-rebuild` (1:1 field mapping) and the un-stubbed `ChallengeService.GenerateChallenge` orchestration from `free-practice-mode` (S3).

The change is additive: curated and free-practice modes are unaffected. Rollback drops the `code_health_*` DB tables, unregisters the 4 new MCP tools, and removes the `/repo/:owner/:name/code-health` route.

## Requirements

### Requirement: REQ-CODE-HEALTH-001 — Import Repository from Gogs

The system SHALL accept a Gogs repo descriptor `{owner, name, branch}` plus a session-resolved Gogs token, fetch the file tree via `gogsClient.ListFiles(owner, name, branch)`, and reject non-source / oversize files. The list SHALL be capped at 300 entries with a 1 MB per-file cap.

#### Scenario: Happy-path Gogs import

- GIVEN the user pastes `gogs://ggogsmic/academy-mic` on branch `master`
- WHEN the system resolves the Gogs token from the session and calls `ListFiles`
- THEN it returns the repo tree, filters out `node_modules/`, `.git/`, `dist/`, `vendor/`, binaries, and any file >1 MB
- AND the source-file count is ≤300 (warning if exceeded)

#### Scenario: Repo with >300 source files

- GIVEN the filtered source-file count is 412
- WHEN the system prepares the analysis
- THEN it returns a user-facing warning: "Repo exceeds 300 files; the first 300 will be analyzed."
- AND offers an "Analyze full repo" confirmation that overrides the cap

#### Scenario: Empty repo

- GIVEN the repo contains no source files after filtering
- WHEN the system returns the import result
- THEN it returns `{sourceFiles: [], skipped: 0, reason: "No source files"}` and the UI surfaces a localized "empty repo" message

### Requirement: REQ-CODE-HEALTH-002 — Import Repository from GitHub

The system SHALL accept a GitHub repo descriptor `{owner, name, branch}` plus an app-wide `GITHUB_TOKEN` env var, fetch the file tree via the new `githubClient.ListFiles`, and translate GitHub-specific errors (`404`, `403 rate-limit`) into user-facing messages.

#### Scenario: Happy-path GitHub import

- GIVEN `GITHUB_TOKEN` env is set and the user submits `github.com/anomalyco/codeauditor`
- WHEN the system calls `githubClient.ListFiles("anomalyco", "codeauditor", "master")`
- THEN it returns the same shape as Gogs: `[]FileEntry{Path, Size, SHA}`
- AND the analysis pipeline runs identically

#### Scenario: Private or missing repo (404)

- GIVEN the user submits `github.com/anomalyco/secret-repo` and it does not exist or is private to the user
- WHEN the GitHub API responds `404 Not Found`
- THEN the system returns a user-facing error "Repo not found / not accessible" with HTTP 404
- AND does NOT start an analysis

#### Scenario: GitHub rate limit

- GIVEN the GitHub API responds `403` and `x-ratelimit-remaining: 0`
- WHEN the system parses the response
- THEN it returns `ErrGitHubRateLimited` (HTTP 429) and the UI surfaces "GitHub rate limit hit, retry in <reset> seconds"

### Requirement: REQ-CODE-HEALTH-003 — Async Analysis Job

The system SHALL run the analysis in a background goroutine and return an `analysis_id` immediately. The persisted analysis row SHALL carry a status enum `pending|running|completed|failed`. Clients poll `mcp-code-health.get_analysis(analysis_id)` to read transitions.

#### Scenario: Submit returns analysis_id synchronously

- GIVEN the user invokes `mcp-code-health.analyze_repo(owner, name, "gogs", branch)`
- WHEN the tool validates input and inserts the analysis row with `status=pending`
- THEN the response is `{analysis_id}` returned within 500ms
- AND a goroutine picks the row up and transitions it `pending → running`

#### Scenario: Polling sees status transitions

- GIVEN an analysis with `analysis_id="a-..."` is in `status=running`
- WHEN the client polls `get_analysis("a-...")`
- THEN the response includes `status`, `health_score` (null until completed), `top_issues` (null until completed), and `updated_at`

#### Scenario: Failed analysis

- GIVEN the goroutine encounters an unrecoverable error (e.g., Gogs API down for >60s)
- WHEN the goroutine writes back
- THEN `status=failed` and `error_message` is non-empty
- AND the UI surfaces the failure with a "Retry" CTA

### Requirement: REQ-CODE-HEALTH-004 — Health Score Formula

The system SHALL compute the health score as `max(0, min(100, 100 − Σ(weight_i × count_i)))` where weights are `critical=15, high=8, medium=3, low=1`. The score MUST be persisted on every analysis row and exposed on the `get_analysis` response.

#### Scenario: Zero issues yields 100

- GIVEN an analysis with no detected issues
- WHEN the system computes the health score
- THEN `health_score = 100` and is persisted

#### Scenario: Mixed severity score

- GIVEN an analysis with 2 critical, 3 high, 5 medium, 10 low issues
- WHEN the system computes `100 − (2×15 + 3×8 + 5×3 + 10×1)` = `100 − 89` = `11`
- THEN `health_score = 11`

#### Scenario: Score clamps at 0

- GIVEN an analysis with 20 critical issues (= 300 weight)
- WHEN the system computes `100 − 300` = `-200`
- THEN `health_score = 0` (clamped, never negative)

### Requirement: REQ-CODE-HEALTH-005 — Ranked Issues

The system SHALL sort detected issues by `(severity_weight DESC, line_number ASC, category ASC)`. Severity weights follow REQ-CODE-HEALTH-004. The top 10 MUST be exposed as `top_issues`; the full ranked list MUST be queryable via `mcp-code-health.get_repo_issues`.

#### Scenario: Two critical issues ordered by line

- GIVEN two `critical` issues at lines `42` and `17`
- WHEN the system ranks
- THEN line `17` appears first (lower line number wins)

#### Scenario: Same severity, same line, ordered by category

- GIVEN two `high` issues at the same line, one `security` and one `style`
- WHEN the system ranks
- THEN `security` appears before `style` (alphabetical category tie-break)

### Requirement: REQ-CODE-HEALTH-006 — Issue Categories

The system SHALL classify every detected issue into one of exactly 5 categories: `security`, `performance`, `refactor`, `style`, `architecture`. Every rule in the registry MUST declare its category + severity + weight.

#### Scenario: All categories covered by seeded rules

- GIVEN the seeded rules registry
- WHEN the system enumerates category coverage
- THEN every one of the 5 categories has at least 1 rule assigned

#### Scenario: Unknown category rejected

- GIVEN a rule declares `category="concurrency"`
- WHEN the rule is registered
- THEN the registry returns `ErrCodeHealthInvalidCategory` and the rule is not loaded

### Requirement: REQ-CODE-HEALTH-007 — Rules Registry Seed

The system SHALL seed exactly 6 rules across the 5 categories, each with a stable ID, category, severity, weight, and detection function. Seeded rule IDs follow the format `RULE-<CAT>-<NAME>-<NNN>`.

| Rule ID | Category | Severity | Weight | Detection |
|---|---|---|---|---|
| `RULE-SEC-SECRET-001` | security | critical | 15 | Hardcoded secret regex (`(?i)(api[_-]?key\|password\|token)\s*=\s*['"]`) |
| `RULE-SEC-INJECT-001` | security | critical | 15 | `eval(` / `exec(` / SQL string concat in dynamic calls |
| `RULE-PERF-N1-001` | performance | high | 8 | Loop calling DB / ORM inside its body |
| `RULE-REFAC-GOD-001` | refactor | medium | 3 | Method with >50 LOC or class with >10 public methods |
| `RULE-STYLE-MAGIC-001` | style | low | 1 | Numeric literal >2 digits outside `const`/`I18n` |
| `RULE-ARCH-LEAK-001` | architecture | high | 8 | Direct dependency on another module's internals (`internal/`) |

#### Scenario: Registry returns 6 rules

- GIVEN the seeded registry is initialized at boot
- WHEN the system enumerates `Rules()`
- THEN the list has 6 items with non-empty ID, category, severity, weight

#### Scenario: Adding a 7th rule

- GIVEN a developer adds `add-code-health-rule.harness.md`-shaped rule for a new smell
- WHEN the new file is dropped under `rules/` and the registry is rebuilt
- THEN the count is 7 and the new rule is queryable

### Requirement: REQ-CODE-HEALTH-008 — LLM Verification Path

The system SHALL run regex rules first as a fast path, and SHALL invoke the LLM ONLY for ambiguous cases (regex weak-matches). For files ≤50 KB the LLM SHALL be the Ollama `qwen2.5-coder:3b` model; for files >50 KB it SHALL be DeepSeek V3 via OpenRouter. Every LLM-emitted finding SHALL carry a `confidence` score in `[0.0, 1.0]`.

#### Scenario: Regex match skips LLM

- GIVEN a file contains `api_key = "abc123"`
- WHEN the analyzer runs
- THEN `RULE-SEC-SECRET-001` matches via regex with `confidence=1.0` and no LLM call is made

#### Scenario: Ambiguous case invokes LLM

- GIVEN a file has 5 dynamic `exec` calls but only one is in user-input context (the others are constants)
- WHEN the regex match fires weakly
- THEN the LLM is called with the file content + rule prompt
- AND the returned issue carries a `confidence` (e.g., `0.82`)

#### Scenario: User marks "not an issue" invalidates the finding

- GIVEN the user clicks "Not an issue" on a finding with `rule=RULE-SEC-INJECT-001` and `file_path=src/foo.py`
- WHEN the feedback is persisted
- THEN the next analysis for the same `file_path + rule + line` is suppressed
- AND a `suppressions` audit row is written

### Requirement: REQ-CODE-HEALTH-009 — Analysis Cache

The system SHALL cache per-file analysis results in an `analysis_cache` table keyed `analysis:<file_sha>` (the file's git SHA, not its path) and SHALL evict entries older than 7 days. The "Re-analyze" CTA SHALL bypass the cache for that repo only.

#### Scenario: Cache hit returns prior result

- GIVEN the cache has an entry at `analysis:<sha>` with TTL 5 days remaining
- WHEN the analyzer processes a file whose SHA matches
- THEN the cached result is returned and no LLM call is made
- AND `cache_hits` increments in the run log

#### Scenario: Cache miss stores new result

- GIVEN the cache has no entry for `analysis:<sha>` of the new file
- WHEN the analyzer finishes
- THEN the result is written with `expires_at = now() + 7 days`

#### Scenario: Re-analyze CTA bypasses cache

- GIVEN the user clicks "Re-analyze" on a previously analyzed repo
- WHEN the analysis job starts
- THEN every file in that repo is re-analyzed regardless of cache presence
- AND the cache is overwritten (not appended) with fresh results

### Requirement: REQ-CODE-HEALTH-010 — Generate Challenge from Issue

The system SHALL expose `mcp-code-health.generate_health_report(analysis_id, format)` (or a dedicated `generate_challenge_from_issue` tool) that takes an `analysis_id` and an `issue_id`, and SHALL call `ChallengeService.GenerateChallenge(ctx, userID, GenerateChallengeHint{SourceIssueID: issue_id, Language: ..., Difficulty: ..., Origin: "code_health"})`. The persisted `Challenge` row SHALL carry `sourcePath` set to the issue's `file_path`.

#### Scenario: Generate challenge from a critical issue

- GIVEN an analysis with `analysis_id="a-..."` and a critical `RULE-SEC-SECRET-001` issue on `src/api/auth.ts:14`
- WHEN the user invokes `generate_challenge_from_issue(analysis_id, issue_id, language="typescript", difficulty="mid")`
- THEN the system calls `ChallengeService.GenerateChallenge` (S3 un-stubbed path)
- AND the persisted row has `origin="code_health"`, `sourcePath="src/api/auth.ts"`, `language="typescript"`, `difficulty="mid"`
- AND the response includes the new `challenge_id` and the dojo URL `/dojo/<challenge_id>`

#### Scenario: Reuse of S3 generation pipeline

- GIVEN the LLM-generated `solutionCode` passes the sandbox gate `ExecuteWithTests` on the first run
- WHEN the system persists
- THEN no new orchestrator is involved; S3's `validateChallengeV2` + `sandboxGate.ExecuteWithTests` runs unchanged

#### Scenario: Sandbox gate fails 3 times

- GIVEN three consecutive sandbox runs fail for the generated challenge
- WHEN the system reaches the retry ceiling
- THEN it returns `ErrChallengeGenerationFailedSandbox` (same as S3) and no row is persisted

### Requirement: REQ-CODE-HEALTH-011 — Per-User Daily Analysis Cap

The system SHALL enforce a per-user soft cap of 5 analyses per UTC day, tracked client-side in `localStorage` and surfaced in the UI as "remaining today". The server SHALL additionally return HTTP 429 with `Retry-After` if the cap is exceeded (defense-in-depth).

#### Scenario: Under-cap user can analyze

- GIVEN a user has run 2 analyses today
- WHEN the user submits a 3rd analysis
- THEN the request proceeds and the remaining counter shows 2

#### Scenario: At-cap user is rejected

- GIVEN a user has run 5 analyses today
- WHEN the user submits a 6th analysis
- THEN the server returns HTTP 429 with `Retry-After: <seconds until UTC midnight>`
- AND the UI shows a "Daily limit reached, resets at 00:00 UTC" notice

#### Scenario: Counter resets at UTC midnight

- GIVEN a user ran 5 analyses yesterday
- WHEN the user opens the page after UTC midnight
- THEN the counter shows 5 remaining

### Requirement: REQ-CODE-HEALTH-012 — Health-Score History

The system SHALL persist every completed analysis in an append-only `code_health_analyses` history table. The dashboard SHALL display the last 10 analyses per repo, sorted by `created_at DESC`.

#### Scenario: New analysis appends to history

- GIVEN a user runs a 2nd analysis on `ggogsmic/academy-mic`
- WHEN the analysis completes
- THEN a new row is appended (no UPDATE on the prior row) with `health_score`, `top_issues`, `languages`, `created_at`

#### Scenario: Last 10 analyses returned per repo

- GIVEN the user has 15 analyses for the same repo
- WHEN the dashboard calls `get_repo_history(owner, name, "gogs")`
- THEN the response is the 10 most recent analyses, sorted by `created_at DESC`

### Requirement: REQ-CODE-HEALTH-013 — Authentication on All Endpoints

Every `mcp-code-health.*` tool and every `/api/v1/code-health/*` endpoint SHALL require an authenticated `user_id` in context. Missing or invalid auth SHALL return HTTP 401 on REST endpoints and JSON-RPC code `-32602` on MCP tool calls.

#### Scenario: Authenticated tool call succeeds

- GIVEN a valid JWT for user `ggogsmic`
- WHEN the user invokes any `mcp-code-health.*` tool
- THEN `ctx.Value("user_id")` returns `ggogsmic`
- AND the tool executes and persists under that user

#### Scenario: Missing user_id is rejected

- GIVEN a request without a valid JWT to `/api/v1/code-health/analyses`
- WHEN the request reaches the handler
- THEN it returns HTTP 401 with `{"error":"unauthorized"}`

#### Scenario: Missing user_id in MCP tool call

- GIVEN a `mcp-code-health.analyze_repo` call without a JWT
- WHEN the MCP layer validates context
- THEN it returns JSON-RPC `{"error":{"code":-32602,"message":"user_id required"}}`

## Out of Scope

The following are explicitly **not** delivered by `code-health-dashboard` (S4):

- **GitHub OAuth flow** — MVP uses app-wide `GITHUB_TOKEN` env (PAT). Per-user OAuth is deferred to S6+.
- **`mcp-inspiration` (Exercism + GitHub education repos)** — deferred since S2; remains S3+ follow-up.
- **Server-side daily cap** — S4 enforces client-side only; S5 (`llm-cascade`) generalizes the cascade and owns the server-side cap.
- **REST `/api/v1/code-health/*` endpoints** — MCP-only is the S4 transport; the frontend consumes exclusively via `MCPClientAdapter`. The REST surface (and its HTTP 429 `Retry-After` daily-cap response) lands in S5+.
- **Multi-user sharing, leaderboards, public reports** — single-user dev (`ggogsmic`).
- **Curriculum authoring UI for code-health topics** — S6+.
- **Inline / snapshot rule rendering in the dojo** — independent of S4.
- **Auto-fix / patch generation** — read-only analysis mode.
- **Real-time push of analysis results (SSE / WebSocket)** — polling is sufficient for the 30s–2min cadence.