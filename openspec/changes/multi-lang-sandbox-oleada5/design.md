# Design: multi-lang-sandbox-oleada5

## Technical Approach
Expand the sandbox support from 23 to 29 languages by adding C#, Swift, R, Haskell, Elixir, and Clojure. 

We will strictly follow the established Provider Pattern by creating one new file per language in the `providers` package. 

- **C#**: Will use `mono:latest` to compile and run via `mcs` and `mono` in a shell wrapper. This avoids the heavy scaffolding required by `dotnet run` for single isolated script files.
- **Swift, R, Haskell, Elixir, Clojure**: Will use their respective official/standard docker images and execute directly via their CLI runners (`swift`, `Rscript`, `runhaskell`, `elixir`, `clojure -M`).

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `providers/csharp.go` | Create | C# provider (uses `mono`) |
| `providers/csharp_test.go` | Create | Tests for C# provider |
| `providers/swift.go` | Create | Swift provider |
| `providers/swift_test.go` | Create | Tests for Swift provider |
| `providers/r.go` | Create | R provider |
| `providers/r_test.go` | Create | Tests for R provider |
| `providers/haskell.go` | Create | Haskell provider |
| `providers/haskell_test.go` | Create | Tests for Haskell provider |
| `providers/elixir.go` | Create | Elixir provider |
| `providers/elixir_test.go` | Create | Tests for Elixir provider |
| `providers/clojure.go` | Create | Clojure provider |
| `providers/clojure_test.go` | Create | Tests for Clojure provider |
| `providers/registry.go` | Modify | Register all 6 new providers |
| `providers/registry_test.go` | Modify | Update sorted keys array (23 → 29) |
| `handlers/gogs_handler.go` | Modify | Add `.r`, `.hs`, `.ex`, `.exs`, `.clj` inference cases |
| `handlers/gogs_handler_test.go` | Modify | Add cases for the new extensions |