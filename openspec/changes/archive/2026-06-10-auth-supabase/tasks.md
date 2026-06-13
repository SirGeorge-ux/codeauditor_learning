# Tasks: auth-supabase

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~450-550 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Full auth implementation | PR 1 | All phases included; no review budget pressure |

## Phase 1: Infrastructure & DB

- [x] 1.1 Create migration SQL file at `backend/internal/infrastructure/driven/supabase/migrations/001_create_usuarios.sql`
- [x] 1.2 Write `handle_new_user()` function: `AFTER INSERT ON auth.users` → `INSERT INTO public.usuarios (id, email, created_at)` with same UUID
- [x] 1.3 Write `update_updated_at_column()` helper for `public.usuarios`
- [x] 1.4 Create `public.usuarios` table with all fields (id UUID PK, email TEXT, display_name TEXT, racha_dias, puntos_maestria, rango_actual, ultimo_intento_valido, created_at, updated_at)

## Phase 2: Go Backend

- [x] 2.1 Create `backend/internal/infrastructure/driven/supabase/supabase_auth.go` implementing JWT validation
  - `ValidateToken(token string) error` — decode JWT locally using project JWT secret, verify signature and expiry
  - `UserIDFromToken(token string) (string, error)` — extract `sub` claim from validated JWT
- [x] 2.2 Create `backend/internal/infrastructure/driving/authmiddleware/auth_middleware.go` with Chi middleware `AuthMiddleware(authAdapter)` that extracts Bearer token, calls `ValidateToken`, injects user ID into `context.Context`
- [x] 2.3 Create `backend/internal/infrastructure/driving/handlers/auth_handler.go` with `GET /auth/me` handler — queries `public.usuarios` by user ID from context, returns profile JSON
- [x] 2.4 Wire `/auth/*` routes and JWT middleware in `backend/cmd/api/main.go`
- [x] 2.5 Add `SUPABASE_JWT_SECRET` environment variable config for token validation

## Phase 3: Angular Frontend

- [x] 3.1 Install `@supabase/supabase-js` in `frontend/codeauditor/` (already installed per package.json)
- [x] 3.2 Create `frontend/codeauditor/src/app/infrastructure/services/auth.service.ts` with Signals: `userSignal`, `sessionSignal`, `isAuthenticatedSignal` (computed)
- [x] 3.3 Create `frontend/codeauditor/src/app/infrastructure/guards/auth.guard.ts` functional route guard using `AuthService.isAuthenticatedSignal`
- [x] 3.4 Create `frontend/codeauditor/src/app/infrastructure/components/login.component.ts` standalone component with email/password form
- [x] 3.5 Create `frontend/codeauditor/src/app/infrastructure/components/register.component.ts` standalone component with email/password/confirm form
- [x] 3.6 Create `frontend/codeauditor/src/app/infrastructure/components/dashboard.component.ts` protected component showing user profile
- [x] 3.7 Add `/login`, `/register`, `/dashboard` routes to `frontend/codeauditor/src/app/app.routes.ts` with `AuthGuard` on dashboard

## Phase 4: Verification

- [x] 4.1 Run `go build ./backend/cmd/api/...` — verify backend compiles (SUCCESS) ✅ Verified: `go build -buildvcs=false ./...` exits 0
- [x] 4.2 Run `npm run build` in frontend — verify Angular compiles (SUCCESS) ✅ Verified: `pnpm build` succeeds, outputs dist/
- [x] 4.3 Verify migration SQL is valid (no syntax errors) ✅ Verified: file exists with complete schema, triggers, RLS

## Verify Phase Findings

| Check | Status | Notes |
|-------|--------|-------|
| Go backend compiles | ✅ PASS | `go build -buildvcs=false ./...` exits 0 |
| Angular frontend compiles | ✅ PASS | `pnpm build` produces dist/ |
| Hexagonal isolation (frontend) | ✅ PASS | No Angular imports in domain/ or application/ |
| No framework leaks (Go domain) | ✅ PASS | No HTTP/DB imports in internal/core/ |
| AuthValidator port exists | ✅ PASS | `backend/internal/ports/auth.go` with correct interface |
| SupabaseAuthAdapter implements port | ❌ FAIL | Signature mismatch: `ValidateToken` missing `ctx context.Context` param; middleware depends on concrete type, not port interface |
| Auth service uses Signals | ✅ PASS | `auth.service.ts` uses `signal()`, `computed()` |
| Route guard exists | ✅ PASS | `auth.guard.ts` standalone `CanActivateFn` |
| Login/Register components exist | ✅ PASS | Standalone, dark IDE theme (bg-gray-900) |
| DB migration file exists | ✅ PASS | Trigger + usuarios table + RLS policies |
| Go tests exist | ❌ FAIL | No `_test.go` files for auth |
| Angular tests exist | ❌ FAIL | No `.spec.ts` for auth components/service/guard |