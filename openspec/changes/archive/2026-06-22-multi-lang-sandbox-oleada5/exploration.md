# Exploration: multi-lang-sandbox-oleada5

## Current State
The sandbox uses a `ProviderRegistry` with **23 providers**. The `gogs_handler.go` already maps `.cs` (csharp) and `.swift` (swift). However, it lacks mappings for `.r`, `.hs`, `.ex`/`.exs`, and `.clj`.

## Target Languages
Adding the "Functional + .NET + Data + Apple" ecosystem (6 languages):
1. **C#** (csharp)
2. **Swift** (swift)
3. **R** (r)
4. **Haskell** (haskell)
5. **Elixir** (elixir)
6. **Clojure** (clojure)

## Approaches & Docker Images
To run these languages as single-file scripts in the sandbox:

1. **C#**: `mcr.microsoft.com/dotnet/sdk:8.0`. Since we are running a single script, we can use `dotnet script` or compile it via `csc` (Mono). A lightweight approach for single C# files in .NET 8 without a `.csproj` is to use the `dotnet run` wrapper, but since the sandbox provides isolated files, the easiest is to compile via `csc -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe` using `mono:latest`. Or better: use `mcr.microsoft.com/dotnet/sdk:8.0-alpine` and `sh -c "cp /code/code.cs /tmp/Program.cs && cd /tmp && dotnet new console && dotnet run"`. Wait, the simplest for C# scripts is using a `dotnet-script` global tool or just a simple `mcs` / `mono` flow. Let's use `mono:latest` with `mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe`.
2. **Swift**: `swift:latest`. Command: `swift /code/code.swift`.
3. **R**: `r-base:latest`. Command: `Rscript /code/code.r`.
4. **Haskell**: `haskell:latest`. Command: `runhaskell /code/code.hs`.
5. **Elixir**: `elixir:alpine`. Command: `elixir /code/code.exs`.
6. **Clojure**: `clojure:latest`. Command: `clojure -M /code/code.clj`.

### Handler Mappings to Add
- `"r"` -> `"r"`
- `"hs"` -> `"haskell"`
- `"ex", "exs"` -> `"elixir"`
- `"clj"` -> `"clojure"`

### Recommendation
**Viable — proceed.**
The Provider Pattern scales perfectly for these 6 languages. We'll use one file per language in `providers/`. C# will use the `mono:latest` image to compile and run a single `.cs` file without needing a full `.csproj` structure. Swift, R, Haskell, Elixir, and Clojure all have official images and single-file execution commands (`swift`, `Rscript`, `runhaskell`, `elixir`, `clojure -M`). 

### Risks
- Mono might not support the absolute latest C# features compared to .NET 8, but for algorithm challenges in the Dojo, it is standard and much faster to spin up without MSBuild boilerplate.
- The `clojure:latest` image requires the script to be executed with `-M`.
- Registry will grow from 23 to 29 languages.

### Ready for Proposal
**Yes.**