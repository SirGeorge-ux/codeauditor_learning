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
1. mcp-curriculum.get_topic("typescript", "inheritance")
   ↓
2. mcp-pedagogical.get_learning_objective("typescript", "inheritance", "D")
   → exercise, prerequisites, restrictions, common_mistakes
   ↓
3. mcp-pedagogical.generate_story_context("typescript", "inheritance")
   → "Eres un dev en una startup de finanzas..."
   ↓
4. LLM (Groq qwen-2.5-coder-32b) generates a challenge:
   - given: story_context + learning_objective + restrictions
   - output: { code (vulnerable), expected_findings, solution_code, test_cases, linter_rules, hints }
   ↓
5. mcp-pedagogical.validate_solution (loop, max 3 attempts):
   - Run solution_code in sandbox with test_cases
   - If fails, send error back to LLM and regenerate
   ↓
6. mcp-challenges.validate_challenge (sanity checks):
   - expected_findings mentions things that are actually in code
   - hints are progressive (1, 2, 3)
   - difficulty matches level
   ↓
7. Persist as Challenge with origin="generated"
   ↓
8. Return to user (open in /dojo/<tempId>)
```

## Step-by-step (when implementing or debugging)

### Step 1 — Detect language

The user says "Herencias en TypeScript". Split into:
- language = "typescript"
- topic = "inheritance" (or "Herencias" → mapped to "inheritance")

If the topic is in Spanish and the curriculum is in English, use the topic's `display_name_es` or map manually.

If language is missing, ask the user (or default to their `most_used_language` from `LearningProfile`).

If topic is missing, ask the user (or offer 3 topics from their current level).

### Step 2 — Get the topic from curriculum

```go
topic, err := mcpCurriculum.GetTopic(ctx, "typescript", "inheritance")
// Returns: { slug, difficulty, prerequisites, description, exercises, common_mistakes, ... }
```

If the topic doesn't exist in the curriculum:

```go
// Try to suggest it via the LLM
suggestion, err := mcpCurriculum.SuggestTopic(ctx, "typescript", "Herencias")
// Returns a proposed topic for user review
```

**Don't auto-create the topic.** Show the suggestion to the user and let them approve.

### Step 3 — Get the learning objective

```go
objective, err := mcpPedagogical.GetLearningObjective(ctx, "typescript", "inheritance", "D")
// Returns:
// {
//   exercise: "Crea una clase Animal con un método speak(). Crea una subclase Dog extends Animal...",
//   prerequisites: ["classes"],
//   restrictions: ["No uses generics todavía", "No uses decorators"],
//   common_mistakes: ["Olvidar super() en el constructor", "No llamar a super.speak() en el override"],
//   estimated_difficulty: "D"
// }
```

The `restrictions` are CRITICAL. They prevent the LLM from generating a challenge that requires concepts the user doesn't know yet.

### Step 4 — Generate the story context

```go
context, err := mcpPedagogical.GenerateStoryContext(ctx, "typescript", "inheritance")
// Returns: "Eres un desarrollador en una startup de fintech..."
```

PBL (Problem-Based Learning) makes the exercise more engaging.

### Step 5 — Generate the challenge with the LLM

Use the LLM cascade (see `docs/llm-strategy.md`). Default: `qwen-2.5-coder-32b` (Groq free tier).

```python
# Pseudocódigo del prompt
system = load_prompt("tutor-generator-system.md")
prompt = f"""
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
response = llm.generate(system, prompt, json_mode=True)
challenge = parse_json(response)
```

### Step 6 — Validate the solution (loop, max 3 attempts)

```go
for attempt := 1; attempt <= 3; attempt++ {
    result, err := mcpPedagogical.ValidateSolution(ctx, language, challenge.SolutionCode, testCases)
    if err == nil && result.Valid {
        break
    }
    // If failed, send the error back to the LLM and regenerate
    feedback := fmt.Sprintf("The solution code failed: %s. Please fix.", result.Feedback)
    challenge = regenerate(prompt + feedback)
}
```

If all 3 attempts fail, **discard the challenge and try a different topic or level**. Don't serve a broken challenge.

### Step 7 — Validate the challenge structure

```go
if err := mcpChallenges.ValidateChallenge(ctx, challenge); err != nil {
    return err // has detailed error
}
```

Checks:
- `expected_findings` mentions things that are actually in `code` (regex match or LLM check).
- `hints` are progressive (level 1, 2, 3 with cost 0, 10, 25).
- `difficulty` matches `level` (F=junior, E=junior-mid, D=mid, C=mid-senior, B=senior, A=senior-architect, S=architect).
- `code` is not identical to `solution_code` (the smell must be present).
- `code` length is 20-200 lines.

### Step 8 — Persist and return

```go
challenge.Origin = "generated"
challenge.GeneratedBy = "openrouter-groq-qwen-2.5-coder-32b"
challenge.CreatedBy = userID
err := mcpChallenges.CreateChallenge(ctx, challenge)
```

Return the challenge to the user (open in `/dojo/<tempId>`).

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

- ❌ Don't auto-create topics in the curriculum. Always show suggestions for user review.
- ❌ Don't skip the validation loop. A broken challenge destroys trust.
- ❌ Don't serve the same challenge twice in a row (cache by hash of topic+level+userID).
- ❌ Don't expose the LLM API key in the prompt or the response.
- ❌ Don't use MiniMax M3 by default. Use the OpenRouter cascade.
- ❌ Don't ignore the `restrictions` from `get_learning_objective` (they're the pedagogical filter).
- ❌ Don't generate a challenge without a `story_context` (PBL > dry exercise).

## Resources

- **Spec:** `openspec/changes/free-practice-mode/`
- **Doc:** `docs/business-domain.md` §3.2
- **Doc:** `docs/llm-strategy.md` (cascade)
- **Doc:** `docs/curricula-format.md` (topics)
- **Doc:** `docs/mcp-tools.md` §3.6 (`mcp-pedagogical`)
- **Harness:** `.atl/harnesses/add-curriculum-topic.harness.md`
