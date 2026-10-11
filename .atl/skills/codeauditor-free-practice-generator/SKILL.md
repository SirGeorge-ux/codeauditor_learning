---
name: codeauditor-free-practice-generator
description: Use this skill when an agent needs to generate an on-demand challenge for free practice mode. Trigger phrases include "generate challenge", "free practice", "ejercicio sobre X", "I want to learn X", "dame un ejercicio de Y", "level F/E/D/C/B/A/S", "Herencias en TypeScript". Coordinates the LLM + currícula + sandbox via MCP tools.
---

# Free Practice Generator (S3+)

> **Outcome:** a complete challenge (with `code`, `expected_findings`, `hints`, `solution_code`, `test_cases`, `linter_rules`) generated on-demand based on the user's language, topic, and level.
> **Time estimate:** 10-30 seconds per challenge.
> **LLM used:** OpenRouter cascade (Groq `qwen-2.5-coder-32b` for code, `llama-3.3-70b-versatile` for narrative).

## When to load

Load this skill when the user asks:
- "Dame un ejercicio de Herencias en TypeScript"
- "Generate a challenge about generics in Rust"
- "Quiero practicar N+1 queries en Python nivel mid"
- "Free practice, dame algo de C++"
- "Siguiente ejercicio"
- "Otro más difícil"
- "Otro más fácil"

## Pre-flight

1. Read `AGENTS.md` (root) if you haven't.
2. Read `docs/business-domain.md` §3.2 (Free practice mode flow).
3. Read `docs/llm-strategy.md` (which model to use).
4. Read `docs/curricula-format.md` (the topic structure).
5. Read `docs/mcp-tools.md` §3.6 (`mcp-pedagogical` tools).

## The flow (high level)

```
User: "Dame un ejercicio de Herencias en TypeScript nivel D"
   ↓
1. mcp-curriculum.get_topic({ language: "typescript", topic_id: "inheritance" })
   → { slug, difficulty, prerequisites, description, exercises, common_mistakes, ... }
   ↓
2. mcp-pedagogical.get_learning_objective({ language: "typescript", topic_id: "inheritance", level: "D" })
   → { exercise, prerequisites, restrictions, common_mistakes, estimated_difficulty }
   ↓
3. mcp-pedagogical.generate_story_context({ language: "typescript", topic_id: "inheritance", user_level: "D" })
   → { story, scenario }
   ↓
4. LLM cascade (Groq qwen-2.5-coder-32b) via LLMClientPort.Complete(JSON-mode):
   - input: story_context + learning_objective + restrictions
   - output: { code, expected_findings, solution_code, test_cases, linter_rules, hints }
   ↓
5. mcp-challenges.generate_challenge({ language, topicId, level })
   → produces a Challenge row with origin="generated", generatedBy="free-practice-v1", createdBy=userID
   - internally runs sandbox.ExecuteWithTests(solutionCode, testCases, ≤3 attempts) as the gate
   - persistence happens inside the tool, so the orchestrator does NOT need a separate "create" step
   ↓
6. Return to user (open in /dojo/<tempId>)
```

> **Note:** the wire shapes above match `TOOL_GENERATE_CHALLENGE = "mcp-challenges.generate_challenge"` in
> `frontend/.../application/practice.use-case.ts` and the canonical MCP tool specs under
> `backend/internal/infrastructure/mcp/{challenges,curriculum,pedagogical}/*_tool.go`.
> Read those files before wiring a new caller — tool names and arg keys are not negotiable.

## Step-by-step (when implementing or debugging)

### Step 1 — Detect language

The user says "Herencias en TypeScript". Split into:
- language = "typescript"
- topic = "inheritance" (or "Herencias" → mapped to "inheritance")

If the topic is in Spanish and the curriculum is in English, use the topic's `display_name_es` or map manually.

If language is missing, ask the user (or default to their `most_used_language` from `LearningProfile`).

If topic is missing, ask the user (or offer 3 topics from their current level).

### Step 2 — Get the topic from curriculum

The actual wire is `mcp-curriculum.get_topic` (snake_case `topic_id`):

```ts
const topic = await mcpClient.callTool('mcp-curriculum.get_topic', {
  language: 'typescript',
  topic_id: 'inheritance',
});
// Returns: { slug, difficulty, prerequisites, description, exercises, common_mistakes, ... }
```

If the topic doesn't exist in the curriculum:

```ts
const suggestion = await mcpClient.callTool('mcp-curriculum.suggest_topic', {
  language: 'typescript',
  hint: 'Herencias',
});
// Returns a proposed topic for user review
```

**Don't auto-create the topic.** Show the suggestion to the user and let them approve.
Use `mcp-curriculum.get_topics(language)` first to enumerate the canonical slugs
before guessing one.

### Step 3 — Get the learning objective

The actual wire is `mcp-pedagogical.get_learning_objective` (snake_case `topic_id`,
optional `level`):

```ts
const objective = await mcpClient.callTool('mcp-pedagogical.get_learning_objective', {
  language: 'typescript',
  topic_id: 'inheritance',
  level: 'D', // F..S; informational only today, accepted as a hint
});
// Returns:
// {
//   exercise: "Crea una clase Animal con un método speak(). Crea una subclase Dog extends Animal...",
//   prerequisites: ["classes"],
//   restrictions: ["No uses generics todavía", "No uses decorators"],
//   common_mistakes: ["Olvidar super() en el constructor", "No llamar a super.speak() en el override"],
//   estimated_difficulty: "D"
// }
```

The `restrictions` are CRITICAL. They prevent the LLM from generating a challenge that
requires concepts the user doesn't know yet.

### Step 4 — Generate the story context

```ts
const ctx = await mcpClient.callTool('mcp-pedagogical.generate_story_context', {
  language: 'typescript',
  topic_id: 'inheritance',
  user_level: 'D', // optional F..S
  // scenario: '<optional custom prefix>'  // optional
});
// Returns: { story, scenario }
```

PBL (Problem-Based Learning) makes the exercise more engaging.

### Step 5 — Generate the challenge draft with the LLM

Use the LLM cascade (see `docs/llm-strategy.md`). Default: `qwen-2.5-coder-32b`
(Groq free tier) via `LLMClientPort.Complete(ctx, CompleteRequest{...})` —
the JSON-mode path added by S3 PR 1 (NOT `StreamChat`). The cascade falls through
Groq → DeepSeek → MiniMax with a 5s whole-response timeout.

```text
# Pseudocódigo del prompt (se pasa como CompleteRequest.System + CompleteRequest.User)
system = load_prompt("tutor-generator-system.md")
prompt = """
You are a senior code auditor generating a challenge for a student.

Language: {language}
Topic: {topic}
Level: {level}  # F, E, D, C, B, A, or S

Story context:
{context}

Learning objective:
{objective.exercise}

Prerequisites (student already knows): {objective.prerequisites}
Restrictions (student does NOT know yet): {objective.restrictions}
Common mistakes to watch for: {objective.common_mistakes}

Generate a challenge with:
1. `code` (20-50 lines, vulnerable, runnable, demonstrates exactly one smell)
2. `expected_findings` (1-3 findings, each with severity, category, message, evidence, suggested_fix)
3. `solution_code` (correct implementation, passes the test_cases)
4. `test_cases` (2-4 functional tests, with weight)
5. `linter_rules` (1-3 rules that fail on `code` and pass on `solution_code`)
6. `hints` (3 progressive hints, levels 1, 2, 3 with cost 0, 10, 25)

Output as JSON, no commentary.
"""
response = await llmClient.Complete(ctx, { system, user: prompt, jsonMode: true })
draft = parseJSON(response)
```

### Step 6 — Hand off to `mcp-challenges.generate_challenge`

The orchestrator does NOT call sandbox/validation/persist directly. It hands the draft
plus `topicId` + `level` to the challenges tool, which owns the gate:

```ts
const persisted = await mcpClient.callTool('mcp-challenges.generate_challenge', {
  language: 'typescript',
  topicId: 'inheritance',
  level: 'D',
  // optional:
  // difficulty: 'mid',       // 4-bucket — derived from level when omitted
  // focus: 'n+1',            // optional code-smell focus
});
// Returns: full Challenge JSON with origin="generated",
// generatedBy="free-practice-v1", createdBy=userID,
// plus solutionCode + solutionExplanation.
```

Internally (`backend/internal/core/services/challenge_service.go`), the tool:

1. Re-fetches the topic + objective from curriculum/pedagogical (defense in depth).
2. Loops `sandbox.ExecuteWithTests(solutionCode, testCases)` up to 3 times. A
   non-passing run augments the prompt with the failure and regenerates; 3 fails
   in a row returns `ErrChallengeGenerationFailedSandbox` and the tool surfaces it
   as JSON-RPC `-32603` (do NOT persist a broken challenge).
3. Maps F–S → 4-bucket difficulty (`F,E→junior`, `D,C→mid`, `B,A→senior`, `S→architect`).
4. Persists the row via `repo.Insert(... origin="generated", generatedBy="free-practice-v1", createdBy=userID)`.

> **Don't try to run the sandbox gate from this skill.** It's owned by the challenges tool;
> calling `ExecuteWithTests` from the orchestrator duplicates the retry loop and bypasses
> the persistence metadata.

### Step 7 — (Optional) Dry-run validation with `validate_challenge`

If you want to lint a hand-authored draft BEFORE persisting (e.g. when scaffolding
a custom practice session), use the validator:

```ts
const v = await mcpClient.callTool('mcp-challenges.validate_challenge', {
  title, description, difficulty, language,
  expectedFindings, testCases, hints,
});
// Returns: { valid: bool, errors: string[] }
```

Checks: `expected_findings` 1..3, `test_cases` 2..4, exactly 3 hints (costs 0/10/25),
no spoiler keywords in `description`, `difficulty` ∈ {junior, mid, senior, architect}.

### Step 8 — Return to the user

The persisted Challenge JSON is the response. The `/practice/free` UI navigates to
`/dojo/<id>` (the existing dojo route) and hands the row to the same render path
curated challenges use — no special-case UX.

## Edge cases

| Caso | Manejo |
|---|---|
| Topic no existe en la currícula | Sugerir via `mcp-curriculum.suggest_topic` y pedir aprobación |
| LLM genera código que no compila | Loop de validación, max 3 intentos |
| LLM genera un challenge idéntico al último | Detectar con hash, regenerar |
| User pide 5 challenges seguidos | Cachear (no regenerar el mismo topic+level en <1h) |
| User pide un topic que requiere un lenguaje no soportado | Sugerir el lenguaje más cercano + un equivalente en su lenguaje |
| `solution_code` tiene el mismo smell que `code` | Rechazar, regenerar (el LLM a veces "no arregla" nada) |
| LLM tarda >30s | Mostrar "Generando..." con un spinner, timeout a 60s, fallback a Ollama local |

## Don't do

- ❌ Don't auto-create topics in the curriculum. Always show suggestions for user review via `mcp-curriculum.suggest_topic`.
- ❌ Don't bypass `mcp-challenges.generate_challenge` to call `sandbox.ExecuteWithTests` directly from the orchestrator — the tool owns the gate, the retry loop, the F–S → 4-bucket mapping, AND the persistence metadata (`origin="generated"`, `generatedBy="free-practice-v1"`, `createdBy=userID`). Re-implementing it duplicates code and breaks lineage.
- ❌ Don't send `camelCase` keys to `mcp-curriculum.*` or `mcp-pedagogical.*` tools — they expect `topic_id`, `user_level`, etc. (snake_case). The dispatcher will silently no-op the wrong keys and you'll get empty returns.
- ❌ Don't send `snake_case` keys to `mcp-challenges.*` tools — they expect `topicId`, `sourceRepo`, `reveal` (camelCase).
- ❌ Don't call any `mcp-challenges.*` tool without an authenticated user in context (returns `-32602`). The MCP bridge injects `user_id` automatically on production routes; tests must use `ContextWithUserID`.
- ❌ Don't ignore the `restrictions` from `get_learning_objective` — they're the pedagogical filter that prevents the LLM from using concepts the user hasn't mastered yet.
- ❌ Don't skip the `story_context` step. PBL beats dry exercises.
- ❌ Don't use `MiniMax M3` by default — use the OpenRouter cascade (Groq → DeepSeek → MiniMax opt-in via premium flag). M3 is reserved for paid `premium opt-in` paths.
- ❌ Don't expose any LLM API key in prompts, logs, or frontend code — the cascade runs server-side via `LLMClientPort`.
- ❌ Don't generate a challenge without going through `mcp-challenges.generate_challenge` — direct persistence from this skill skips the sandbox gate and will produce unverified rows.

## Tools involved (canonical names + wire)

| Tool | Args | Returns | Source of truth |
|---|---|---|---|
| `mcp-curriculum.get_topics` | `{ language }` | `[Topic]` (slug, title, short desc) | `backend/.../mcp/curriculum/curriculum_tool.go:189` |
| `mcp-curriculum.get_topic` | `{ language, topic_id }` | full `Topic` (prerequisites, exercises, common_mistakes, …) | `curriculum_tool.go:245` |
| `mcp-curriculum.suggest_topic` | `{ language, hint? }` | proposed topic for user review | `curriculum_tool.go:307` |
| `mcp-curriculum.review_topic` | `{ language, topic_id }` | review verdict | `curriculum_tool.go:380` |
| `mcp-pedagogical.get_learning_objective` | `{ language, topic_id, level? }` | `{ exercise, prerequisites, restrictions, common_mistakes, estimated_difficulty }` | `pedagogical_tool.go:283` |
| `mcp-pedagogical.generate_story_context` | `{ language, topic_id, user_level?, scenario? }` | `{ story, scenario }` | `pedagogical_tool.go:355` |
| `mcp-pedagogical.create_scaffolded_steps` | `{ language, topic_id, objective, levels[] }` | `[ScaffoldingStep]` | `pedagogical_tool.go:415` |
| `mcp-pedagogical.validate_solution` | `{ language, topic_id, code }` | `{ passed, feedback[], score }` (static checks only — NOT sandbox) | `pedagogical_tool.go:477` |
| `mcp-pedagogical.get_socratic_prompt` | `{ language, topic_id, level (0..3), context? }` | `{ prompt, breadcrumbs? }` | `pedagogical_tool.go:533` |
| `mcp-pedagogical.peer_review_topic` | `{ … }` | peer review verdict | `pedagogical_tool.go:601` |
| `mcp-challenges.list_challenges` | `{}` | `[Challenge]` (no solution) | `challenges_tool.go:171` |
| `mcp-challenges.get_challenge` | `{ id, reveal? }` | single `Challenge` | `challenges_tool.go:214` |
| `mcp-challenges.create_temp_challenge` | `{ title, description, difficulty, language, code?, category?, … }` | persisted `Challenge` (+ `deduped` flag) | `challenges_tool.go:274` |
| **`mcp-challenges.generate_challenge`** | `{ language?, difficulty?, focus?, topicId?, level? }` (all optional) | full persisted `Challenge` with `origin="generated"`, `generatedBy="free-practice-v1"`, `createdBy=userID`, **plus** `solutionCode` + `solutionExplanation` | `challenges_tool.go:345` |
| `mcp-challenges.validate_challenge` | `{ title, description, difficulty, language, expectedFindings?, testCases?, hints? }` | `{ valid, errors[] }` | `challenges_tool.go:404` |

> **Naming rules to remember:**
> - `mcp-curriculum.*` and `mcp-pedagogical.*` use **snake_case** arg keys (`topic_id`, `user_level`).
> - `mcp-challenges.*` uses **camelCase** arg keys (`topicId`, `reveal`, `sourceRepo`).
> - All five `mcp-challenges.*` tools require an authenticated `user_id` in context
>   (injected by the MCP bridge); missing one returns JSON-RPC `-32602`.

## Resources

- **Spec (canonical):** `openspec/specs/free-practice/spec.md` (synced from `archive/2026-10-10-free-practice-mode/`)
- **Spec archive:** `openspec/changes/archive/2026-10-10-free-practice-mode/` (proposal, design, tasks, verify-report)
- **Wire shapes:** `backend/internal/infrastructure/mcp/{challenges,curriculum,pedagogical}/*_tool.go`
- **LLM cascade:** `backend/internal/infrastructure/driven/llm_cascade/` (Groq → DeepSeek → MiniMax via `LLMClientPort.Complete`)
- **Sandbox gate:** `backend/internal/infrastructure/driven/sandbox/{tsrunner,dockersandbox,localsandbox}.go` (`ExecuteWithTests`)
- **Frontend caller:** `frontend/codeauditor/src/app/application/practice.use-case.ts` (signals + daily cap + `mcp-challenges.generate_challenge`)
- **Doc:** `docs/business-domain.md` §3.2 (free practice mode flow)
- **Doc:** `docs/llm-strategy.md` (cascade selection)
- **Doc:** `docs/curricula-format.md` (`.atl/curricula/<lang>.md` schema)
- **Doc:** `docs/mcp-tools.md` (full MCP tool catalogue)
- **Harness:** `.atl/harnesses/add-curriculum-topic.harness.md` (when adding a new language)
