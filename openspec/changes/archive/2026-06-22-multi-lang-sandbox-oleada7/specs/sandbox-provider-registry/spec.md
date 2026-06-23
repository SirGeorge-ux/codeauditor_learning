# Delta Spec: Legacy+Niche Sandbox Providers

## Requirement: New Language Providers (Oleada 7)

The system MUST implement `LanguageProvider` for PowerShell, Objective-C, F#, Cobol, and Racket. 
- PowerShell MUST use `mcr.microsoft.com/powershell:latest` and execute via `pwsh -File /code/<filename>`.
- Objective-C MUST use `gcc:latest` and execute via `sh -c "gcc -x objective-c -o /tmp/out /code/<filename> -lobjc && /tmp/out"`.
- F# MUST use `mcr.microsoft.com/dotnet/sdk:8.0-alpine` and execute via `dotnet fsi /code/<filename>`.
- Cobol MUST use `alpine:latest` and execute via `sh -c "apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/<filename> && /tmp/out"`.
- Racket MUST use `racket/racket:latest` and execute via `racket /code/<filename>`.

### Scenario: Compiled legacy and scripts
- GIVEN the PowerShell, Objective-C, F#, Cobol, or Racket provider
- WHEN `DockerCommand("code.ext")` is called
- THEN it MUST return the appropriate runner command (`pwsh`, `gcc`, `dotnet fsi`, `cobc`, `racket`) with the `/code/code.ext` path mapping.

## Requirement: Registry Registration (Oleada 7)

The `NewDefaultRegistry()` function MUST register the 5 new providers, bringing the total supported languages to 39. `Languages()` MUST return the 39 keys in alphabetical order. 

The `UnknownKey` test in `registry_test.go` MUST NOT use `"cobol"`. It MUST use `"brainfuck"` to avoid false test failures.

### Scenario: 39 Languages registered
- GIVEN a default `ProviderRegistry`
- WHEN `Languages()` is called
- THEN it MUST return exactly 39 items
- AND the list MUST include "powershell", "objective-c", "fsharp", "cobol", and "racket".

## Requirement: Handler Extension Mapping (Oleada 7)

The `inferLanguage` function in `gogs_handler.go` MUST correctly map new file extensions to their corresponding language keys.

### Scenario: Extension mapping
- GIVEN a file path
- WHEN `inferLanguage` is called
- THEN `.ps1` MUST return `"powershell"`
- AND `.m` MUST return `"objective-c"`
- AND `.fs` and `.fsx` MUST return `"fsharp"`
- AND `.cbl` and `.cob` MUST return `"cobol"`
- AND `.rkt` MUST return `"racket"`