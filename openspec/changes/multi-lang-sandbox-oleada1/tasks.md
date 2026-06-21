# Tasks: Multi-Lang Sandbox — Provider Pattern + 6 Languages

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~730 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Interface + registry + TS/Go extraction | PR 1 | base = feature/tracker branch |
| 2 | 6 new language providers + update registry | PR 2 | base = PR #1 branch |
| 3 | Sandbox refactoring, tests, wiring, fixes | PR 3 | base = PR #2 branch |

## Phase 1: Provider Interface & Registry

- [x] 1.1 Create `LanguageProvider` interface — `backend/internal/ports/provider.go`
- [x] 1.2 Create `ProviderRegistry` with `Register()`/`Get()` — `providers/registry.go`
- [x] 1.3 Write registry tests (valid key, unknown key, overwrite, Languages) — `providers/registry_test.go`

## Phase 2: Extract Existing Language Providers

- [x] 2.1 Extract TypeScript/JavaScript provider from current switch — `providers/typescript.go`
- [x] 2.2 Extract Go provider from current switch — `providers/go.go`
- [x] 2.3 Write unit tests for TS and Go providers — `providers/typescript_test.go`, `providers/go_test.go`

## Phase 3: New Language Providers

- [x] 3.1 Create Python provider — `providers/python.go`
- [x] 3.2 Create Ruby provider — `providers/ruby.go`
- [x] 3.3 Create PHP provider — `providers/php.go`
- [x] 3.4 Create Lua provider — `providers/lua.go`
- [x] 3.5 Create Bash provider — `providers/bash.go`
- [x] 3.6 Create Perl provider — `providers/perl.go`
- [x] 3.7 Write unit tests for all 6 new providers — `providers/*_test.go`
- [x] 3.8 Update `NewDefaultRegistry()` to register all 6 — `providers/registry.go`

## Phase 4: Refactor Sandboxes

- [x] 4.1 Replace `switch language` with `registry.Get(lang)` in LocalSandbox — `localsandbox.go`
- [x] 4.2 Replace `switch language` with `registry.Get(lang)` in DockerSandbox — `dockersandbox.go`
- [x] 4.3 Refactor `localsandbox_test.go` to table-driven (all 8 languages) — `localsandbox_test.go`

## Phase 5: DockerSandbox Tests

- [x] 5.1 Add unit tests for command generation (no Docker daemon) — `dockersandbox_test.go`
- [x] 5.2 Add integration tests (skippable with `-short`) — `dockersandbox_test.go`

## Phase 6: Wiring & Fixes

- [x] 6.1 Wire `providers.NewDefaultRegistry()` into both sandboxes — `backend/cmd/api/main.go`
- [x] 6.2 Fix `"shell"` → `"bash"` in `inferLanguage()` — `gogs_handler.go`
- [x] 6.3 Update `SandboxExecutor` comment — supported languages — `ports/sandbox.go`

## Phase 7: Verification

- [ ] 7.1 Run `go vet ./...` — no new warnings
- [ ] 7.2 Run `go test ./...` — all existing + new tests pass
- [ ] 7.3 Run `go test -short ./...` — Docker integration skipped cleanly
