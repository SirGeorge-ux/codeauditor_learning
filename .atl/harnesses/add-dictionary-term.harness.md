# Harness: Add a Term to the Dictionary

> **Step-by-step workflow** for adding a new term to the personal tech glossary.
> **Audience:** any user, devs, agents.
> **Estimated time:** 2-5 minutes.
> **Output:** a new term in the `dictionary_terms` table, visible in `/dictionary` and searchable by the tutor chat.

---

## Prerequisites

- [ ] You're logged in.
- [ ] The backend is running.

---

## Method 1: Via the UI (recommended)

1. Go to `/dictionary` in the frontend.
2. Click the **"Add new"** button (top right).
3. Fill in the form:

| Field | Required | Constraints | Example |
|---|---|---|---|
| **Term** | ✅ | max 50 chars, lowercase | `hardcode` |
| **Category** | ✅ | one of: Concepto, Patrón, Herramienta, Abreviatura, Anti-patrón, Otro | `Concepto` |
| **Explanation** | ✅ | min 10 chars, max 500 | `Escribir valores literales en el código en vez de usar constantes.` |
| **Example** | ❌ | max 500 chars | `if (status === 0) en vez de if (status === STATUS.OK)` |
| **Synonyms** | ❌ | comma-separated, max 5 | `hardcodear, hard-coded` |
| **Tags** | ❌ | comma-separated, max 5 | `javascript, general` |

4. Click **Save**.
5. Verify the term appears in the list and in search.

---

## Method 2: Via the API

```bash
curl -X POST http://localhost:8080/api/v1/dictionary \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "term": "hardcode",
    "category": "Concepto",
    "explanation": "Escribir valores literales en el código en vez de usar constantes o variables con nombre.",
    "example": "if (status === 0) en vez de if (status === STATUS.OK)",
    "synonyms": ["hardcodear", "hard-coded"],
    "tags": ["javascript", "general"]
  }'
```

Expected response (201 Created):

```json
{
  "id": "uuid",
  "user_id": "uuid",
  "term": "hardcode",
  "category": "Concepto",
  "explanation": "...",
  "example": "...",
  "synonyms": ["hardcodear", "hard-coded"],
  "tags": ["javascript", "general"],
  "created_at": "2026-07-22T...",
  "updated_at": "2026-07-22T..."
}
```

---

## Method 3: Via the MCP tool

```bash
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": "1",
    "method": "tools/call",
    "params": {
      "name": "mcp-dictionary.add_term",
      "arguments": {
        "term": "hardcode",
        "category": "Concepto",
        "explanation": "...",
        "example": "...",
        "synonyms": ["hardcodear"],
        "tags": ["javascript"]
      }
    }
  }'
```

---

## Validation rules

| Rule | What happens if violated |
|---|---|
| `term` is empty | 400 Bad Request: "term is required" |
| `term` > 50 chars | 400: "term must be at most 50 characters" |
| `term` contains uppercase | Auto-lowercased (or 400 if strict) |
| `term` already exists for the user | 409 Conflict: "term already exists" |
| `explanation` < 10 chars | 400: "explanation must be at least 10 characters" |
| `explanation` > 500 chars | 400: "explanation must be at most 500 characters" |
| `category` not in the list | 400: "category must be one of: ..." |
| `synonyms` > 5 | 400: "synonyms must be at most 5" |
| `tags` > 5 | 400: "tags must be at most 5" |

---

## Edit a term

**UI:** `/dictionary/<term>` → click **Edit** → modify → **Save**.

**API:** `PUT /api/v1/dictionary/<id>` with the new fields.

---

## Delete a term

**UI:** `/dictionary/<term>` → click **Delete** → confirm.

**API:** `DELETE /api/v1/dictionary/<id>`.

**Note:** deletion is soft (the term is marked as deleted, not removed) for 30 days. Then it's hard-deleted.

---

## Test the term end-to-end

1. Add the term via UI.
2. Go to `/dictionary` → verify it appears.
3. Use the search bar → search for part of the term → verify it appears.
4. Open the term → verify all fields are correct.
5. Open the tutor chat (if available) → ask "qué es <term>?" → verify the tutor uses the term.
6. Edit the term → verify the change persists.
7. Delete the term → verify it disappears.

---

## Categories (predefined)

| Categoría | Ejemplos |
|---|---|
| **Concepto** | closure, hoisting, async, promise, callback |
| **Patrón** | Singleton, Observer, Factory, Strategy, Repository |
| **Herramienta** | Docker, Kubernetes, ESLint, Prettier, Jest |
| **Abreviatura** | PR, MR, WIP, LGTM, RFC, SLA, TDD, BDD |
| **Anti-patrón** | God object, Spaghetti code, Magic numbers, Yo-yo problem |
| **Otro** | Cualquier otra cosa |

If you need a new category, propose it in the issue tracker.

---

## Best practices

- **Use lowercase.** `hardcode`, not `Hardcode` or `HARDCODE`.
- **Be concise in the explanation.** 1-3 sentences. The user can Google for more.
- **Always include an example.** It makes the term 10x clearer.
- **Use synonyms.** If you know the term has common variations (e.g. "hardcode" vs "hardcodear"), add them.
- **Tag appropriately.** Tags help you filter later.
- **Don't duplicate.** If the term exists, edit it instead of creating a new one.

---

## Don't do

- ❌ Don't add terms >50 chars (typo, probably).
- ❌ Don't add empty explanations.
- ❌ Don't use uppercase (auto-lowercased anyway).
- ❌ Don't add terms that are too niche (only you use it).
- ❌ Don't add terms with secrets or sensitive info.
- ❌ Don't add a term to a category that doesn't fit. If unsure, use "Otro".

---

## Resources

- **Spec:** `openspec/changes/dictionary/`
- **Doc:** `docs/business-domain.md` §2.5
- **Doc:** `docs/mcp-tools.md` §3.5
- **Skill:** `codeauditor-dictionary`
