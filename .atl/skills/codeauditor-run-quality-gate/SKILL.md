---
name: codeauditor-run-quality-gate
description: Use this skill when an agent needs to run, fix, or interpret the project's quality gates. Trigger phrases include "run validate", "quality gate", "make validate", "lint", "format", "fix", "the build is broken", "fallan los tests", "pre-deploy", "calidad". Loads the Makefile targets, the toolchain, and the standard troubleshooting flow.
---

# Run the CodeAuditor Quality Gate

> **Outcome:** the codebase passes `make validate` cleanly, ready to commit / push / deploy.
> **Time estimate:** 2-10 minutes.
> **Frequency:** before every commit, every push, every deploy.

## When to load

Load this skill when the user asks:
- "Run quality gate"
- "Make validate"
- "Pasa el build?"
- "Fallan los tests"
- "Pre-deploy check"
- "Quiero pushear"
- "The build is broken"

## The 4 quality gates

| Gate | Command | Catches |
|---|---|---|
| Format | `make fix` | gofumpt (Go), Prettier (TS) — estilo |
| Lint | `make lint` | golangci-lint, ESLint — code smells |
| Test | `make test` | tests fallando |
| Validate (pre-deploy) | `make validate` | format + lint (sin tests) |

**Standard order:**
1. `make fix` — arreglar formato automáticamente.
2. `make validate` — confirmar que lint + formato pasan.
3. `make test` — confirmar que los tests pasan.
4. `make ci` — pipeline completo (lint + test + build).
5. `make build` — sanity final.

## Step-by-step

### Step 1 — Format first

```bash
make fix
```

Esto corre:
- `gofumpt -w .` en backend (formato Go más estricto que gofmt).
- `pnpm format` en frontend (Prettier).

**No requiere intervención.** Si no hay nada que formatear, sale silencioso.

### Step 2 — Run the pre-deploy gate

```bash
make validate
```

Esto corre:
- `gofumpt -l .` (en backend) — falla si hay archivos sin formatear.
- `golangci-lint run` (en backend) — lint con 12+ linters activados.
- `pnpm exec prettier --check "src/**/*.{ts,html,css}"` (en frontend) — falla si no está formateado.
- `pnpm lint` (en frontend) — ESLint + angular-eslint.

**Si falla:** lee el error. Suele ser una de estas 3:
- Archivo sin formatear → vuelve a `make fix`.
- Lint error real → arreglar el código.
- Lint rule nuevo que no conocías → leer la doc de la regla.

### Step 3 — Run the tests

```bash
make test
```

Esto corre:
- `go test -count=1 ./internal/...` (backend) — sin caché.
- `npx ng test --watch=false` (frontend) — Vitest en modo single-run.

**Si un test falla:**

1. Lee el nombre del test → qué archivo y qué función.
2. Lee el mensaje de error → qué esperaba vs qué obtuvo.
3. Si es un test que tú escribiste → probablemente tu código.
4. Si es un test que NO tocaste → **reproduce en isolation**:
   ```bash
   cd backend && go test -run TestFailingName -v ./internal/...
   # o
   cd frontend/codeauditor && npx ng test --include='**/failing.spec.ts'
   ```
5. Si sigue sin tener sentido, lee el test y el código que prueba, lado a lado.

### Step 4 — Run the build (sanity)

```bash
make build
```

Esto corre:
- `go build -o api ./cmd/api/` (backend).
- `npx ng build` (frontend).

Si el build falla pero los tests pasan, el problema es de:
- Tipos TS no usados en runtime.
- Imports cíclicos.
- Variables no inicializadas.
- Dependencias no instaladas (`pnpm install` o `go mod download`).

### Step 5 — Run the full CI locally

```bash
make ci
```

Equivalente a: `make lint && make test && make build`. Esto es lo que correría Coolify antes de hacer deploy.

## Troubleshooting común

### "gofumpt not found"

```bash
go install mvdan.cc/gofumpt@latest
# Asegúrate de que $(go env GOPATH)/bin está en tu PATH
```

### "golangci-lint not found"

```bash
# Ver https://golangci-lint.run/welcome/install/
# O: brew install golangci-lint
```

### "ESLint config not found"

Probablemente estás ejecutando `npx eslint` directamente. Usa siempre `pnpm lint` (que lee la config del proyecto).

### "Prettier wants to reformat X"

```bash
make fix
```

Si el reformateo rompe algo (ej. ternarias largas), ajusta el código manualmente o usa `// prettier-ignore` con justificación.

### "Test passes locally but fails in CI"

Probablemente:
- Dependencia de red (test asume que algo está corriendo).
- Path absoluto hardcodeado.
- Variable de entorno no definida en CI.
- Versión de Go o Node distinta.

Mira el log de CI con cuidado, las versiones suelen estar en el log.

### "Chromium missing libglib-2.0.so.0" (Playwright)

Es un problema de **setup del host**, no del código. Soluciones:

```bash
# Debian/Ubuntu:
sudo apt-get install -y libglib2.0-0 libnss3 libnspr4 libdbus-1-3 \
  libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 \
  libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 \
  libpango-1.0-0 libcairo2 libasound2

# macOS:
brew install chromium
```

O bien corre los E2E dentro de Docker:

```bash
docker run --rm -v $(pwd):/work -w /work mcr.microsoft.com/playwright:v1.60.0-jammy \
  pnpm e2e
```

### "Mock challenge repository not found"

Si `frontend/.../infrastructure/repositories/mock-challenge.repository.ts` desapareció (porque lo borraste siguiendo H-6 de la auditoría), el import puede fallar. Verifica que no haya imports huérfanos.

## Pre-deploy checklist

Antes de pushear a `master`:

- [ ] `make fix` ejecutado.
- [ ] `make validate` pasa.
- [ ] `make test` pasa.
- [ ] `make build` pasa.
- [ ] No hay commits con `console.log`, `TODO`, o `FIXME` nuevos (excepto con issue link).
- [ ] Si tocaste un spec, las tasks están en `[x]`.
- [ ] Si añadiste un endpoint, hay test del handler.
- [ ] Si añadiste un componente, hay test o está en el flujo E2E.

## Don't do

- ❌ Don't `git commit --no-verify` para saltarte hooks.
- ❌ Don't disable un lint rule globalmente para hacer pasar tu código.
- ❌ Don't add `//nolint` sin un comentario explicando por qué.
- ❌ Don't `pnpm tsc --noEmit` ignorando errores de tipo.
- ❌ Don't skip tests con `t.Skip()` sin razón válida.
- ❌ Don't commit `.env` con secrets.

## Quick reference

```bash
make help         # Lista todos los targets
make test         # Todos los tests
make test-backend # Solo Go
make test-frontend # Solo Angular
make lint         # Lint de los dos stacks
make validate     # Format + lint (pre-deploy)
make fix          # Auto-formato
make build        # Compila los dos
make ci           # lint + test + build
make clean        # Borra artefactos de build
```

Si tienes prisa, **mínimo indispensable:** `make fix && make validate && make test`.
