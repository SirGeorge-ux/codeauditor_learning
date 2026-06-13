# Design: Supabase Authentication

## Technical Approach

We implement an end-to-end authentication flow using Supabase. The Go backend exposes explicit authentication endpoints (login, register, logout, me) that delegate to the Supabase Auth REST API. A Chi middleware validates JWTs using the `ports.AuthValidator` interface, keeping the middleware agnostic of Supabase internals. The Angular frontend manages session state reactively using Signals in an `AuthService`, protecting routes with a functional guard.

The Supabase interaction is split into two adapters:
- **`supabase_client.go`** — REST client for SignUp, SignIn, SignOut calling Supabase Auth HTTP endpoints
- **`supabase_auth.go`** — Local JWT token validation implementing the `ports.AuthValidator` interface with a compile-time type assertion

## Architecture Decisions

### Decision: Backend Auth Endpoints vs Direct Frontend-Supabase

**Choice**: Implement Go HTTP routes (`/auth/register`, `/auth/login`, etc.) that act as an intermediary to Supabase.
**Alternatives considered**: Allow the Angular frontend to authenticate directly with Supabase via `supabase-js`, bypassing the backend for login/register.
**Rationale**: The design requirements specifically request Go HTTP routes for these actions. This encapsulates the identity provider logic on the server and provides a uniform REST API for the frontend.

### Decision: JWT Middleware Port Dependency

**Choice**: The auth middleware depends strictly on `ports.AuthValidator`.
**Alternatives considered**: Hardcode the Supabase JWT verification inside the middleware.
**Rationale**: Adhering to the Hexagonal Architecture pattern defined in the codebase. The middleware remains agnostic of Supabase, only relying on the port to validate tokens and extract the UserID. A compile-time assertion (`var _ ports.AuthValidator = (*SupabaseAuthAdapter)(nil)`) enforces this contract.

### Decision: `ValidateToken` receives `context.Context`

**Choice**: The port interface passes `ctx context.Context` as the first parameter to `ValidateToken`.
**Alternatives considered**: Stateless validation without context.
**Rationale**: Allows future extensions (logging, tracing, cancellation, per-request configuration) without changing the interface. The middleware forwards the HTTP request context.

### Decision: Type-Safe Context Key

**Choice**: Use an empty struct `UserIDContextKey struct{}` as the context key for user identity.
**Alternatives considered**: String-based context key.
**Rationale**: Empty struct keys are zero-allocation and type-safe — no other package can accidentally collide with or overwrite the key.

### Decision: Angular State Management for Auth

**Choice**: Use Angular Signals in an `AuthService`.
**Alternatives considered**: RxJS BehaviorSubjects or NgRx.
**Rationale**: Signals provide a cleaner, synchronous read API for templates and route guards, aligning with modern Angular best practices and reducing cognitive load.

## Data Flow

```
[Angular UI] ──(credentials)──→ [Go /auth/login] ──→ [Supabase Auth API]
     │                                 │                    │
     │                            (JWT Token)        (Validates/Signs)
     │                                 │                    │
[AuthService] ──(requests w/ JWT)──→ [Go Middleware] ───────┘
(Signal State)                        │ (calls ports.AuthValidator)
     │                                 ↓
[AuthGuard]                      [Protected Handlers]
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/cmd/api/main.go` | Modify | Register `/auth/*` routes and middleware |
| `backend/internal/infrastructure/driving/handlers/auth_handler.go` | Create | Implements `/auth/register`, `/auth/login`, `/auth/logout`, `/auth/me` |
| `backend/internal/infrastructure/driving/authmiddleware/auth_middleware.go` | Create | Chi middleware using `ports.AuthValidator`; includes `AuthMiddleware` and `UserIDParam` |
| `backend/internal/infrastructure/driven/supabase/supabase_auth.go` | Create | Implements `ports.AuthValidator` with local JWT validation (HS256) |
| `backend/internal/infrastructure/driven/supabase/supabase_client.go` | Create | REST client for Supabase Auth API (SignUp, SignIn, SignOut) |
| `backend/internal/infrastructure/driven/supabase/migrations/001_create_usuarios.sql` | Create | DB migration: `public.usuarios` table with triggers + RLS |
| `frontend/codeauditor/src/app/infrastructure/services/auth.service.ts` | Create | Angular service with signals for session state |
| `frontend/codeauditor/src/app/infrastructure/guards/auth.guard.ts` | Create | Functional route guard using `AuthService` |
| `frontend/codeauditor/src/app/infrastructure/components/login.component.ts` | Create | Standalone login component with reactive forms |
| `frontend/codeauditor/src/app/infrastructure/components/register.component.ts` | Create | Standalone register component with reactive forms |
| `frontend/codeauditor/src/app/infrastructure/components/dashboard.component.ts` | Create | Protected dashboard with user profile |
| `frontend/codeauditor/src/app/app.routes.ts` | Modify | Add `/login`, `/register`, `/dashboard` routes with guard |

## Interfaces / Contracts

### Port: AuthValidator

```go
// backend/internal/ports/auth.go
type AuthValidator interface {
    ValidateToken(ctx context.Context, token string) error
    UserIDFromToken(token string) (string, error)
}
```

### Backend Middleware Contract

The middleware extracts the Bearer token, calls `ValidateToken` with the HTTP request context, and injects the user ID into `context.Context` using a type-safe empty struct key.

```go
type UserIDContextKey struct{}

func AuthMiddleware(validator ports.AuthValidator) func(http.Handler) http.Handler
func GetUserID(ctx context.Context) string
func UserIDParam(validator ports.AuthValidator) func(http.Handler) http.Handler
```

### Frontend AuthService

```typescript
@Injectable({ providedIn: 'root' })
export class AuthService {
  readonly currentUser = signal<User | null>(null);
  readonly isAuthenticated = computed(() => this.currentUser() !== null);

  constructor(private supabase: SupabaseClient) {}

  async login(email: string, password: string): Promise<void> { /* calls backend /auth/login */ }
  async register(email: string, password: string): Promise<void> { /* calls backend /auth/register */ }
  async logout(): Promise<void> { /* calls backend /auth/logout */ }
}
```

### Frontend Route Guard

```typescript
export const authGuard: CanActivateFn = (route, state) => {
  const authService = inject(AuthService);
  if (authService.isAuthenticated()) return true;
  return inject(Router).parseUrl('/login');
};
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | JWT Middleware | Mock `AuthValidator` port, test valid/invalid/missing tokens and context injection |
| Unit | AuthService | Mock `AuthPort`, verify signal updates on login/logout success and failure |
| Unit | AuthGuard | Mock `AuthService` state, verify router redirects |
| Integration | Go Auth Handlers | Mock Supabase responses to test `/auth/*` HTTP status codes and payloads |
| Integration | DB Migrations | Verify SQL syntax, trigger creation, RLS policies |
| Compile-time | Port contract | `var _ ports.AuthValidator = (*SupabaseAuthAdapter)(nil)` ensures adapter matches interface |

## Known Gaps

- No test files (`_test.go`) exist for backend auth components
- No `.spec.ts` files exist for frontend auth components/service/guard
- `auth_handler.go` Me handler reads `user_id` from context using a string key rather than the typed `UserIDContextKey` — minor inconsistency pending refactor

## Migration / Rollout

No migration required. This is a new feature addition. The SQL migration file is included for the `public.usuarios` table, applied via Supabase's own migration runner.

## Open Questions

- [ ] Does the Go backend need to store users in a local database table for domain relations, or will we strictly rely on Supabase Auth's user IDs?
- [ ] Should the frontend `SupabaseAuthAdapter` be rewritten to call our new Go HTTP routes instead of using `supabase-js` directly, to avoid duplicated auth flows?
- [ ] Address the context key inconsistency in `Me` handler (string key vs typed struct key)
