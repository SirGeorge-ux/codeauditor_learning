# Proposal: multi-lang-sandbox-oleada6

## Intent
Expand the sandbox from 29 to 34 languages by adding the "Crypto + Niche + BEAM" ecosystem: Solidity, Erlang, Dart, Julia, and Nim.

## Scope
**In scope:**
- Implement `LanguageProvider` for solidity, erlang, dart, julia, nim.
- Register the 5 new providers in `NewDefaultRegistry()`.
- Add handler mappings for `.sol`, `.erl`, `.dart`, `.jl`, `.nim` in `gogs_handler.go`.
- Update `registry_test.go` (29 → 34 languages).
- Update spec for sandbox-provider-registry and audit.

**Out of scope:**
- Legacy/Science ecosystem (reserved for Oleada 7).
- Full EVM execution for Solidity (we will just use compilation validation).

## Approach
Reuse the established Provider Pattern. One file per language under `providers`.

### Execution Strategy & Images
1. **Solidity** (`ethereum/solc:stable`): `solc /code/<filename>`
2. **Erlang** (`erlang:latest`): `escript /code/<filename>`
3. **Dart** (`dart:latest`): `dart run /code/<filename>`
4. **Julia** (`julia:latest`): `julia /code/<filename>`
5. **Nim** (`nimlang/nim:alpine`): `nim c -r --hints:off /code/<filename>`

## Risks
- None. These images and commands align perfectly with the 29 previously integrated languages.

## Rollback Plan
Remove the 5 new files and revert the `NewDefaultRegistry` and `inferLanguage` additions. No state or external systems are modified.