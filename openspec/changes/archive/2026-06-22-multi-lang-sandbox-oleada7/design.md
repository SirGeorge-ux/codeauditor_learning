# Design: multi-lang-sandbox-oleada7

## Technical Approach
Complete the sandbox ecosystem expansion to 39 languages by adding PowerShell, Objective-C, F#, Cobol, and Racket. 

We will strictly follow the established Provider Pattern by creating one new file per language in the `providers` package. 

All 5 languages have well-defined CLI strategies. Objective-C and Cobol require standard C-like compilation wrappers (`gcc` and `cobc` respectively). PowerShell, F# (`fsi`), and Racket all execute directly as scripts. 

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `providers/powershell.go` | Create | PowerShell provider (`pwsh`) |
| `providers/powershell_test.go` | Create | Tests for PowerShell provider |
| `providers/objective_c.go` | Create | Objective-C provider (`gcc`) |
| `providers/objective_c_test.go` | Create | Tests for Objective-C provider |
| `providers/fsharp.go` | Create | F# provider (`dotnet fsi`) |
| `providers/fsharp_test.go` | Create | Tests for F# provider |
| `providers/cobol.go` | Create | Cobol provider (`cobc`) |
| `providers/cobol_test.go` | Create | Tests for Cobol provider |
| `providers/racket.go` | Create | Racket provider (`racket`) |
| `providers/racket_test.go` | Create | Tests for Racket provider |
| `providers/registry.go` | Modify | Register all 5 new providers |
| `providers/registry_test.go` | Modify | Update sorted keys array (34 → 39). Change `"cobol"` test to `"brainfuck"` |
| `handlers/gogs_handler.go` | Modify | Add `.ps1`, `.m`, `.fs`, `.fsx`, `.cbl`, `.cob`, `.rkt` cases |
| `handlers/gogs_handler_test.go` | Modify | Add cases for the new extensions |