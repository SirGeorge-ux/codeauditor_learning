# Delta Spec: Functional+.NET+Data+Apple Audit

## Requirement: Language Audit (Oleada 5)

The audit system MUST support code auditing for C#, Swift, R, Haskell, Elixir, and Clojure. Execution MUST correctly reflect standard output for valid code, and MUST capture standard error and non-zero exit codes for invalid syntax or compilation errors.

### Scenario: C# Compilation error
- GIVEN a C# snippet with invalid syntax
- WHEN the C# sandbox executes it
- THEN it MUST return an error payload
- AND stderr MUST contain a `mcs` compiler error

### Scenario: Functional language execution
- GIVEN a valid Elixir, Haskell, or Clojure snippet
- WHEN the respective sandbox executes it
- THEN it MUST complete successfully
- AND stdout MUST match the expected output
