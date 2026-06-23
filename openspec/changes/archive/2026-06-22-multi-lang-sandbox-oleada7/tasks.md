# Tasks: multi-lang-sandbox-oleada7

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~420 |
| 400-line budget risk | Low-Medium |
| Chained PRs recommended | No (mechanical boilerplate, single-pr acceptable with exception) |
| Suggested split | Single PR |
| Delivery strategy | single-pr-default |
| Chain strategy | pending |

*Exception approved for >400 lines: changes are purely additive mechanical boilerplate following existing patterns.*

## Phase 1: Legacy Providers

- [x] 1.1 Create `cobol.go` and `cobol_test.go` — `alpine:latest`, `sh -c "apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/code.cbl && /tmp/out"`
- [x] 1.2 Create `objective_c.go` and `objective_c_test.go` — `gcc:latest`, `sh -c "gcc -x objective-c -o /tmp/out /code/code.m -lobjc && /tmp/out"`

## Phase 2: .NET and Niche Providers

- [x] 2.1 Create `fsharp.go` and `fsharp_test.go` — `mcr.microsoft.com/dotnet/sdk:8.0-alpine`, `dotnet fsi /code/code.fs`
- [x] 2.2 Create `powershell.go` and `powershell_test.go` — `mcr.microsoft.com/powershell:latest`, `pwsh -File /code/code.ps1`
- [x] 2.3 Create `racket.go` and `racket_test.go` — `racket/racket:latest`, `racket /code/code.rkt`

## Phase 3: Integration

- [x] 3.1 Update `registry.go` — Register Cobol, Objective-C, F#, PowerShell, Racket in `NewDefaultRegistry()`
- [x] 3.2 Update `registry_test.go` — Assert 39 languages sorted, update `UnknownKey` test to use `"brainfuck"` instead of `"cobol"`
- [x] 3.3 Update `gogs_handler.go` y `gogs_handler_test.go` — Add `.cbl`, `.cob`, `.m`, `.fs`, `.fsx`, `.ps1`, `.rkt` inferences