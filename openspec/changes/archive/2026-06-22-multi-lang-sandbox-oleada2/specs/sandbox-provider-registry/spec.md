# Delta for Sandbox Provider Registry

## ADDED Requirements

### Requirement: JVM Provider — Java

The system MUST provide a `JavaProvider` implementing `LanguageProvider`.

| Method | Value |
|--------|-------|
| `Language()` | `"java"` |
| `FileExtension()` | `".java"` |
| `DockerImage()` | `"eclipse-temurin:21-jdk-alpine"` |
| `DockerCommand(filename)` | `["javac", "-d", "/tmp", filename]` |
| `LocalCommand()` | `"javac"` |
| `InstallHint()` | `"SDKMAN: sdk install java 21-tem (or apt/brew install openjdk-21-jdk)"` |

#### Scenario: Java provider returns canonical key

- GIVEN a `JavaProvider`
- WHEN `Language()` is called
- THEN it MUST return `"java"`

#### Scenario: Java Docker command uses /tmp output

- GIVEN a `JavaProvider`
- WHEN `DockerCommand("Main.java")` is called
- THEN it MUST return `["javac", "-d", "/tmp", "Main.java"]`

#### Scenario: Java local command is compiler

- GIVEN a `JavaProvider`
- WHEN `LocalCommand()` is called
- THEN it MUST return `"javac"`

---

### Requirement: JVM Provider — Kotlin

The system MUST provide a `KotlinProvider` implementing `LanguageProvider`.

| Method | Value |
|--------|-------|
| `Language()` | `"kotlin"` |
| `FileExtension()` | `".kt"` |
| `DockerImage()` | `"codeauditor/kotlin-compiler:2.0-alpine"` (or pinned community image) |
| `DockerCommand(filename)` | `["kotlinc", "-d", "/tmp", filename]` |
| `LocalCommand()` | `"kotlinc"` |
| `InstallHint()` | `"SDKMAN: sdk install kotlin"` |

#### Scenario: Kotlin provider returns canonical key

- GIVEN a `KotlinProvider`
- WHEN `Language()` is called
- THEN it MUST return `"kotlin"`

#### Scenario: Kotlin Docker command uses /tmp output

- GIVEN a `KotlinProvider`
- WHEN `DockerCommand("App.kt")` is called
- THEN it MUST return `["kotlinc", "-d", "/tmp", "App.kt"]`

---

### Requirement: JVM Provider — Scala

The system MUST provide a `ScalaProvider` implementing `LanguageProvider`.

| Method | Value |
|--------|-------|
| `Language()` | `"scala"` |
| `FileExtension()` | `".scala"` |
| `DockerImage()` | `"scala:3.3.1-slim"` |
| `DockerCommand(filename)` | `["scalac", "-d", "/tmp", filename]` |
| `LocalCommand()` | `"scalac"` |
| `InstallHint()` | `"SDKMAN: sdk install scala (or use Coursier: cs setup)"` |

#### Scenario: Scala provider returns canonical key

- GIVEN a `ScalaProvider`
- WHEN `Language()` is called
- THEN it MUST return `"scala"`

#### Scenario: Scala Docker command uses /tmp output

- GIVEN a `ScalaProvider`
- WHEN `DockerCommand("App.scala")` is called
- THEN it MUST return `["scalac", "-d", "/tmp", "App.scala"]`

---

### Requirement: JVM Provider — Groovy

The system MUST provide a `GroovyProvider` implementing `LanguageProvider`.

| Method | Value |
|--------|-------|
| `Language()` | `"groovy"` |
| `FileExtension()` | `".groovy"` |
| `DockerImage()` | `"groovy:4.0-jdk21-alpine"` |
| `DockerCommand(filename)` | `["groovyc", "-d", "/tmp", filename]` |
| `LocalCommand()` | `"groovyc"` |
| `InstallHint()` | `"SDKMAN: sdk install groovy"` |

#### Scenario: Groovy provider returns canonical key

- GIVEN a `GroovyProvider`
- WHEN `Language()` is called
- THEN it MUST return `"groovy"`

#### Scenario: Groovy Docker command uses /tmp output

- GIVEN a `GroovyProvider`
- WHEN `DockerCommand("Script.groovy")` is called
- THEN it MUST return `["groovyc", "-d", "/tmp", "Script.groovy"]`

---

### Requirement: ProviderRegistry Expanded to 13 Languages

The system MUST register all 13 providers in `NewDefaultRegistry()`: the 9 existing (python, ruby, php, lua, bash, perl, typescript, javascript, go) plus 4 new (java, kotlin, scala, groovy).

#### Scenario: Registry lists 13 languages

- GIVEN all providers are registered
- WHEN `registry.Languages()` is called
- THEN it MUST return exactly 13 sorted keys including `"java"`, `"kotlin"`, `"scala"`, `"groovy"`

#### Scenario: Registry resolves new JVM languages

- GIVEN the default registry
- WHEN `registry.Get("java")`, `registry.Get("kotlin")`, `registry.Get("scala")`, and `registry.Get("groovy")` are called
- THEN each MUST return the corresponding provider without error

#### Scenario: Registry test expects 13 keys

- GIVEN `registry_test.go`
- WHEN the `Languages()` test assertion runs
- THEN the expected slice MUST contain 13 entries
