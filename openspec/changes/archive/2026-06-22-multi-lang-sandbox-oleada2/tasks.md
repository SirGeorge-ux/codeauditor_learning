# Tasks: Multi-Lang Sandbox — JVM Languages (Oleada 2)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~354 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-forecast |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | All 4 JVM providers + registry + gogs | PR 1 | Single PR (<400 lines), base = feature/tracker |

## Phase 1: Java Provider

- [x] 1.1 Create `providers/java.go` — `JavaProvider` with `javac -d /tmp`
- [x] 1.2 Create `providers/java_test.go` — test Language, FileExtension, DockerImage, LocalCommand, DockerCommand, InstallHint

## Phase 2: Kotlin Provider

- [x] 2.1 Create `providers/kotlin.go` — `KotlinProvider` with `kotlinc -d /tmp`
- [x] 2.2 Create `providers/kotlin_test.go` — same test structure as Java

## Phase 3: Scala Provider

- [x] 3.1 Create `providers/scala.go` — `ScalaProvider` with `scalac -d /tmp`
- [x] 3.2 Create `providers/scala_test.go` — same test structure

## Phase 4: Groovy Provider

- [x] 4.1 Create `providers/groovy.go` — `GroovyProvider` with `groovyc -d /tmp`
- [x] 4.2 Create `providers/groovy_test.go` — same test structure

## Phase 5: Integration & Fixes

- [x] 5.1 Register all 4 providers in `providers/registry.go` — add `NewJavaProvider`, `NewKotlinProvider`, `NewScalaProvider`, `NewGroovyProvider` calls to `NewDefaultRegistry()`
- [x] 5.2 Update `providers/registry_test.go` — add `java`, `kotlin`, `scala`, `groovy` to registered keys; update `Languages()` want-slice from 9 to 13
- [x] 5.3 Add `case "groovy": return "groovy"` in `inferLanguage()` at `handlers/gogs_handler.go`

## Phase 6: Verification

- [x] 6.1 Run `go vet ./...` — no new warnings
- [x] 6.2 Run `go test ./...` — all tests pass (13 registered, 4 new providers)
- [x] 6.3 Run `go test -short ./...` — Docker integration skipped cleanly
