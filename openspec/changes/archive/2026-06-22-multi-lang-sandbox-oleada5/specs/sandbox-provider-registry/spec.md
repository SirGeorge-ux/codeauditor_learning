# Delta Spec: Functional+.NET+Data+Apple Sandbox Providers

## Requirement: New Language Providers (Oleada 5)

The system MUST implement `LanguageProvider` for C# (csharp), Swift, R, Haskell, Elixir, and Clojure. 
- C# MUST use `mono:latest` and execute via `sh -c "mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe"`.
- Swift MUST use `swift:latest` and execute via `swift /code/<filename>`.
- R MUST use `r-base:latest` and execute via `Rscript /code/<filename>`.
- Haskell MUST use `haskell:latest` and execute via `runhaskell /code/<filename>`.
- Elixir MUST use `elixir:alpine` and execute via `elixir /code/<filename>`.
- Clojure MUST use `clojure:latest` and execute via `clojure -M /code/<filename>`.

### Scenario: C# Compilation and Execution
- GIVEN the C# provider
- WHEN `DockerCommand("code.cs")` is called
- THEN it MUST return `[]string{"sh", "-c", "mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe"}`

### Scenario: Single-file execution environments
- GIVEN the Swift, R, Haskell, Elixir, or Clojure provider
- WHEN `DockerCommand("code.ext")` is called
- THEN it MUST return the appropriate runner command (`swift`, `Rscript`, `runhaskell`, `elixir`, `clojure`) with the `/code/code.ext` path.

## Requirement: Registry Registration (Oleada 5)

The `NewDefaultRegistry()` function MUST register the 6 new providers, bringing the total supported languages to 29. `Languages()` MUST return the 29 keys in alphabetical order.

### Scenario: 29 Languages registered
- GIVEN a default `ProviderRegistry`
- WHEN `Languages()` is called
- THEN it MUST return exactly 29 items
- AND the list MUST include "csharp", "swift", "r", "haskell", "elixir", and "clojure".

## Requirement: Handler Extension Mapping (Oleada 5)

The `inferLanguage` function in `gogs_handler.go` MUST correctly map new file extensions to their corresponding language keys.

### Scenario: Extension mapping
- GIVEN a file path
- WHEN `inferLanguage` is called
- THEN `.r` MUST return `"r"`
- AND `.hs` MUST return `"haskell"`
- AND `.ex` and `.exs` MUST return `"elixir"`
- AND `.clj` MUST return `"clojure"`
*(Note: `.cs` and `.swift` are already mapped)*