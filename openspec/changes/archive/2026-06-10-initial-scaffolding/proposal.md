# Proposal: Initial Scaffolding

## Intent

To establish the foundational monorepo structure, core architecture, and security boundaries for the CodeAuditor project, enabling concurrent frontend (Angular) and backend (Go) development.

## Scope

### In Scope
- Monorepo directory structure (`backend/` and `frontend/`).
- Supabase Auth foundation (JWT validation middleware).
- Server-Sent Events (SSE) streaming pipeline for LLM interactions.
- Docker-based secure sandbox execution design for user-submitted code.

### Out of Scope
- Full implementation of domain logic and UI components.
- Deployment infrastructure (CI/CD pipelines).
- Gogs/MCP integration (deferred to future change).

## Capabilities

### New Capabilities
- `project-scaffolding`: Monorepo setup, build tools, and hexagonal boundaries.
- `auth-foundation`: Supabase Auth and JWT validation middleware.
- `llm-streaming`: SSE pipeline connecting Go backend and Angular frontend.
- `sandbox-execution`: Secure Docker container execution strategy.

### Modified Capabilities
- None

## Approach

We will adopt a monorepo structure separating the Go backend and Angular frontend. The backend will implement a Hexagonal Architecture using Supabase Auth for identity management, Server-Sent Events (SSE) for streaming Ollama LLM responses, and Docker one-shot containers for sandboxing untrusted code execution. The frontend will follow a pure TypeScript domain model with Angular adapters.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/` | New | Go hexagonal architecture scaffolding |
| `frontend/` | New | Angular 21 SPA scaffolding |
| `openspec/` | New | SDD artifact tracking and initialization |
| `docker-compose.yml` | New | Local dev environment definition |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Sandbox escape via container | Low | Use `no-new-privileges`, `cap-drop ALL`, `read-only rootfs`, and `--network none`. |
| Angular/Hexagonal purity violation | Medium | Implement ESLint rules to prevent framework imports in domain layers. |
| SSE connection limits | Low | Close connections promptly and reuse streams where possible. |

## Rollback Plan

Since this is the initial scaffolding, a rollback entails reverting the initial commit or removing the `backend/` and `frontend/` directories to return to the blank state.

## Dependencies

- Docker daemon on the host for sandbox execution.
- Supabase instance (local or remote) for authentication.
- Ollama local instance for LLM streaming tests.

## Success Criteria

- [ ] `backend/` and `frontend/` directories exist with base configurations.
- [ ] Go API entry point and Angular `main.ts` run without errors.
- [ ] Docker-compose file successfully orchestrates local dependencies (Supabase, Ollama).