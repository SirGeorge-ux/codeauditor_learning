# Proposal: multi-lang-sandbox-oleada5

## Intent
Expand the sandbox from 23 to 29 languages by adding the "Functional + .NET + Data + Apple" ecosystem: C#, Swift, R, Haskell, Elixir, and Clojure.

## Scope
**In scope:**
- Implement `LanguageProvider` for csharp, swift, r, haskell, elixir, clojure.
- Register the 6 new providers in `NewDefaultRegistry()`.
- Add handler mappings for `.r`, `.hs`, `.ex`, `.exs`, `.clj` in `gogs_handler.go`.
- Update `registry_test.go` (23 → 29 languages).
- Update spec for sandbox-provider-registry and audit.

**Out of scope:**
- Other language ecosystems (Legacy/Niche are reserved for Oleadas 6/7).
- .NET full project compilation (we will use Mono for fast single-file execution).

## Approach
Reuse the established Provider Pattern. One file per language under `providers`.

### Execution Strategy & Images
1. **C#** (`mono:latest`): `mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe`
2. **Swift** (`swift:latest`): `swift /code/code.swift`
3. **R** (`r-base:latest`): `Rscript /code/code.r`
4. **Haskell** (`haskell:latest`): `runhaskell /code/code.hs`
5. **Elixir** (`elixir:alpine`): `elixir /code/code.exs`
6. **Clojure** (`clojure:latest`): `clojure -M /code/code.clj`

## Risks
- Mono C# compiler (`mcs`) doesn't support the latest C# 11/12 syntax features. However, for Dojo algorithm challenges, it is the most efficient way to compile and run a single un-projected `.cs` file without heavy MSBuild scaffolding inside the container.
- Large image sizes for Haskell and Clojure. (Acceptable, lazy-pulled).

## Rollback Plan
Remove the 6 new files and revert the `NewDefaultRegistry` and `inferLanguage` additions. No state or external systems are modified.