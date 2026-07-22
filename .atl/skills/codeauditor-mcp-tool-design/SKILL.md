---
name: codeauditor-mcp-tool-design
description: Use this skill when an agent needs to design, add, or modify a tool in the CodeAuditor MCP monolith. Trigger phrases include "new MCP tool", "add tool", "modify tool", "diseñar tool", "create capability", "expand MCP", "nueva capability para el chat". Applies to the 9 tool groups (sandbox, challenges, user, curriculum, dictionary, pedagogical, code_health, external_docs, inspiration).
---

# MCP Tool Design (S2+)

> **Outcome:** a new MCP tool correctly designed, implemented, registered, tested, and documented.
> **Time estimate:** 30-60 minutes per tool.
> **Location:** `backend/internal/infrastructure/mcp/<group>/<tool_name>.go`

## When to load

Load this skill when:
- Adding a new tool to the MCP monolith.
- Modifying an existing tool's signature or behavior.
- Designing a tool that the tutor chat will use.
- Auditing a tool for security or performance.

## Pre-flight

1. Read `AGENTS.md` (root).
2. Read `docs/mcp-tools.md` (the catalogue).
3. Read `docs/hexagonal-architecture.md` §8 (MCP as a new layer).
4. Read `.atl/harnesses/add-mcp-tool.harness.md` (step-by-step).

## Decision: which group?

The tool MUST go in one of the 9 groups. Pick the right one:

| Group | Purpose | Examples |
|---|---|---|
| `sandbox/` | Execute code, list languages, healthcheck | `run_code`, `list_languages` |
| `challenges/` | Manage challenges (curated, generated, imported) | `list_challenges`, `generate_challenge` |
| `user/` | Profile, progress, history, learning_profile | `get_language_progress`, `get_history` |
| `curriculum/` | Read/manage the currícula | `get_topics`, `suggest_topic` |
| `dictionary/` | Personal tech glossary | `search_term`, `add_term` |
| `pedagogical/` ⭐ | Pedagógica: learning objectives, socratic prompts, validation | `get_learning_objective`, `validate_solution` |
| `code_health/` | Repo analysis | `analyze_repo`, `get_repo_issues` |
| `external_docs/` | Context7, Brave Search | `get_library_docs`, `search_documentation` |
| `inspiration/` | Exercism, GitHub education | `get_exercism_exercise`, `get_github_education_example` |

If none fits, propose a new group. Don't force-fit.

## Design checklist

Before writing code, answer these 8 questions:

1. **Name:** What's the tool name? Format: `mcp-<group>.<action>` (e.g. `mcp-challenges.generate_challenge`).
2. **Purpose:** What does it do in 1 sentence?
3. **Args:** What inputs does it take? (name, type, required, description)
4. **Returns:** What does it return? (shape)
5. **Errors:** What can go wrong? (4xx cases)
6. **Auth:** Does it need the user? (default: yes, current user from JWT)
7. **Side effects:** Does it write to the DB? (default: read-only)
8. **Cost:** Does it call a paid LLM? (default: no, use cascade)

If any of these is unclear, **ask the user before writing code**.

## Architectural rule (CRITICAL)

**Tools NEVER do IO directly. They call a service in `core/services/`.**

```go
// ✅ CORRECT
type GetLearningObjectiveTool struct {
    curriculum *services.CurriculumService  // ← passes through service
    llm        ports.LLMClient
}

// ❌ INCORRECT
type GetLearningObjectiveTool struct {
    db *sql.DB  // ← direct DB access
}
```

**Why:** tests, hexagonal purity, and the ability to compose tools with services.

## The `MCPTool` interface

```go
// backend/internal/infrastructure/mcp/tool.go
type MCPTool interface {
    Name() string
    Description() string
    Schema() openai.Tool
    Execute(ctx context.Context, args json.RawMessage) (any, error)
}
```

## Implementation template

```go
// backend/internal/infrastructure/mcp/<group>/<tool_name>.go
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
    llm     ports.LLMClient  // optional, if the tool needs LLM
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
  - <arg1> (<type>, <required|optional>, <description>)
  - <arg2> (<type>, <required|optional>, <description>)

Returns:
  - <field1> (<type>): <description>
  - <field2> (<type>): <description>

Example:
  Input: {"<arg1>": "<value>"}
  Output: {"<field1>": "<value>"}
>`
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

    // Validation
    if input.Arg1 == "" {
        return nil, fmt.Errorf("arg1 is required")
    }

    // Business logic via service
    result, err := t.service.<MethodName>(ctx, input.Arg1, input.Arg2)
    if err != nil {
        return nil, fmt.Errorf("<method_name>: %w", err)
    }

    return result, nil
}
```

## Register the tool

```go
// backend/internal/infrastructure/mcp/registry.go
func NewRegistry(
    curriculum *services.CurriculumService,
    challenges *services.ChallengeService,
    userProgress *services.UserProgressService,
    llm ports.LLMClient,
    // ... other services
) *Registry {
    r := &Registry{}

    // ... existing tools ...

    // Add the new tool
    r.Register(<group>.New<ToolName>Tool(<service>, llm))

    return r
}
```

## Test the tool

```go
// backend/internal/infrastructure/mcp/<group>/<tool_name>_test.go
func Test<ToolName>Tool_Execute(t *testing.T) {
    // Setup
    mockService := &mock<ServiceName>{}
    tool := New<ToolName>Tool(mockService, nil)

    // Execute
    result, err := tool.Execute(context.Background(), json.RawMessage(`{
        "arg1": "value1"
    }`))

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "expected", result["<field1>"])
}
```

## Document in `docs/mcp-tools.md`

Add a row to the table for the group:

```markdown
### 3.X `mcp-<group>` (<purpose>)

| Tool | Args | Returns | Notas |
|---|---|---|---|
| `<tool_name>` | `{arg1, arg2}` | `{field1, field2}` | <one-line description> |
```

## Don't do

- ❌ Don't access the DB directly in the tool. Always go through a service.
- ❌ Don't call a paid LLM by default. Use the cascade.
- ❌ Don't expose the LLM API key in the response.
- ❌ Don't write a tool that returns >200K tokens. Truncate.
- ❌ Don't name a tool with a verb that's already taken (check the registry).
- ❌ Don't forget the test. Even one happy-path test is mandatory.
- ❌ Don't make the tool's auth implicit. Document if it needs the current user.

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Tool takes 5+ minutes to run | Add timeout. If it must be long, return a job_id and poll. |
| Tool returns 100MB of data | Paginate or summarize. |
| Tool name conflicts with another group | Use the group prefix: `mcp-challenges.generate`, not just `generate`. |
| LLM doesn't use the tool | Improve the `Description`. Add a clear example. |
| Tool breaks under load | Add rate limiting per user (e.g. 10 calls/min). |
| Tool exposes PII | Sanitize. The LLM should never see passwords, tokens, etc. |

## Resources

- **Spec:** `openspec/changes/mcp-monolith/`
- **Doc:** `docs/mcp-tools.md` (full catalogue)
- **Doc:** `docs/hexagonal-architecture.md` §8
- **Harness:** `add-mcp-tool.harness.md` (step-by-step)
