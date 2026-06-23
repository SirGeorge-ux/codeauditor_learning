# Exploration: multi-lang-sandbox-oleada7

## Current State
The sandbox uses a `ProviderRegistry` with **34 providers**. The `registry_test.go` uses `"cobol"` as a dummy key to test unknown language rejection.

## Target Languages
Adding 5 languages (Legacy + Heavy Niche + Academic):
1. **PowerShell** (powershell)
2. **Objective-C** (objective-c)
3. **F#** (fsharp)
4. **Cobol** (cobol)
5. **Racket** (racket)

## Approaches & Docker Images
To run these languages as single-file scripts in the sandbox:

1. **PowerShell**: `mcr.microsoft.com/powershell:latest`. Command: `pwsh -File /code/<filename>`.
2. **Objective-C**: `gcc:latest`. Command: `sh -c "gcc -x objective-c -o /tmp/out /code/<filename> -lobjc && /tmp/out"`. Using GCC directly avoids pulling heavy macOS-specific images.
3. **F#**: `mcr.microsoft.com/dotnet/sdk:8.0-alpine`. Command: `dotnet fsi /code/<filename>`. F# interactive allows executing `.fs`/`.fsx` files directly without `.fsproj`.
4. **Cobol**: `alpine:latest`. Command: `sh -c "apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/<filename> && /tmp/out"`. Lightweight wrapper for Cobol.
5. **Racket**: `racket/racket:latest`. Command: `racket /code/<filename>`.

### Handler Mappings to Add
- `"ps1"` -> `"powershell"`
- `"m"` -> `"objective-c"`
- `"fs", "fsx"` -> `"fsharp"`
- `"cbl", "cob"` -> `"cobol"`
- `"rkt"` -> `"racket"`

### Test Updates
- Change the `UnknownKey` test in `registry_test.go` from `"cobol"` to `"brainfuck"` to prevent false negatives now that Cobol is officially supported.

### Recommendation
**Viable — proceed.**
The Provider Pattern fits these perfectly. F# via `dotnet fsi` and Cobol via `gnucobol` are great lightweight approaches. The registry will max out at 39 languages.

### Risks
- Large image sizes for PowerShell and .NET SDK. Handled by lazy-pulling in Docker.
- Objective-C via GCC is standard GNU step, might lack Foundation frameworks, but perfectly sufficient for standard algorithmic logic in a Dojo.

### Ready for Proposal
**Yes.**