# Harness: Add a Curated Challenge (S1+ new model)

> **Step-by-step workflow** for adding a new challenge to CodeAuditor with the **new model** (`expected_findings`, `hints`, `solution_code`, `test_cases`, `linter_rules`).
> **Audience:** devs, content creators, agents.
> **Estimated time:** 30-60 minutes.
> **Output:** a new challenge persisted in Postgres with reproducible scoring.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/business-domain.md` §2.2 (new Challenge model).
- [ ] You've read `docs/curricula-format.md` (the topic structure).
- [ ] The smell you want to teach is **not** in the existing 8.
- [ ] You can run the backend locally with the DB.
- [ ] You can verify the challenge end-to-end in the Dojo.

---

## Step 1 — Open a spec (if material)

**Open a spec** in `openspec/changes/add-<smell>-challenge/` if:
- You're adding a category that needs UI changes.
- You're changing the scoring algorithm.
- The challenge requires new infra (e.g. a new linter rule).

For pure data additions (most cases), skip the spec.

---

## Step 2 — Design the challenge

Answer these 5 questions:

1. **Smell:** what's the user going to detect?
2. **Language:** what language? Must be supported (see `docs/sandbox-providers.md` §8).
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

---

## Step 3 — Write the challenge as YAML first (recommended)

The YAML format makes it easy to review before converting to SQL. Save as `db/challenges-draft/<smell-slug>.yaml`.

```yaml
id: ch-n-plus-one
title: N+1 Query en historial de usuario
description: |
  Esta función carga el historial de pedidos de un usuario. Funciona
  correctamente, pero con muchos pedidos el tiempo de respuesta crece
  linealmente. Detecta el problema y propón una solución.
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
    suggested_fix: "Usar un JOIN o un IN clause"
  - severity: medium
    category: performance
    message: "Posible N+1 también en getUser"
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
  Un LEFT JOIN entre orders y order_items, agrupando con json_agg,
  resuelve el N+1 en una sola query.

base_points: 25
bonus_points: 30
time_bonus: true
```

---

## Step 4 — Convert YAML to SQL migration

**File:** `db/migrations/<NN>_<smell-slug>.sql`

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

### Run the migration

```bash
psql $DATABASE_URL -f db/migrations/<NN>_<smell-slug>.sql
```

---

## Step 5 — Verify the challenge end-to-end

1. Start backend + frontend.
2. Log in.
3. Go to `/dashboard`. **Verify the new challenge appears.**
4. Click it → opens Dojo.
5. Verify:
   - Code renders in Monaco with correct syntax.
   - 3 hints are visible.
   - Submit a fix → score is calculated.
   - Hints cost points.
6. Verify the solution:
   - Submit `solution_code` → score should be maximum.
7. Verify the scoring breakdown:
   - Use 3 hints → points deducted.
   - Submit in <15 min → time bonus.

---

## Step 6 — Add a smoke test

**File:** `backend/internal/core/services/challenge_service_test.go` (append):

```go
func TestChallengeService_NewChallengePersisted(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    service := services.NewChallengeService(db)

    // Fetch the new challenge
    challenge, err := service.GetByID(context.Background(), "ch-n-plus-one")

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "N+1 Query en historial de usuario", challenge.Title)
    assert.Equal(t, "mid", challenge.Difficulty)
    assert.Len(t, challenge.ExpectedFindings, 2)
    assert.Len(t, challenge.Hints, 3)
    assert.NotEmpty(t, challenge.SolutionCode)
    assert.Len(t, challenge.TestCases, 2)
}
```

---

## Step 7 — Commit and PR

```bash
git add db/migrations/<NN>_<smell-slug>.sql \
        db/challenges-draft/<smell-slug>.yaml \
        backend/internal/core/services/challenge_service_test.go

git commit -m "feat(challenges): add <smell-slug> challenge

- Category: <category>
- Difficulty: <difficulty>
- Language: <language>
- 3 progressive hints (0/10/25 points)
- <N> expected findings
- <N> test cases
- <N> linter rules
- Scoring verified end-to-end"

git push origin feature/add-<smell-slug>-challenge
gh pr create --title "feat(challenges): add <smell-slug> challenge" \
             --body "Adds a new <difficulty> challenge for <smell> in <language>. Scoring verified."
```

---

## Checklist

- [ ] Smell chosen (not in existing 8).
- [ ] 5 design questions answered.
- [ ] YAML written and saved in `db/challenges-draft/`.
- [ ] SQL migration created and applied.
- [ ] Challenge visible in `/dashboard`.
- [ ] 3 hints work and cost points.
- [ ] Score = base + tests + lint + findings - hints + time.
- [ ] `solution_code` gets max score.
- [ ] Description does NOT spoil the answer.
- [ ] Smoke test added.
- [ ] Conventional commit + clear PR.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Description spoiles the smell | Re-read: would I know the answer just from the description? If yes, rewrite. |
| Solution doesn't pass its own tests | Run `solution_code` first, before committing. |
| Hints reveal the answer | Hints 1 and 2 are observations, not solutions. Hint 3 is the closest. |
| Test cases are too strict | Tests should be functional, not implementation. |
| Linter rules don't catch the smell | Run the linter on `code` → fails. On `solution_code` → passes. |
| Scoring too punitive | Adjust `base_points` and `bonus_points` so a user who finds all findings without hints gets a satisfying score. |
| Code > 50 lines | Junior: ≤30 lines. Architect: ≤80 lines. |
| `solution_code` is identical to `code` | The smell must be present in `code`. If they're identical, the LLM (or you) forgot to add the fix. |
