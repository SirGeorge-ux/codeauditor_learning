---
name: codeauditor-onboarding
description: Use this skill when an agent or junior dev lands on the CodeAuditor repo for the first time and needs to understand the project before doing anything. Trigger phrases include "onboarding", "first time in this repo", "explícame el proyecto", "qué hace este repo", "cómo está organizado", "what does this project do", "where do I start", "qué modo de práctica", "free vs curated vs code health".
---

# CodeAuditor Onboarding (S0+)

> **Audience:** new agents, new dev, anyone landing on this repo without prior context.
> **Outcome:** the agent knows what CodeAuditor is, the 3 practice modes, where to look, what the rules are, and what NOT to do.

## When to load

Load this skill when the user asks any of:
- "Explícame este proyecto"
- "Onboarding"
- "First time in this repo"
- "Where do I start?"
- "What does this project do?"
- "What stack do we use?"
- "¿Por dónde empiezo?"
- "Qué modo de práctica me conviene"
- "Free vs curated vs code health"

## What to do (step by step)

### Step 1 — Read the system prompt

Read `/AGENTS.md` (root). This is the canonical source of truth for the project's rules, stack, and architecture.

If `/AGENTS.md` is missing or stale, **stop and tell the user** — the project is in a broken state.

### Step 2 — Skim the docs

Read in this order (skim, don't memorize):

1. `docs/business-domain.md` — what the product IS, who it's for, the 3 practice modes, the new Challenge model.
2. `docs/hexagonal-architecture.md` — the architectural rule (dual hexagonal, Go + TS).
3. `docs/ui-and-stack.md` — only if you'll touch frontend.
4. `docs/sandbox-providers.md` — only if you'll add a language.
5. `docs/llm-strategy.md` — only if you'll touch the LLM cascade.
6. `docs/mcp-tools.md` — only if you'll add an MCP tool.
7. `docs/curricula-format.md` — only if you'll add a curriculum topic.

Each doc is ~10-15 min of reading. Total skim: 60-90 min.

### Step 3 — Map the current sprint

Look at `openspec/changes/`. Any folder there = work in progress.

- Read `proposal.md` of the active changes.
- Read `tasks.md` to see what's `[x]` (done) and what's `[ ]` (todo).
- Check the roadmap in `AGENTS.md` §12 to know where you are in S0-S6.

If you find a change with all tasks `[x]` and code already on master, **suggest archiving it** (move to `openspec/changes/archive/`).

### Step 4 — Confirm the stack is what AGENTS.md says

```bash
# Quick stack verification
cat frontend/codeauditor/package.json | grep -E '"@angular/(core|common)"'
cat backend/go.mod | head -5
cat openspec/changes/*/specs/*/spec.md | head -30
```

- Angular should be 21.x (NOT 17.x — the old AGENTS.md was wrong).
- Go should be 1.23+.
- There should be 8 specs planned for S0-S6 (challenge-rebuild, language-progress, dictionary, tutor-chat, mcp-monolith, free-practice-mode, code-health-dashboard, llm-cascade).

If versions don't match, **flag it to the user** before doing anything.

### Step 5 — Check quality gates work

```bash
make validate
```

This should pass cleanly. If it fails, **don't try to fix it without understanding why**. Read the error, propose, then act.

### Step 6 — Orient yourself in the code

| Goal | Look at |
|---|---|
| Understand the audit flow | `backend/internal/core/services/audit_service.go` + `frontend/.../infrastructure/services/audit.service.ts` |
| Understand the challenges (nuevo modelo) | `docs/business-domain.md` §2.2 + `db/migrations/` (search for `expected_findings`) |
| Understand the sandbox | `backend/internal/infrastructure/driven/sandbox/providers/registry.go` |
| Understand the UI flow | `frontend/.../infrastructure/components/dojo/dojo-page.component.ts` |
| Understand openspec | `openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/` (best example) |
| Understand the MCP tools (S2) | `docs/mcp-tools.md` + `backend/internal/infrastructure/mcp/` (when exists) |
| Understand the currícula (S1) | `.atl/curricula/typescript.md` (when exists) |
| Understand the LLM cascade (S5) | `docs/llm-strategy.md` + `config/llm-cascade.yaml` (when exists) |

### Step 7 — Decide which practice mode to suggest

The user has 3 modes. Help them pick based on their goal:

| If the user wants... | Suggest... |
|---|---|
| "Practicar lo clásico, code smells conocidos" | **Curated mode** (`/dashboard`) |
| "Aprender un lenguaje desde cero con ejercicios" | **Free practice** (`/practice/free`) |
| "Mejorar mi propio código / repo" | **Code health** (`/repo/:owner/:name/code-health`) |
| "No sé por dónde empezar" | Curated (los 8 challenges nuevos dan una base sólida) |
| "Quiero ser mejor en X lenguaje" | Free practice (la IA adapta el nivel) |
| "Auditar un proyecto real" | Code health (análisis automático del repo) |

Load the relevant skill:
- For Curated: `codeauditor-add-challenge` (or just the normal flow).
- For Free practice: `codeauditor-free-practice-generator`.
- For Code health: `codeauditor-code-health`.

## Don't do

- ❌ Don't propose "let's set up the backend" if there is one.
- ❌ Don't propose Angular 17 or NgModules.
- ❌ Don't propose switching off Tailwind.
- ❌ Don't try to add Tailwind config.
- ❌ Don't expose tokens in frontend.
- ❌ Don't start coding without reading AGENTS.md.
- ❌ Don't suggest just 1 practice mode — the user has 3, present all of them.
- ❌ Don't spoil the curated challenges' answers.

## Red flags (tell the user, don't act)

- AGENTS.md doesn't exist or is severely outdated.
- `make validate` fails.
- Active spec has all tasks `[x]` and is not archived.
- Newer versions of Angular exist than 21.
- Go version in go.mod is not 1.23+.
- The 8 specs of S0-S6 don't exist as proposals (debate cerrado, spec creado).
- No curriculum exists for a language the user wants to practice (`.atl/curricula/<lang>.md`).

## Success criteria

After loading this skill, you should be able to answer:
- What is CodeAuditor in 1 sentence.
- What are the 3 practice modes and when to use each.
- What stack is it on (frontend, backend, DB, sandbox, LLM).
- Where the hexagonal layers live (Go and TS).
- What's the current sprint (S0-S6, active spec in `changes/`).
- Where to look to start a specific task.
- Which of the 12 skills to load next.
