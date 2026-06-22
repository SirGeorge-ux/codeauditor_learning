# Tasks: Multi-lang Sandbox — Oleada 3 (Rust, C, C++, Zig)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~320–350 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr-default |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | All 4 providers + registry update + handler + tests | Single PR | Base branch: main |

## Phase 1: Rust Provider

- [x] 1.1 Create `backend/internal/infrastructure/driven/sandbox/providers/rust.go` with `RustProvider` (Language: "rust", Ext: ".rs", Image: "rust:1.96-alpine", DockerCommand: `sh -c rustc + exec`, LocalCommand: "rustc", InstallHint)
- [x] 1.2 Create `backend/internal/infrastructure/driven/sandbox/providers/rust_test.go` — verify Language(), FileExtension(), DockerImage(), LocalCommand(), DockerCommand(), non-empty InstallHint

## Phase 2: C Provider

- [x] 2.1 Create `backend/internal/infrastructure/driven/sandbox/providers/c.go` with `CProvider` (Language: "c", Ext: ".c", Image: "gcc:15.3.0", DockerCommand: `sh -c gcc + exec`, LocalCommand: "gcc", InstallHint)
- [x] 2.2 Create `backend/internal/infrastructure/driven/sandbox/providers/c_test.go` — verify all provider methods

## Phase 3: C++ Provider

- [x] 3.1 Create `backend/internal/infrastructure/driven/sandbox/providers/cpp.go` with `CppProvider` (Language: "cpp", Ext: ".cpp", Image: "gcc:15.3.0", DockerCommand: `sh -c g++ + exec`, LocalCommand: "g++", InstallHint)
- [x] 3.2 Create `backend/internal/infrastructure/driven/sandbox/providers/cpp_test.go` — verify all provider methods

## Phase 4: Zig Provider

- [x] 4.1 Create `backend/internal/infrastructure/driven/sandbox/providers/zig.go` with `ZigProvider` (Language: "zig", Ext: ".zig", Image: "alpine:latest", DockerCommand: `sh -c` wrapper that apk adds zig, copies to /tmp, build-exe, and executes; LocalCommand: "zig", InstallHint)
- [x] 4.2 Create `backend/internal/infrastructure/driven/sandbox/providers/zig_test.go` — verify Language(), FileExtension(), DockerImage(), LocalCommand(), non-empty InstallHint, DockerCommand returns `sh -c` wrapper with zig build-exe flow

## Phase 5: Registry and Handler Integration

- [x] 5.1 Modify `backend/internal/infrastructure/driven/sandbox/providers/registry.go` — register RustProvider, CProvider, CppProvider, ZigProvider in `NewDefaultRegistry()`
- [x] 5.2 Modify `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` — update `TestProviderRegistry_Languages` expected count from 13 to 17 (add rust, c, cpp, zig sorted); update `TestProviderRegistry_Get_UnknownKey` (rust is no longer unknown)
- [x] 5.3 Modify `backend/internal/infrastructure/driving/handlers/gogs_handler.go` — add `case "zig": return "zig"` to `inferLanguage()`
- [x] 5.4 Add `{"main.zig", "zig"}` test case to `inferLanguage` table test in `gogs_handler_test.go`
