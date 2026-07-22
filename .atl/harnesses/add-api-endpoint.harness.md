# Harness: Add a New API Endpoint

> **Step-by-step workflow** for adding a new HTTP endpoint to CodeAuditor (backend + frontend).
> **Audience:** devs, agents.
> **Estimated time:** 1-2 hours.
> **Output:** an HTTP endpoint, auth-protected, with Go handler + service + tests, and Angular adapter + use case + tests.

---

## Prerequisites

- [ ] You've read `AGENTS.md` (root).
- [ ] You've read `docs/hexagonal-architecture.md` (§3 backend, §4 frontend).
- [ ] You've skimmed `backend/internal/infrastructure/driving/handlers/audit_handler.go` and one frontend service.
- [ ] The endpoint's contract is clear: method, path, input, output, errors.

---

## Step 1 — Open a spec (if material)

**Open a spec** in `openspec/changes/<name>/` if:
- The endpoint introduces a new port (talks to a new external service).
- The endpoint is part of a larger feature.
- It touches >300 lines across both stacks.

**Skip the spec** if:
- It's a simple GET that returns existing data.
- The handler logic is <50 lines.

---

## Step 2 — Define the contract (answer first)

```markdown
Endpoint: <METHOD> /api/v1/<path>
Auth: required (under /api/v1)
Input: <body, query, params>
Output (200): <JSON shape>
Errors: <4xx cases>
```

If any of these is unclear, **ask the user before writing code**.

---

## Step 3 — Backend: types and service

### 3.1 — Domain types (if new shape)

**File:** `backend/internal/core/domain/models/<name>.go`

```go
package models

// <Name> represents a <what it is>.
type <Name> struct {
    ID        string    `json:"id"`
    // ... fields
    CreatedAt time.Time `json:"createdAt"`
}
```

### 3.2 — Service method

**File:** `backend/internal/core/services/<name>_service.go` (or existing service)

```go
func (s *<Name>Service) <MethodName>(ctx context.Context, <args>) (*models.<Name>, error) {
    // implementation
}
```

**Rules:**
- `ctx context.Context` always first.
- Errors wrapped: `fmt.Errorf("get user stats: %w", err)`.
- Add a test: `<name>_service_test.go`.

### 3.3 — Service test

Table-driven if N variants:

```go
func Test<Name>Service_<MethodName>(t *testing.T) {
    tests := []struct {
        name    string
        args    args
        want    *models.<Name>
        wantErr bool
    }{
        // {name: "happy path", args: {...}, want: {...}},
        // {name: "not found", args: {...}, wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

---

## Step 4 — Backend: handler

**File:** `backend/internal/infrastructure/driving/handlers/<name>_handler.go`

```go
package handlers

import (
    "encoding/json"
    "net/http"

    "github.com/anomalyco/codeauditor/backend/internal/core/services"
    "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

type <Name>Handler struct {
    service *services.<Name>Service
}

func New<Name>Handler(service *services.<Name>Service) *<Name>Handler {
    return &<Name>Handler{service: service}
}

func (h *<Name>Handler) <MethodName>(w http.ResponseWriter, r *http.Request) {
    userID, ok := authmiddleware.UserIDFromContext(r.Context())
    if !ok {
        writeJSONError(w, http.StatusUnauthorized, "missing user id")
        return
    }

    result, err := h.service.<MethodName>(r.Context(), userID)
    if err != nil {
        writeJSONError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

### Handler test

**File:** `<name>_handler_test.go`

Use `httptest` + Chi router. Copy the pattern from `gogs_handler_test.go`.

```go
func Test<Name>Handler_<MethodName>(t *testing.T) {
    // Setup
    service := &mock<Name>Service{/* ... */}
    handler := New<Name>Handler(service)

    // Build request
    req := httptest.NewRequest("GET", "/api/v1/<path>", nil)
    req = req.WithContext(authmiddleware.ContextWithUserID(req.Context(), "user-123"))

    // Build response recorder
    w := httptest.NewRecorder()

    // Call
    handler.<MethodName>(w, req)

    // Assert
    if w.Code != http.StatusOK {
        t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
    }
    var got <Name>
    if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
        t.Fatal(err)
    }
    if got.ID != "expected" {
        t.Errorf("ID = %q, want %q", got.ID, "expected")
    }
}
```

---

## Step 5 — Backend: register route

**File:** `backend/cmd/api/main.go`

Inside the `r.Route("/api/v1", ...)` block:

```go
<name>Handler := handlers.New<Name>Handler(<name>Service)
r.Get("/<path>", <name>Handler.<MethodName>)
```

⚠️ **Always** under `/api/v1` and the auth middleware.

---

## Step 6 — Backend: verify

```bash
cd backend
go test ./internal/...
go test -short ./...
make validate
```

All must pass.

---

## Step 7 — Frontend: types and port

### 7.1 — Domain types

**File:** `frontend/.../domain/models/<name>.ts`

```typescript
export interface <Name> {
  id: string;
  // ... fields matching Go struct
  createdAt: Date;
}
```

Re-export in `domain/models/index.ts`.

### 7.2 — Port (or extend existing)

**File:** `frontend/.../domain/ports/<name>.port.ts`

```typescript
import { InjectionToken } from '@angular/core';
import { Observable } from 'rxjs';
import { <Name> } from '../models/<name>';

export interface <Name>Port {
  get<Name>(id: string): Observable<<Name>>;
}

export const <NAME>_PORT = new InjectionToken<<Name>Port>('<NAME>_PORT');
```

Or extend an existing port with a new method.

---

## Step 8 — Frontend: service adapter

**File:** `frontend/.../infrastructure/services/<name>.service.ts`

```typescript
import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';
import { <Name> } from '../../domain/models/<name>';
import { <Name>Port } from '../../domain/ports/<name>.port';

@Injectable({ providedIn: 'root' })
export class <Name>Service implements <Name>Port {
  private http = inject(HttpClient);

  get<Name>(id: string): Observable<<Name>> {
    return this.http.get<<Name>>(`${environment.apiUrl}/api/v1/<path>`);
  }
}
```

**Update barrel:** `infrastructure/services/index.ts` debe exportar este nuevo servicio.

### Service test

**File:** `<name>.service.spec.ts`

```typescript
import { TestBed } from '@angular/core/testing';
import { HttpClientTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideHttpClient } from '@angular/common/http';
import { <Name>Service } from './<name>.service';

describe('<Name>Service', () => {
  let service: <Name>Service;
  let httpMock: HttpClientTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting(), <Name>Service]
    });
    service = TestBed.inject(<Name>Service);
    httpMock = TestBed.inject(HttpClientTestingController);
  });

  it('fetches <name>', () => {
    service.get<Name>('id-1').subscribe(item => {
      expect(item.id).toBe('id-1');
    });
    const req = httpMock.expectOne('/api/v1/<path>');
    expect(req.request.method).toBe('GET');
    req.flush({ id: 'id-1', /* ... */ });
  });
});
```

---

## Step 9 — Frontend: use case (if needed)

**File:** `frontend/.../application/<name>.use-case.ts`

```typescript
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { <Name> } from '../domain/models/<name>';
import { <Name>Port } from '../domain/ports/<name>.port';

@Injectable({ providedIn: 'root' })
export class Get<Name>UseCase {
  private port = inject<<Name>Port>(<NAME>_PORT);

  execute(id: string): Observable<<Name>> {
    return this.port.get<Name>(id);
  }
}
```

**Use case is optional** for simple 1:1 mappings. Add it when you need:
- Combining multiple API calls.
- Caching.
- Mapping types.
- Error handling business logic.

---

## Step 10 — Frontend: bind port to service in `app.config.ts`

If you used `InjectionToken`, bind it:

```typescript
// app.config.ts
providers: [
  // ...
  { provide: <NAME>_PORT, useExisting: <Name>Service },
]
```

If the port is an interface implemented directly by the service, no binding needed.

---

## Step 11 — Frontend: wire in a component

```typescript
import { Component, inject, signal } from '@angular/core';
import { Get<Name>UseCase } from '../../../application/<name>.use-case';
import { <Name> } from '../../../domain/models/<name>';

@Component({
  standalone: true,
  // ...
})
export class <Page>Component {
  private get<Name> = inject(Get<Name>UseCase);

  data = signal<<Name> | null>(null);

  ngOnInit() {
    this.get<Name>.execute('id-1').subscribe(d => this.data.set(d));
  }
}
```

---

## Step 12 — Frontend: verify

```bash
cd frontend/codeauditor
pnpm test
pnpm lint
pnpm build
```

All must pass.

---

## Step 13 — Manual E2E test

1. Start backend + frontend.
2. Log in.
3. Navigate to the page that uses the new endpoint.
4. Verify the call succeeds (network tab).
5. Verify the UI renders the data.
6. Verify error states (try with invalid input).

---

## Step 14 — Commit and PR

```bash
git add backend/internal/core/domain/models/<name>.go \
        backend/internal/core/services/<name>_service.go \
        backend/internal/core/services/<name>_service_test.go \
        backend/internal/infrastructure/driving/handlers/<name>_handler.go \
        backend/internal/infrastructure/driving/handlers/<name>_handler_test.go \
        backend/cmd/api/main.go \
        frontend/.../domain/models/<name>.ts \
        frontend/.../domain/ports/<name>.port.ts \
        frontend/.../infrastructure/services/<name>.service.ts \
        frontend/.../infrastructure/services/<name>.service.spec.ts \
        frontend/.../application/<name>.use-case.ts \
        frontend/.../infrastructure/services/index.ts

git commit -m "feat(api): add <METHOD> /api/v1/<path>

Returns <what>. Auth-protected. Tests: handler (httptest), service
(table-driven), adapter (HttpClientTestingController)."

git push origin feature/add-<name>-endpoint
gh pr create --title "feat(api): add <METHOD> /api/v1/<path>" \
             --body "New endpoint: <description>. See openspec/changes/<name>/ for context."
```

---

## Checklist

- [ ] Contract defined (method, path, input, output, errors).
- [ ] Domain types added.
- [ ] Service method added with test.
- [ ] Handler added with test.
- [ ] Route registered in `main.go` under `/api/v1` + auth.
- [ ] Frontend types added.
- [ ] Frontend port + adapter + test.
- [ ] (Optional) Use case.
- [ ] (If InjectionToken) Bound in `app.config.ts`.
- [ ] Component wired.
- [ ] `make validate` passes.
- [ ] `make test` passes.
- [ ] `make build` passes.
- [ ] Manual E2E test passes.
- [ ] (If material) Spec in `openspec/changes/`.
- [ ] Conventional commit + clear PR.

---

## Common pitfalls

| Pitfall | How to avoid |
|---|---|
| Forgot auth middleware | Always register under `/api/v1` (which has the middleware). |
| Service called from handler with wrong context | Always pass `r.Context()`, not `context.Background()`. |
| Logged tokens or secrets | Never log headers. Sanitize error messages. |
| Frontend hardcoded URL | Use `environment.apiUrl`. |
| Service has no test | Even one happy-path test is mandatory. |
| Type mismatch Go ↔ TS | Field names are `camelCase` in both (Go's `json:"..."` tag must be `camelCase`). |
| Date format | Use ISO 8601 in JSON (`time.Time` in Go → `Date` in TS, format `new Date(isoString)`). |
| Async test timeout | Default Vitest timeout is 5s. Increase if your test needs more: `it('...', ..., 10000)`. |
| `provideHttpClient` missing | Angular 18+ requires explicit `provideHttpClient()` in test bed. |
| Returned `null` instead of empty array | Decide the contract: `null` for "not found", `[]` for "empty". Don't mix. |
