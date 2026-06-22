# Proposal: multi-lang-sandbox-oleada2

## Intent

Expand the sandbox from 9 to 13 languages by adding Java, Kotlin, Scala, and Groovy. These JVM languages are high-demand enterprise targets that extend coverage beyond scripting languages.

## Scope

### In Scope
- 4 new providers using compilation (`javac`, `kotlinc`, `scalac`, `groovyc`)
- Register providers in `ProviderRegistry`; update `registry_test.go` count 9 → 13
- Add `.groovy` mapping in `gogs_handler.go`
- Docker images with `-d /tmp` for compiler output

### Out of Scope
- JVM execution mode, dependency resolution (Maven/Gradle/sbt)
- Kotlin script (`.kts`) or Scala script (`.sc`) support
- Style linters (ktlint, scalafmt, Checkstyle, CodeNarc)
- Custom lightweight Scala/Kotlin images

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `sandbox-provider-registry`: expand from 9 to 13 languages; add JVM compilation providers
- `audit`: extend `AuditRequest` language validation to include `java`, `kotlin`, `scala`, `groovy`

## Approach

Follow the Oleada 1 Provider Pattern: one `~32` line file per language under `providers/`. All four use compilation (`-d /tmp`) because Docker mounts `/code:ro` and JVM compilers emit `.class` files. No interface changes.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `providers/java.go` | New | `javac -d /tmp` provider |
| `providers/kotlin.go` | New | `kotlinc -d /tmp` provider |
| `providers/scala.go` | New | `scalac -d /tmp` provider |
| `providers/groovy.go` | New | `groovyc -d /tmp` provider |
| `providers/registry.go` | Modified | Register 4 new providers |
| `providers/registry_test.go` | Modified | Update want-slice to 13 keys |
| `handlers/gogs_handler.go` | Modified | Add `.groovy` → `"groovy"` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Kotlin Docker image unavailable | Med | Pin `zenika/kotlin` or build custom on Temurin Alpine |
| Scala image size (~350 MB) | High | Accept for Oleada 2; custom Alpine image later |
| JVM cold-start near timeout | Low | Monitor; increase timeout if needed |
| Read-only mount + compiler output | High | Pass `-d /tmp` in every `DockerCommand` |

## Rollback Plan

Revert the commit. Pure additive change; removing provider files and registry lines restores the 9-language state.

## Dependencies

None beyond Oleada 1.

## Success Criteria

- [ ] All 4 JVM languages compile in `LocalSandbox` and `DockerSandbox`
- [ ] `ProviderRegistry` lists exactly 13 languages
- [ ] `.groovy` files map to `"groovy"` in Gogs
- [ ] Docker compilation writes `.class` to `/tmp`, not `/code`
