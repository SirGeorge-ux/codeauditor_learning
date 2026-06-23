# Design: multi-lang-sandbox-oleada6

## Technical Approach
Expand the sandbox support from 29 to 34 languages by adding Solidity, Erlang, Dart, Julia, and Nim. 

We will strictly follow the established Provider Pattern by creating one new file per language in the `providers` package. 

All 5 languages have standard or official execution strategies that fit perfectly into the `LanguageProvider` port interface without any complex wrappers or workarounds.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `providers/solidity.go` | Create | Solidity provider (`solc`) |
| `providers/solidity_test.go` | Create | Tests for Solidity provider |
| `providers/erlang.go` | Create | Erlang provider (`escript`) |
| `providers/erlang_test.go` | Create | Tests for Erlang provider |
| `providers/dart.go` | Create | Dart provider (`dart run`) |
| `providers/dart_test.go` | Create | Tests for Dart provider |
| `providers/julia.go` | Create | Julia provider (`julia`) |
| `providers/julia_test.go` | Create | Tests for Julia provider |
| `providers/nim.go` | Create | Nim provider (`nim c -r --hints:off`) |
| `providers/nim_test.go` | Create | Tests for Nim provider |
| `providers/registry.go` | Modify | Register all 5 new providers |
| `providers/registry_test.go` | Modify | Update sorted keys array (29 → 34) |
| `handlers/gogs_handler.go` | Modify | Add `.sol`, `.erl`, `.dart`, `.jl`, `.nim` inference cases |
| `handlers/gogs_handler_test.go` | Modify | Add cases for the new extensions |