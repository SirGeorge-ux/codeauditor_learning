# Exploration: multi-lang-sandbox-oleada3

## Topic
Add four systems-programming languages (Rust, C, C++, Zig) to the sandbox using the existing Provider Pattern, without modifying LocalSandbox or DockerSandbox code.

## Current State

The sandbox uses a `ProviderRegistry` (`backend/internal/infrastructure/driven/sandbox/providers/registry.go`) that currently holds **13 providers**:

- TypeScript, JavaScript, Go (original 3)
- Python, Ruby, PHP, Lua, Bash, Perl (scripting wave)
- Java, Kotlin, Scala, Groovy (JVM wave — oleada 2)

Each provider is a ~30-line struct implementing `ports.LanguageProvider`:
- `Language()` — canonical key
- `FileExtension()` — with dot
- `DockerImage()` — pinned tag
- `DockerCommand(filename)` — argv for `docker run`
- `LocalCommand()` — binary name for healthcheck
- `InstallHint()` — human-readable install instruction

Both `LocalSandbox` and `DockerSandbox` delegate 100% of language-specific logic to the registry. There are **zero switch statements** on language in the sandbox executors.

### Handler Mappings
`backend/internal/infrastructure/driving/handlers/gogs_handler.go` maps extensions to language keys:

| Extension | Language |
|-----------|----------|
| `.rs` | `rust` ✅ |
| `.c` | `c` ✅ |
| `.cpp`, `.cc`, `.cxx` | `cpp` ✅ |
| `.zig` | **missing** ❌ |

`.zig` must be added to `inferLanguage()`.

### Existing Specs
`openspec/specs/sandbox-provider-registry/spec.md` documents the 13-language registry and the per-provider contract. It does **not** yet cover systems languages.

## Affected Areas

| File | Why affected |
|------|--------------|
| `backend/internal/infrastructure/driven/sandbox/providers/registry.go` | Register 4 new providers |
| `backend/internal/infrastructure/driven/sandbox/providers/rust.go` | New provider |
| `backend/internal/infrastructure/driven/sandbox/providers/c.go` | New provider |
| `backend/internal/infrastructure/driven/sandbox/providers/cpp.go` | New provider |
| `backend/internal/infrastructure/driven/sandbox/providers/zig.go` | New provider |
| `backend/internal/infrastructure/driving/handlers/gogs_handler.go` | Add `.zig` mapping |
| `openspec/specs/sandbox-provider-registry/spec.md` | Update spec to 17 languages |
| `backend/internal/infrastructure/driven/sandbox/providers/*_test.go` | 4 new unit-test files |
| `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` | Update expected count from 13 → 17 |

## Approaches

### Option A — Pure Provider Pattern (recommended for Rust, C, C++)
Create one file per language following the exact Groovy/Java template. Register in `NewDefaultRegistry()`. No other code changes.

- **Pros**: Zero friction; identical to oleada 2; tests are copy-paste of existing `_test.go`.
- **Cons**: None for Rust/C/C++. Zig has a read-only mount issue (see Risks).
- **Effort**: Low

### Option B — Zig with `sh -c` wrapper
Because `zig build-exe` writes the binary to the current working directory and `DockerSandbox` mounts `/code` as read-only, the provider can return a DockerCommand that copies the source to `/tmp` and compiles there:

```go
func (p *ZigProvider) DockerCommand(filename string) []string {
    return []string{"sh", "-c", "cp /code/" + filename + " /tmp/ && cd /tmp && zig build-exe " + filename}
}
```

- **Pros**: No sandbox code changes; works in both Docker and local (LocalSandbox sets `cmd.Dir = tmpDir`, so `cd /tmp` is harmless).
- **Cons**: `DockerCommand()[0]` (`sh`) diverges from `LocalCommand()` (`zig`). Healthcheck still looks for `zig`, which is correct, but the local execution path is less clean.
- **Effort**: Low

### Option C — Custom Docker image for Zig with entrypoint wrapper
Build `codeauditor/zig-compiler:0.16-alpine` that wraps `zig build-exe` in a script which first `cd /tmp`.

- **Pros**: Keeps provider code pure (`["zig", "build-exe", filename]`); mirrors the Kotlin custom image approach.
- **Cons**: Requires maintaining a new image; adds build/publish step.
- **Effort**: Medium

### Option D — Extend `LanguageProvider` interface with `DockerWorkDir()`
Add an optional method so `DockerSandbox` can set `-w /tmp` per provider.

- **Pros**: Cleanest long-term solution; future-proofs for other compile-to-cwd languages.
- **Cons**: Touches `ports/LanguageProvider`, both sandboxes, and all 13 existing tests. Violates the "no existing code changes" goal.
- **Effort**: High

## Docker Images

| Language | Recommended Image | Size (approx) | Notes |
|----------|-------------------|---------------|-------|
| **Rust** | `rust:1.96-alpine` | ~200 MB | Official, Alpine-based, pinned. |
| **C** | `gcc:15.3.0` | ~526 MB | Official, covers `gcc`. Same image reused for C++. |
| **C++** | `gcc:15.3.0` | ~526 MB | Reuses C image; command uses `g++`. |
| **Zig** | `codeauditor/zig-compiler:0.16-alpine` (custom) | ~80–150 MB | No official image. Alpine edge has `zig 0.16.0-r1` (36 MB pkg). Building a custom image follows the Kotlin precedent. Alternative: `rawpair/zig:bookworm` (~354 MB, unofficial, older). |

## Local Execution Commands

| Language | Local binary | Typical install | Command |
|----------|--------------|-----------------|---------|
| Rust | `rustc` | `rustup` | `rustc -o /tmp/out code.rs` |
| C | `gcc` | `apt install gcc` / `brew install gcc` | `gcc -o /tmp/out code.c` |
| C++ | `g++` | `apt install g++` / `brew install gcc` | `g++ -o /tmp/out code.cpp` |
| Zig | `zig` | `apk add zig` (Alpine) / download from ziglang.org | `zig build-exe code.zig` (local tmpDir is writable) |

## Risks

1. **Zig read-only mount friction** — `zig build-exe` emits the binary to the current working directory. In DockerSandbox `/code` is read-only. The `sh -c` wrapper (Option B) or a custom image (Option C) mitigates this. If we choose Option B, the provider is slightly less idiomatic but functionally correct.
2. **Large Docker images** — `gcc:15.3.0` is >500 MB. Pulling it for the first time will be slow, but the sandbox already pulls images lazily during healthcheck. This is acceptable for a dev/audit tool.
3. **Zig version stability** — Zig is pre-1.0 (0.16). Syntax and CLI flags may change between releases. Pinning the image tag mitigates breakage.
4. **Handler extension missing** — `.zig` is not mapped in `gogs_handler.go`. Easy fix, but if missed the frontend will report "unknown" language for `.zig` files.
5. **Registry test hard-codes language count** — `registry_test.go` asserts exactly 13 languages. Adding 4 providers will break this test until updated to 17.

## Recommendation

**Viable — proceed with Option A for Rust/C/C++ and Option B (or C) for Zig.**

The Provider Pattern scales perfectly for Rust, C, and C++. Each is a 30-line provider file plus a 50-line test file, plus one registry line. No existing sandbox code needs to change.

For Zig, the read-only mount is the only wrinkle. The **recommended path** is:
1. Try Option B (`sh -c` wrapper) in the provider file. It keeps everything self-contained in Go code.
2. If the team dislikes the `sh` indirection, fall back to Option C (custom image) following the Kotlin precedent.

Either way, the change is low-risk and low-effort.

## Ready for Proposal

**Yes.** The orchestrator can tell the user:
> "Exploration confirms the Provider Pattern scales cleanly. We need 4 new provider files, 4 tests, registry wiring, and a `.zig` handler mapping. Rust/C/C++ are trivial. Zig needs a small workaround for the read-only Docker mount. Total estimated effort: low."

## Key Learnings

- DockerSandbox mounts `/code` as read-only with `--tmpfs /tmp:rw`. Any compiler that writes output to the working directory (like `zig build-exe`) needs special handling.
- The `gcc` official image covers both C (`gcc`) and C++ (`g++`), so C and C++ providers can share the same `DockerImage()` value.
- `registry_test.go` hard-codes the expected language count and sorted list; any new oleada must update it.
