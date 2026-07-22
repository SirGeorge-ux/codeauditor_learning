---
name: codeauditor-add-api-endpoint
description: Use this skill when an agent needs to add a new HTTP endpoint to the CodeAuditor backend (and its frontend adapter). Trigger phrases include "new endpoint", "nuevo endpoint", "add API", "create a route", "expose X", "GET /api/v1/...", "POST handler". Loads the hexagonal layering, the handler pattern, the auth middleware, and the frontend adapter mirror.
---

# Add an API Endpoint to CodeAuditor

> **Outcome:** a new HTTP endpoint, auth-protected, with Go handler + service + tests, and Angular adapter + use case + tests.
> **Time estimate:** 1-2 hours.
> **PR size:** 2-6 files, ~300-500 lines.

## When to load

Load this skill when the user asks:
- "Add an endpoint to..."
- "Create a route for..."
- "Expose X via API"
- "Quiero un POST que..."

## Pre-flight

1. Read `AGENTS.md` (root).
2. Read `docs/hexagonal-architecture.md` (sections 3 + 4).
3. Skim 1 existing handler: `backend/internal/infrastructure/driving/handlers/audit_handler.go` is a good reference.
4. Skim 1 existing service: `backend/internal/core/services/audit_service.go`.
5. Skim 1 existing frontend service: `frontend/.../infrastructure/services/audit.service.ts`.

## Architectural checklist (answer first)

| Question | If you don't know |
|---|---|
| What HTTP method + path? | GET vs POST vs DELETE? Singular or plural? |
| Auth required? | Yes by default (under `/api/v1`). |
| What does it do? | Single sentence: "Returns the user's audit history paginated." |
| What input does it take? | Query params? Body? Headers? |
| What does it return? | JSON shape. |
| What can go wrong? | 4xx cases, 5xx cases. |
| Does it need a new port? | Only if it talks to a system not yet abstracted. |
| Does it need a new service? | If no existing service fits, yes. |
| Does the frontend already have a port? | If yes, just add a method. If no, add it. |

If any of these is unclear, **ask the user before writing code**.

## Step-by-step (backend)

### Step 1 — Define the contract (Go types)

If the endpoint returns a new shape, add it to the appropriate file in `backend/internal/core/domain/models/`.

```go
// Example: stats endpoint
type UserStats struct {
    TotalAudits       int       `json:"totalAudits"`
    CompletedAudits   int       `json:"completedAudits"`
    CurrentStreak     int       `json:"currentStreak"`
    LongestStreak     int       `json:"longestStreak"`
    PointsByLanguage  map[string]int `json:"pointsByLanguage"`
    LastAuditAt       *time.Time `json:"lastAuditAt,omitempty"`
}
```

### Step 2 — Service method (or new service)

If the existing service fits, add a method. Otherwise, create a new one in `backend/internal/core/services/`.

```go
// UserProgressService already has a `GetUserStats` method? Use it.
// Otherwise, add it:
func (s *UserProgressService) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
    // Implementation
}
```

**Rules:**
- Receive `ctx context.Context` first.
- Return wrapped errors: `fmt.Errorf("get user stats: %w", err)`.
- Add a test: `user_progress_service_test.go` (table-driven if N variants).

### Step 3 — Handler

Create or edit `backend/internal/infrastructure/driving/handlers/<name>_handler.go`:

```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/anomalyco/codeauditor/backend/internal/core/services"
    "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

type StatsHandler struct {
    progress *services.UserProgressService
}

func NewStatsHandler(progress *services.UserProgressService) *StatsHandler {
    return &StatsHandler{progress: progress}
}

func (h *StatsHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
    userID, ok := authmiddleware.UserIDFromContext(r.Context())
    if !ok {
        writeJSONError(w, http.StatusUnauthorized, "missing user id")
        return
    }

    stats, err := h.progress.GetUserStats(r.Context(), userID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}
```

### Step 4 — Handler test

Create `<name>_handler_test.go` using `httptest`:

```go
func TestStatsHandler_GetUserStats(t *testing.T) {
    // Setup mock service
    // Build request with userID in context
    // Assert response
}
```

Use the patterns from `gogs_handler_test.go` (read it first).

### Step 5 — Register the route in `main.go`

Edit `backend/cmd/api/main.go`:

```go
statsHandler := handlers.NewStatsHandler(userProgressService)
r.Route("/api/v1", func(r chi.Router) {
    r.Use(authmiddleware.Authenticator(supabaseClient))
    // ... existing routes ...
    r.Get("/stats", statsHandler.GetUserStats)
})
```

⚠️ **Always** under `/api/v1` and under the auth middleware. No exceptions except `/health`.

### Step 6 — Run tests

```bash
cd backend
go test ./internal/...
go test -short ./...
make validate
```

All must pass.

## Step-by-step (frontend)

### Step 7 — Add types to the domain

`frontend/.../domain/models/stats.ts`:

```typescript
export interface UserStats {
  totalAudits: number;
  completedAudits: number;
  currentStreak: number;
  longestStreak: number;
  pointsByLanguage: Record<string, number>;
  lastAuditAt?: Date;
}
```

Re-export in `domain/models/index.ts`.

### Step 8 — Extend the port (if needed)

If the port `StatsPort` doesn't exist:

```typescript
// domain/ports/stats.port.ts
export abstract class StatsPort {
  abstract getUserStats(): Observable<UserStats>;
}
```

Or extend the existing port with a new method.

### Step 9 — Implement the adapter

`frontend/.../infrastructure/services/stats.service.ts`:

```typescript
import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { UserStats } from '../../domain/models/stats';
import { StatsPort } from '../../domain/ports/stats.port';

@Injectable({ providedIn: 'root' })
export class StatsService extends StatsPort {
  private http = inject(HttpClient);

  getUserStats(): Observable<UserStats> {
    return this.http.get<UserStats>(`${environment.apiUrl}/api/v1/stats`);
  }
}
```

### Step 10 — Add the use case (if the component needs orchestration)

`frontend/.../application/stats.use-case.ts`:

```typescript
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { UserStats } from '../domain/models/stats';
import { StatsPort } from '../domain/ports/stats.port';

@Injectable({ providedIn: 'root' })
export class GetUserStatsUseCase {
  private stats = inject(StatsPort);

  execute(): Observable<UserStats> {
    return this.stats.getUserStats();
  }
}
```

**Use case is optional** if the adapter is a simple 1:1 with the API. Use it when you need to combine multiple calls or apply business logic.

### Step 11 — Wire it in the component

```typescript
@Component({
  standalone: true,
  // ...
})
export class DashboardPageComponent {
  private getStats = inject(GetUserStatsUseCase);

  userStats = signal<UserStats | null>(null);

  ngOnInit() {
    this.getStats.execute().subscribe(s => this.userStats.set(s));
  }
}
```

### Step 12 — Test the adapter

`frontend/.../infrastructure/services/stats.service.spec.ts`:

```typescript
import { TestBed } from '@angular/core/testing';
import { HttpClientTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideHttpClient } from '@angular/common/http';
import { StatsService } from './stats.service';

describe('StatsService', () => {
  let service: StatsService;
  let httpMock: HttpClientTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), StatsService]
    });
    service = TestBed.inject(StatsService);
    httpMock = TestBed.inject(HttpClientTestingController);
  });

  it('fetches user stats', () => {
    service.getUserStats().subscribe(stats => {
      expect(stats.totalAudits).toBe(42);
    });
    const req = httpMock.expectOne('/api/v1/stats');
    expect(req.request.method).toBe('GET');
    req.flush({ totalAudits: 42, /* ... */ });
  });
});
```

### Step 13 — Run frontend tests

```bash
cd frontend/codeauditor
pnpm test
pnpm lint
```

## Step-by-step (open spec)

If the endpoint is **material** (>100 lines, changes architecture, breaks compat):
- Open a spec in `openspec/changes/<name>/` (see `docs/openspec-workflow.md`).
- Add tasks in `tasks.md` for each step above.
- Mark `[x]` as you go.

If the endpoint is **trivial** (single method, no new port, no new service):
- Skip the spec.
- Conventional commit + direct PR.

## Don't do

- ❌ Don't create a handler without a service (logic in handlers = code smell).
- ❌ Don't bypass the auth middleware (no `/api/v1/public` without explicit approval).
- ❌ Don't put endpoint URLs in the frontend (use `environment.apiUrl`).
- ❌ Don't return raw errors to the frontend (sanitize for security).
- ❌ Don't log tokens in the handler.
- ❌ Don't call `db` directly from a handler (always go through a service).
- ❌ Don't skip the test. Even one happy-path test is mandatory.
- ❌ Don't forget to update `frontend/.../infrastructure/services/index.ts` (barrel) when you add a new service.

## Success criteria

- [ ] Handler exists, compiles, has a test.
- [ ] Service method exists, has a test.
- [ ] Route is registered in `main.go` under auth.
- [ ] Frontend adapter exists, has a test.
- [ ] `make validate` passes.
- [ ] `make test` passes.
- [ ] `make build` succeeds.
- [ ] (If material) Spec exists in `openspec/changes/<name>/` and tasks are marked.
