# Delta Spec: Crypto+Niche+BEAM Sandbox Providers

## Requirement: New Language Providers (Oleada 6)

The system MUST implement `LanguageProvider` for Solidity, Erlang, Dart, Julia, and Nim. 
- Solidity MUST use `ethereum/solc:stable` and execute via `solc /code/<filename>`.
- Erlang MUST use `erlang:latest` and execute via `escript /code/<filename>`.
- Dart MUST use `dart:latest` and execute via `dart run /code/<filename>`.
- Julia MUST use `julia:latest` and execute via `julia /code/<filename>`.
- Nim MUST use `nimlang/nim:alpine` and execute via `nim c -r --hints:off /code/<filename>`.

### Scenario: Single-file execution environments
- GIVEN the Solidity, Erlang, Dart, Julia, or Nim provider
- WHEN `DockerCommand("code.ext")` is called
- THEN it MUST return the appropriate runner command (`solc`, `escript`, `dart run`, `julia`, `nim c -r`) with the `/code/code.ext` path.

## Requirement: Registry Registration (Oleada 6)

The `NewDefaultRegistry()` function MUST register the 5 new providers, bringing the total supported languages to 34. `Languages()` MUST return the 34 keys in alphabetical order.

### Scenario: 34 Languages registered
- GIVEN a default `ProviderRegistry`
- WHEN `Languages()` is called
- THEN it MUST return exactly 34 items
- AND the list MUST include "solidity", "erlang", "dart", "julia", and "nim".

## Requirement: Handler Extension Mapping (Oleada 6)

The `inferLanguage` function in `gogs_handler.go` MUST correctly map new file extensions to their corresponding language keys.

### Scenario: Extension mapping
- GIVEN a file path
- WHEN `inferLanguage` is called
- THEN `.sol` MUST return `"solidity"`
- AND `.erl` MUST return `"erlang"`
- AND `.dart` MUST return `"dart"`
- AND `.jl` MUST return `"julia"`
- AND `.nim` MUST return `"nim"`