# Design: multi-lang-sandbox-oleada3

## Technical Approach

Expand the sandbox support from 13 to 17 languages by adding the Systems ecosystem: Rust, C, C++, and Zig. We will strictly follow the established Provider Pattern by creating one new file per language in the `providers` package. The providers will encapsulate language-specific configuration (Docker image, execution commands) and will be registered in `NewDefaultRegistry()`. 

For C and C++, the GCC image will be shared. For Zig, due to how `zig build-exe` writes to the current directory (which is a read-only mount in the Docker sandbox), we will wrap the command in `sh -c` to copy the source to `/tmp` before compiling.

## Architecture Decisions

### Decision: C and C++ Image

**Choice**: Use `gcc:latest` for both C and C++.
**Alternatives considered**: Use `alpine` with `apk add gcc g++`, or use `clang` images.
**Rationale**: `gcc:latest` provides the official compiler toolchain out of the box and is well known. The image size (~526 MB) is acceptable since we rely on lazy-pulls and caching. Reusing the same image for two providers is efficient.

### Decision: Zig Compilation Workaround

**Choice**: Use `alpine:latest` and execute a `sh -c` wrapper that copies the source to `/tmp`, installs zig, compiles, and runs.
**Alternatives considered**: Create a custom Docker image for Zig that sets the entrypoint and working directory, or use `zig run` which writes temporary files to a cache directory.
**Rationale**: In the Docker sandbox, the user's code is mounted at `/code` as read-only. `zig build-exe` writes the output binary to the current working directory, which will fail if executed from `/code`. Wrapping the command `["sh", "-c", "apk add --no-cache zig && cp /code/code.zig /tmp/ && cd /tmp && zig build-exe code.zig && ./code"]` keeps the logic entirely in the Go provider, avoiding the maintenance overhead of custom Docker images.

### Decision: Local Command Assumption

**Choice**: Assume standard toolchain binaries exist on the host system: `rustc`, `gcc`, `g++`, `zig`.
**Alternatives considered**: Add checks or fallback installations.
**Rationale**: Consistent with other providers (like Python's `python3` or Java's `java`). The `InstallHint()` provides guidance if the binary is missing, keeping the execution logic pure.

## Data Flow

The data flow remains identical to Oleada 1 and 2, leveraging the ProviderRegistry:

    Client Request (Zig code) ──→ API Handler ──→ DockerSandbox
                                                        │
                                                        ▼
                                       Registry.Get("zig")
                                                        │
                                                        ▼
                            Docker API ◄── Provider.DockerCommand()

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/driven/sandbox/providers/rust.go` | Create | Rust provider (`LanguageProvider` impl) |
| `backend/internal/infrastructure/driven/sandbox/providers/rust_test.go` | Create | Unit tests for Rust provider |
| `backend/internal/infrastructure/driven/sandbox/providers/c.go` | Create | C provider |
| `backend/internal/infrastructure/driven/sandbox/providers/c_test.go` | Create | Unit tests for C provider |
| `backend/internal/infrastructure/driven/sandbox/providers/cpp.go` | Create | C++ provider |
| `backend/internal/infrastructure/driven/sandbox/providers/cpp_test.go` | Create | Unit tests for C++ provider |
| `backend/internal/infrastructure/driven/sandbox/providers/zig.go` | Create | Zig provider with `sh -c` wrapper |
| `backend/internal/infrastructure/driven/sandbox/providers/zig_test.go` | Create | Unit tests for Zig provider |
| `backend/internal/infrastructure/driven/sandbox/providers/registry.go` | Modify | Register rust, c, cpp, zig in `NewDefaultRegistry()` |
| `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` | Modify | Update sorted languages assertion (13 → 17 keys) |
| `backend/internal/infrastructure/driving/handlers/gogs_handler.go` | Modify | Add `"zig": "zig"` to `inferLanguage` mapping |

## Interfaces / Contracts

No new interfaces are created. The 4 new struct types will implement the existing `ports.LanguageProvider` interface:

```go
type LanguageProvider interface {
    Language() string
    FileExtension() string
    DockerImage() string
    DockerCommand(codePath string) []string
    LocalCommand() string
    InstallHint() string
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Language Provider Methods | Verify Language(), DockerCommand(), etc. return expected string/slice literals. |
| Unit | Registry Count & Sorting | Verify `Languages()` returns 17 sorted keys including `rust`, `c`, `cpp`, `zig`. |
| Unit | Gogs Handler Inference | Verify `.zig` is inferred as `"zig"`. |
| Integration | DockerSandbox Execution | Manually verify compilation/execution runs correctly via the API endpoint for these 4 languages. |

## Migration / Rollout

No migration required. Deploying the backend binary automatically adds the 4 new languages to the registry in memory.

## Open Questions

- None.
