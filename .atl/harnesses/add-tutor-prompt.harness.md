# Harness: Add or Iterate a Tutor Prompt

> **Step-by-step workflow** for adding a new prompt to the tutor chat or iterating an existing one.
> **Audience:** devs, prompt engineers, agents.
> **Estimated time:** 30-60 minutes per prompt.
> **Output:** a `.atl/prompts/tutor-<level>.md` file that the `mcp-pedagogical.get_socratic_prompt` returns.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/business-domain.md` §4 (Socratic levels).
- [ ] You've read `codeauditor-tutor-chat` skill.
- [ ] You have at least 5 test cases (user_message → expected_behavior).
- [ ] You can run the backend locally to test.

---

## Step 1 — Identify the level

The 4 socratic levels:

| Nivel | Comportamiento | Archivo |
|---|---|---|
| 0 (Directa) | "Hay un SQL injection en línea 3, usa queries parametrizadas" | `.atl/prompts/tutor-socratic-0.md` |
| 1 (Guiada) | "Mira la línea 3. ¿Qué pasa si `username = \"' OR 1=1--\"`?" | `.atl/prompts/tutor-socratic-1.md` |
| 2 (Socrática pura) | "¿Qué tipo de dato es `username`? ¿Y si el usuario lo controla?" | `.atl/prompts/tutor-socratic-2.md` |
| 3 (Descubrimiento) | La IA NO responde. Espera hipótesis del user. | `.atl/prompts/tutor-socratic-3.md` |

If you're adding a new level (e.g. level 4), use `.atl/prompts/tutor-socratic-4.md` and update the `mcp-pedagogical.get_socratic_prompt` to accept it.

---

## Step 2 — Write the prompt

The prompt has 4 parts:

```markdown
# Tutor Socrático - Nivel <N>

## Identidad

Eres un tutor de código socrático de nivel <N>.
[descripción del nivel: directa / guiada / socrática pura / descubrimiento]

## Reglas de comportamiento

1. [regla 1]
2. [regla 2]
3. [regla 3]
...

## Restricciones

- [restricción 1: lo que NO debes hacer]
- [restricción 2]

## Ejemplos

### Ejemplo 1: <caso>

**User:** <mensaje del user>
**Context:** <contexto del user: language, challenge, code, socratismo_level>
**Tu respuesta:** <respuesta esperada, en el nivel correcto>

### Ejemplo 2: <caso>

**User:** <mensaje del user>
**Context:** <contexto>
**Tu respuesta:** <respuesta esperada>

(... al menos 3-5 ejemplos ...)
```

---

## Step 3 — Example: Nivel 2 (Socrática pura)

```markdown
# Tutor Socrático - Nivel 2

## Identidad

Eres un tutor de código socrático. Tu trabajo NO es dar la respuesta, es
hacer las preguntas correctas para que el usuario la encuentre por sí mismo.

Nivel 2 = Socrática pura: solo preguntas, NUNCA respuestas directas.

## Reglas de comportamiento

1. Responde con 1-3 preguntas, no con afirmaciones.
2. Las preguntas deben llevar al usuario a la respuesta, no a la confusión.
3. Si el usuario se atasca, haz una pregunta más concreta (no des la respuesta).
4. Usa el código del usuario como referencia: "Mira la línea X...".
5. Si el usuario pide "dame la respuesta", recuerda: "Mi trabajo es ayudarte a pensar, no pensar por ti. ¿Qué has intentado hasta ahora?"

## Restricciones

- NUNCA digas "El problema es X" o "La solución es Y".
- NUNCA pegues código que resuelva el problema.
- NUNCA reveles el `expected_finding` directamente.
- Si el usuario te pide la respuesta 3 veces, sube al nivel 3 (descubrimiento): "Escribe tu hipótesis. No hay respuestas correctas, solo intentos de pensar."

## Ejemplos

### Ejemplo 1: SQL Injection

**User:** "No sé qué tiene de malo mi login"
**Context:** language=typescript, challenge=ch-sqli, code=`<código vulnerable>`, socratismo_level=2
**Tu respuesta:** "¿Qué tipo de dato es `username`? ¿Quién controla ese valor? Si yo fuera un usuario malicioso y mi username fuera `' OR 1=1--`, ¿qué crees que pasaría con la query SQL?"

### Ejemplo 2: N+1 Query

**User:** "Mi función tarda mucho con 100 pedidos"
**Context:** language=typescript, challenge=ch-n-plus-one, code=`<código vulnerable>`, socratismo_level=2
**Tu respuesta:** "Cuántas queries a la base de datos crees que se ejecutan con 100 pedidos y 1 usuario? Mira el bucle `for`. ¿Cuántas veces se llama a `db.query` dentro?"

### Ejemplo 3: Herencia

**User:** "Mi Dog extends Animal no funciona"
**Context:** language=typescript, code=`<código con error>`, socratismo_level=2
**Tu respuesta:** "En TypeScript, ¿qué tienes que hacer en el constructor de una subclase antes de poder usar `this`?"
```

---

## Step 4 — Test the prompt with 5+ test cases

Manually run the LLM with the prompt + 5 different test cases. Verify the output matches the expected behavior.

```bash
# Use a script or the LLM directly
curl -X POST https://openrouter.ai/api/v1/chat/completions \
  -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d @- <<'EOF'
{
  "model": "meta-llama/llama-3.3-70b-versatile",
  "messages": [
    {"role": "system", "content": "$(cat .atl/prompts/tutor-socratic-2.md)"},
    {"role": "user", "content": "No sé qué tiene de malo mi login"}
  ]
}
EOF
```

Verify the response is in the level 2 style (questions, not answers).

---

## Step 5 — Add a unit test for the prompt

In the backend test suite, add a test that verifies the prompt is loaded and returned correctly.

```go
// backend/internal/infrastructure/mcp/pedagogical/get_socratic_prompt_test.go
func TestGetSocraticPromptTool_ReturnsCorrectLevel(t *testing.T) {
    tool := NewGetSocraticPromptTool(/*...*/)

    for level := 0; level <= 3; level++ {
        result, err := tool.Execute(context.Background(), json.RawMessage(fmt.Sprintf(`{
            "level": %d,
            "language": "typescript",
            "topic": "inheritance",
            "user_message": "test"
        }`, level)))

        require.NoError(t, err)
        assert.NotEmpty(t, result.(map[string]any)["system_prompt"])
    }
}
```

---

## Step 6 — Test the tutor in the UI

1. Start backend + frontend.
2. Go to a challenge (`/dojo/<id>`).
3. Open the tutor chat (right panel).
4. Set the socratic level to your new level.
5. Send a few messages.
6. Verify the responses match the expected style.

---

## Step 7 — Commit

```bash
git add .atl/prompts/tutor-socratic-<N>.md \
        backend/internal/infrastructure/mcp/pedagogical/get_socratic_prompt_test.go

git commit -m "feat(tutor): add socratic level <N> prompt

- Level: <N> (<directa|guiada|pura|descubrimiento>)
- 3+ examples
- 5+ test cases verified
- UI test passed"
```

---

## Checklist

- [ ] Level identified (0-3 or new).
- [ ] Prompt has 4 parts: identity, rules, restrictions, examples.
- [ ] At least 3 examples in the prompt.
- [ ] 5+ test cases run with real LLM.
- [ ] Output matches the expected style for the level.
- [ ] No spoilers in the prompt.
- [ ] Unit test added.
- [ ] UI test passed.
- [ ] Conventional commit.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| The prompt contains the answer | Re-read each example. Would the user know the answer after the tutor's response? If yes, rewrite as a question. |
| The prompt is too vague | Add specific rules. "Respond with 1-3 questions" is better than "be socratic". |
| Examples are too easy | Vary the difficulty. Add an "user is stuck" example and a "user is on the right track" example. |
| The prompt doesn't use the user's context | Add rules like "Reference the user's code, not generic examples". |
| Mixing levels (e.g. 1 and 2) | Decide which level and stick to it. The 4 levels are mutually exclusive. |
| The prompt is in English but the user speaks Spanish | Either write 2 versions, or add a rule "respond in the user's language (preferences.idioma)". |
| The prompt is too long (>2000 tokens) | Keep it under 1000 tokens. The LLM has context limits. |

## Resources

- **Doc:** `docs/business-domain.md` §4 (Socratic levels)
- **Doc:** `docs/mcp-tools.md` §3.6 (`mcp-pedagogical`)
- **Skill:** `codeauditor-tutor-chat`
