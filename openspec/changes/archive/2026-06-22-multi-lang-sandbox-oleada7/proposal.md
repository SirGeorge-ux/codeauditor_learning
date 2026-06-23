# Proposal: multi-lang-sandbox-oleada7

## Intent
Complete the sandbox ecosystem by adding 5 legacy, niche, and academic languages: PowerShell, Objective-C, F#, Cobol, and Racket. This brings the total language count to 39.

## Scope
**In scope:**
- Implement `LanguageProvider` for powershell, objective-c, fsharp, cobol, racket.
- Register the 5 new providers in `NewDefaultRegistry()`.
- Add handler mappings for `.ps1`, `.m`, `.fs`, `.fsx`, `.cbl`, `.cob`, `.rkt` in `gogs_handler.go`.
- Update `registry_test.go` (34 → 39 languages).
- Update the unknown language test dummy key to `"brainfuck"`.
- Update spec for sandbox-provider-registry and audit.

**Out of scope:**
- Foundation/Cocoa frameworks for Objective-C (pure GNU GCC logic only).

## Approach
Reuse the established Provider Pattern. One file per language under `providers`.

### Execution Strategy & Images
1. **PowerShell** (`mcr.microsoft.com/powershell:latest`): `pwsh -File /code/<filename>`
2. **Objective-C** (`gcc:latest`): `sh -c "gcc -x objective-c -o /tmp/out /code/<filename> -lobjc && /tmp/out"`
3. **F#** (`mcr.microsoft.com/dotnet/sdk:8.0-alpine`): `dotnet fsi /code/<filename>`
4. **Cobol** (`alpine:latest`): `sh -c "apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/<filename> && /tmp/out"`
5. **Racket** (`racket/racket:latest`): `racket /code/<filename>`

## Risks
- None. This is the final and thoroughly validated pattern.

## Rollback Plan
Remove the 5 new files and revert the `NewDefaultRegistry` and `inferLanguage` additions. No state or external systems are modified.