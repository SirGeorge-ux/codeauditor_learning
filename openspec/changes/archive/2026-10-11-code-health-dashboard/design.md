# Design: Code Health Dashboard (S4)

## Overview

S4 ships the third practice mode: the user imports a real repo (Gogs or GitHub) and the backend produces an async health dashboard — a 0–100 health score, ranked issues across 5 categories (`security` / `performance` / `refactor` / `style` / `architecture`), a per-language breakdown, and a "generate challenges from these" CTA that reuses the S3 generation pipeline to turn issues into dojo challenges with `origin="code_health"`. The design adds a `code_health` core package (service + analyzer + 6 seeded rules), a `SourceClient` port with Gogs + GitHub adapters, a DB-backed `AnalysisCache`, a Supabase migration (`010_`), the 9th MCP group (`mcp-code-health`, 4 tools), and a new `/repo/:owner/:name/code-health` Angular route. Regex rules run first (fast path, `confidence=1.0`, no LLM); the LLM (Ollama `qwen2.5-coder:3b` ≤50 KB, DeepSeek V3 >50 KB) verifies only ambiguous cases. The change is additive — curated and free-practice modes are untouched.

## Architecture diagram

```
Frontend (Angular 21, standalone + signals)
  /repo/:owner/:name/code-health → CodeHealthPageComponent
        │  CodeHealthPort (delegates to MCPClientPort.callTool)
        ▼
  MCP monolith (mcpkit: JSON-RPC 2.0, JWT, rate-limit, 30s timeout)
        │  mcp-code-health.{analyze_repo, get_analysis, get_repo_issues, generate_health_report}
        ▼
  CodeHealthService (core/services/code_health)
        ├─ SourceClient ───────────▶ GogsClient / GitHubClient     (driven, outbound HTTP)  [sync]
        ├─ AnalysisCache ──────────▶ Supabase analysis_cache       (driven, DB)             [sync]
        ├─ CodeHealthRepository ───▶ Supabase code_health_*        (driven, DB)             [sync]
        ├─ OllamaGenerator ────────▶ ollama.Client (≤50KB lane)                             [sync LLM]
        └─ LLMClientPort ──────────▶ llm_cascade (DeepSeek V3, >50KB lane)                  [sync LLM]
        └─ (generate-from-issue) ──▶ ChallengeService.GenerateChallenge (S3) ──▶ sandbox gate

  ┌─ async boundary ─────────────────────────────────────────────────────┐
  │  analyze_repo → INSERT pending → return analysis_id (<500ms)         │
  │  goroutine: pending→running → analyze() sync core → completed|failed │
  └──────────────────────────────────────────────────────────────────────┘
```

The only async boundary is the analysis job: everything else (source fetch, cache read/write, per-file analysis, persistence) is synchronous inside the goroutine. There is no worker queue — a single goroutine per job is correct for a single-user app.

## Module / package layout

### PR 1 — Backend foundation

| Path | Action | Purpose |
|---|---|---|
| `backend/internal/core/domain/models/code_health_analysis.go` | Create | `CodeHealthAnalysis`, `CodeHealthStatus` (`pending|running|completed|failed`) |
| `backend/internal/core/domain/models/code_health_issue.go` | Create | `CodeHealthIssue` (mirrors `ExpectedFinding` + `rule`/`confidence`/`sha`/`line`) |
| `backend/internal/core/domain/models/health_score.go` | Create | `Severity`, `IssueCategory`, `Weight(sev)`, `HealthScore(...)`, `RankIssues(...)` |
| `backend/internal/ports/code_health.go` | Create | `SourceClient`, `FileEntry`, `FileContent`, `CodeHealthRepository`, `AnalysisCache`, `OllamaGenerator` |
| `backend/internal/core/services/code_health/service.go` | Create | `CodeHealthService`, option funcs, `AnalyzeRepo` (async) + `analyze` (sync), `GetAnalysis`, `GetRepoIssues`, `GenerateHealthReport` |
| `backend/internal/core/services/code_health/analyzer.go` | Create | per-file pipeline: filter → cache → regex → LLM → cache write |
| `backend/internal/core/services/code_health/rules/registry.go` | Create | `Rule` struct + `Registry` (Register/Rules/category-coverage) |
| `backend/internal/core/services/code_health/rules/secret.go` | Create | `RULE-SEC-SECRET-001` |
| `backend/internal/core/services/code_health/rules/injection.go` | Create | `RULE-SEC-INJECT-001` |
| `backend/internal/core/services/code_health/rules/n_plus_one.go` | Create | `RULE-PERF-N1-001` |
| `backend/internal/core/services/code_health/rules/god_method.go` | Create | `RULE-REFAC-GOD-001` |
| `backend/internal/core/services/code_health/rules/magic_number.go` | Create | `RULE-STYLE-MAGIC-001` |
| `backend/internal/core/services/code_health/rules/leaky_abstraction.go` | Create | `RULE-ARCH-LEAK-001` |
| `backend/internal/infrastructure/driven/github/github_client.go` | Create | `GitHubClient` mirroring `gogs_client.go` (`ListFiles` + `GetFileContent`) |
| `backend/internal/infrastructure/driven/gogs/gogs_client.go` | Modify | add `ListFiles(ctx, owner, repo, branch) ([]FileEntry, error)` |
| `backend/internal/infrastructure/driven/supabase/migrations/010_create_code_health.sql` | Create | 3 tables (`code_health_analyses`, `code_health_issues`, `analysis_cache`) + indexes + RLS |
| `backend/internal/infrastructure/driven/supabase/code_health_repository.go` | Create | `SupabaseCodeHealthRepository` (implements `ports.CodeHealthRepository`) |
| `backend/internal/infrastructure/driven/supabase/analysis_cache_repository.go` | Create | `SupabaseAnalysisCache` (implements `ports.AnalysisCache`) |
| `backend/cmd/api/main.go` | Modify | read `GITHUB_TOKEN`, construct `GitHubClient`/`AnalysisCache`/`CodeHealthRepository`/`CodeHealthService` |

### PR 2 — MCP group + challenges wiring

| Path | Action | Purpose |
|---|---|---|
| `backend/internal/infrastructure/mcp/code_health/adapter.go` | Create | local `Service` interface + `NewServiceAdapter(*services.CodeHealthService)` |
| `backend/internal/infrastructure/mcp/code_health/analyze_repo.go` + `_test.go` | Create | `mcp-code-health.analyze_repo` |
| `backend/internal/infrastructure/mcp/code_health/get_analysis.go` + `_test.go` | Create | `mcp-code-health.get_analysis` |
| `backend/internal/infrastructure/mcp/code_health/get_repo_issues.go` + `_test.go` | Create | `mcp-code-health.get_repo_issues` |
| `backend/internal/infrastructure/mcp/code_health/generate_health_report.go` + `_test.go` | Create | `mcp-code-health.generate_health_report` |
| `backend/cmd/api/wiring.go` | Modify | add `CodeHealthService` to `WiringDeps`, register 4 tools in `BuildMCPRegistry` |
| `backend/internal/core/domain/models/challenge_models.go` | Modify | add `SourceIssueID string` to `GenerateChallengeHint` (additive, omitempty) |
| `backend/internal/core/services/challenge_service.go` | Modify | `GenerateChallenge` branches on `SourceIssueID`: `Origin="code_health"`, `GeneratedBy="code-health-v1"`, `SourcePath=issue.file_path` |
| `backend/internal/infrastructure/mcp/challenges/adapter.go` + `challenges_tool.go` | Modify | map `sourceIssueId` wire arg into `GenerateChallengeHint.SourceIssueID` |

### PR 3 — Frontend dashboard

| Path | Action | Purpose |
|---|---|---|
| `frontend/.../domain/models/code-health.ts` | Create | `CodeHealthIssue`, `CodeHealthAnalysis`, severity/category/status/source unions |
| `frontend/.../domain/ports/code-health.port.ts` | Create | `CodeHealthPort` (analyzeRepo/getAnalysis/getRepoIssues/generateHealthReport) |
| `frontend/.../application/code-health.use-case.ts` | Create | signals + polling + daily cap (localStorage) |
| `frontend/.../infrastructure/repositories/code-health.repository.ts` | Create | `CodeHealthPort` impl delegating to `MCPClientPort.callTool` |
| `frontend/.../components/code-health/repo-url-bar.component.ts` | Create | dumb input |
| `frontend/.../components/code-health/import-button.component.ts` | Create | dumb button |
| `frontend/.../components/code-health/health-gauge.component.ts` | Create | dumb 0–100 gauge |
| `frontend/.../components/code-health/issues-table.component.ts` | Create | dumb ranked-issues table |
| `frontend/.../components/code-health/category-breakdown.component.ts` | Create | dumb 5-category bar |
| `frontend/.../components/code-health/code-health-page.component.ts` | Create | composes the 5 + CTA |
| `frontend/.../components/code-health/generate-challenges-cta.component.ts` | Create | "generate challenges from these" |
| `frontend/.../app.routes.ts` | Modify | `/repo/:owner/:name/code-health` |
| `frontend/.../components/dashboard/dashboard-page.component.ts` | Modify | code-health tile |

## Per-PR breakdown

### PR 1 — Backend foundation (~900 lines)

**Goal:** the analyzer + persistence + source clients + cache, fully unit-tested, no transport yet.

**Seeded rules (canonical, from spec REQ-CODE-HEALTH-007):**

| Rule ID | Category | Severity | Weight | Regex / detector |
|---|---|---|---|---|
| `RULE-SEC-SECRET-001` | security | critical | 15 | `(?i)(api[_-]?key|password|token|secret)\s*[:=]\s*['"][^'"]{4,}['"]` |
| `RULE-SEC-INJECT-001` | security | critical | 15 | `eval(\s*`, `exec(\s*`, or SQL string concat (`+` into a query) |
| `RULE-PERF-N1-001` | performance | high | 8 | loop body contains a DB/ORM call (`.Query(`, `.Find(`, `await.*Repository`) |
| `RULE-REFAC-GOD-001` | refactor | medium | 3 | method > 50 LOC, or class with > 10 public methods |
| `RULE-STYLE-MAGIC-001` | style | low | 1 | numeric literal with > 2 digits outside `const`/`readonly`/`enum` |
| `RULE-ARCH-LEAK-001` | architecture | high | 8 | import of another module's `internal/` package |

> **Note:** the proposal's rule list named `console-log-leak`; the spec's canonical table (REQ-CODE-HEALTH-007) replaces it with `RULE-PERF-N1-001` (N+1). The spec is authoritative — the 6 rules above ship.

**Migration `010_create_code_health.sql`** — see *Database schema* below.

**`CodeHealthService`** — the sync core is `analyze(ctx, analysisID, repo, force)` (unit-testable with fakes); the async wrapper `AnalyzeRepo` inserts a `pending` row, spawns a goroutine that flips `pending→running`, runs `analyze`, then `running→completed|failed`. `force` (Re-analyze) bypasses the cache read but still overwrites cache entries.

**GitHub client** — `NewGitHubClient(baseURL, token)` mirroring `gogs_client.go`; methods `ListFiles(ctx, owner, repo, branch)` and `GetFileContent(ctx, owner, repo, branch, path)`. `ListFiles` walks the git tree (`GET /repos/{owner}/{repo}/git/trees/{branch}?recursive=1`), filters to `type=blob`, returns `[]FileEntry{Path, Size, SHA}`. `GetFileContent` decodes base64 (GitHub returns base64, unlike Gogs). Error translation: `404 → ErrGitHubNotFound`, `403 + x-ratelimit-remaining:0 → ErrGitHubRateLimited` (429).

**Gogs `ListFiles`** — `GET /api/v1/repos/{owner}/{repo}/git/trees/{branch}?recursive=1` → same `[]FileEntry` shape.

### PR 2 — MCP group + S3 integration (~700 lines)

**4 tools** (wire shapes in *Wire shapes* below). Each is a `mcpkit.MCPTool` (`Name`/`Description`/`Schema`/`Execute`), delegates to `codeHealthService` via `NewServiceAdapter`, validates `user_id` in context, and never imports `database/sql`/`net/http`/GitHub/Gogs SDKs.

**`SourceIssueID` extension** — `GenerateChallengeHint` gains `SourceIssueID string \`json:"sourceIssueId,omitempty"\`` (additive, zero-value = S3 no-op). `ChallengeService.GenerateChallenge` branches: when `SourceIssueID != ""`, the persisted row uses `Origin="code_health"`, `GeneratedBy="code-health-v1"`, `SourcePath=<issue.file_path>`. The orchestration (LLM → sandbox ≤3 → validate → persist) is identical for both origins. `mcp-challenges.generate_challenge` gains the optional `sourceIssueId` wire arg.

**Smallest proof of value:** `curl mcp-code-health.analyze_repo({owner,name,source:"gogs"})` → poll `get_analysis` → `health_score` + `top_issues`.

### PR 3 — Frontend dashboard (~800 lines)

**`CodeHealthUseCase`** mirrors `PracticeUseCase`: signals (`analysis`, `loading`, `error`, `topIssues`, `languages`, `remainingToday`), command flow (`submitAnalysis` → `pollAnalysis` with exponential backoff 2s→30s), and a 5/day localStorage cap under `codeHealth.dailyCount.<UTC-date>`. Depends on `CodeHealthPort` + injected `Storage`/`now()` for testability. Components are dumb (inputs/outputs only); the page injects the use case via an `@Injectable` wrapper.

## Contracts (interfaces)

### Go — `backend/internal/ports/code_health.go`

```go
// SourceClient abstracts a git host's tree + blob API. GogsClient and
// GitHubClient both satisfy it; the service selects by source string.
type SourceClient interface {
    ListFiles(ctx context.Context, owner, repo, branch string) ([]FileEntry, error)
    GetFileContent(ctx context.Context, owner, repo, branch, path string) (*FileContent, error)
}

type FileEntry struct {
    Path string
    Size int64
    SHA  string
}

type FileContent struct {
    Path    string
    Content string // decoded text (base64-decoded for GitHub)
    SHA     string
    Size    int64
}

// CodeHealthRepository persists analyses + issues. Every read scopes by
// userID at the SQL layer (mirrors Dictionary/ChallengeRepository).
type CodeHealthRepository interface {
    InsertAnalysis(ctx context.Context, a *models.CodeHealthAnalysis) error
    UpdateStatus(ctx context.Context, id, userID, status, errMsg string) error
    Complete(ctx context.Context, id, userID string, score int,
        topIssues []models.CodeHealthIssue, languages map[string]float64, summary string) error
    FindByID(ctx context.Context, id, userID string) (models.CodeHealthAnalysis, error)
    ListHistory(ctx context.Context, owner, name, source, userID string, limit int) ([]models.CodeHealthAnalysis, error)
    CountSince(ctx context.Context, userID string, since time.Time) (int, error) // daily cap
    InsertIssue(ctx context.Context, i *models.CodeHealthIssue) error
    ListIssues(ctx context.Context, analysisID string, severity string, limit int) ([]models.CodeHealthIssue, error)
}

// AnalysisCache is the per-file cache, keyed analysis:<file_sha>,
// namespaced by user_id. DB-backed (Supabase), 7d TTL.
type AnalysisCache interface {
    Get(ctx context.Context, userID, fileSHA string) ([]models.CodeHealthIssue, bool, error)
    Set(ctx context.Context, userID, fileSHA string, issues []models.CodeHealthIssue, ttl time.Duration) error
}

// OllamaGenerator is the non-streaming JSON-mode Ollama contract for the
// ≤50KB lane. The audit-path OllamaClient port is streaming-only, so code
// health declares its own narrow port instead of bloating that contract.
// The driven ollama.Client already satisfies it (has Generate).
type OllamaGenerator interface {
    Generate(ctx context.Context, system, prompt string) (string, error)
}
```

### TS — `frontend/.../domain/ports/code-health.port.ts`

```ts
import { CodeHealthAnalysis, CodeHealthIssue, CodeHealthSeverity, CodeHealthSource } from '../models/code-health';

export interface AnalyzeRepoArgs {
  owner: string;
  name: string;
  source: CodeHealthSource;
  branch?: string;
  force?: boolean; // Re-analyze bypass
}

export interface CodeHealthPort {
  analyzeRepo(args: AnalyzeRepoArgs): Promise<{ analysis_id: string; status: 'pending' }>;
  getAnalysis(analysis_id: string): Promise<CodeHealthAnalysis>;
  getRepoIssues(args: {
    owner: string; name: string; source: CodeHealthSource;
    severity?: CodeHealthSeverity; limit?: number;
  }): Promise<{ issues: CodeHealthIssue[] }>;
  generateHealthReport(args: { analysis_id: string; format: 'md' | 'pdf' }):
    Promise<{ report_id: string; content: string; format: string }>;
}
```

## Wire shapes

Arg keys are **snake_case** (mirroring the S2 `docs/mcp-tools.md` §3.7 canonical table), unlike `mcp-challenges` which used camelCase. Return shapes carry snake_case keys matching the Go JSON tags.

```jsonc
// 1. analyze_repo — fire-and-forget
{ "name": "mcp-code-health.analyze_repo",
  "args": { "owner": "anomalyco", "name": "codeauditor", "source": "gogs", "branch": "master", "force": false } }
// → { "analysis_id": "a-...", "status": "pending" }

// 2. get_analysis — poll
{ "name": "mcp-code-health.get_analysis", "args": { "analysis_id": "a-..." } }
// → { "analysis_id": "a-...", "status": "completed", "health_score": 78,
//     "top_issues": [ ...10 ], "languages": { "go": 0.6, "ts": 0.3 },
//     "summary": "...", "created_at": "...", "completed_at": "...", "error_message": null }

// 3. get_repo_issues — filter + limit
{ "name": "mcp-code-health.get_repo_issues",
  "args": { "owner": "anomalyco", "name": "codeauditor", "source": "gogs",
            "severity": "critical", "limit": 50 } }
// → { "issues": [ { "id": "issue-3", "severity": "critical", "category": "security",
//     "file_path": "src/api/auth.ts", "line": 14, "message": "...", "evidence": "...",
//     "suggested_fix": "...", "rule": "RULE-SEC-SECRET-001", "confidence": 1.0 } ] }

// 4. generate_health_report — render artifact
{ "name": "mcp-code-health.generate_health_report", "args": { "analysis_id": "a-...", "format": "md" } }
// → { "report_id": "r-...", "content": "<markdown>", "format": "md" }
```

## Database schema (migration `010_create_code_health.sql`)

```sql
CREATE TABLE IF NOT EXISTS public.code_health_analyses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    source        TEXT NOT NULL CHECK (source IN ('gogs','github')),
    owner         TEXT NOT NULL,
    name          TEXT NOT NULL,
    branch        TEXT NOT NULL DEFAULT 'master',
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','running','completed','failed')),
    health_score  INTEGER,
    top_issues    JSONB NOT NULL DEFAULT '[]'::jsonb,
    languages     JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary       TEXT,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours')
);
-- append-only history: last 10 analyses per repo (REQ-CODE-HEALTH-012)
CREATE INDEX IF NOT EXISTS idx_code_health_analyses_history
    ON public.code_health_analyses (user_id, owner, name, source, created_at DESC);

CREATE TABLE IF NOT EXISTS public.code_health_issues (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id   UUID NOT NULL REFERENCES public.code_health_analyses(id) ON DELETE CASCADE,
    severity      TEXT NOT NULL CHECK (severity IN ('critical','high','medium','low')),
    category      TEXT NOT NULL CHECK (category IN ('security','performance','refactor','style','architecture')),
    file_path     TEXT NOT NULL,
    line          INTEGER,
    message       TEXT NOT NULL,
    evidence      TEXT,
    suggested_fix TEXT,
    rule          TEXT NOT NULL,
    confidence    REAL NOT NULL DEFAULT 1.0,
    sha           TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_code_health_issues_analysis
    ON public.code_health_issues (analysis_id);
CREATE INDEX IF NOT EXISTS idx_code_health_issues_severity
    ON public.code_health_issues (analysis_id, severity);

-- per-file cache, keyed (user_id, file_sha), 7d TTL (REQ-CODE-HEALTH-009)
CREATE TABLE IF NOT EXISTS public.analysis_cache (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES public.usuarios(id) ON DELETE CASCADE,
    file_sha   TEXT NOT NULL,
    result     JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    CONSTRAINT analysis_cache_user_sha_unique UNIQUE (user_id, file_sha)
);
CREATE INDEX IF NOT EXISTS idx_analysis_cache_expires_at ON public.analysis_cache (expires_at);
```

The 7d TTL is enforced on read (`WHERE expires_at > NOW()`) and by a periodic/opportunistic `DELETE ... WHERE expires_at < NOW()`. RLS policies mirror `009_create_dictionary_terms.sql` (SELECT/INSERT/UPDATE/DELETE scoped to `auth.uid() = user_id`) as defense-in-depth — the app's `database/sql` pool already scopes `user_id` in every WHERE clause.

## Health-score formula

```go
func HealthScore(issues []CodeHealthIssue) int {
    weights := map[Severity]int{critical: 15, high: 8, medium: 3, low: 1}
    sum := 0
    for _, i := range issues { sum += weights[i.Severity] }
    score := 100 - sum
    if score < 0 { score = 0 }
    if score > 100 { score = 100 }
    return score
}
```

**Worked example (per orchestrator):** 2 critical + 4 high + 6 medium + 10 low →
`100 − (2×15 + 4×8 + 6×3 + 10×1) = 100 − (30 + 32 + 18 + 10) = 100 − 90 = 10` (no clamp needed).

`RankIssues` sorts by `(Weight(severity) DESC, line ASC, category ASC)`.

## Async job lifecycle

```
                    analyze_repo (MCP)
                          │
                          ▼
              ┌──────────────────────┐
              │ INSERT status=pending │──▶ return analysis_id (<500ms)
              └──────────┬───────────┘
                         │ goroutine
                         ▼
              ┌──────────────────────┐
              │ pending → running     │
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │  analyze() sync core  │── success ──▶ running → completed
              │  (fetch/filter/analyze│                (health_score, top_issues,
              │   /rank/aggregate)    │                 languages, summary, completed_at)
              └──────────┬───────────┘
                         │ error
                         ▼
                     running → failed (error_message)
```

- `analysis_id` is returned immediately; `get_analysis` polls it.
- Job TTL 24h (`expires_at`): a `pending`/`running` row with `expires_at < NOW()` and no `completed_at` is reported as `failed` ("job expired") by `get_analysis`.
- `get_analysis` returns `status` always; `health_score`/`top_issues`/`languages` are `null` until `completed`.

## LLM usage

| Case | Model | Lane |
|---|---|---|
| Regex match (fast path) | none | `confidence=1.0`, no LLM call |
| Ambiguous (weak regex), file ≤ 50 KB | Ollama `qwen2.5-coder:3b` | `OllamaGenerator.Generate` (JSON-mode) |
| Ambiguous, file > 50 KB | DeepSeek V3 via OpenRouter | `LLMClientPort.Complete(PreferredProvider:"deepseek")` |

**Ambiguous-case prompt** (from `.atl/skills/codeauditor-code-health/SKILL.md` §Step 5), assembled as `System` + `Prompt`:

```
System: You are a senior code auditor. Return ONLY JSON matching the schema.
Prompt:
Analyze this <language> file for code health issues.

File: <path>
Language: <language>

```<language>
<content>
```

Detect issues in these categories: security, performance, refactor, style, architecture.
Output as JSON: {"issues":[{"severity":"low|medium|high|critical","category":"...",
"line":<int>,"message":"...","evidence":"...","suggested_fix":"..."}]}
```

Every LLM finding carries a `confidence ∈ [0,1]`. The file content is passed as data only; the Gogs/GitHub token is never sent to the LLM (skill "Don't do" list).

## Cache strategy

- Key: `analysis:<file_sha>` (git SHA, not path — a rename doesn't invalidate), namespaced by `user_id`.
- TTL: 7 days (`expires_at = NOW() + INTERVAL '7 days'`).
- **Re-analyze** (`force=true`) bypasses the read but overwrites the entry (never appends).
- Cache hit skips the LLM entirely; `cache_hits` increments in the run log (success criterion: >70% hit rate on a second visit to a SHA-stable fixture).

## Daily cap

- **Client:** `codeHealth.dailyCount.<UTC-date>` in localStorage (mirrors `practice.use-case.ts`); counter auto-resets at UTC midnight because the key embeds the date. Remaining surfaced as "remaining today".
- **Server (defense-in-depth):** `analyze_repo` calls `CountSince(userID, midnightUTC)`; ≥5 → reject. Because S4 ships MCP-only (no REST handler), the spec's "HTTP 429 + Retry-After" maps to the MCP rate-limit error `-32609` with message "Daily limit reached, resets at 00:00 UTC". (The full server-side cap moves to S5's cascade work; this is a lightweight count query.)

## Tradeoffs

| Option | Alternatives considered | Decision |
|---|---|---|
| DB-backed `AnalysisCache` vs in-memory adapter | Proposal hinted an in-memory adapter (like `session_cache.go`), but spec REQ-CODE-HEALTH-009 mandates an `analysis_cache` table with `expires_at` eviction and a >70% cross-session cache-hit criterion | **DB-backed `AnalysisCache` port** (Supabase impl). Spec is canonical; 7d TTL only means something with persistence |
| `generate_health_report` vs a 5th `generate_challenge_from_issue` tool | A dedicated tool would bloat the group beyond the locked 4-tool inventory and duplicate S3's orchestrator | **`generate_health_report` renders reports only**; "generate from issue" reuses `mcp-challenges.generate_challenge` + `SourceIssueID`. REQ-CHALLENGES-CODE-HEALTH-003 forbids a new orchestrator |
| 6 rules vs 20+ | 20+ rules inflates PR 1 and raises Ollama false-positive risk | **6 seeded rules** (one per category, security has 2). More land via `add-code-health-rule.harness.md` |
| Client-side cap vs server-only | Server-only requires a REST handler or a heavier rate-limiter S4 doesn't ship | **Client localStorage + server count query (`-32609`)**. Full server cap is S5 |
| New `OllamaGenerator` port vs bloat `OllamaClient` | `OllamaClient` is documented as the audit path's narrow streaming contract | **Narrow `OllamaGenerator` port** (the driven `ollama.Client` already has `Generate`) |
| Per-user OAuth (GitHub) vs app-wide token | OAuth flow is S6+ scope and adds a redirect/callback surface | **App-wide `GITHUB_TOKEN` env** (MVP); OAuth deferred |

## Rollback

- **PR 3 revert:** delete frontend files + route + tile. No backend regression.
- **PR 2 revert:** remove the 4 tool registrations from `wiring.go` + delete `infrastructure/mcp/code_health/`; drop the `sourceIssueId` mapping in `challenges` adapter. `GenerateChallenge` returns to S3 behavior (empty `SourceIssueID` = no-op). Registry returns to 33/8.
- **PR 1 revert:** drop `code_health_analyses`, `code_health_issues`, `analysis_cache`; delete the `code_health` service + ports + github client; revert `gogs_client.go` `ListFiles`. No existing rows touched.

## Migration safety

- **New tables only** (3), no backfill, no ALTER to existing tables.
- **Down migration:** `DROP TABLE analysis_cache; DROP TABLE code_health_issues; DROP TABLE code_health_analyses;` (order matters — issues FK → analyses).
- FK to `public.usuarios(id)` preserved (`ON DELETE CASCADE`), matching `009_`.

## Threat matrix

Applicable boundaries: remote source fetch (user-controlled `owner/name/branch`), file-content → LLM prompt. Routing/shell/subprocess/executable-classification are N/A (analysis is in-process; the sandbox gate is reused unchanged).

| Threat Case | Applicable? | Failure Mode | Test Strategy |
|---|---|---|---|
| SSRF via malicious repo descriptor | Yes | `owner`/`name`/`branch` with `../` or scheme tricks escape the fixed baseURL | RED test: `analyze_repo` with `owner="../../evil"` → rejected `-32602` before any outbound call |
| Prompt injection via file content | Yes | Malicious code in a file subverts the LLM prompt | RED test: file containing `ignore previous instructions` still yields JSON-schema-conforming findings; no instruction executed |
| Token leakage to LLM | Yes | `GITHUB_TOKEN`/`GOGS_TOKEN` sent to the LLM | Review: `analyze()` passes only `file.Content` into `Prompt`; grep gate asserts token never enters prompt builder |
| Sandbox escape (generate-from-issue) | No (reused) | N/A | S3's `ExecuteWithTests` gate with `--cap-drop=ALL --network=none --read-only` reused unchanged |
| Shell / subprocess injection | No | N/A | No shell/subprocess added |

## Testing strategy

| Layer | What | Approach |
|---|---|---|
| Go service | `health_score_test.go`, `analyzer_test.go`, `service_test.go` | Table-driven with fakes (SourceClient/AnalysisCache/Repository/OllamaGenerator/LLM). Assert formula, ranking tie-breaks, cache hit/miss, async status transitions |
| Go rules | 6 `rules/*_test.go` | Each rule: ≥5 true-positives + ≥5 true-negatives (per harness) |
| Go ports | compile-time guard | `var _ ports.SourceClient = (*gogs... )` etc. |
| Go adapters | `github_client_test.go`, `code_health_repository_test.go`, `analysis_cache_repository_test.go` | `httptest` for GitHub; `sqlmock` for Supabase |
| Go MCP | 4 `*_test.go` | Happy path with mocked service; arg parsing; `-32602` on missing user_id |
| Angular use case | `code-health.use-case.spec.ts` | Signal transitions, polling backoff, daily-cap (≥6 rejected), localStorage |
| Angular components | 6 specs | Dumb render/emit via inputs/outputs |

## Open questions

- [ ] **`sourceIssueID` persistence** — the challenges delta scenario says the row "carries `sourceIssueID`", but its own migration note marks the `source_issue_id` column "out of scope for S4". S4 already ships migration `010_`; adding one nullable `source_issue_id TEXT` column there is trivial. Recommend: add it in `010_` (satisfies the scenario); otherwise traceability relies on `sourcePath` + `origin` alone. Resolve in `sdd-tasks`.
- [ ] **PDF report** — `generate_health_report(format:"pdf")` is spec-listed but heavy. MVP ships `md`; `pdf` returns `-32603` (not implemented) until S5. Confirm acceptable.

## Resolved decisions (from explore/proposal open questions)

1. **Async lifecycle** — goroutine + DB status enum; `analysis_id` returned immediately. ✅
2. **GitHub auth** — app-wide `GITHUB_TOKEN` env (MVP); OAuth S6+. ✅
3. **Cache** — `AnalysisCache` port, DB-backed (spec canonicalized over the proposal's in-memory hint). ✅
4. **Daily cap** — 5/day, client localStorage + server count (`-32609`). ✅
5. **Rule seed** — 6 rules per spec REQ-CODE-HEALTH-007 (N+1 replaces the proposal's `console-log-leak`). ✅
6. **`generate_health_report` vs `generate_challenge_from_issue`** — report-only tool; challenge generation reuses `mcp-challenges.generate_challenge` + `SourceIssueID`. ✅
