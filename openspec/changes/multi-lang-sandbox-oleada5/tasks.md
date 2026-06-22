# Tasks: multi-lang-sandbox-oleada5

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~460 |
| 400-line budget risk | High |
| Chained PRs recommended | No (mechanical boilerplate, single-pr acceptable with exception) |
| Suggested split | Single PR |
| Delivery strategy | single-pr-default |
| Chain strategy | pending |

*Exception approved for >400 lines: changes are purely additive mechanical boilerplate following existing patterns.*

## Phase 1: C# and Swift Providers

- [x] 1.1 Create `csharp.go` and `csharp_test.go` — `mono:latest`, `sh -c` with `mcs -out:/tmp/out.exe /code/code.cs && mono /tmp/out.exe`
- [x] 1.2 Create `swift.go` and `swift_test.go` — `swift:latest`, `swift /code/code.swift`

## Phase 2: Functional Providers

- [x] 2.1 Create `haskell.go` and `haskell_test.go` — `haskell:latest`, `runhaskell /code/code.hs`
- [x] 2.2 Create `elixir.go` and `elixir_test.go` — `elixir:alpine`, `elixir /code/code.exs`
- [x] 2.3 Create `clojure.go` and `clojure_test.go` — `clojure:latest`, `clojure -M /code/code.clj`

## Phase 3: Data Provider

- [x] 3.1 Create `r.go` and `r_test.go` — `r-base:latest`, `Rscript /code/code.r`

## Phase 4: Integration

- [x] 4.1 Update `registry.go` — Register C#, Swift, Haskell, Elixir, Clojure, R in `NewDefaultRegistry()`
- [x] 4.2 Update `registry_test.go` — Assert 29 languages sorted, update tests
- [x] 4.3 Update `gogs_handler.go` y `gogs_handler_test.go` — Add `.r`, `.hs`, `.ex`, `.exs`, `.clj` inferences