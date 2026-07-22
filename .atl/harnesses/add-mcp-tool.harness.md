# Harness: Add a New MCP Tool

> **Step-by-step workflow** for adding a new tool to the CodeAuditor MCP monolith.
> **Audience:** devs backend, agents.
> **Estimated time:** 30-60 minutes.
> **Output:** a new tool in `infrastructure/mcp/<group>/`, registered, tested, documented.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/mcp-tools.md` (the catalogue).
- [ ] You've read `docs/hexagonal-architecture.md` §8 (MCP as a new layer).
- [ ] You know which group the tool belongs to.
- [ ] The corresponding service exists in `core/services/`.

---

## Step 1 — Design the tool

Answer the 8 questions from `codeauditor-mcp-tool-design` skill:

1. **Name:** `mcp-<group>.<action>` (e.g. `mcp-challenges.generate_challenge`).
2. **Purpose:** 1 sentence.
3. **Args:** name, type, required, description.
4. **Returns:** shape (JSON object).
5. **Errors:** 4xx cases.
6. **Auth:** yes/no.
7. **Side effects:** read/write.
8. **Cost:** paid LLM? (default: no).

If any is unclear, **ask the user** before writing code.

---

## Step 2 — Define or extend the port

If the tool needs a new port, add it to `backend/internal/ports/mcp.go` (or a new file).

```go
// backend/internal/ports/mcp.go (append)
type MCPPedagogicalPort interface {
    GetLearningObjective(ctx context.Context, language, topic, level string) (*LearningObjective, error)
    GenerateStoryContext(ctx context.Context, language, topic string) (string, error)
    // ... etc
}
```

If the tool just needs an existing port (e.g. `ports.SandboxExecutor`), skip this step.

---

## Step 3 — Implement the service (if needed)

The tool calls a service. If the method doesn't exist, add it to the corresponding `core/services/<name>_service.go`.

```go
// backend/internal/core/services/curriculum_service.go (append)
func (s *CurriculumService) GetTopic(ctx context.Context, language, topic string) (*models.Topic, error) {
    // 1. Read from .atl/curricula/<language>.md
    // 2. Parse the markdown
    // 3. Return the topic
}
```

Add a test:

```go
// backend/internal/core/services/curriculum_service_test.go
func TestCurriculumService_GetTopic(t *testing.T) {
    // Setup
    // Call
    // Assert
}
```

---

## Step 4 — Create the tool file

**File:** `backend/internal/infrastructure/mcp/<group>/<tool_name>.go`

```go
package <group>

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/anomalyco/codeauditor/backend/internal/core/services"
    "github.com/anomalyco/codeauditor/backend/internal/ports"
)

type <ToolName>Tool struct {
    service *services.<ServiceName>
    llm     ports.LLMClient  // optional
}

func New<ToolName>Tool(s *services.<ServiceName>, l ports.LLMClient) *<ToolName>Tool {
    return &<ToolName>Tool{service: s, llm: l}
}

func (t *<ToolName>Tool) Name() string {
    return "mcp-<group>.<tool_name>"
}

func (t *<ToolName>Tool) Description() string {
    return `<human-readable description for the LLM.

Args:
  - <arg1> (<type>, <required|optional>): <description>
  - <arg2> (<type>, <required|optional>): <description>

Returns:
  - <field1> (<type>): <description>
  - <field2> (<type>): <description>

Example:
  Input: {"<arg1>": "<value>"}
  Output: {"<field1>": "<value>"}
`
}

func (t *<ToolName>Tool) Schema() openai.Tool {
    return openai.Tool{
        Type: "function",
        Function: openai.FunctionDefinition{
            Name:        t.Name(),
            Description: t.Description(),
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "<arg1>": {"type": "<type>", "description": "<desc>"},
                    "<arg2>": {"type": "<type>", "description": "<desc>"}
                },
                "required": ["<arg1>"]
            }`),
        },
    }
}

func (t *<ToolName>Tool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
    var input struct {
        Arg1 string `json:"arg1"`
        Arg2 string `json:"arg2"`
    }
    if err := json.Unmarshal(args, &input); err != nil {
        return nil, fmt.Errorf("invalid args: %w", err)
    }

    if input.Arg1 == "" {
        return nil, fmt.Errorf("arg1 is required")
    }

    result, err := t.service.<MethodName>(ctx, input.Arg1, input.Arg2)
    if err != nil {
        return nil, fmt.Errorf("<method_name>: %w", err)
    }

    return result, nil
}
```

---

## Step 5 — Create the test file

**File:** `backend/internal/infrastructure/mcp/<group>/<tool_name>_test.go`

```go
package <group>

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

type mock<ServiceName> struct {
    returnValue any
    returnError error
}

func (m *mock<ServiceName>) <MethodName>(ctx context.Context, arg1, arg2 string) (any, error) {
    return m.returnValue, m.returnError
}

func Test<ToolName>Tool_Execute_Success(t *testing.T) {
    mock := &mock<ServiceName>{
        returnValue: map[string]any{"<field1>": "<expected>"},
    }
    tool := New<ToolName>Tool(/* inject mock somehow */, nil)

    result, err := tool.Execute(context.Background(), json.RawMessage(`{
        "arg1": "value1"
    }`))

    require.NoError(t, err)
    assert.Equal(t, "<expected>", result.(map[string]any)["<field1>"])
}

func Test<ToolName>Tool_Execute_MissingRequired(t *testing.T) {
    tool := New<ToolName>Tool(nil, nil)

    _, err := tool.Execute(context.Background(), json.RawMessage(`{
        "arg2": "value2"
    }`))

    require.Error(t, err)
    assert.Contains(t, err.Error(), "arg1 is required")
}

func Test<ToolName>Tool_Execute_ServiceError(t *testing.T) {
    mock := &mock<ServiceName>{
        returnError: fmt.Errorf("service unavailable"),
    }
    tool := New<ToolName>Tool(/* inject mock */, nil)

    _, err := tool.Execute(context.Background(), json.RawMessage(`{
        "arg1": "value1"
    }`))

    require.Error(t, err)
    assert.Contains(t, err.Error(), "service unavailable")
}
```

**Note:** the mock injection is tricky. You may need to define an interface in the tool:

```go
type curriculumService interface {
    GetTopic(ctx context.Context, language, topic string) (*models.Topic, error)
}

type <ToolName>Tool struct {
    service curriculumService
    llm     ports.LLMClient
}
```

This way, the test can inject a mock without depending on the real service.

---

## Step 6 — Register the tool

**File:** `backend/internal/infrastructure/mcp/registry.go`

Add to `NewRegistry`:

```go
func NewRegistry(
    curriculum *services.CurriculumService,
    // ... other services
    llm ports.LLMClient,
) *Registry {
    r := &Registry{}

    // ... existing tools ...

    // Add the new tool
    r.Register(<group>.New<ToolName>Tool(curriculum, llm))

    return r
}
```

---

## Step 7 — Update `cmd/api/main.go` if needed

If the tool needs a new service, add it to `main.go`:

```go
curriculumService := services.NewCurriculumService(...)
// ...
registry := mcp.NewRegistry(curriculumService, ..., llm)
```

---

## Step 8 — Document in `docs/mcp-tools.md`

Add a row to the table for the group:

```markdown
### 3.X `mcp-<group>` (<purpose>)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `<tool_name>` | `{arg1, arg2}` | `{field1, field2}` | <description> |
```

---

## Step 9 — Run tests + quality gate

```bash
cd backend

# Unit tests
go test ./internal/infrastructure/mcp/<group>/...

# All tests
go test -short ./...

# Quality gate
make validate
```

All must pass.

---

## Step 10 — Test the tool end-to-end (optional)

1. Start the backend.
2. From the frontend, send a chat message that triggers the tool.
3. Verify the response is correct.
4. Or, use curl:

```bash
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "tools/call",
    "params": {
      "name": "mcp-<group>.<tool_name>",
      "arguments": {"arg1": "value1"}
    }
  }'
```

---

## Step 11 — Commit and PR

```bash
git add backend/internal/ports/mcp.go \
        backend/internal/core/services/<name>_service.go \
        backend/internal/core/services/<name>_service_test.go \
        backend/internal/infrastructure/mcp/<group>/<tool_name>.go \
        backend/internal/infrastructure/mcp/<group>/<tool_name>_test.go \
        backend/internal/infrastructure/mcp/registry.go \
        backend/cmd/api/main.go \
        docs/mcp-tools.md

git commit -m "feat(mcp): add mcp-<group>.<tool_name>

- <description>
- Group: <group>
- Calls: <service>.<method>
- Tests: <N> unit tests

Refs: openspec/changes/mcp-monolith/"

git push origin feature/mcp-<group>-<tool_name>
gh pr create --title "feat(mcp): add mcp-<group>.<tool_name>" \
             --body "Adds a new MCP tool in the <group> group. See docs/mcp-tools.md."
```

---

## Checklist

- [ ] 8 design questions answered.
- [ ] Port defined (if needed).
- [ ] Service method added (if needed) + test.
- [ ] Tool file created with Name/Description/Schema/Execute.
- [ ] Test file with 3 tests: success, missing required, service error.
- [ ] Registered in `NewRegistry`.
- [ ] `main.go` updated (if needed).
- [ ] `docs/mcp-tools.md` updated.
- [ ] `make validate` passes.
- [ ] `make test` passes.
- [ ] Conventional commit + clear PR.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Tool accesses DB directly | Always go through a service. If the method doesn't exist, add it. |
| Description too short for the LLM | Add a clear example (Input/Output). LLM uses the description to decide WHEN to call. |
| Tool name conflicts with another group | Use the group prefix: `mcp-challenges.generate`, not just `generate`. |
| Tool takes 5+ minutes to run | Add timeout. If it must be long, return a job_id and poll. |
| Tool returns 100MB of data | Paginate or summarize. |
| Test passes alone but fails in suite | Reset state in `beforeEach`. Don't share state. |
| LLM doesn't use the tool | Improve the `Description`. Add an example. Test with real LLM. |
| Forgot to add to `NewRegistry` | The tool exists but isn't reachable. Always register. |
| Forgot to update `main.go` | The dependency isn't injected. Always wire. |

## Resources

- **Spec:** `openspec/changes/mcp-monolith/`
- **Doc:** `docs/mcp-tools.md` (catalogue)
- **Skill:** `codeauditor-mcp-tool-design`
