# Tasks: multi-lang-sandbox-oleada6

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~400 |
| 400-line budget risk | Low-Medium |
| Chained PRs recommended | No (mechanical boilerplate, single-pr acceptable with exception) |
| Suggested split | Single PR |
| Delivery strategy | single-pr-default |
| Chain strategy | pending |

*Exception approved for >400 lines: changes are purely additive mechanical boilerplate following existing patterns.*

## Phase 1: Crypto & BEAM Providers

- [x] 1.1 Create `solidity.go` and `solidity_test.go` — `ethereum/solc:stable`, `solc /code/code.sol`
- [x] 1.2 Create `erlang.go` and `erlang_test.go` — `erlang:latest`, `escript /code/code.erl`

## Phase 2: Niche Providers

- [x] 2.1 Create `dart.go` and `dart_test.go` — `dart:latest`, `dart run /code/code.dart`
- [x] 2.2 Create `julia.go` and `julia_test.go` — `julia:latest`, `julia /code/code.jl`
- [x] 2.3 Create `nim.go` and `nim_test.go` — `nimlang/nim:alpine`, `nim c -r --hints:off /code/code.nim`

## Phase 3: Integration

- [x] 3.1 Update `registry.go` — Register Solidity, Erlang, Dart, Julia, Nim in `NewDefaultRegistry()`
- [x] 3.2 Update `registry_test.go` — Assert 34 languages sorted, update tests
- [x] 3.3 Update `gogs_handler.go` y `gogs_handler_test.go` — Add `.sol`, `.erl`, `.dart`, `.jl`, `.nim` inferences