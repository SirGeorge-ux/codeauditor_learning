---
name: codeauditor-dictionary
description: Use this skill when an agent needs to interact with the personal tech glossary (add, search, list terms). Trigger phrases include "diccionario", "dictionary", "glosario", "glossary", "qué es hardcode", "qué es PR", "add term", "search term", "definir término".
---

# Personal Tech Dictionary (S2+)

> **Outcome:** the user can add, search, and list personal tech terms in their private glossary.
> **Single-user:** all terms are owned by the current user.
> **Storage:** `dictionary_terms` table in Postgres.

## When to load

Load this skill when the user asks:
- "Qué es hardcode"
- "Añade el término PR al diccionario"
- "Busca en el diccionario"
- "Diccionario"
- "Glosario"
- "Lista de términos"

## Pre-flight

1. Read `AGENTS.md` (root).
2. Read `docs/business-domain.md` §2.5 (Dictionary entity, if exists).
3. The `/dictionary` route is in `/dictionary` (search) and `/dictionary/:term` (detail).

## The data model

```typescript
interface Term {
  id: string;                  // UUID
  user_id: string;
  term: string;                // 'hardcode', 'PR', 'race condition'
  explanation: string;         // 'Escribir valores literales en el código...'
  category: 'Concepto' | 'Patrón' | 'Herramienta' | 'Abreviatura' | 'Anti-patrón' | 'Otro';
  example?: string;            // 'if (status === 0) en vez de if (status === STATUS.OK)'
  synonyms: string[];          // ['hardcodear', 'hard-coded']
  tags: string[];              // ['javascript', 'general']
  created_at: Date;
  updated_at: Date;
}
```

## Categories (predefined)

| Categoría | Ejemplos |
|---|---|
| **Concepto** | closure, hoisting, async, promise, callback |
| **Patrón** | Singleton, Observer, Factory, Strategy, Repository |
| **Herramienta** | Docker, Kubernetes, ESLint, Prettier, Jest |
| **Abreviatura** | PR, MR, WIP, LGTM, RFC, SLA, TDD, BDD |
| **Anti-patrón** | God object, Spaghetti code, Magic numbers, Yo-yo problem |
| **Otro** | Cualquier otra cosa |

## Operations

### Add a term

**Frontend:** `/dictionary` → "Add" button → form.

**Backend:** `POST /api/v1/dictionary` with body:

```json
{
  "term": "hardcode",
  "explanation": "Escribir valores literales en el código en vez de usar constantes o variables con nombre.",
  "category": "Concepto",
  "example": "if (status === 0) en vez de if (status === STATUS.OK)",
  "synonyms": ["hardcodear", "hard-coded"],
  "tags": ["javascript", "general"]
}
```

**Validation:**
- `term` is required, max 50 chars, lowercase.
- `explanation` is required, min 10 chars, max 500.
- `category` must be one of the predefined.
- `example` is optional but recommended.
- `synonyms` and `tags` are optional.

**MCP tool:** `mcp-dictionary.add_term`.

### Search a term

**Frontend:** search bar at the top of `/dictionary`.

**Backend:** `GET /api/v1/dictionary?q=<query>` (full-text search on `term`, `explanation`, `synonyms`, `tags`).

**MCP tool:** `mcp-dictionary.search_term`.

**Algorithm:**
- Use Postgres `to_tsvector('english', term || ' ' || explanation || ' ' || array_to_string(synonyms, ' ') || ' ' || array_to_string(tags, ' '))`.
- Rank by `ts_rank` (higher = better match).
- Return top 20 results.

### Get term detail

**Frontend:** click on a result → `/dictionary/:term`.

**Backend:** `GET /api/v1/dictionary/:term` (or by id).

**MCP tool:** `mcp-dictionary.get_term`.

### List all terms

**Frontend:** `/dictionary` shows A-Z grid + filters by category.

**Backend:** `GET /api/v1/dictionary?category=<category>&tag=<tag>`.

**MCP tool:** `mcp-dictionary.list_terms`.

### Edit a term

**Frontend:** click "Edit" on the term detail page.

**Backend:** `PUT /api/v1/dictionary/:id`.

### Delete a term

**Frontend:** click "Delete" → confirm.

**Backend:** `DELETE /api/v1/dictionary/:id`.

## UI components

### `/dictionary` (search + list)

```
┌────────────────────────────────────────────────────┐
│ Search: [hardcode___________]  [Add new]          │
├────────────────────────────────────────────────────┤
│ A-Z: A B C D E F G H I J K L M N O P Q R S T...  │
├────────────────────────────────────────────────────┤
│ Filter: [All ▼] [Concepto] [Patrón] [...]         │
├────────────────────────────────────────────────────┤
│ HARDCODE                                          │
│ Categoría: Concepto                                │
│ "Escribir valores literales..."                    │
│ Synonyms: hardcodear, hard-coded                   │
│ Tags: javascript, general                          │
│ [View] [Edit]                                     │
├────────────────────────────────────────────────────┤
│ ...                                               │
└────────────────────────────────────────────────────┘
```

### `/dictionary/:term` (detail)

```
┌────────────────────────────────────────────────────┐
│ ← Back                                            │
│                                                    │
│ HARDCODE                                          │
│ Categoría: Concepto                                │
│                                                    │
│ Explanation:                                       │
│ "Escribir valores literales en el código en vez   │
│  de usar constantes o variables con nombre."       │
│                                                    │
│ Example:                                           │
│ if (status === 0) en vez de                        │
│ if (status === STATUS.OK)                           │
│                                                    │
│ Synonyms: hardcodear, hard-coded                   │
│ Tags: javascript, general                          │
│                                                    │
│ [Edit] [Delete]                                    │
└────────────────────────────────────────────────────┘
```

### "Add new" modal

```
┌────────────────────────────────────────────────────┐
│ Add new term                              [×]     │
├────────────────────────────────────────────────────┤
│ Term*: [_______]                                   │
│ Category*: [Concepto ▼]                            │
│ Explanation*:                                      │
│ [_____________________________________________]   │
│ [_____________________________________________]   │
│ [_____________________________________________]   │
│ Example:                                           │
│ [_____________________________________________]   │
│ Synonyms (comma-separated): [_______]              │
│ Tags (comma-separated): [_______]                  │
│                                                    │
│                          [Cancel]  [Save]         │
└────────────────────────────────────────────────────┘
```

## Integration with the tutor chat

The tutor chat can search the dictionary automatically when the user asks "qué es X":

```
User: "Qué es PR?"
   ↓
Tutor: mcp-dictionary.search_term("PR")
   ↓
   Found: PR = "Pull Request" in "Abreviatura"
   ↓
Tutor: "PR (Pull Request) es una propuesta de cambio..."
```

This is one of the `mcp-dictionary` tool consumers.

## Don't do

- ❌ Don't allow multi-user terms (single-tenant).
- ❌ Don't let the user delete someone else's term (no one else exists, but be safe).
- ❌ Don't allow terms >50 chars (typos usually).
- ❌ Don't allow empty explanation (min 10 chars).
- ❌ Don't auto-suggest categories (the user knows better).
- ❌ Don't suggest terms the user has already added.
- ❌ Don't show the dictionary in the tutor chat unless the user asks.

## Resources

- **Spec:** `openspec/changes/dictionary/`
- **Doc:** `docs/business-domain.md` §2.5
- **Doc:** `docs/mcp-tools.md` §3.5
- **Skill:** `codeauditor-tutor-chat` (how the chat uses the dictionary)
- **Harness:** `add-dictionary-term.harness.md`
