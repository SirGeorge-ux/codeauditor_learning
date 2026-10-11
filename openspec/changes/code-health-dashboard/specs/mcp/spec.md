# Delta for MCP (S4: code-health-dashboard)

> **Capability:** `mcp` (MODIFIED)
> **Change:** `code-health-dashboard` (S4)
> **Depends on:** `mcp-monolith` (S2, archived 2026-10-10).
> **This delta, on archive, lands at:** `openspec/specs/mcp/spec.md` (additive group registration + count update; no protocol change).

## Purpose

The `mcp` capability gains a 9th group — `mcp-code-health` — exposing 4 tools that drive the S4 code-health-dashboard flow. The MCP inventory grows from 33 tools / 8 groups (S2 baseline, frozen on 2026-10-10) to **37 tools / 9 groups** post-archive. The new tools follow every existing rule in `openspec/specs/mcp/spec.md`: `MCPTool` interface shape, JSON-RPC 2.0 transport, JWT auth (`ctx.Value("user_id")`), 30s timeout, 50K response cap, 10/min/user rate limit, and JSON-RPC error codes (`-32602` for invalid params, `-32609` for rate-limit). The `mcp-inspiration` group remains out of scope.

The change is additive: no existing tool is renamed or removed. Rollback unregisters the 4 new tools in `cmd/api/wiring.go` and removes the `mcp-code-health/` package — the count returns to 33 / 8 and the S2 surface is unchanged.

## ADDED Requirements

### Requirement: REQ-MCP-CODE-HEALTH-001 — mcp-code-health Group Registration

The system SHALL register a 9th MCP group `mcp-code-health` in the `Registry`, exposing exactly 4 tools. Every tool SHALL implement the canonical `MCPTool` interface (`Name()`, `Description()`, `Schema()`, `Execute()`), SHALL validate `user_id` in context, SHALL enforce the 30-second execution timeout and 50K-character response cap, and SHALL respect the per-user rate limit (`-32609` on >10 calls/minute/tool/user).

| Tool | Args | Returns |
|---|---|---|
| `mcp-code-health.analyze_repo` | `{owner, name, source: "gogs"\|"github", branch}` | `{analysis_id, status: "pending"}` |
| `mcp-code-health.get_analysis` | `{analysis_id}` | `{analysis_id, status, health_score, top_issues, languages, summary, created_at, completed_at, error_message?}` |
| `mcp-code-health.get_repo_issues` | `{owner, name, source, severity?: "critical"\|"high"\|"medium"\|"low", limit?: int}` | `{issues: [{id, severity, category, file_path, line, message, evidence, suggested_fix, rule, confidence}]}` |
| `mcp-code-health.generate_health_report` | `{analysis_id, format: "md"\|"pdf"}` | `{report_id, content, format}` |

#### Scenario: analyze_repo returns analysis_id synchronously

- GIVEN a valid JWT and the user invokes `mcp-code-health.analyze_repo({owner:"ggogsmic", name:"academy-mic", source:"gogs", branch:"master"})`
- WHEN the tool validates input, inserts the analysis row, and starts the background goroutine
- THEN the response is `{analysis_id: "a-...", status: "pending"}` returned within 500ms
- AND `ctx.Value("user_id")` carries the authenticated user throughout execution

#### Scenario: get_analysis returns full payload

- GIVEN an analysis with `analysis_id="a-..."` has completed with `health_score=78`
- WHEN the user invokes `mcp-code-health.get_analysis({analysis_id:"a-..."})`
- THEN the response includes `status="completed"`, `health_score=78`, `top_issues` (array of 10), `languages` (object), `summary` (string), and timestamps

#### Scenario: get_repo_issues filters by severity

- GIVEN an analysis has 5 critical, 8 high, 12 medium issues
- WHEN the user invokes `mcp-code-health.get_repo_issues({owner, name, source, severity:"critical", limit:50})`
- THEN the response is `issues: [...]` containing only the 5 critical issues (severity filter applied server-side)

#### Scenario: generate_health_report produces markdown

- GIVEN a completed analysis with `analysis_id="a-..."`
- WHEN the user invokes `mcp-code-health.generate_health_report({analysis_id, format:"md"})`
- THEN the response is `{report_id, content: "<markdown>", format:"md"}`
- AND the markdown contains the health score, ranked issues table, language breakdown, and a "generate challenges" hint

#### Scenario: Missing user_id rejected

- GIVEN a `mcp-code-health.*` call without a valid JWT
- WHEN the MCP layer validates context
- THEN it returns `{"error":{"code":-32602,"message":"user_id required"}}` (mirrors the existing Tool Authentication requirement)

#### Scenario: Per-tool rate limit enforced

- GIVEN a user has called `mcp-code-health.get_analysis` 10 times in the last 60s
- WHEN the 11th call arrives
- THEN it returns `{"error":{"code":-32609,"message":"Rate limit exceeded"}}` (mirrors the existing Rate Limiting requirement)

#### Scenario: 30s execution timeout

- GIVEN an `analyze_repo` goroutine has not completed within 30 seconds for some reason
- WHEN the timeout fires
- THEN it returns `{"error":{"code":-32603,"message":"Tool execution timed out after 30s"}}` (mirrors the existing Error Handling requirement)

### Requirement: REQ-MCP-CODE-HEALTH-002 — Hexagonal Purity for mcp-code-health Tools

Every `mcp-code-health/*` tool SHALL delegate business logic exclusively to `core/services/code_health.CodeHealthService` (and indirectly to `ChallengeService` for the "generate from issue" path). Tools MUST NOT import `database/sql`, `net/http`, the GitHub SDK, or the Gogs SDK. Cache access goes through the new `AnalysisCache` driven adapter, not the legacy `session_cache.go` ring buffer.

#### Scenario: Tool delegates to CodeHealthService

- GIVEN `mcp-code-health.analyze_repo` receives valid args
- WHEN `Execute()` runs
- THEN it calls `codeHealthService.AnalyzeRepo(ctx, userID, repoDescriptor)` and returns the result
- AND does not import any `database/sql` driver or HTTP client

#### Scenario: No DB access in tool group

- GIVEN the `infrastructure/mcp/code_health/` directory
- WHEN `grep -r "sql.DB\|sql.Open" backend/internal/infrastructure/mcp/code_health/` runs
- THEN the output is empty (mirrors the existing Tool Execution Delegation rule)

#### Scenario: Generate-from-issue calls ChallengeService

- GIVEN the user invokes `mcp-code-health.generate_health_report({analysis_id, format:"md"})` and clicks "Generate challenge" on an issue in the rendered report (frontend calls the same tool with `issue_id` and `language`)
- WHEN the tool routes the request
- THEN it calls `ChallengeService.GenerateChallenge(ctx, userID, GenerateChallengeHint{SourceIssueID, Language, Difficulty, Origin:"code_health"})`
- AND the persisted row carries `origin="code_health"` (see the `challenges` delta spec)

### Requirement: REQ-MCP-CODE-HEALTH-003 — Tool Test Coverage

Each `mcp-code-health` tool SHALL have at least one unit test covering the happy path. Tests SHALL mock `core/services/code_health/` and `ChallengeService` and MUST NOT require network or Docker.

#### Scenario: Every new tool has a test file

- GIVEN each tool file `analyze_repo.go`, `get_analysis.go`, `get_repo_issues.go`, `generate_health_report.go` in `infrastructure/mcp/code_health/`
- WHEN the corresponding `*_test.go` exists
- THEN each contains at least one `Test<ToolName>Tool_Execute` function

## MODIFIED Requirements

### Requirement: Tool Registry

> **Diff vs. canonical `openspec/specs/mcp/spec.md`:** the count is updated from 33 / 8 (S2) to 37 / 9 (post-S4).

The system SHALL provide a `Registry` that supports `Register(tool MCPTool)`, `Get(name string) (MCPTool, error)`, and `List() []MCPTool`.
(Previously: 33 tools registered across 8 groups)

#### Scenario: Registry returns all registered tools

- GIVEN 37 tools registered across 9 groups (S2 33 tools + S4's 4 `mcp-code-health` tools)
- WHEN `List()` is called
- THEN it returns all 37 tools with valid schemas

#### Scenario: Registry rejects unknown tool name

- GIVEN a registry with registered tools
- WHEN `Get("mcp-nonexistent.foo")` is called
- THEN it returns an error matching JSON-RPC code `-32601` (Method not found)

#### Scenario: Registry prevents duplicate names

- GIVEN a tool `mcp-sandbox.run_code` already registered
- WHEN a second tool with the same name is registered
- THEN the registry panics or returns an error

### Requirement: S2 Tool Inventory

> **Diff vs. canonical `openspec/specs/mcp/spec.md`:** the inventory grows from 33 tools / 8 groups (S2) to 37 tools / 9 groups. The 9th group `mcp-code-health` is added; `mcp-inspiration` remains deferred. The 33-tool S2 baseline is preserved verbatim for the original 8 groups.

The system SHALL register exactly 37 tools across 9 groups (S2 baseline + S4's `mcp-code-health`):
(Previously: exactly 33 tools across 8 groups for S2)

| Group | Tools | Count |
|-------|-------|-------|
| `mcp-sandbox` | `run_code`, `list_languages`, `healthcheck`, `get_install_hint` | 4 |
| `mcp-challenges` | `list_challenges`, `get_challenge`, `create_temp_challenge`, `generate_challenge`, `validate_challenge` | 5 |
| `mcp-user` | `get_profile`, `get_language_progress`, `get_history`, `get_learning_profile`, `record_audit_attempt`, `update_learning_profile` | 6 |
| `mcp-curriculum` | `get_topics`, `get_topic`, `suggest_topic`, `review_topic` | 4 |
| `mcp-dictionary` | `search_term`, `get_term`, `add_term`, `list_terms` | 4 |
| `mcp-pedagogical` | `get_learning_objective`, `generate_story_context`, `create_scaffolded_steps`, `validate_solution`, `get_socratic_prompt`, `peer_review_topic` | 6 |
| `mcp-external_docs` | `get_library_docs`, `search_documentation`, `get_real_example` | 3 |
| `mcp-tutor` | `tutor_lookup` | 1 |
| `mcp-code-health` ⭐ S4 | `analyze_repo`, `get_analysis`, `get_repo_issues`, `generate_health_report` | 4 |

#### Scenario: Tool list count matches inventory

- GIVEN the MCP server starts with all S2 + S4 tools registered
- WHEN `GET /mcp/tools/list` is called
- THEN the response contains exactly 37 tools

#### Scenario: Deferred groups are excluded

- GIVEN the S2 + S4 scope includes all 9 groups listed in the inventory table above and excludes `mcp-inspiration`
- WHEN `GET /mcp/tools/list` is called
- THEN no tool name starts with `mcp-inspiration`

#### Scenario: New group is discoverable

- GIVEN the `mcp-code-health` group is registered
- WHEN a client calls `GET /mcp/tools/list` and filters by group prefix
- THEN 4 tools match the `mcp-code-health.*` prefix
- AND each tool's `Schema()` returns a valid OpenAI function definition (mirrors the existing Schema constraint)

> **Note (tool ownership):** `mcp-code-health` is the 9th group and ships with the archived `code-health-dashboard` change (S4). It is registered by the same change that adds `CodeHealthService`, the GitHub client, and the `AnalysisCache`. Removing it would regress the S4 dashboard; the canonical post-S4 surface is therefore 37 tools across 9 groups.

## Migration Notes

- **No protocol change.** The 4 new tools use the same `MCPTool` interface, transport, auth, error codes, and rate-limit rules as the existing 33 tools.
- **No DB migration required for MCP itself.** The code-health tables (`code_health_analyses`, `code_health_issues`, `analysis_cache`) are added by the same S4 change in migration `010_create_code_health.sql` (see the `code-health` spec).
- **Backward compatibility:** every existing tool name, request shape, and response shape is unchanged. Clients that do not know about `mcp-code-health` ignore the new prefix.
- **Rollback:** unregister the 4 tools in `cmd/api/wiring.go` and delete the `infrastructure/mcp/code_health/` package. The registry returns to 33 tools / 8 groups.