# Harness: Add a Topic to a Language Curriculum

> **Step-by-step workflow** for adding a new topic to `.atl/curricula/<lang>.md`.
> **Audience:** devs, content creators, agents.
> **Estimated time:** 15-30 minutes per topic.
> **Output:** a new topic that the `mcp-curriculum` tool can serve and the `mcp-pedagogical.get_learning_objective` can use.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/curricula-format.md` (the canonical structure).
- [ ] The language already has a curriculum file (`.atl/curricula/<lang>.md`).
- [ ] You know the topic's difficulty (F-S) and prerequisites.
- [ ] You have at least 3 exercises and 2 common mistakes in mind.

---

## Step 1 — Open the curriculum file

```bash
code .atl/curricula/<lang>.md
```

Or in your favorite editor.

---

## Step 2 — Find the right place

The file is organized by **difficulty level**. Find the right section to add the new topic:

```markdown
# TypeScript Curriculum

## Topics

### variables       (F, no prerequisites)
### types           (E, prerequisites: [variables])
### classes         (E, prerequisites: [variables, types])
### inheritance     (D, prerequisites: [classes])
### generics        (C, prerequisites: [classes, types])
### async-await     (D, prerequisites: [types])
### promises        (D, prerequisites: [types, async-await])

(... añadir aquí ...)
```

Add the new topic **in the right difficulty order** (F first, then E, D, C, B, A, S).

---

## Step 3 — Write the topic block

Use the canonical structure from `docs/curricula-format.md`:

```markdown
### <topic-slug>

- **slug:** <topic-slug>
- **difficulty:** F | E | D | C | B | A | S
- **prerequisites:** [<other-topic-slug>, ...]
- **category:** syntax | types | oop | functional | async | concurrency | testing | tooling | patterns | architecture
- **description:** |
  <2-3 párrafos explicando QUÉ es y POR QUÉ importa.
  NO incluyas spoilers. La description es la LECCIÓN, no el EJERCICIO.
- **exercises:**
  - "<ejercicio 1 (sin código, solo el enunciado)>"
  - "<ejercicio 2>"
  - "<ejercicio 3>"
- **common_mistakes:**
  - "<error típico 1>"
  - "<error típico 2>"
- **resources:**
  - "<link a docs oficiales>"
  - "<link a MDN / libro / blog>"
```

### Example

```markdown
### decorators

- **slug:** decorators
- **difficulty:** C
- **prerequisites:** [classes, types, generics]
- **category:** oop
- **description:** |
  Decorators are a way to add metadata and behavior to classes, methods,
  and properties. TypeScript supports them natively (with `experimentalDecorators`)
  and they're heavily used in frameworks like Angular and NestJS.

  A decorator is a function that receives the target (class, method, etc.)
  and can modify or replace it. They're applied with `@decoratorName` syntax.
- **exercises:**
  - "Create a `@log` decorator that logs when a method is called."
  - "Create a `@readonly` decorator for class properties."
  - "Use a parameter decorator to validate method arguments."
- **common_mistakes:**
  - "Forgetting to enable `experimentalDecorators` in tsconfig."
  - "Confusing decorators with higher-order functions."
  - "Using decorators for everything (over-engineering)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/decorators.html
  - https://angular.io/guide/dependency-injection-nav-tree
```

---

## Step 4 — Verify with the linter

```bash
.atl/curricula/lint.sh
```

The linter checks:
- Frontmatter is valid YAML.
- Every topic has `slug`, `difficulty`, `prerequisites`, `exercises`.
- Every `prerequisites` slug exists in the file.
- Difficulty order is consistent (F → E → D → C → B → A → S).

If the linter fails, fix and re-run.

---

## Step 5 — Test the topic with the MCP tool

Once the backend is running, test the topic via the MCP:

```bash
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "tools/call",
    "params": {
      "name": "mcp-curriculum.get_topic",
      "arguments": {
        "language": "typescript",
        "topic": "decorators"
      }
    }
  }'
```

Expected response:

```json
{
  "slug": "decorators",
  "difficulty": "C",
  "prerequisites": ["classes", "types", "generics"],
  "description": "...",
  "exercises": [...],
  "common_mistakes": [...]
}
```

---

## Step 6 — Test the learning objective generation

```bash
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "2",
    "method": "tools/call",
    "params": {
      "name": "mcp-pedagogical.get_learning_objective",
      "arguments": {
        "language": "typescript",
        "topic": "decorators",
        "level": "C"
      }
    }
  }'
```

Verify:
- `exercise` is one of the 3 exercises.
- `prerequisites` matches.
- `restrictions` lists what the user should NOT use yet.
- `common_mistakes` is populated.

---

## Step 7 — Test in the UI (optional)

1. Start backend + frontend.
2. Go to `/practice/free/typescript`.
3. Type "decorators nivel C".
4. Verify a challenge is generated based on the topic.

---

## Step 8 — Commit

```bash
git add .atl/curricula/<lang>.md

git commit -m "docs(curriculum): add <topic-slug> to <lang>

- Difficulty: <F|E|D|C|B|A|S>
- Prerequisites: [<prereq1>, <prereq2>]
- Category: <category>
- 3 exercises
- 3 common mistakes
- <N> resources

Linter: OK
MCP test: OK"
```

---

## Checklist

- [ ] Topic added in the right difficulty order.
- [ ] Slug is unique and kebab-case.
- [ ] All required fields present.
- [ ] Prerequisites are valid (exist in the same file).
- [ ] Description doesn't spoil the answer.
- [ ] Exercises are statements, not code.
- [ ] Common mistakes are real errors, not theoretical.
- [ ] `.atl/curricula/lint.sh` passes.
- [ ] `mcp-curriculum.get_topic` returns the topic.
- [ ] `mcp-pedagogical.get_learning_objective` returns the right objective.
- [ ] Conventional commit + clear message.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Slug contains spaces or uppercase | Use kebab-case lowercase: `async-await`, not `Async Await`. |
| Prerequisites reference a topic that doesn't exist | Add the prerequisite first, or remove the reference. |
| Description contains the answer | Re-read: would I know the answer just from the description? If yes, rewrite. |
| Exercises are too similar | Vary the difficulty or the focus (e.g. one about creation, one about debugging). |
| Common mistakes are theoretical | They should be REAL errors that juniors typically make. |
| Difficulty is too low for the topic | If the topic requires `generics` (C), the difficulty is at least D. |
| Resources are broken links | Test all links before committing. |
| Topic is added in the wrong order | Use the linter to verify the order is F → E → D → C → B → A → S. |

## Resources

- **Doc:** `docs/curricula-format.md` (full format spec)
- **Doc:** `docs/mcp-tools.md` §3.4 (`mcp-curriculum`)
- **Doc:** `docs/mcp-tools.md` §3.6 (`mcp-pedagogical`)
- **Spec:** `openspec/changes/free-practice-mode/`
