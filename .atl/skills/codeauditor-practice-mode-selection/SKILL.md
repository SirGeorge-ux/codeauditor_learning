---
name: codeauditor-practice-mode-selection
description: Use this skill when an agent or user needs to decide which of the 3 practice modes (Curated, Free practice, Code health) to use. Trigger phrases include "qué modo", "free vs curated", "code health", "qué hago primero", "which mode", "I want to practice", "quiero practicar", "quiero mejorar mi código", "auditar mi repo".
---

# Practice Mode Selection

> **Outcome:** the user picks the right practice mode for their goal and lands on the right page.
> **Time estimate:** 30 seconds.
> **Used:** every time the user lands on `/` (home) or asks "¿qué hago?".

## When to load

Load this skill when the user asks:
- "Qué modo de práctica me conviene"
- "Free vs curated vs code health"
- "Quiero practicar"
- "Quiero mejorar mi código"
- "Auditar mi repo"
- "¿Por dónde empiezo?"

Or when the user is on `/` (home) and the page is showing the 3 mode cards.

## The 3 modes (summary)

| Mode | URL | Para qué | Input | Output |
|---|---|---|---|---|
| **Curated** | `/dashboard` | Practicar code smells conocidos con rúbrica | Elige de 8+ challenges | Score reproducible |
| **Free practice** | `/practice/free/:lang` | Aprender/avanzar en un lenguaje con ejercicios on-demand | Elige lenguaje + nivel/topic | Challenge generado por IA |
| **Code health** | `/repo/:owner/:name/code-health` | Analizar y mejorar un repo real | Elige repo (Gogs o GitHub) | Health score + issues rankeados + challenges derivados |

## Decision tree

```
¿El user tiene un repo específico que quiere mejorar?
├─ SÍ → Code health (Gogs o GitHub)
└─ NO ↓

¿El user quiere aprender/avanzar en un lenguaje concreto?
├─ SÍ → Free practice (elige lenguaje + nivel)
└─ NO ↓

¿El user solo quiere "practicar code smells"?
└─ → Curated (8 challenges curados con rúbrica)
```

## Recommendation matrix

| Si el user dice... | Recomienda... | Razón |
|---|---|---|
| "Quiero practicar" | Empezar por **Curated** (1-2 challenges) | Base sólida, score reproducible |
| "Soy junior en X" | **Free practice** X nivel F-E | Ejercicios adaptados al nivel |
| "Quiero aprender Rust" | **Free practice** rust nivel F | Currícula de Rust desde cero |
| "Mi código está hecho un asco" | **Code health** | Análisis automático del repo real |
| "Tengo un PR pendiente" | **Code health** del repo del PR | Detecta issues antes del review |
| "Quiero puntos" | **Curated** + completa rápido | Score reproducible, base + bonus + time |
| "Estoy aburrido" | **Free practice** nuevo lenguaje | Exploración, sin presión |
| "Quiero un desafío" | **Curated** senior/architect | Smells sutiles |
| "No sé qué elegir" | **Curated** `ch-sqli` (junior) | El más claro, el más educativo |
| "Quiero mejorar mi inglés" | **Free practice** en `en` | El chat puede cambiar idioma |

## What to do

### Step 1 — Ask the user 1 question (max)

If the user's intent is unclear, ask ONE question. Never more.

> "¿Quieres practicar sobre código que ya tienes, o quieres algo nuevo generado para ti?"

- "Código que ya tengo" → Code health
- "Algo nuevo" → Free practice o Curated

### Step 2 — Suggest the mode + the first concrete action

Don't just say "use Curated". Say:

> "Te recomiendo empezar con **Curated mode** y abrir `ch-sqli` (junior, SQL Injection). Es el más educativo y el scoring es reproducible. ¿Vamos?"

### Step 3 — Navigate

If the agent can navigate (e.g. through the UI or via a tool), do it. If not, give the user the URL.

## Don't do

- ❌ Don't suggest just 1 mode when 2 are equally good. Present the trade-off.
- ❌ Don't suggest Code health to a user without a repo. They'll be stuck.
- ❌ Don't suggest Free practice to a user who hasn't done Curated first. They'll be overwhelmed.
- ❌ Don't suggest Curated to a user who clearly wants their own code analyzed.
- ❌ Don't push the user to "the hardest mode". Pedagogy > difficulty.
- ❌ Don't make the decision for the user without explaining why.

## Red flags

- The user is on Free practice but has rango F in 10 languages → maybe they need Curated to ground.
- The user is on Curated only and never uses Code health → maybe their code is suffering.
- The user is on Code health only → maybe they need to learn patterns first.

## Next skills to load

After the user picks a mode, load:
- Curated → `codeauditor-add-challenge` (or no skill, just normal flow)
- Free practice → `codeauditor-free-practice-generator`
- Code health → `codeauditor-code-health`

## UI integration

The home page (`/`) should show 3 cards, one per mode. Each card has:
- Title + icon.
- 1-sentence description.
- 1 example of what they'd do.
- "Empezar" button → navigates to the mode.

```
┌──────────────────────────────────────────────────────┐
│  ¿Qué quieres hacer hoy?                            │
├──────────────────────────────────────────────────────┤
│  📚 Curated                                          │
│  Practica 8+ code smells con rúbrica y tests.        │
│  Ejemplo: "Detecta el N+1 query en este código."     │
│  [Empezar →]                                         │
├──────────────────────────────────────────────────────┤
│  🎯 Free practice                                    │
│  Elige un lenguaje y la IA genera ejercicios        │
│  adaptados a tu nivel.                               │
│  Ejemplo: "Dame un ejercicio de Herencias en TS."     │
│  [Empezar →]                                         │
├──────────────────────────────────────────────────────┤
│  🏥 Code health                                      │
│  Analiza tu repo (Gogs o GitHub) y la IA detecta     │
│  code smells, refactors y mejoras.                   │
│  Ejemplo: "Analiza academy-mic y dame los 5 issues." │
│  [Empezar →]                                         │
└──────────────────────────────────────────────────────┘
```
