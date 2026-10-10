---
language: typescript
display_name: TypeScript
version: 1
last_reviewed: 2026-10-10
reviewers: [mic]
---

# TypeScript Curriculum

> Pedagogical source of truth for TypeScript challenges.
> Editable by humans. Consumed by:
> - `mcp-curriculum` tool (returns topics to the LLM)
> - `mcp-pedagogical.get_learning_objective` (filters by level)
> - `free-practice-mode` generator (creates challenges from topics)

## Review notes

This is the canonical example curriculum for the S3 free-practice
spec. Topics span F (Foundation) through S (Sage) so a learner can
walk the full F..S ladder. New topics land via
`add-curriculum-topic.harness.md` follow-up PRs.

## Topics

### variables

- **slug:** variables
- **difficulty:** F
- **prerequisites:** []
- **category:** syntax
- **description:** |
  Variables are the most basic building block. TypeScript has `let`,
  `const`, and `var` (avoid). Use `const` by default, `let` only when
  reassignment is needed. The type can be inferred from the initializer
  or annotated explicitly with `: type`.
- **exercises:**
  - "Declare a `const` for a user's name and a `let` for their age."
  - "Reassign a variable declared with `let` and observe the type error."
  - "Annotate a numeric literal's type explicitly with `: number`."
- **common_mistakes:**
  - "Using `var` instead of `let`/`const`."
  - "Reassigning a `const`."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/variable-declarations.html

### control-flow

- **slug:** control-flow
- **difficulty:** F
- **prerequisites:** [variables]
- **category:** syntax
- **description:** |
  Conditional and looping constructs: `if / else`, `switch`, `for`,
  `while`, `do...while`. TypeScript narrows types inside guarded
  branches — the inference is the language's superpower.
- **exercises:**
  - "Write a guard that narrows `string | number` to its runtime type."
  - "Iterate over a string's characters with a `for...of` loop."
  - "Use a `switch` over a discriminated union."
- **common_mistakes:**
  - "Forgetting `break` in a `switch` case."
  - "Using `==` instead of `===` (loose equality is rarely what you want)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/narrowing.html

### types

- **slug:** types
- **difficulty:** E
- **prerequisites:** [variables]
- **category:** types
- **description:** |
  TypeScript's type system is what makes it TypeScript. Primitives
  (`string`, `number`, `boolean`, `null`, `undefined`), arrays,
  tuples, unions, and intersections. Always prefer the smallest
  type that captures the shape — a too-wide `any` defeats the purpose.
- **exercises:**
  - "Type a function that returns a `string | number`."
  - "Create a tuple representing a coordinate `(x: number, y: number)`."
  - "Use a union type to model a `Result<T, E>`."
- **common_mistakes:**
  - "Using `any` instead of a specific type."
  - "Confusing `string` and `String` (the wrapper object)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/everyday-types.html

### functions

- **slug:** functions
- **difficulty:** E
- **prerequisites:** [variables, types]
- **category:** syntax
- **description:** |
  Functions are first-class in TypeScript. Default + rest parameters,
  parameter destructuring, overload signatures, and the `this` typing
  trick. The `void` return type is useful for callbacks.
- **exercises:**
  - "Write a function with two overloads for `(x: string) => string` and `(x: number) => number`."
  - "Destructure parameters from an options object."
  - "Use a rest parameter to capture additional arguments."
- **common_mistakes:**
  - "Forgetting `return` in a non-void function."
  - "Mutating parameters instead of returning a new value."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/functions.html

### classes

- **slug:** classes
- **difficulty:** E
- **prerequisites:** [variables, types]
- **category:** oop
- **description:** |
  Classes are blueprints for objects. TypeScript classes support
  constructors, methods, fields, access modifiers (public,
  private, protected), inheritance via `extends`, and parameter
  properties for terse initialization.
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
  Inheritance lets a class reuse and extend another class's behavior.
  TypeScript uses `extends` and supports method overriding with
  `super.method()`. Prefer composition over deep inheritance chains.
- **exercises:**
  - "Extend a base class and override a method."
  - "Use `protected` to share state with subclasses."
  - "Mark a method `final` (read-only convention; TypeScript uses `readonly`)."
- **common_mistakes:**
  - "Overriding without calling `super.method()`."
  - "Deep inheritance chains (>3 levels)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/classes.html#inheritance

### interfaces

- **slug:** interfaces
- **difficulty:** D
- **prerequisites:** [types]
- **category:** types
- **description:** |
  Interfaces describe the SHAPE of a value. They are open to
  declaration merging and the compiler erases them at build time.
  Use `interface` for public object types and `type` for unions /
  intersections / computed shapes.
- **exercises:**
  - "Declare an interface for a `User` with name + email."
  - "Extend an interface with a new field."
  - "Implement an interface on a class."
- **common_mistakes:**
  - "Using `interface` for union types (it does not allow `|`)."
  - "Forgetting that interfaces can be declared multiple times."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/objects.html

### generics

- **slug:** generics
- **difficulty:** C
- **prerequisites:** [classes, types]
- **category:** types
- **description:** |
  Generics let you write reusable code that works with multiple
  types. TypeScript uses `<T>` syntax. Constraints: `<T extends
  SomeType>`. Defaults: `<T = string>`.
- **exercises:**
  - "Write a `Box<T>` that wraps any value."
  - "Write a generic `getProperty<T, K extends keyof T>`."
  - "Add a default to a generic parameter."
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
  `async`/`await` is the modern way to handle asynchronous code.
  Always `await` inside `try`/`catch` for error handling. The
  return type of an async function is always `Promise<T>`.
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
  A `Promise` is a placeholder for a value that will be available
  later. Has 3 states: pending, fulfilled, rejected. Chain with
  `.then()` or use `await`. Prefer async / await over long `.then`
  ladders.
- **exercises:**
  - "Create a Promise that resolves after 1s."
  - "Chain 3 Promises sequentially."
  - "Use `Promise.race` to get the first of 3 async operations."
- **common_mistakes:**
  - "Not handling rejections."
  - "Callback-style nested Promises (anti-pattern)."
- **resources:**
  - https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise

### error-handling

- **slug:** error-handling
- **difficulty:** D
- **prerequisites:** [async-await, functions]
- **category:** patterns
- **description:** |
  TypeScript treats errors as values: throw, try/catch, custom error
  classes, and the `Result<T, E>` pattern (a discriminated union of
  `{ok: true, value: T}` and `{ok: false, error: E}`). Choose based
  on the failure-mode budget of the call site.
- **exercises:**
  - "Create a custom error class extending `Error`."
  - "Wrap a throwing call in a `Result<T, E>` builder."
  - "Use `never` to model exhaustive switch fallthrough."
- **common_mistakes:**
  - "Swallowing errors with empty `catch {}`."
  - "Using exceptions for control flow (anti-pattern in TS)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/narrowing.html#throwing-functions

### modules

- **slug:** modules
- **difficulty:** C
- **prerequisites:** [functions, classes]
- **category:** tooling
- **description:** |
  TypeScript modules use ES module syntax (`import` / `export`).
  Each file is a module by default; named exports are preferred
  over default exports for refactor-ability.
- **exercises:**
  - "Export a function and import it in another file."
  - "Re-export a named symbol from an index file."
  - "Use dynamic `import()` for code-splitting."
- **common_mistakes:**
  - "Mixing default and named exports in the same module."
  - "Forgetting `.js` extension in the import (required for ESM)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/modules.html

### design-patterns

- **slug:** design-patterns
- **difficulty:** B
- **prerequisites:** [classes, generics, interfaces]
- **category:** patterns
- **description:** |
  The GoF patterns translated to TypeScript: Factory, Strategy,
  Observer, Decorator. TypeScript's structural typing + generics
  make many patterns lighter than in Java/C#.
- **exercises:**
  - "Implement a `Logger` factory that returns a console or file logger."
  - "Use a Strategy to swap sorting algorithms at runtime."
  - "Build a tiny Observer pattern with a typed event emitter."
- **common_mistakes:**
  - "Over-engineering a small module with a pattern."
  - "Reaching for inheritance when composition would do."
- **resources:**
  - https://refactoring.guru/design-patterns/typescript

### type-level-programming

- **slug:** type-level-programming
- **difficulty:** A
- **prerequisites:** [generics, interfaces]
- **category:** types
- **description:** |
  TypeScript's type system is Turing-complete. Mapped types,
  conditional types, `infer`, recursive types — used to derive
  shapes from other shapes without runtime code.
- **exercises:**
  - "Write `Partial<T>` from scratch using a mapped type."
  - "Use `infer` to extract the awaited type from a Promise."
  - "Build a recursive type that flattens nested arrays."
- **common_mistakes:**
  - "Over-using type-level programming when a simple `type` alias would do."
  - "Recursive types without a base case (compiler blows the stack)."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/types-from-types.html

### domain-modeling

- **slug:** domain-modeling
- **difficulty:** S
- **prerequisites:** [type-level-programming, design-patterns, error-handling]
- **category:** architecture
- **description:** |
  Modeling a domain in TypeScript: sum types (discriminated
  unions), phantom types for units of measure, branded primitives
  for IDs, and parsing/serialization boundaries. This is where
  TypeScript's type system earns its keep.
- **exercises:**
  - "Model an `OrderId` as a branded string so it can't be assigned a raw string."
  - "Parse a JSON payload into a discriminated union via a type guard."
  - "Build a sum type for `PaymentMethod` (cash / card / crypto)."
- **common_mistakes:**
  - "Reaching for classes when a discriminated union is a better fit."
  - "Mixing branded and unbranded primitives in the same module."
- **resources:**
  - https://www.typescriptlang.org/docs/handbook/2/narrowing.html