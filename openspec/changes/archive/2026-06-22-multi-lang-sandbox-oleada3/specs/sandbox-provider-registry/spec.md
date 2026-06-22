# Delta for Sandbox Provider Registry

## ADDED Requirements

### Requirement: Systems Language Providers (Rust, C, C++, Zig)

The system MUST provide four new `LanguageProvider` implementations for systems programming languages.

#### Rust Provider

| Method | Value |
|--------|-------|
| `Language()` | `"rust"` |
| `FileExtension()` | `".rs"` |
| `DockerImage()` | `"rust:1.96-alpine"` |
| `DockerCommand(filename)` | `["sh", "-c", "rustc -o /tmp/out /tmp/code.rs && /tmp/out"]` |
| `LocalCommand()` | `"rustc"` |
| `InstallHint()` | `"rustup: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh"` |

#### C Provider

| Method | Value |
|--------|-------|
| `Language()` | `"c"` |
| `FileExtension()` | `".c"` |
| `DockerImage()` | `"gcc:15.3.0"` |
| `DockerCommand(filename)` | `["sh", "-c", "gcc -o /tmp/out /tmp/code.c && /tmp/out"]` |
| `LocalCommand()` | `"gcc"` |
| `InstallHint()` | `"apt install gcc (or brew install gcc)"` |

#### C++ Provider

| Method | Value |
|--------|-------|
| `Language()` | `"cpp"` |
| `FileExtension()` | `".cpp"` |
| `DockerImage()` | `"gcc:15.3.0"` |
| `DockerCommand(filename)` | `["sh", "-c", "g++ -o /tmp/out /tmp/code.cpp && /tmp/out"]` |
| `LocalCommand()` | `"g++"` |
| `InstallHint()` | `"apt install g++ (or brew install gcc)"` |

#### Zig Provider

| Method | Value |
|--------|-------|
| `Language()` | `"zig"` |
| `FileExtension()` | `".zig"` |
| `DockerImage()` | `"alpine:latest"` |
| `DockerCommand(filename)` | `["sh", "-c", "apk add zig && cp /code/" + filename + " /tmp/ && cd /tmp && zig build-exe " + filename + " && ./" + filename]` |
| `LocalCommand()` | `"zig"` |
| `InstallHint()` | `"apk add zig (Alpine) or download from ziglang.org"` |

#### Scenario: Rust provider returns canonical key

- GIVEN a `RustProvider`
- WHEN `Language()` is called
- THEN it MUST return `"rust"`

#### Scenario: C provider returns correct extension

- GIVEN a `CProvider`
- WHEN `FileExtension()` is called
- THEN it MUST return `".c"`

#### Scenario: C++ Docker command compiles and runs

- GIVEN a `CppProvider`
- WHEN `DockerCommand("main.cpp")` is called
- THEN it MUST return `["sh", "-c", "g++ -o /tmp/out /tmp/code.cpp && /tmp/out"]`

#### Scenario: Zig Docker command uses sh -c wrapper

- GIVEN a `ZigProvider`
- WHEN `DockerCommand("main.zig")` is called
- THEN it MUST return a `sh -c` command that copies source to `/tmp`, compiles with `zig build-exe`, and executes

#### Scenario: Zig local command is compiler

- GIVEN a `ZigProvider`
- WHEN `LocalCommand()` is called
- THEN it MUST return `"zig"`

---

### Requirement: Registry Registration — 17 Languages

The system MUST register all 17 providers in `NewDefaultRegistry()`: the 13 existing (python, ruby, php, lua, bash, perl, typescript, javascript, go, java, kotlin, scala, groovy) plus 4 new (rust, c, cpp, zig).

#### Scenario: Registry lists 17 languages

- GIVEN all providers are registered
- WHEN `registry.Languages()` is called
- THEN it MUST return exactly 17 sorted keys including `"rust"`, `"c"`, `"cpp"`, `"zig"`

#### Scenario: Registry resolves new systems languages

- GIVEN the default registry
- WHEN `registry.Get("rust")`, `registry.Get("c")`, `registry.Get("cpp")`, and `registry.Get("zig")` are called
- THEN each MUST return the corresponding provider without error

#### Scenario: Registry test expects 17 keys

- GIVEN `registry_test.go`
- WHEN the `Languages()` test assertion runs
- THEN the expected slice MUST contain 17 entries

---

### Requirement: Handler Extension Mapping — Zig

The system MUST map `.zig` file extensions to the `"zig"` language key in `gogs_handler.go` `inferLanguage()`.

#### Scenario: Zig file maps to zig

- GIVEN a `.zig` file is imported via Gogs
- WHEN `inferLanguage("src/main.zig")` is called
- THEN it MUST return `"zig"`

#### Scenario: Existing systems extensions verified

- GIVEN `.rs`, `.c`, `.cpp` file extensions
- WHEN `inferLanguage()` is called for each
- THEN they MUST return `"rust"`, `"c"`, `"cpp"` respectively
