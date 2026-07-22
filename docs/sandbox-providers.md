# Sandbox Providers (CodeAuditor)

> **Audiencia:** devs backend, agentes que vayan a añadir un lenguaje.
> **Lee esto antes de:** añadir soporte para un nuevo lenguaje, modificar el sandbox, o tocar los providers.

---

## 1. Arquitectura del sandbox en 30 segundos

```
        ┌─────────────────────┐
        │  AuditService       │   ← core/services/
        │  (orquesta)         │
        └──────────┬──────────┘
                   │ usa puerto
                   ▼
        ┌─────────────────────┐
        │ SandboxExecutor     │   ← ports/sandbox.go (interface)
        └──────────┬──────────┘
                   │ implementado por
        ┌──────────┴──────────┐
        ▼                     ▼
  LocalSandbox          DockerSandbox       ← infrastructure/driven/sandbox/
        │                     │
        └──────────┬──────────┘
                   │ ambos consultan
                   ▼
        ┌─────────────────────┐
        │ ProviderRegistry    │   ← infrastructure/driven/sandbox/providers/registry.go
        │  (map[lang]→prov)   │
        └──────────┬──────────┘
                   │ contiene
        ┌──────────┴─────────────────────┐
        ▼                                ▼
  GoProvider                       PythonProvider
  TypeScriptProvider               RubyProvider
  JavaScriptProvider               BashProvider
  JavaProvider                     ... (50+)
```

**Regla:** `LocalSandbox` y `DockerSandbox` **nunca** tienen un `switch language{}`. Siempre delegan al registry.

---

## 2. El puerto `LanguageProvider`

**Archivo:** `backend/internal/ports/provider.go`

```go
type LanguageProvider interface {
    // Canonical key (lowercase): "go", "typescript", "python", etc.
    Language() string

    // Extensión de archivo con punto: ".go", ".ts", ".py"
    FileExtension() string

    // Imagen Docker pública (Alpine-based, lightweight).
    DockerImage() string

    // argv (sin la imagen) que se pasa a `docker run`.
    // filename es el basename dentro del /code montado.
    DockerCommand(filename string) []string

    // Ejecutable en PATH para el modo local.
    // Debe existir en la máquina de desarrollo para que el healthcheck pase.
    LocalCommand() string

    // Sugerencia accionable de instalación cuando LocalCommand no está.
    // Ej: "pip install ruff", "npm install -g eslint"
    InstallHint() string
}
```

---

## 3. Anatomía de un provider

**Archivo de ejemplo:** `backend/internal/infrastructure/driven/sandbox/providers/python.go`

```go
package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

type PythonProvider struct{}

func NewPythonProvider() *PythonProvider {
    return &PythonProvider{}
}

func (p *PythonProvider) Language() string     { return "python" }
func (p *PythonProvider) FileExtension() string { return ".py" }
func (p *PythonProvider) DockerImage() string   { return "python:3.12-alpine" }
func (p *PythonProvider) LocalCommand() string  { return "python3" }
func (p *PythonProvider) InstallHint() string   { return "Instala Python 3: https://python.org/downloads" }

func (p *PythonProvider) DockerCommand(filename string) []string {
    return []string{"python3", filename}
}

// Compile-time check
var _ ports.LanguageProvider = (*PythonProvider)(nil)
```

**Tamaño típico:** 20-40 líneas por provider.

**Test típico** (`python_test.go`):

```go
func TestPythonProvider(t *testing.T) {
    p := NewPythonProvider()

    if p.Language() != "python" {
        t.Errorf("Language() = %q, want %q", p.Language(), "python")
    }
    if p.FileExtension() != ".py" {
        t.Errorf("FileExtension() = %q, want %q", p.FileExtension(), ".py")
    }
    if p.DockerImage() != "python:3.12-alpine" {
        t.Errorf("DockerImage() = %q", p.DockerImage())
    }
    cmd := p.DockerCommand("main.py")
    if len(cmd) < 2 || cmd[0] != "python3" || cmd[1] != "main.py" {
        t.Errorf("DockerCommand() = %v", cmd)
    }
}
```

---

## 4. El registry

**Archivo:** `backend/internal/infrastructure/driven/sandbox/providers/registry.go`

```go
type ProviderRegistry struct {
    providers map[string]ports.LanguageProvider
}

func NewProviderRegistry() *ProviderRegistry { ... }

func (r *ProviderRegistry) Register(p ports.LanguageProvider) error {
    if _, exists := r.providers[p.Language()]; exists {
        return fmt.Errorf("provider %q already registered", p.Language())
    }
    r.providers[p.Language()] = p
    return nil
}

func (r *ProviderRegistry) Get(language string) (ports.LanguageProvider, error) {
    p, ok := r.providers[language]
    if !ok {
        return nil, fmt.Errorf("no provider for %q (supported: %s)",
            language, r.Languages())
    }
    return p, nil
}

func (r *ProviderRegistry) Languages() []string { ... }

func NewDefaultRegistry() *ProviderRegistry {
    r := NewProviderRegistry()
    _ = r.Register(NewTypeScriptProvider())
    _ = r.Register(NewJavaScriptProvider())
    _ = r.Register(NewGoProvider())
    // ... uno por línea
    return r
}
```

---

## 5. Cómo lo consumen los sandboxes

**LocalSandbox** (extracto):

```go
func (s *LocalSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
    provider, err := s.registry.Get(language)
    if err != nil {
        return nil, err
    }

    filename := fmt.Sprintf("code%s", provider.FileExtension())
    cmd := exec.CommandContext(ctx, provider.LocalCommand(), filename)
    // ... ejecutar y devolver el Reader
}
```

**DockerSandbox** (extracto):

```go
func (s *DockerSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
    provider, err := s.registry.Get(language)
    if err != nil {
        return nil, err
    }

    args := []string{
        "run", "--rm",
        "-i",
        "--cap-drop=ALL",
        "--network=none",
        "--read-only",
        "-v", fmt.Sprintf("%s:/code:ro", tmpDir),
        provider.DockerImage(),
    }
    args = append(args, provider.DockerCommand(filename)...)

    cmd := exec.CommandContext(ctx, "docker", args...)
    // ...
}
```

**Observa:** en ningún caso hay `switch language{}`. Solo `provider.DockerImage()`, `provider.DockerCommand()`.

---

## 6. Añadir un nuevo lenguaje (workflow completo)

Ver **`.atl/harnesses/add-language.harness.md`** para el paso a paso, o la skill **`.atl/skills/codeauditor-add-language/SKILL.md`**.

**TL;DR:**
1. Crear `providers/<lenguaje>.go` (~30 líneas).
2. Crear `providers/<lenguaje>_test.go` (~20 líneas).
3. Añadir `_ = r.Register(NewLenguajeProvider())` en `NewDefaultRegistry()`.
4. (Opcional) Actualizar el comentario de `SandboxExecutor` con el nuevo lenguaje soportado.
5. Correr `go test ./internal/infrastructure/driven/sandbox/providers/...` → debe pasar.
6. Correr `go test -short ./...` → no rompe nada existente.

**Lo que NO necesitas tocar:**
- LocalSandbox.
- DockerSandbox.
- El frontend (Angular auto-acepta el nuevo lenguaje si el backend lo soporta).

### 6.1 Currícula del lenguaje (NUEVO en S1)

Cuando añades un lenguaje al sandbox, también debes crear su currícula en `.atl/curricula/<lang>.md`. Ver `docs/curricula-format.md` para el formato.

El sistema no requiere que la currícula exista para que el lenguaje funcione en el sandbox. Pero el `mcp-curriculum.get_topics` devolverá 0 topics y el generador de challenges no podrá crear ejercicios pedagógicos. **Recomendación:** crea al menos 3 topics básicos (hello-world, variables, types) al añadir el lenguaje.

---

## 7. Decisiones de diseño recurrentes

### 7.1 ¿Qué imagen Docker uso?

- **Alpine-based** siempre que sea posible (`node:22-alpine`, `python:3.12-alpine`, `ruby:3.3-alpine`).
- **Sin** versiones `:latest` (imagen reproducible).
- **Sin** imágenes de 1 GB. Si tu imagen es >300 MB, replantéate el approach.
- **Sin** imágenes oficiales de Windows.

### 7.2 ¿Qué linter/ejecutable uso en local?

- **Preferente:** el ejecutable del lenguaje (`python3`, `go`, `node`).
- **Si hace falta un linter:** el del ecosistema (`ruff` para Python, `eslint` para JS, `shellcheck` para Bash).
- **Si no hay binario ligero:** considerar ejecutar el linter dentro de un contenedor de un solo uso (modo `docker` siempre).

### 7.3 ¿Cómo distingo entre "ejecutar" y "validar"?

- **Ejecutar** = corre el código del usuario. Necesita el binario del lenguaje.
- **Validar** = aplica un linter/analizador. Necesita el linter.

El `DockerCommand` puede ser:
- `["python3", filename]` → ejecutar.
- `["ruff", "check", filename]` → validar.

Si el challenge es "auditoría", normalmente se ejecuta Y se valida. El backend decide.

### 7.4 ¿Cómo se mapea la extensión a lenguaje en el frontend?

- `inferLanguage(filePath)` en `gogs_handler.go` (backend) mapea `.py` → `python`, `.sh` → `bash`, etc.
- En el frontend, el `DojoPage` recibe el `language` y muestra el icono + sintaxis Monaco correcta.

Si añades un lenguaje, **asegúrate de añadir el mapeo en `inferLanguage`**.

---

## 8. Proveedores actuales (snapshot)

| Lenguaje | Provider | Imagen Docker | Local Cmd | Install Hint |
|---|---|---|---|---|
| typescript | TypeScriptProvider | `node:22-alpine` | `npx` | `npm install -g typescript` |
| javascript | JavaScriptProvider | `node:22-alpine` | `node` | `nodejs.org` |
| go | GoProvider | `golang:1.23-alpine` | `go` | `go.dev/dl` |
| python | PythonProvider | `python:3.12-alpine` | `python3` | `python.org/downloads` |
| ruby | RubyProvider | `ruby:3.3-alpine` | `ruby` | `rvm.io` |
| php | PhpProvider | `php:8.3-alpine` | `php` | `php.net` |
| lua | LuaProvider | `lua:5.4-alpine` | `lua` | `lua.org` |
| bash | BashProvider | `bash:5.2-alpine` | `bash` | (built-in en Linux/macOS) |
| perl | PerlProvider | `perl:5.38-alpine` | `perl` | `perl.org` |
| java | JavaProvider | `eclipse-temurin:21-alpine` | `java` | `adoptium.net` |
| kotlin | KotlinProvider | `eclipse-temurin:21-alpine` | `kotlinc` | `kotlinlang.org` |
| scala | ScalaProvider | `eclipse-temurin:21-alpine` | `scala` | `scala-lang.org` |
| groovy | GroovyProvider | `eclipse-temurin:21-alpine` | `groovy` | `groovy-lang.org` |
| rust | RustProvider | `rust:1.82-alpine` | `rustc` | `rustup.rs` |
| c | CProvider | `gcc:14-alpine` | `gcc` | `gcc.gnu.org` |
| cpp | CppProvider | `gcc:14-alpine` | `g++` | `gcc.gnu.org` |
| zig | ZigProvider | `zig:0.13-alpine` | `zig` | `ziglang.org` |
| html | HTMLProvider | `htmlhint:alpine` | `htmlhint` | `npm i -g htmlhint` |
| css | CSSProvider | `stylelint:alpine` | `stylelint` | `npm i -g stylelint` |
| xml | XMLProvider | `xmllint:alpine` | `xmllint` | `libxml2-utils` |
| json | JSONProvider | `jq:alpine` | `jq` | `jqlang.org` |
| yaml | YAMLProvider | `yaml:alpine` | `yamllint` | `pip install yamllint` |
| sql | SQLProvider | `sqlfluff:alpine` | `sqlfluff` | `sqlfluff.com` |
| csharp | CSharpProvider | `mcr.microsoft.com/dotnet/sdk:8.0-alpine` | `dotnet` | `dotnet.microsoft.com` |
| swift | SwiftProvider | `swift:5.10-alpine` | `swift` | `swift.org` |
| haskell | HaskellProvider | `haskell:9.0-alpine` | `ghc` | `haskell.org` |
| elixir | ElixirProvider | `elixir:1.17-alpine` | `elixir` | `elixir-lang.org` |
| clojure | ClojureProvider | `clojure:tools-deps-alpine` | `clojure` | `clojure.org` |
| r | RProvider | `r:4.4-alpine` | `Rscript` | `r-project.org` |
| solidity | SolidityProvider | `ethereum/solc:0.8-alpine` | `solc` | `soliditylang.org` |
| erlang | ErlangProvider | `erlang:26-alpine` | `erl` | `erlang.org` |
| dart | DartProvider | `dart:3.5-alpine` | `dart` | `dart.dev` |
| julia | JuliaProvider | `julia:1.10-alpine` | `julia` | `julialang.org` |
| nim | NimProvider | `nim:2.0-alpine` | `nim` | `nim-lang.org` |
| cobol | CobolProvider | `gnucobol:alpine` | `cobc` | `gnucobol.sourceforge.io` |
| powershell | PowerShellProvider | `powershell:7.4-alpine` | `pwsh` | `microsoft.com/powershell` |
| racket | RacketProvider | `racket:8.13-alpine` | `racket` | `racket-lang.org` |
| objective-c | ObjectiveCProvider | `objc:alpine` | `clang` | `llvm.org` |
| fsharp | FSharpProvider | `mcr.microsoft.com/dotnet/sdk:8.0-alpine` | `dotnet` | `dotnet.microsoft.com` |

(50+ lenguajes en total. Lista exacta en `providers/registry.go`.)

---

## 9. Anti-patrones

| Anti-patrón | Solución |
|---|---|
| Provider con `switch` interno | El provider es plano, no tiene branches. |
| Provider que hace IO en `Language()` o `DockerImage()` | Esos métodos son puros, sin side effects. |
| Provider que depende del registry (circular) | Provider no conoce al registry. Registry conoce a los providers. |
| Hardcodear lista de lenguajes en `LocalSandbox`/`DockerSandbox` | Siempre pasar por `registry.Get(lang)`. |
| Imagen Docker de >1 GB | Buscar alternativa alpine o multi-stage. |
| Linter que requiere GPU o 8 GB de RAM | Incompatible con sandbox. Buscar versión ligera o ejecutar en host. |

---

## 10. Recursos

- **Spec activo que define el registry:** `openspec/specs/sandbox-provider-registry/spec.md`
- **Spec archivado de la creación del pattern:** `openspec/changes/archive/2026-06-22-multi-lang-sandbox-oleada1/`
- **Puerto `LanguageProvider`:** `backend/internal/ports/provider.go`
- **Registry:** `backend/internal/infrastructure/driven/sandbox/providers/registry.go`
- **Harness paso a paso:** `.atl/harnesses/add-language.harness.md`
- **Currícula:** `.atl/curricula/<lang>.md` (S1+)
- **Formato de currícula:** `docs/curricula-format.md`

### 10.1 Uso pedagógico (S2+)

A partir de S2, el sandbox es consumido por el `mcp-pedagogical` tool:

- `mcp-pedagogical.validate_solution(language, code, expected_solution, test_cases)` ejecuta el código del user en el sandbox y compara con el expected.
- `mcp-sandbox.run_code(language, code, tests, timeout_seconds)` ejecuta código arbitrario del LLM (usado por el generador de challenges para validar que la solución propuesta funciona).
- `mcp-sandbox.healthcheck(language)` se usa al iniciar el sistema para saber qué lenguajes están disponibles y avisar al user si falta alguno.

**Trampa:** el LLM puede generar código infinito o malicioso. El sandbox tiene timeout de 30s y `--cap-drop=ALL --network=none --read-only`. Si el LLM pide más tiempo o más permisos, rechazar.
