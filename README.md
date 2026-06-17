# CodeAuditor — Dojo de Auditoría

Plataforma interactiva para practicar auditoría de código, refactorización y análisis de seguridad. Conecta repositorios reales vía Gogs, los convierte en desafíos gamificados y evalúa soluciones en un sandbox aislado con feedback en tiempo real vía SSE y análisis de IA con Ollama.

## Features

- 🔐 **Autenticación completa** — registro, login, JWT via Supabase, guards de ruta
- 🏗️ **Hexagonal estricta** — dominio puro en Go y TypeScript, puertos e interfaces, cero acoplamiento a frameworks
- ⚡ **Auditoría en tiempo real** — SSE streaming del sandbox (Docker o local) con salida coloreada ANSI
- 🤖 **Análisis de IA** — Ollama (`qwen2.5-coder:3b`) analiza el código token por token vía SSE
- 📊 **Gamificación** — racha diaria, puntos de maestría, rangos (Junior → Mid → Senior → Architect)
- 📁 **Vault** — historial de auditorías persistido en PostgreSQL con estadísticas agregadas
- 🔌 **Integración Gogs** — explorá repositorios, importá archivos como desafíos temporales
- 🎨 **Dojo Layout** — IDE oscuro con sidebar colapsable, Monaco Editor, terminal xterm.js
- 🐳 **Sandbox aislado** — ejecución segura con `--cap-drop=ALL`, `--network=none`, `--read-only`
- ✅ **Quality gates** — `make validate` con golangci-lint, gofumpt, ESLint, Prettier

## Arquitectura

Hexagonal estricta (Puertos y Adaptadores) en **ambos stacks**, con dominio, aplicación e infraestructura completamente independientes.

```
academy-mic/
├── backend/                          # API en Go (Chi, SSE, Docker sandbox)
│   ├── cmd/api/main.go               # Entry point
│   └── internal/
│       ├── core/
│       │   ├── domain/models/        # Entidades puras (AuditRequest, UserProfile)
│       │   └── services/             # AuditService, UserProgressService, AuditHistoryService
│       ├── ports/                    # Interfaces (SandboxExecutor, AuthValidator, SSEStreamer)
│       └── infrastructure/
│           ├── driven/               # Adapters: Supabase, Ollama, Gogs, Sandbox (Local/Docker)
│           └── driving/              # HTTP handlers, auth middleware
├── frontend/codeauditor/             # SPA Angular 21 (Standalone, Signals, Tailwind v4)
│   └── src/app/
│       ├── domain/                   # Modelos + puertos (cero imports de Angular)
│       ├── application/              # Casos de uso (AuditUseCase, ChallengeUseCase)
│       └── infrastructure/           # Componentes, servicios, guards, adaptadores
├── openspec/                         # SDD change tracking (proposals, specs, tasks)
└── docker-compose.yml                # Stack de desarrollo local
```

### ¿Por qué Hexagonal?

- El dominio no conoce HTTP, bases de datos, ni frameworks
- Los puertos definen **qué** necesita el sistema; los adaptadores definen **cómo** lo hace
- Cambiar Supabase por otro provider de auth → solo un adaptador nuevo
- Testear en aislamiento sin levantar infraestructura

## Stack

| Capa | Tecnología |
|------|-----------|
| **Backend** | Go 1.23, Chi v5, JWT, SSE streaming |
| **Frontend** | Angular 21 (Standalone), TypeScript 5.9, Tailwind CSS 4, Signals, RxJS 7 |
| **Base de datos** | PostgreSQL 15 (Supabase) |
| **API Gateway** | Kong 3.4 (desarrollo) |
| **Sandbox** | Docker (`--cap-drop=ALL`, `--network=none`, `--read-only`) + fallback local |
| **IA** | Ollama (`qwen2.5-coder:3b`) |
| **Editor** | Monaco Editor 0.55 |
| **Terminal** | xterm.js 6.0 |
| **Íconos** | Lucide Angular |
| **Testing** | Go testing, Vitest 4, Playwright 1.60 |
| **Deploy** | Coolify + Docker Compose |

## Empezar

### Prerequisitos

- Go 1.23+
- Node.js + [pnpm](https://pnpm.io)
- Docker & Docker Compose
- [golangci-lint](https://golangci-lint.run) + [gofumpt](https://github.com/mvdan/gofumpt) (quality gates)
- Ollama (opcional, para análisis de IA)

### 1. Stack de desarrollo

```bash
cp .env.example .env
docker compose up -d
# PostgreSQL :5432, Kong :8000/:8443, Supabase Studio :3000, Ollama :11434
```

### 2. Backend

```bash
cd backend
cp .env.example .env
go run cmd/api/main.go    # :8080
```

### 3. Frontend

```bash
cd frontend/codeauditor
pnpm install
pnpm start                # :4200
```

### Variables de entorno

| Variable | Descripción | Default |
|----------|-------------|---------|
| `DATABASE_URL` | Conexión PostgreSQL | *requerido* |
| `SUPABASE_URL` | URL de Supabase/Kong | *requerido* |
| `SUPABASE_ANON_KEY` | Supabase anon key | *requerido* |
| `SUPABASE_JWT_SECRET` | Secreto para validar JWT | *requerido* |
| `SANDBOX_MODE` | `docker`, `local`, o `auto` | `auto` |
| `PORT` | Puerto del servidor | `8080` |
| `OLLAMA_BASE_URL` | Endpoint de Ollama | `http://localhost:11434` |
| `OLLAMA_MODEL` | Modelo de lenguaje | `qwen2.5-coder:3b` |
| `GOGS_BASE_URL` | API de Gogs | — |
| `GOGS_TOKEN` | Token de acceso Gogs | — |

## API

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| `GET` | `/health` | No | Health check con JSON |
| `POST` | `/auth/register` | No | Registro |
| `POST` | `/auth/login` | No | Login |
| `POST` | `/auth/logout` | Sí | Logout |
| `GET` | `/auth/me` | Sí | Perfil del usuario |
| `POST` | `/api/v1/audit` | Sí | Auditoría en tiempo real (SSE) |
| `GET` | `/api/v1/audit/history` | Sí | Historial de sesiones |
| `GET` | `/api/v1/audit/stats` | Sí | Estadísticas agregadas |
| `GET` | `/api/v1/gogs/repos` | Sí | Listar repositorios Gogs |
| `POST` | `/api/v1/gogs/file` | Sí | Obtener archivo de Gogs |

## Rutas del frontend

| Ruta | Componente | Auth | Descripción |
|------|-----------|------|-------------|
| `/` | `HomeComponent` | No | Landing page |
| `/login` | `LoginComponent` | No | Login |
| `/register` | `RegisterComponent` | No | Registro |
| `/dashboard` | `DashboardPageComponent` | Sí | Grid de desafíos + perfil |
| `/dojo` | `DojoPageComponent` | Sí | Workspace de auditoría |
| `/dojo/:id` | `DojoPageComponent` | Sí | Auditoría con desafío específico |
| `/mcp` | `McpPageComponent` | Sí | Explorador de repos Gogs |
| `/vault` | `VaultPageComponent` | Sí | Historial de auditorías |

## Testing

### Unitarios

```bash
make test-backend     # Go: go test -count=1 ./internal/...
make test-frontend    # Angular: npx ng test --watch=false (Vitest)
make test             # Ambos
```

### End-to-End (Playwright)

```bash
make e2e              # pnpm e2e → playwright test
```

### Cobertura

- **Backend**: servicios, adaptadores (sandbox, auth, ollama, gogs), handlers — 99 tests
- **Frontend**: servicios (auth, gogs, vault), componentes (mcp-page), guards — 31 tests
- **E2E**: navegación, auth guards, formularios — 6 tests

## Calidad

```bash
make validate         # Quality gate pre-deploy (gofumpt + golangci-lint + Prettier + ESLint)
make fix              # Auto-formato (gofumpt -w + prettier --write)
make lint             # Solo linting (ambos stacks)
make ci               # Pipeline completo: lint + test + build
```

### Herramientas

| Stack | Herramienta | Configuración |
|-------|------------|--------------|
| Go | golangci-lint | `backend/.golangci.yml` (errcheck, staticcheck, gofmt, goimports, misspell, +8 más) |
| Go | gofumpt | Formato Go más estricto |
| TS | ESLint | `angular-eslint` + `typescript-eslint` flat config |
| TS | Prettier | `pnpm format` → `prettier --write` |

## Despliegue (Coolify)

El deploy en producción usa `docker-compose.prod.yml` con tres redes externas:

| Red | Propósito |
|-----|-----------|
| `coolify` | Orquestación de Coolify |
| `net-external` | Enrutamiento Nginx Proxy Manager |
| `mic-supabase-access` | Conexión directa a PostgreSQL interno |

```bash
# Quality gate obligatorio antes de deployar
make validate
# Si pasa → commit → push → Coolify deploya automáticamente
```

El backend se construye desde `./backend/Dockerfile` (multi-stage: Go 1.23 → Debian slim).

## SDD Changes

El proyecto usa [Spec-Driven Development](https://github.com/anomalyco/gentle-ai) trackeado en `openspec/`.

| Change | Estado | Entregable |
|--------|--------|-----------|
| `initial-scaffolding` | ✅ | Monorepo, Go + Angular 21, Docker Compose |
| `auth-supabase` | ✅ | Auth flow completo (register, login, JWT, guards) |
| `dojo-layout` | ✅ | IDE oscuro, sidebar, routing |
| `terminal-audit` | ✅ | SSE audit pipeline, sandbox, xterm.js |
| `real-challenges` | ✅ | 8 desafíos con código vulnerable realista |
| `docker-sandbox` | ✅ | Sandbox Docker aislado con flags de seguridad |
| `ollama-ai-analysis` | ✅ | Análisis IA con streaming token-by-token |
| `user-progress` | ✅ | Rachas, puntos, rangos |
| `audit-vault` | ✅ | Historial persistido + estadísticas |
| `mcp-integration` | ✅ | Proxy Gogs, explorador de repos, importación de archivos |

## Roadmap

- 🔲 Persistencia de challenges importados desde Gogs (hoy en memoria)
- 🔲 Detección automática de categoría/dificultad para archivos importados
- 🔲 Más lenguajes en el sandbox (Python, Rust, Java)
- 🔲 Herramientas de auditoría avanzadas (semgrep, trivy)
- 🔲 OAuth providers (GitHub, Google)
- 🔲 Leaderboard y multijugador
