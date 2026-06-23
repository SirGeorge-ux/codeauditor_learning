# Exploration: multi-lang-sandbox-oleada6

## Current State
The sandbox uses a `ProviderRegistry` with **29 providers**. We need to add the "Crypto + Niche + BEAM" ecosystem to reach 34 languages.

## Target Languages
Adding 5 languages:
1. **Solidity** (solidity)
2. **Erlang** (erlang)
3. **Dart** (dart)
4. **Julia** (julia)
5. **Nim** (nim)

## Approaches & Docker Images
To run these languages as single-file scripts in the sandbox:

1. **Solidity**: `ethereum/solc:stable`. We can't "execute" a smart contract without a blockchain or EVM wrapper. However, for auditing code, syntax compilation is the primary requirement. Command: `solc /code/<filename>`.
2. **Erlang**: `erlang:latest`. Command: `escript /code/<filename>`. (Running Erlang as a script is easier than compiling modules).
3. **Dart**: `dart:latest`. Command: `dart run /code/<filename>`.
4. **Julia**: `julia:latest`. Command: `julia /code/<filename>`.
5. **Nim**: `nimlang/nim:alpine`. Command: `nim c -r --hints:off /code/<filename>`. Nim compiles and runs immediately with `-r`.

### Handler Mappings to Add
- `"sol"` -> `"solidity"`
- `"erl"` -> `"erlang"`
- `"dart"` -> `"dart"`
- `"jl"` -> `"julia"`
- `"nim"` -> `"nim"`

### Recommendation
**Viable — proceed.**
The Provider Pattern perfectly fits these. Solidity just needs a compilation check via `solc`. Erlang via `escript` works seamlessly for single files. Dart, Julia, and Nim have official execution wrappers.

### Risks
- `solc` will only output syntax and compilation errors; it won't produce dynamic execution output unless there's an error. This is fine for a static auditor.
- Registry will grow from 29 to 34 languages.

### Ready for Proposal
**Yes.**