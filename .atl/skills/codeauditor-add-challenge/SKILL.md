---
name: codeauditor-add-challenge
description: Use this skill when an agent needs to add a new curated challenge to CodeAuditor with the NEW model (expected_findings, hints, solution_code, test_cases, linter_rules). Trigger phrases include "add challenge v2", "nuevo reto pedagógico", "challenge con rúbrica", "smell con tests", "ejercicio de X con scoring". Does NOT apply to free practice mode (use codeauditor-free-practice-generator instead).
---

# Add a Curated Challenge (S1+ new model)

> **Outcome:** a new challenge persisted in Postgres with the **new model**: `expected_findings`, 3 progressive `hints`, `solution_code`, `test_cases`, `linter_rules`, scoring reproducible.
> **Time estimate:** 30-60 minutes per challenge.
> **PR size:** 1 SQL migration + 1 commit.

## When to load

Load this skill when the user asks:
- "Add a challenge for X smell (curated)"
- "Nuevo reto pedagógico"
- "Challenge con rúbrica"
- "Quiero un smell de X"

**Do NOT use this for free practice mode** (challenges generated on-demand) — use `codeauditor-free-practice-generator` instead.

## Pre-flight

1. Read `AGENTS.md` (root) if you haven't.
2. Read `docs/business-domain.md` §2.2 (new Challenge model).
3. Read `docs/curricula-format.md` (the canonical topic format — challenges are derived from topics).
4. Check the 8 existing challenges (in `db/seed/01_challenges.sql` or the new `challenges` table).

## Step 1 — Pick a smell NOT in the existing 8

Existing 8 (deprecated model, will be migrated in S1):
1. ch-sqli (SQL Injection)
2. ch-xss (XSS)
3. ch-god (God Function)
4. ch-callback (Callback Hell)
5. ch-mutation (Prop Mutation)
6. ch-dead (Dead Code)
7. ch-errors (Silent Failures)
8. ch-naming (Poor Naming)

**Find a smell not in this list** (or migrate one of these to the new model).

Examples of smells NOT yet covered: N+1 Query, Memory Leak, Race Condition, Leaky Abstraction, Null Object Pattern Missing, Off-by-one Error, Magic String, Inappropriate Intimacy, Long Parameter List, Primitive Obsession, Refused Bequest, Speculative Generality.

## Step 2 — Design the challenge

Answer these 5 questions:

1. **Smell:** what is the user going to detect? (must be a NEW smell or a new angle of an existing one)
2. **Language:** what language? Must be supported in the sandbox.
3. **Difficulty:** junior / mid / senior / architect?
4. **Category:** security / performance / refactor / style / concurrency / architecture?
5. **Code:** the vulnerable code, ~20-50 lines, realistic, runnable.

### Difficulty guidelines

| Difficulty | Smell clarity | Code complexity | Time to solve |
|---|---|---|---|
| **junior** | Hinted in description | Simple functions | 5-10 min |
| **mid** | Noticeable, requires thought | Multiple functions | 10-20 min |
| **senior** | Subtle | Cross-cutting | 20-40 min |
| **architect** | Architectural | Multiple files | 40+ min |

## Step 3 — Write the Challenge (YAML first, then SQL)

Use the new model from `docs/business-domain.md` §2.2. Best practice: write it as a YAML first for human review, then convert to SQL.

### 3.1 — YAML format (recommended for design)

```yaml
id: ch-n-plus-one
title: N+1 Query en historial de usuario
description: |
  Esta función carga el historial de pedidos de un usuario. Funciona
  correctamente, pero con muchos pedidos el tiempo de respuesta crece
  linealmente. Detecta el problema y propón una solución.
  # NO spoiler: no digas "es un N+1", no menciones el patrón.
difficulty: mid
category: performance
language: typescript
learning_objectives:
  - "Entender el patrón N+1"
  - "Aplicar JOINs o batch loading"
estimated_time_minutes: 15
code: |
  import { db } from './database';
  export async function getUserOrderHistory(userId: string) {
    const user = await db.query('SELECT * FROM users WHERE id = $1', [userId]);
    const orders = await db.query('SELECT * FROM orders WHERE user_id = $1', [userId]);
    const items = [];
    for (const order of orders.rows) {
      const orderItems = await db.query('SELECT * FROM order_items WHERE order_id = $1', [order.id]);
      items.push({ ...order, items: orderItems.rows });
    }
    return { ...user.rows[0], orders: items };
  }
expected_findings:
  - severity: high
    category: performance
    message: "N+1 query: cada order dispara una query adicional a order_items"
    evidence: "for (const order of orders.rows) { await db.query(...) }"
    suggested_fix: "Usar un JOIN o un IN clause: db.query('SELECT * FROM order_items WHERE order_id = ANY($1)', [orderIds])"
  - severity: medium
    category: performance
    message: "Posible N+1 también en getUser (1 query + N queries de orders)"
    evidence: "const user = await db.query(...)"
    suggested_fix: "Combinar en una sola query con LEFT JOIN"
hints:
  - level: 1
    content: "Cuenta cuántas queries a la base de datos se ejecutan con 100 pedidos."
    cost_points: 0
  - level: 2
    content: "Hay un patrón clásico para evitar N+1: cargar todo en una sola query."
    cost_points: 10
  - level: 3
    content: "La solución es un JOIN entre orders y order_items, o un WHERE order_id = ANY($1)."
    cost_points: 25
common_mistakes:
  - "Hacer solo 1 JOIN y dejar el otro loop"
  - "Olvidar el LIMIT o no paginar"
  - "Devolver datos que el frontend no necesita (over-fetching)"
test_cases:
  - name: "Resuelve en menos de 5 queries"
    input: { users_count: 1, orders_per_user: 100 }
    expected_output: { total_queries: 2 }
    weight: 5
  - name: "Mantiene la misma respuesta"
    input: { user_id: "u-1" }
    expected_output_matches: "getUserOrderHistory_baseline"
    weight: 3
linter_rules:
  - type: complexity-max
    config: { max: 5 }
  - type: no-n-plus-one
    config: { max_queries_in_loop: 1 }
solution_code: |
  import { db } from './database';
  export async function getUserOrderHistory(userId: string) {
    const user = await db.query('SELECT * FROM users WHERE id = $1', [userId]);
    const orders = await db.query(`
      SELECT o.*, json_agg(oi.*) as items
      FROM orders o
      LEFT JOIN order_items oi ON oi.order_id = o.id
      WHERE o.user_id = $1
      GROUP BY o.id
    `, [userId]);
    return { ...user.rows[0], orders: orders.rows };
  }
solution_explanation: |
  Un LEFT JOIN entre orders y order_items, agrupando los items con json_agg,
  resuelve el N+1 en una sola query. El user se carga en otra query porque
  típicamente no necesita JOINs adicionales.
base_points: 25
bonus_points: 30
penalty_per_hint: 0  # 0 porque los hints ya tienen cost_points
time_bonus: true
```

### 3.2 — Convert to SQL migration

`db/migrations/<NN>_<smell-slug>.sql`:

```sql
INSERT INTO challenges (
  id, title, description, difficulty, category, language,
  code, solution_code, solution_explanation,
  base_points, bonus_points, time_bonus,
  estimated_time_minutes,
  learning_objectives, hints, expected_findings,
  test_cases, linter_rules, common_mistakes,
  origin, created_at
) VALUES (
  'ch-n-plus-one',
  'N+1 Query en historial de usuario',
  'Esta función carga el historial de pedidos de un usuario. Funciona correctamente, pero con muchos pedidos el tiempo de respuesta crece linealmente. Detecta el problema y propón una solución.',
  'mid', 'performance', 'typescript',
  $code$
import { db } from './database';
export async function getUserOrderHistory(userId: string) {
  // ... (código vulnerable)
}
  $code$,
  $code$
import { db } from './database';
export async function getUserOrderHistory(userId: string) {
  // ... (solución canónica)
}
  $code$,
  'Un LEFT JOIN entre orders y order_items, agrupando con json_agg, resuelve el N+1 en una sola query.',
  25, 30, true,
  15,
  '["Entender el patrón N+1", "Aplicar JOINs o batch loading"]'::jsonb,
  '[
    {"level": 1, "content": "Cuenta cuántas queries a la base de datos se ejecutan con 100 pedidos.", "cost_points": 0},
    {"level": 2, "content": "Hay un patrón clásico para evitar N+1: cargar todo en una sola query.", "cost_points": 10},
    {"level": 3, "content": "La solución es un JOIN entre orders y order_items, o un WHERE order_id = ANY($1).", "cost_points": 25}
  ]'::jsonb,
  '[
    {"severity": "high", "category": "performance", "message": "N+1 query: cada order dispara una query adicional a order_items", "evidence": "for (const order of orders.rows) { await db.query(...) }", "suggested_fix": "Usar un JOIN o un IN clause"},
    {"severity": "medium", "category": "performance", "message": "Posible N+1 también en getUser", "evidence": "...", "suggested_fix": "Combinar en una sola query"}
  ]'::jsonb,
  '[
    {"name": "Resuelve en menos de 5 queries", "input": {"users_count": 1, "orders_per_user": 100}, "expected_output": {"total_queries": 2}, "weight": 5}
  ]'::jsonb,
  '[
    {"type": "complexity-max", "config": {"max": 5}},
    {"type": "no-n-plus-one", "config": {"max_queries_in_loop": 1}}
  ]'::jsonb,
  '["Hacer solo 1 JOIN y dejar el otro loop", "Olvidar el LIMIT", "Devolver datos que el frontend no necesita"]'::jsonb,
  'curated',
  NOW()
)
ON CONFLICT (id) DO NOTHING;
```

### 3.3 — Run the migration

```bash
psql $DATABASE_URL -f db/migrations/<NN>_<smell-slug>.sql
```

## Step 4 — Test the challenge end-to-end

1. Start backend + frontend.
2. Log in.
3. Go to `/dashboard`. **Verify the new challenge appears.**
4. Click it → opens Dojo.
5. Verify:
   - Code renders in Monaco with correct syntax.
   - 3 hints are visible (first one is free).
   - Submit a fix → score is calculated correctly.
   - Hints cost points.
6. Verify the solution works:
   - Submit `solution_code` → score should be maximum (all tests pass, all findings matched, no hints used).
7. Verify the scoring breakdown:
   - Use 3 hints → points deducted.
   - Use only 1 hint → less deduction.
   - Submit in < 15 min → time bonus.

## Step 5 — Commit and PR

```bash
git add db/migrations/<NN>_<smell-slug>.sql
git commit -m "feat(challenges): add <smell-slug> challenge

- Category: <category>
- Difficulty: <difficulty>
- Language: <language>
- 3 progressive hints (0/10/25 points)
- <N> expected findings
- <N> test cases
- <N> linter rules
- Scoring verified end-to-end"
```

## Checklist

- [ ] Smell chosen (not in existing 8).
- [ ] 5 design questions answered.
- [ ] YAML written for human review.
- [ ] SQL migration created.
- [ ] Migration applied to DB.
- [ ] Challenge visible in `/dashboard`.
- [ ] 3 hints work and cost points.
- [ ] Score = base + tests + lint + findings - hints + time.
- [ ] `solution_code` gets max score.
- [ ] Description does NOT spoil the answer.
- [ ] Conventional commit + clear PR.

## Don't do

- ❌ Don't use the old `mock-challenge.repository.ts` (deprecated S1).
- ❌ Don't spoiler the smell in the description.
- ❌ Don't make hints > 3 (cognitive overload).
- ❌ Don't make `solution_code` identical to `code` (the smell must be present in `code`).
- ❌ Don't skip `test_cases` (the score needs them).
- ❌ Don't skip `linter_rules` (objective evaluation needs them).
- ❌ Don't make `expected_findings` > 5 (focus).
- ❌ Don't commit without testing end-to-end.

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Description spoiles the smell | Re-read: would I know the answer just from the description? If yes, rewrite. |
| Solution doesn't pass its own tests | Run `solution_code` first, before committing. |
| Hints reveal the answer | Hints 1 and 2 should be observations, not solutions. Hint 3 is the closest to the answer. |
| Test cases are too strict | Tests should be functional, not implementation. Different correct solutions should pass. |
| Linter rules don't catch the smell | Run the linter on `code` → should fail. Run on `solution_code` → should pass. |
| Scoring too punitive | Adjust `base_points` and `bonus_points` so a user who finds all findings without hints gets a satisfying score. |
