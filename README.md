# CodeAuditor — Dojo de Auditoría

Plataforma interactiva para practicar auditoría de código, refactorización y análisis de seguridad. Consume repositorios reales vía MCP, los convierte en desafíos gamificados y evalúa las soluciones del usuario en un sandbox aislado con feedback en tiempo real vía SSE.

## Arquitectura

Hexagonal estricta en frontend y backend, con capas independientes de dominio, aplicación e infraestructura.

```
academy-mic/
├── backend/                  # API en Go (Chi, SSE, Docker sandbox)
│   ├── cmd/api/main.go       # Entry point
│   └── internal/
│       ├── core/             # Domain models + application services
│       ├── ports/            # Interfaces (SandboxExecutor, AuthValidator)
│       └── infrastructure/   # Adapters (Supabase, Docker, HTTP handlers)
├── frontend/codeauditor/     # SPA Angular 21 (Standalone, Signals, Tailwind v4)
│   └── src/app/
│       ├── domain/           # Models + repository ports (zero Angular imports)
│       ├── application/      # Use cases (AuditUseCase, ChallengeUseCase)
│       └── infrastructure/   # Components, services, guards, adapters
├── openspec/                 # SDD change tracking (proposals, specs, tasks)
├── docs/                     # Arquitectura, UI, dominio de negocio
└── docker-compose.yml        # Stack de desarrollo local
```

## Stack

| Capa | Tecnología |
|------|-----------|
| Frontend | Angular 21 (SPA), TypeScript 5.9, Tailwind CSS 4, Signals, RxJS 7 |
| Backend | Go 1.23, Chi v5, JWT, SSE streaming |
| Base de datos | PostgreSQL 15 (vía Supabase) |
| API Gateway | Kong 3.4 |
| Sandbox | Docker (aislado: `--cap-drop=ALL`, `--network=none`, `--read-only`) |
| LLM | Ollama (qwen2.5-coder:3b) |
| Editor | Monaco Editor 0.55 |
| Terminal | xterm.js 6.0 (fit addon) |

## Cómo levantar el entorno

### 1. Stack de desarrollo (Docker Compose)

```bash
cp .env.example .env      # Ajustar si es necesario
docker compose up -d       # PostgreSQL, Kong, Supabase Studio, Ollama
```

### 2. Backend

```bash
cd backend
cp .env.example .env      # Configurar DATABASE_URL, SUPABASE_*, SANDBOX_MODE
go run cmd/api/main.go    # Arranca en :8080
```

### 3. Frontend

```bash
cd frontend/codeauditor
pnpm install
pnpm start                # Arranca en :4200
```

## Variables de entorno

| Variable | Descripción | Default |
|----------|-------------|---------|
| `DATABASE_URL` | Conexión PostgreSQL | *requerido* |
| `SUPABASE_URL` | URL de Supabase (Kong) | *requerido* |
| `SUPABASE_ANON_KEY` | Supabase anon key | *requerido* |
| `SUPABASE_JWT_SECRET` | Secreto para validar JWT | *requerido* |
| `SANDBOX_MODE` | `docker`, `local`, o `auto` | `auto` |
| `PORT` | Puerto del servidor | `8080` |

## Endpoints

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| `GET` | `/health` | No | Health check |
| `POST` | `/auth/register` | No | Registro |
| `POST` | `/auth/login` | No | Login |
| `POST` | `/auth/logout` | Sí | Logout |
| `GET` | `/auth/me` | Sí | Perfil del usuario |
| `POST` | `/api/v1/audit` | Sí | Auditoría en tiempo real (SSE) |
