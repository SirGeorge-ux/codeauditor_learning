# Curricula Format (CodeAuditor)

> **Audiencia:** devs, content creators, agentes que vayan a añadir topics.
> **Lee esto antes de:** añadir un topic a un lenguaje, generar un challenge, o pedirle al LLM que enseñe algo.

---

## 1. Por qué currículas versionadas en `.md`

**Decisión de diseño:** la currícula vive en `.atl/curricula/<lang>.md`, no en la DB, no en Notion, no en un CMS. Razones:

1. **Single source of truth = repo.** El repo ya tiene todo el código y la documentación. La pedagogía también.
2. **Versionado en Git.** Cada cambio de currícula es un commit con diff. Se puede revisar, revertir, hacer blame.
3. **Legible por humanos y por LLMs.** Un markdown con estructura clara es fácil de leer para un dev Y fácil de parsear para el `mcp-curriculum` tool.
4. **Ligero.** No requiere DB ni servicio extra.
5. **Editable en cualquier editor.** VSCode, vim, lo que sea.

**Migración futura (S4):** si la currícula crece mucho (>100 topics por lenguaje), se puede migrar a tabla Postgres con `.md` como seed. Por ahora, `.md` es suficiente.

---

## 2. Estructura del archivo

### 2.1 Path

```
.atl/curricula/<language>.md
```

Donde `<language>` es el **canonical key** del `LanguageProvider` (lowercase): `typescript.md`, `python.md`, `rust.md`, `go.md`, etc.

### 2.2 Frontmatter (opcional)

```markdown
---
language: typescript
display_name: TypeScript
version: 1
last_reviewed: 2026-07-22
reviewers: [mic]
---
```

### 2.3 Header

```markdown
# <Language> Curriculum

> Pedagogical source of truth for <Language> challenges.
> Editable by humans. Consumed by:
> - `mcp-curriculum` tool (returns topics to the LLM)
> - `mcp-pedagogical.get_learning_objective` (filters by level)
> - `free-practice-mode` generator (creates challenges from topics)

## Topics

(... topics abajo ...)

## Review notes

(... opcional, notas para el siguiente reviewer ...)
```

### 2.4 Topic (estructura canónica)

```markdown
### <topic-slug>

- **slug:** <topic-slug>
- **difficulty:** F | E | D | C | B | A | S
- **prerequisites:** [<other-topic-slug>, ...]
- **category:** syntax | types | oop | functional | async | concurrency | testing | tooling | patterns | architecture
- **description:** |
  <2-3 párrafos explicando QUÉ es y POR QUÉ importa>
- **exercises:**
  - "<ejercicio 1>"
  - "<ejercicio 2>"
  - "<ejercicio 3>"
- **common_mistakes:**
  - "<error típico 1>"
  - "<error típico 2>"
- **resources:**
  - "<link a docs oficiales>"
  - "<link a MDN / libro / blog>"
```

### 2.5 Ejemplo completo (`typescript.md`)

```markdown
---
language: typescript
display_name: TypeScript
version: 1
last_reviewed: 2026-07-22
---

# TypeScript Curriculum

## Topics

### variables

- **slug:** variables
- **difficulty:** F
- **prerequisites:** []
- **category:** syntax
- **description:** |
  Variables are the most basic building block. TypeScript has `let`, `const`,
  and `var` (avoid). Use `const` by default, `let` only when reassignment is needed.
- **exercises:**
  - "Declare a `const` for a user's name and a `let` for their age."
  - "Reassign a variable declared with `let` and observe the type error."
- **common_mistakes:**
  - "Using `var` instead of `let`/`const`."
  - "Reassigning a `const`."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/variable-declarations.html

### types

- **slug:** types
- **difficulty:** E
- **prerequisites:** [variables]
- **category:** types
- **description:** |
  TypeScript's type system is what makes it TypeScript. Primitives (`string`,
  `number`, `boolean`), arrays, tuples, unions, and intersections.
- **exercises:**
  - "Type a function that returns a `string | number`."
  - "Create a tuple representing a coordinate `(x: number, y: number)`."
  - "Use a union type to model a `Result<T, E>`."
- **common_mistakes:**
  - "Using `any` instead of a specific type."
  - "Confusing `string` and `String` (the wrapper object)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/everyday-types.html

### classes

- **slug:** classes
- **difficulty:** E
- **prerequisites:** [variables, types]
- **category:** oop
- **description:** |
  Classes are blueprints for objects. TypeScript classes support constructors,
  methods, fields, access modifiers (public/private/protected), and inheritance
  via `extends`.
- **exercises:**
  - "Create a `User` class with name and email."
  - "Add a `greet()` method to `User`."
  - "Create an `Admin extends User` with role."
- **common_mistakes:**
  - "Forgetting `super()` in constructor of subclass."
  - "Public fields exposed when they should be private."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/classes.html

### inheritance

- **slug:** inheritance
- **difficulty:** D
- **prerequisites:** [classes]
- **category:** oop
- **description:** |
  Inheritance lets a class reuse and extend another class's behavior. TypeScript
  uses `extends` and supports method overriding with `super.method()`.
- **exercises:**
  - "Extend a base class and override a method."
  - "Use `protected` to share state with subclasses."
- **common_mistakes:**
  - "Overriding without calling `super.method()`."
  - "Deep inheritance chains (>3 levels)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/classes.html#inheritance

### generics

- **slug:** generics
- **difficulty:** C
- **prerequisites:** [classes, types]
- **category:** types
- **description:** |
  Generics let you write reusable code that works with multiple types. TypeScript
  uses `<T>` syntax. Constraints: `<T extends SomeType>`.
- **exercises:**
  - "Write a `Box<T>` that wraps any value."
  - "Write a generic `getProperty<T, K extends keyof T>`."
- **common_mistakes:**
  - "Forgetting constraints (`extends keyof T`)."
  - "Over-using generics where a union type would do."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/generics.html

### async-await

- **slug:** async-await
- **difficulty:** D
- **prerequisites:** [types]
- **category:** async
- **description:** |
  `async`/`await` is the modern way to handle asynchronous code. Always `await`
  inside `try`/`catch` for error handling.
- **exercises:**
  - "Convert a Promise chain to async/await."
  - "Handle errors with try/catch around an await."
  - "Run N async operations in parallel with `Promise.all`."
- **common_mistakes:**
  - "Forgetting `await` (the function returns a Promise, not the value)."
  - "Not handling rejections."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/release-notes/typescript-1-7.html#asyncawait

### promises

- **slug:** promises
- **difficulty:** D
- **prerequisites:** [types, async-await]
- **category:** async
- **description:** |
  A `Promise` is a placeholder for a value that will be available later. Has 3
  states: pending, fulfilled, rejected. Chain with `.then()` or use `await`.
- **exercises:**
  - "Create a Promise that resolves after 1s."
  - "Chain 3 Promises sequentially."
  - "Use `Promise.race` to get the first of 3 async operations."
- **common_mistakes:**
  - "Not handling rejections."
  - "Callback-style nested Promises (anti-pattern)."
- **resources:**
  - https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise
```

---

## 3. Sistema de difficulty (F-S)

**Solo para free practice mode.** Code health usa Junior-Architect.

| Letra | Puntos | Nombre | Significado |
|---|---|---|---|
| F | 0-49 | Foundation | Primer contacto con el lenguaje |
| E | 50-149 | Explorer | Entiende la sintaxis básica |
| D | 150-399 | Developer | Resuelve challenges con guidance |
| C | 400-899 | Competent | Resuelve challenges solo |
| B | 900-1999 | Builder | Resuelve challenges complejos |
| A | 2000-3999 | Architect | Diseña sistemas |
| S | 4000+ | Sage | Master |

**Mapeo a topics:**

- F → variables, hello-world, io básico
- E → types, classes, funciones, control flow
- D → herencia, async/await, promises, módulos
- C → generics, decorators, iterators
- B → design patterns, async avanzado, metaprogramming
- A → arquitectura, type-level programming, monads
- S → sistemas distribuidos, formal verification, etc.

---

## 4. Cómo añadir un topic

### 4.1 Manualmente (recomendado para el MVP)

1. Editar `.atl/curricula/<lang>.md`.
2. Añadir el bloque `### <topic-slug>` siguiendo la estructura canónica.
3. Verificar:
   - `difficulty` es coherente con el orden pedagógico.
   - `prerequisites` son topics que ya existen.
   - `description` no contiene spoilers (es la "lección", no el "ejercicio").
   - `exercises` son retos cortos, sin código (los snippets los genera el LLM).
   - `common_mistakes` son errores reales, no teóricos.
4. Commit con `docs(curriculum): add <topic> to <lang>`.

### 4.2 Con sugerencia de la IA (peer review)

Cuando el sistema está en S2+:

1. El user abre `/curriculum/review` en el frontend.
2. La IA propone un topic nuevo basado en:
   - Topics que el user ya ha dominado.
   - Gaps en la currícula (e.g. "typescript tiene async-await pero no generators").
   - Patrones vistos en code health del user.
3. El user revisa y edita la propuesta.
4. El user hace commit.

**Trampa:** la IA no debe proponer topics que ya existen. El `mcp-curriculum.suggest_topic` debe primero listar los existentes.

---

## 5. Cómo lo consume el sistema

### 5.1 `mcp-curriculum.get_topics(language)`

Lee `.atl/curricula/<lang>.md`, lo parsea, devuelve:

```typescript
{
  language: 'typescript',
  topics: [
    { slug: 'variables', difficulty: 'F', prerequisites: [], ... },
    { slug: 'types', difficulty: 'E', prerequisites: ['variables'], ... },
    ...
  ]
}
```

### 5.2 `mcp-curriculum.get_topic(language, topic)`

Devuelve el detalle de un topic.

### 5.3 `mcp-pedagogical.get_learning_objective(language, topic, level)`

1. Llama a `mcp-curriculum.get_topic(language, topic)`.
2. Filtra los `exercises` por `level` (F=primer ejercicio, D=segundo, C=tercer).
3. Devuelve:
   - El ejercicio (sin código, solo el enunciado).
   - Las `prerequisites` (lo que el user debe saber antes).
   - Las `restricciones` (lo que NO debe usar todavía, e.g. "no generics" si es nivel F).
   - Los `common_mistakes` (para que el tutor sepa qué vigilar).

### 5.4 `mcp-pedagogical.generate_story_context(language, topic)`

Usa la currícula + un LLM para crear un contexto narrativo (PBL):

> "Eres un desarrollador en una startup de finanzas. Tu equipo acaba de recibir el código de un nuevo proveedor de pagos. El primer archivo que tienes que revisar es `PaymentProcessor.ts`..."

---

## 6. Plantilla para nuevos lenguajes

Cuando añades un lenguaje al sandbox, también debes crear su currícula (o al menos el esqueleto).

```bash
# Crear esqueleto
touch .atl/curricula/<newlang>.md
```

Plantilla mínima:

```markdown
---
language: <newlang>
display_name: <Display Name>
version: 0
last_reviewed: YYYY-MM-DD
---

# <Display Name> Curriculum

> Pedagogical source of truth for <Display Name> challenges.

## Topics

### hello-world

- **slug:** hello-world
- **difficulty:** F
- **prerequisites:** []
- **category:** syntax
- **description:** |
  First contact with the language: how to run a program, basic syntax,
  printing output.
- **exercises:**
  - "Print 'Hello, World!' to stdout."
- **common_mistakes:**
  - "..."
- **resources:**
  - <link>

(... añade más topics según sea necesario ...)
```

---

## 7. Validación de la currícula

El sistema valida automáticamente en CI (`.atl/curricula/lint.sh`):

```bash
#!/bin/bash
# .atl/curricula/lint.sh

set -e

for file in .atl/curricula/*.md; do
  echo "Linting $file..."

  # 1. Frontmatter válido (yaml)
  yq eval '.language' "$file" > /dev/null

  # 2. Todos los topics tienen los campos obligatorios
  for topic in $(grep -E '^### ' "$file" | awk '{print $2}'); do
    grep -A 2 "^### $topic$" "$file" | grep -q "^- \*\*slug:\*\* $topic$"
    grep -A 4 "^### $topic$" "$file" | grep -q "^- \*\*difficulty:\*\*"
    grep -A 5 "^### $topic$" "$file" | grep -q "^- \*\*prerequisites:\*\*"
    grep -A 6 "^### $topic$" "$file" | grep -q "^- \*\*exercises:\*\*"
  done

  # 3. Todos los prerequisites existen
  for prereq in $(grep -E "prerequisites:" -A 1 "$file" | grep -E "^  - " | awk '{print $2}' | tr -d ','); do
    if ! grep -q "^- \*\*slug:\*\* $prereq$" "$file"; then
      echo "ERROR: prerequisite '$prereq' not found in $file"
      exit 1
    fi
  done
done

echo "All curricula OK."
```

---

## 8. Recursos

- **Skill:** `codeauditor-free-practice-generator` (cómo generar un challenge desde un topic).
- **Harness:** `add-curriculum-topic.harness.md` (cómo añadir un topic paso a paso).
- **Spec:** `openspec/changes/free-practice-mode/` (S3).
- **Spec:** `openspec/changes/mcp-monolith/` (S2) — incluye `mcp-curriculum` y `mcp-pedagogical`.
