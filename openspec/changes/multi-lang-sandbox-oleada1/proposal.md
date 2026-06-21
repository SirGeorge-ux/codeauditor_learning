# Proposal: multi-lang-sandbox-oleada1

## Intent

Expand the sandbox from 2 to 8 languages and eliminate inline switch duplication by introducing a **Provider pattern** (Strategy + Registry). This creates a scalable foundation for 35+ languages in future oleadas.

## Scope

### In Scope
- 6 new language providers: `python`, `ruby`, `php`, `lua`, `bash`, `perl`
- `LanguageProvider` interface + `ProviderRegistry` with `Register()`/`Get()`
- Refactor `LocalSandbox` and `DockerSandbox` to delegate to registry
- Extract existing `typescript`/`javascript`/`go` into providers too
- `dockersandbox_test.go` (unit + integration, skippable with `-short`)
- Fix `"shell"` → `"bash"` mismatch in `gogs_handler.go`
- `InstallHint()` for local tools; never auto-install

### Out of Scope
- PowerShell (future "heavy" oleada, ~300MB image)
- YAML-based `.auditor-rules.yaml` linting configs (follow-up change)
- Auto-installation of missing local tools

## Capabilities

### New Capabilities
- `sandbox-provider-registry`: `LanguageProvider` interface, `ProviderRegistry`, and one file per language provider (~30 lines each)

### Modified Capabilities
- `audit`: expand `SandboxExecutor` language support from 2 to 8 languages; update port comment

## Approach

Introduce a `LanguageProvider` interface in `internal/core/provider/` (or `ports/`) with methods: `LocalCmd()`, `DockerImage()`, `DockerCmd()`, `Filename()`, `InstallHint()`. Build a `ProviderRegistry` populated at startup. Both `LocalSandbox` and `DockerSandbox` call `registry.Get(lang)` instead of switches. Each language lives in its own file under `sandbox/providers/`. Healthcheck iterates providers and reports availability per language with install hints.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `sandbox/providers/` | New | One provider file per language |
| `sandbox/localsandbox.go` | Modified | Delegate to registry; remove switch |
| `sandbox/dockersandbox.go` | Modified | Delegate to registry; remove switch |
| `sandbox/dockersandbox_test.go` | New | Table-driven unit + integration tests |
| `sandbox/localsandbox_test.go` | Modified | Refactor to table-driven; test all 8 languages |
| `handlers/gogs_handler.go` | Modified | `.sh` → `"bash"` |
| `ports/sandbox.go` | Modified | Update supported-language comment |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Docker image pull latency (8 images) | Med | Timeout-configurable healthcheck; lazy pull on first use |
| No existing DockerSandbox tests | High | Create `dockersandbox_test.go` as part of this change |
| Local tool missing | Med | `InstallHint()` returns command; healthcheck reports gracefully |

## Rollback Plan

Revert to the pre-change git commit. The Provider pattern is a pure refactor with no data migration; old switch logic is fully replaced by registry delegation.

## Dependencies

None.

## Success Criteria

- [ ] All 8 languages execute correctly in `LocalSandbox` (when tool installed) and `DockerSandbox`
- [ ] `ProviderRegistry` returns the correct provider for every supported language key
- [ ] `DockerSandbox` has unit tests (command generation) and integration tests (skippable with `-short`)
- [ ] `gogs_handler.go` returns `"bash"` for `.sh` files
- [ ] Healthcheck reports missing local tools gracefully with `InstallHint()` output
