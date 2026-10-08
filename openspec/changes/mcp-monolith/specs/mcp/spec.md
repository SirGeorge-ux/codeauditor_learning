# MCP Monolith Specification

## Purpose

Define the MCP (Model Context Protocol) monolith: a single Go binary exposing 32 tools across 7 groups via HTTP+SSE (JSON-RPC 2.0). Every tool delegates to `core/services/` — zero direct DB/IO. This is the backbone for context-aware tutor chat and Angular frontend via `MCPClientPort`.

## ADDED Requirements

### Requirement: MCPTool Interface

The system SHALL define a `MCPTool` interface with four methods: `Name() string`, `Description() string`, `Schema() openai.Tool`, and `Execute(ctx context.Context, args json.RawMessage) (any, error)`.

| Field | Constraint |
|-------|-----------|
| `Name()` | Format `mcp-<group>.<action>`, globally unique |
| `Description()` | Human-readable, includes Args/Returns/Example sections |
| `Schema()` | OpenAI-compatible function definition for LLM function calling |
| `Execute()` | Parses JSON args, validates, delegates to service, returns result |

#### Scenario: Tool implements all four methods

- GIVEN a new tool struct in `infrastructure/mcp/<group>/` (tool groups; the shared runtime lives in `internal/mcpkit/`)
- WHEN the tool implements `Name()`, `Description()`, `Schema()`, and `Execute()`
- THEN it satisfies the `MCPTool` interface and compiles

#### Scenario: Tool name follows naming convention

- GIVEN a tool for the pedagogical group
- WHEN `Name()` is called
- THEN it returns `mcp-pedagogical.<action>` format

#### Scenario: Schema is OpenAI-compatible

- GIVEN any registered tool
- WHEN `Schema()` is called
- THEN it returns `openai.Tool` with `type: "function"` and valid JSON Schema parameters

### Requirement: Tool Registry

The system SHALL provide a `Registry` that supports `Register(tool MCPTool)`, `Get(name string) (MCPTool, error)`, and `List() []MCPTool`.

#### Scenario: Registry returns all registered tools

- GIVEN 32 tools registered across 7 groups
- WHEN `List()` is called
- THEN it returns all 32 tools with valid schemas

#### Scenario: Registry rejects unknown tool name

- GIVEN a registry with registered tools
- WHEN `Get("mcp-nonexistent.foo")` is called
- THEN it returns an error matching JSON-RPC code `-32601` (Method not found)

#### Scenario: Registry prevents duplicate names

- GIVEN a tool `mcp-sandbox.run_code` already registered
- WHEN a second tool with the same name is registered
- THEN the registry panics or returns an error

### Requirement: HTTP+SSE Transport

The system SHALL expose three HTTP endpoints under `/mcp/`:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/mcp/tools/list` | GET | Return all tools with OpenAI-compatible schemas |
| `/mcp/tools/call` | POST | Execute a tool via JSON-RPC 2.0 |
| `/mcp/events` | GET (SSE) | Stream long-running tool results |

#### Scenario: List tools returns valid JSON array

- GIVEN a request `GET /mcp/tools/list` with valid JWT
- WHEN the request succeeds
- THEN it returns a JSON array of tool objects with `name`, `description`, and `parameters`

#### Scenario: Tool call via JSON-RPC 2.0

- GIVEN a POST to `/mcp/tools/call` with body `{"jsonrpc":"2.0","id":"req-1","method":"tools/call","params":{"name":"mcp-sandbox.list_languages","arguments":{}}}`
- WHEN the tool executes successfully
- THEN it returns `{"jsonrpc":"2.0","id":"req-1","result":{...}}`

#### Scenario: SSE streams long-running results

- GIVEN a tool that takes >5 seconds (e.g., sandbox execution)
- WHEN the client connects to `GET /mcp/events`
- THEN it receives `text/event-stream` with incremental `data:` events until completion

#### Scenario: Invalid JSON-RPC request

- GIVEN a POST to `/mcp/tools/call` with malformed JSON
- WHEN the server parses the request
- THEN it returns `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`

### Requirement: Tool Execution Delegation (Hexagonal Purity)

Every tool in `infrastructure/mcp/<group>/` SHALL delegate business logic exclusively to services in `core/services/`. Tools MUST NOT import `database/sql`, `net/http`, or any external SDK directly. The shared runtime (`internal/mcpkit/`) is exempt from the service-delegation rule — it is protocol infrastructure, not business logic.

#### Scenario: Tool delegates to service

- GIVEN `mcp-pedagogical.get_learning_objective`
- WHEN `Execute()` is called
- THEN it calls `CurriculumService.GetTopic()` and returns the result

#### Scenario: No direct DB access in MCP layer

- GIVEN the tool-group directories under `infrastructure/mcp/`
- WHEN `grep -r "sql.DB\|sql.Open" backend/internal/infrastructure/mcp/ backend/internal/mcpkit/` runs
- THEN the output is empty

### Requirement: Tool Authentication

The system SHALL validate JWT on all `/mcp/*` endpoints except `/health`. The user identity extracted from the JWT SHALL be injected into the tool execution context.

#### Scenario: Unauthenticated request is rejected

- GIVEN a POST to `/mcp/tools/call` without an `Authorization` header
- WHEN the request is processed
- THEN it returns HTTP 401

#### Scenario: User context is available to tools

- GIVEN a valid JWT for user `ggogsmic`
- WHEN any tool's `Execute()` runs
- THEN `ctx.Value("user_id")` returns the authenticated user ID

### Requirement: Rate Limiting

The system SHALL enforce per-user, per-tool rate limits. The default limit SHALL be 10 calls per minute per tool per user. Exceeding the limit SHALL return JSON-RPC error `-32609` (Rate limited).

#### Scenario: Rate limit is enforced

- GIVEN a user making 11 calls to `mcp-sandbox.run_code` within 60 seconds
- WHEN the 11th call arrives
- THEN it returns `{"error":{"code":-32609,"message":"Rate limit exceeded"}}`

#### Scenario: Rate limit resets after window

- GIVEN a user hit the rate limit
- WHEN 60 seconds pass
- THEN the next call succeeds normally

### Requirement: Error Handling

The system SHALL use standard JSON-RPC 2.0 error codes and include a `message` and optional `data` field. Tool execution SHALL have a 30-second timeout. Responses SHALL be capped at 50K characters.

| Error Code | Condition |
|------------|-----------|
| `-32600` | Invalid Request (missing method, wrong params) |
| `-32601` | Method not found (tool not in registry) |
| `-32602` | Invalid params (schema validation failed) |
| `-32603` | Internal error (service panic, DB failure) |
| `-32609` | Rate limit exceeded |

#### Scenario: Tool timeout

- GIVEN a tool that takes >30 seconds
- WHEN the timeout elapses
- THEN it returns `{"error":{"code":-32603,"message":"Tool execution timed out after 30s"}}`

#### Scenario: Response exceeds character cap

- GIVEN a tool returning 60K characters
- WHEN the response is processed
- THEN it is truncated to 50K characters with a `truncated: true` flag

#### Scenario: Service returns error

- GIVEN `mcp-challenges.get_challenge` with a non-existent ID
- WHEN the service returns `ErrNotFound`
- THEN the tool returns JSON-RPC error with `message: "challenge not found"`

### Requirement: MCPClientAdapter (Frontend)

The Angular frontend SHALL provide `MCPClientAdapter` implementing `MCPClientPort` with three methods: `listTools()`, `callTool<T>(name, args)`, and `streamTool<T>(name, args)`. The adapter SHALL use `HttpClient` for POST calls and an SSE client for streaming.

#### Scenario: Frontend lists available tools

- GIVEN the Angular app is loaded and user is authenticated
- WHEN `mcp.listTools()` is called
- THEN it returns an `Observable<MCPTool[]>` via `GET /mcp/tools/list`

#### Scenario: Frontend calls a tool

- GIVEN `mcp.callTool("mcp-pedagogical.get_socratic_prompt", {level: 2, language: "typescript", ...})`
- WHEN the call completes
- THEN it returns the tool result as an `Observable<T>`

#### Scenario: Frontend streams a long-running tool

- GIVEN `mcp.streamTool("mcp-sandbox.run_code", {...})`
- WHEN the sandbox execution produces output
- THEN the SSE client emits incremental events until completion

### Requirement: S2 Tool Inventory

The system SHALL register exactly 32 tools across 7 groups for S2:

| Group | Tools | Count |
|-------|-------|-------|
| `mcp-sandbox` | `run_code`, `list_languages`, `healthcheck`, `get_install_hint` | 4 |
| `mcp-challenges` | `list_challenges`, `get_challenge`, `create_temp_challenge`, `generate_challenge`, `validate_challenge` | 5 |
| `mcp-user` | `get_profile`, `get_language_progress`, `get_history`, `get_learning_profile`, `record_audit_attempt`, `update_learning_profile` | 6 |
| `mcp-curriculum` | `get_topics`, `get_topic`, `suggest_topic`, `review_topic` | 4 |
| `mcp-dictionary` | `search_term`, `get_term`, `add_term`, `list_terms` | 4 |
| `mcp-pedagogical` | `get_learning_objective`, `generate_story_context`, `create_scaffolded_steps`, `validate_solution`, `get_socratic_prompt`, `peer_review_topic` | 6 |
| `mcp-external_docs` | `get_library_docs`, `search_documentation`, `get_real_example` | 3 |

#### Scenario: Tool list count matches inventory

- GIVEN the MCP server starts with all S2 tools registered
- WHEN `GET /mcp/tools/list` is called
- THEN the response contains exactly 32 tools

#### Scenario: Deferred groups are excluded

- GIVEN the S2 scope excludes `mcp-inspiration` and `mcp-code_health`
- WHEN `GET /mcp/tools/list` is called
- THEN no tool name starts with `mcp-inspiration` or `mcp-code_health`

### Requirement: Tool Test Coverage

Each tool SHALL have at least one unit test covering the happy path. Tests SHALL mock `core/services/` and MUST NOT require network or Docker.

#### Scenario: Every tool has a test file

- GIVEN each tool file `<tool_name>.go` in `infrastructure/mcp/<group>/`
- WHEN a corresponding `<tool_name>_test.go` exists
- THEN it contains at least one `Test<ToolName>Tool_Execute` function
