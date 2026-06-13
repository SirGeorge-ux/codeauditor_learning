# Exploration: Initial Scaffolding & Architecture

## Current State

The project is a blank slate. The `openspec/` skeleton exists (config.yaml with placeholder values) but there are zero source code files — no Go, no Angular, no configuration files. The documentation in `../docs/` defines the business domain (CodeAuditor Dojo), hexagonal architecture for the frontend, infrastructure boundaries (Supabase, Ollama, Gogs/MCP), and the UI/UX manifesto (Dark IDE / Cyber-Minimalista with Tailwind v4).

The openspec config.yaml currently has `Architecture: To be determined` and `Testing: To be initialized`, confirming this is the first architectural pass.

## Affected Areas

The entire repository is affected — this exploration defines the structure and technology decisions before any code is written.

## Approaches

### 1. Directory Structure (Monorepo with Backend/ + Frontend/)

The workspace `academy-mic/academy-mic/` is the git repo root. Both Go backend and Angular frontend live inside it, isolated but co-managed.

**Proposed layout:**

```
academy-mic/
├── backend/                    # Go backend — Hexagonal Architecture
│   ├── cmd/
│   │   └── api/
│   │       └── main.go         # Entry point, DI wiring
│   ├── internal/
│   │   ├── domain/             # Core — entities, value objects, repository ports
│   │   │   ├── entity/
│   │   │   ├── repository/     # Interfaces (ports)
│   │   │   └── service/        # Domain services
│   │   ├── application/        # Use cases (orchestration, no framework deps)
│   │   │   └── usecase/
│   │   └── infrastructure/     # Adapters — implementations of ports
│   │       ├── api/            # HTTP handlers, SSE streaming, middleware
│   │       ├── database/       # Supabase/PostgreSQL repositories
│   │       ├── llm/            # Ollama client (streaming)
│   │       ├── mcp/            # Gogs/MCP integration
│   │       └── sandbox/        # Test execution sandbox (Docker)
│   ├── pkg/                    # Public shared packages (if any)
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── frontend/                   # Angular 21 SPA — Hexagonal Architecture
│   ├── src/
│   │   ├── app/
│   │   │   ├── domain/         # Pure TS — Entities, Ports (interfaces)
│   │   │   │   ├── entities/
│   │   │   │   └── ports/
│   │   │   ├── application/    # Pure TS — Use cases
│   │   │   │   └── usecases/
│   │   │   └── infrastructure/ # Angular-aware — Components, Services
│   │   │       ├── adapters/   # Driving: Components, Pages, Layouts
│   │   │       │   ├── components/
│   │   │       │   ├── pages/
│   │   │       │   └── layouts/
│   │   │       └── services/   # Driven: HTTP, Supabase Auth client
│   │   ├── styles.css          # Tailwind v4 @theme config
│   │   ├── index.html
│   │   └── main.ts
│   ├── angular.json
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── tsconfig.json
│   └── Dockerfile
├── openspec/                   # SDD artifacts
│   ├── config.yaml
│   ├── specs/
│   └── changes/
│       ├── archive/
│       └── initial-scaffolding/
│           └── exploration.md
├── scripts/                    # Dev scripts, docker-compose helpers
├── docker-compose.yml          # Local dev: Go API + Ollama + Supabase
├── Makefile                    # Top-level commands (dev, build, test, lint)
├── .env.example
├── .gitignore
└── README.md
```

- **Pros**: Clean separation of concerns; each half can be developed by different teams; Go tooling works naturally in `backend/`; Angular CLI works naturally in `frontend/`; shared CI/CD at root level.
- **Cons**: Monorepo means git history mixes both concerns; need to manage two package ecosystems (`go.mod` + `pnpm`).
- **Effort**: Low (just directory creation and CLI init).

### 2. Authentication — Recommended: Supabase Auth

**The problem:** Users need to authenticate to the Dojo. The system tracks per-user progress (racha, mastery points, rank), which requires identity.

**Options:**

| Approach | Pros | Cons | Complexity |
|----------|------|------|------------|
| **Supabase Auth** | Already using Supabase DB; built-in GoTrue server; supports email/password + OAuth (GitHub, Google); JWT tokens with RLS; has Go client (`supabase-community/gotrue-go`) and Angular client (`@supabase/supabase-js`); row-level security maps naturally to user_id | Vendor coupling to Supabase; offline/failover requires planning | Low |
| **Self-managed JWT (Go)** | Full control; no external dependency; can use any DB schema | Must implement password hashing, sessions, refresh tokens, MFA — wheel reinvention; audit burden | High |
| **Gogs OAuth delegation** | Leverages existing Gogs instance; zero new auth infrastructure | Couples auth to code hosting platform; users must have Gogs account; breaks if Gogs goes down | Medium |

**Recommendation: Supabase Auth.**
- Angular uses `@supabase/supabase-js` for login flows (sign-up, sign-in, OAuth, password reset).
- Supabase issues a JWT signed with their HMAC-SHA256 secret.
- Go backend validates the JWT on every request by fetching the Supabase JWKS or verifying against the shared secret.
- The `user_id` from the JWT sub claim is used as the foreign key in all domain entities (racha, mastery, etc.).
- Go's `internal/infrastructure/api/middleware/` contains an `AuthMiddleware` that extracts and validates the JWT, injecting `user_id` into the request context.

### 3. Real-time LLM Streaming — Recommended: Server-Sent Events (SSE)

**The problem:** Ollama's Qwen2.5-coder:3b takes seconds to generate a response. The UI needs a "hacker terminal" typing effect, requiring token-by-token streaming.

**Options:**

| Approach | Pros | Cons | Complexity |
|----------|------|------|------------|
| **SSE (Server-Sent Events)** | Native browser API (`EventSource`); unidirectional (perfect for LLM→client); built into Go's `net/http` with `Flusher` interface; low overhead; RxJS Observable wrapper in Angular; Ollama API already streams JSONL natively | Only server→client (no client→server mid-stream, but that's not needed here); connection limit per browser (6-8 per domain) | Low |
| **WebSockets** | Bidirectional; single TCP connection; widespread support | Overkill for this use case (no client→server data mid-stream); needs gorilla/websocket or nhooyr.io/websocket in Go; more complex error handling | Medium |
| **Chunked Transfer + Polling** | Simple to implement | Wasteful; latency between chunks; not truly streaming; defeats the hacker terminal effect | Low (but poor UX) |

**Recommendation: SSE.**
- Go backend receives a POST to `/api/dojo/evaluate` (or similar).
- Backend sends a structured request to Ollama's `/api/chat` endpoint with `"stream": true`.
- Ollama returns a JSONL stream: each line is a JSON object with `{"message": {"content": "..."}}`.
- Go reads line-by-line using `bufio.Scanner`, extracts the token text, and writes each chunk to the SSE response with `flush()`.
- Angular creates a typed `EventSource` wrapper that emits `string` values into an RxJS Observable, consumed by a Signal in the component.
- The terminal component accumulates tokens into a Signal `responseText` and applies a typewriter CSS animation.

**Sequence:**
```
Angular                          Go Backend                        Ollama
   │                                 │                                │
   ├── POST /api/dojo/evaluate ──────►│                                │
   │                                 ├── POST /api/chat (stream:true)─►│
   │                                 │                                ├── {token1}
   │                                 │◄── {token1}                   │
   │   SSE: data: {token1} ──────────►│                                │
   │                                 │                                ├── {token2}
   │                                 │◄── {token2}                   │
   │   SSE: data: {token2} ──────────►│                                │
   │                                 │  ...                          │
   │                                 │◄── [DONE]                     │
   │   SSE: event: done ─────────────►│                                │
```

### 4. Test Sandboxing (Critical — Phase 2 RCE Prevention) — Recommended: Docker One-Shot Containers

**The problem:** Phase 2 requires users to write and run tests against challenge code. If Go naively `exec.Command("go test", ...)` or `exec.Command("python", ...)` user-submitted code, a malicious user can trivially execute `rm -rf /`, read server environment variables, or pivot to other internal services.

**Options:**

| Approach | Pros | Cons | Complexity |
|----------|------|------|------------|
| **Docker one-shot containers** | Industry standard; Go Docker SDK available; strong isolation; CPU/memory limits; no-network mode; read-only rootfs; timeout enforced at container level; easy to clean up | Requires Docker daemon on the host; container startup latency (~200-500ms); image needs to be pre-pulled | Medium |
| **gVisor (runsc)** | Extra kernel-level sandbox over Docker; defends against container escape vulnerabilities | Added complexity; some syscalls not supported (Go tests might fail); must configure as Docker runtime | High |
| **nsjail** | Lightweight; uses Linux namespaces; very fast startup; seccomp policies; no daemon needed | Linux-only; must compile/install; less documented than Docker; custom seccomp profile needed | High |
| **WebAssembly (Wazero)** | Extremely safe; no OS access; pure Go Wasm runtime; no Docker dependency; fast startup | User code must compile to Wasm first (Go→Wasm is supported, but other languages need extra tooling); standard library coverage varies | Medium-High |
| **chroot + rlimit** | Simple; no dependencies | Easily escapable; not secure against determined attacker; do NOT use alone | Low (but UNSAFE) |

**Recommendation: Docker one-shot containers (primary) + nsjail (lightweight fallback).**

**Docker implementation:**
- Each test execution creates a disposable container from a hardened "runner" image.
- The image has: the language runtime (Go, Python, Node.js), test framework, and a read-only filesystem.
- The submitted code is mounted as a temp volume or piped via stdin.
- Container runs with:
  - `--read-only`
  - `--network none`
  - `--memory 256m --memory-swap 256m`
  - `--cpus 0.5`
  - `--pids-limit 50` (prevents fork bombs)
  - `--security-opt no-new-privileges:true`
  - `--cap-drop ALL`
  - A hard timeout (e.g., 30s) via `timeout` command inside container + `docker stop` fallback
- stdout/stderr captured, returned to client, container removed via `--rm`.

**Go implementation (pseudo):**
```go
func (s *DockerSandbox) Execute(ctx context.Context, req SandboxRequest) (*SandboxResult, error) {
    resp, err := s.client.ContainerCreate(ctx, &container.Config{
        Image:      "codeauditor/runner-go:latest",
        Cmd:        []string{"sh", "-c", req.Command},
        WorkingDir: "/workspace",
    }, &container.HostConfig{
        ReadonlyRootfs: true,
        NetworkMode:    container.NetworkMode("none"),
        Resources: container.Resources{
            Memory:   256 * 1024 * 1024,
            NanoCPUs: 500_000_000,
            PidsLimit: 50,
        },
        CapDrop: strslice.StrSlice{"ALL"},
        SecurityOpt: []string{"no-new-privileges:true"},
        AutoRemove: true,
    }, nil, nil, "")
    // ... start, wait with timeout, capture logs
}
```

**Runner images** should be pre-built per language (Go, Python, JS/TS for future) and stored in a local registry or built at deploy time. They MUST NOT contain secrets, network tools, or sensitive binaries.

## Recommendation Summary

| Concern | Decision | Rationale |
|---------|----------|-----------|
| Directory layout | `backend/` + `frontend/` monorepo | Clean hexagonal separation per stack, common CI at root |
| Authentication | Supabase Auth (JWT) | Already committed to Supabase; zero extra infra; built-in user management |
| LLM Streaming | Server-Sent Events (SSE) | Native browser API; Ollama streams JSONL natively; minimal overhead |
| Test Sandboxing | Docker one-shot containers | Industry standard isolation; Go SDK; network/cpu/mem limits; auto-cleanup |

## Risks

1. **Sandbox escape via container breakout**: Mitigated by `no-new-privileges`, `cap-drop ALL`, `read-only rootfs`, and no-network. For production, consider gVisor as an additional runtime class.
2. **Docker daemon dependency**: If Docker is not available, the sandbox falls back to a degraded mode. Mitigation: detect Docker availability at startup, cache result.
3. **Ollama latency variance**: 3B parameter model on CPU can be slow. SSE mitigates UX impact but the total response time remains. Mitigation: consider response caching for repeated evaluations, or quantized model.
4. **Angular + hexagonal purity**: The docs mandate pure TS in domain/application layers. This requires discipline: no `@angular/core` imports in domain entities. Mitigation: enforce via ESLint rules (`import/no-restricted-paths` or similar).
5. **SSE connection limits**: Browsers limit 6-8 concurrent connections per domain. If multiple SSE streams are open, this could exhaust the pool. Mitigation: use a single SSE connection with multiplexed events, or close connections promptly.

## Ready for Proposal

Yes. The analysis is complete and ready to proceed to `sdd-propose`. The orchestrator should present these findings and get stakeholder alignment before moving to formal specification.

## Next Steps

1. Review and align the recommendations with stakeholders.
2. Proceed to **sdd-propose** to formalize scope, approach, and rollback plan.
3. Then **sdd-design** for detailed architecture (sequence diagrams, interface contracts).
4. Then **sdd-tasks** to break into implementable work units.
