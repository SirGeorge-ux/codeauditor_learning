# Proposal: Multi-Lang Sandbox — Systems Oleada 3 (Rust, C, C++, Zig)

## Intent

Expand the sandbox from 13 to 17 languages by adding the systems-programming ecosystem (Rust, C, C++, Zig). This continues the proven Provider Pattern strategy with minimal risk and no changes to LocalSandbox or DockerSandbox.

## Scope

### In Scope
- Implement `LanguageProvider` for rust, c, cpp, zig
- Register the 4 providers in `NewDefaultRegistry()`
- Add `.zig` extension mapping in `gogs_handler.go`
- Update `registry_test.go` (13 → 17 languages)
- Update `openspec/specs/sandbox-provider-registry/spec.md`

### Out of Scope
- Additional language ecosystems beyond these 4
- Custom Docker image for Zig (use `sh -c` wrapper)
- Changes to `LanguageProvider` interface or sandbox executor logic

## Capabilities

### New Capabilities
- `sandbox-provider-rust`: Rust sandbox execution via `rust:1.96-alpine`
- `sandbox-provider-c`: C sandbox execution via `gcc:15.3.0`
- `sandbox-provider-cpp`: C++ sandbox execution via `gcc:15.3.0`
- `sandbox-provider-zig`: Zig sandbox execution via `alpine:latest` + `apk add zig` with `sh -c` wrapper

### Modified Capabilities
- `sandbox-provider-registry`: Expand from 13 to 17 registered languages; update spec requirements and test expectations

## Approach

Reuse the existing Provider Pattern. One file per language (~30 lines) plus matching `_test.go`. No sandbox executor changes.

- **Rust**: `rustc -o /tmp/out /tmp/code.rs && /tmp/out`
- **C**: `gcc -o /tmp/out /tmp/code.c && /tmp/out`
- **C++**: `g++ -o /tmp/out /tmp/code.cpp && /tmp/out`
- **Zig**: Use a `sh -c` wrapper in `DockerCommand` to copy source to `/tmp`, compile there, and run. This avoids read-only `/code` mount issues without changing DockerSandbox.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/driven/sandbox/providers/rust.go` | New | Rust provider |
| `backend/internal/infrastructure/driven/sandbox/providers/c.go` | New | C provider |
| `backend/internal/infrastructure/driven/sandbox/providers/cpp.go` | New | C++ provider |
| `backend/internal/infrastructure/driven/sandbox/providers/zig.go` | New | Zig provider |
| `backend/internal/infrastructure/driven/sandbox/providers/registry.go` | Modified | Register 4 new providers |
| `backend/internal/infrastructure/driven/sandbox/providers/registry_test.go` | Modified | Assert 17 languages |
| `backend/internal/infrastructure/driving/handlers/gogs_handler.go` | Modified | Add `.zig` → `zig` mapping |
| `openspec/specs/sandbox-provider-registry/spec.md` | Modified | Document 17 languages |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Large gcc image (~526 MB) slows first pull | Low | Lazy-pull already in place; image shared by C and C++ |
| Zig pre-1.0 CLI instability | Low | Pin `zig` package version in `apk add` |
| `zig build-exe` writes to cwd (read-only mount) | High | Use `sh -c` wrapper to compile in `/tmp` |
| Registry test hard-codes 13 | Med | Update to 17 in this change |

## Rollback Plan

1. Revert the single commit adding these 4 providers.
2. If partially deployed, remove the 4 provider files and revert registry wiring; no other code depends on them.

## Dependencies

- `gcc:15.3.0` and `rust:1.96-alpine` images must be pullable from Docker Hub.
- `alpine:latest` must have `zig` package available in default repositories.

## Success Criteria

- [ ] `registry.Languages()` returns 17 keys including `rust`, `c`, `cpp`, `zig`
- [ ] `registry.Get("zig")` returns a provider with `FileExtension() == ".zig"`
- [ ] `gogs_handler.go` maps `.zig` to `zig`
- [ ] Docker healthcheck passes for all 17 providers
- [ ] All provider unit tests pass (`go test ./...`)
