# Proposal: auth-supabase

## Intent

Add end-to-end user authentication—covering registration, login, JWT validation, and protected routes—using Supabase Auth for the CodeAuditor project.

## Scope

### In Scope
- Go HTTP handlers for register, login, and logout operations.
- Supabase JWT validation middleware in Go (implementing the existing `AuthValidator` port).
- Angular authentication service using signals to manage reactive session state.
- Angular UI components for login and registration.
- Angular route guards to protect authenticated views.

### Out of Scope
- Third-party OAuth providers (e.g., GitHub, Google).
- Password reset and email verification flows.
- Multi-Factor Authentication (MFA).
- Refresh token rotation (deferred to future work).

## Capabilities

### New Capabilities
- `user-auth`: Full user lifecycle covering registration, login, logout, and frontend session state management using signals.

### Modified Capabilities
- `auth-foundation`: Extending the scaffolding's existing JWT validation concept to support the full REST auth flow.

## Approach

We will use the Supabase Auth REST API as our identity provider. The Go backend will expose thin HTTP handlers that coordinate with Supabase, alongside JWT validation middleware built around the `AuthValidator` port to protect domain boundaries. The Angular frontend will consume these endpoints, managing session state reactively with an `AuthService` built on signals, and restricting access via route guards.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/http/` | New | Auth handlers (register, login, logout) and middleware |
| `backend/internal/infrastructure/auth/` | Modified | Supabase JWT implementation for `AuthValidator` |
| `frontend/src/app/infrastructure/` | New | Angular `AuthService` using signals |
| `frontend/src/app/ui/auth/` | New | Login and register UI components |
| `frontend/src/app/app.routes.ts` | Modified | Addition of Auth route guards |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| API Rate Limits | Low | Handle 429 status codes gracefully in the frontend UI. |
| Session Expiry | Medium | Implement basic client-side token expiration checks before making API requests. |
| Dev Environment | Low | Ensure the local `docker-compose.yml` fully spins up the Supabase Auth/Kong services. |

## Rollback Plan

We will wrap the backend authentication requirement behind a feature flag (e.g., `AUTH_ENABLED=false`). If critical issues arise, we can disable the flag and bypass frontend route guards to restore unauthenticated access until fixed.

## Dependencies

- Existing Supabase services (postgres, kong, auth, studio) orchestrated via `docker-compose.yml`.

## Success Criteria

- [ ] Users can successfully register a new account from the Angular UI.
- [ ] Users can log in and receive a valid JWT session.
- [ ] Backend middleware accurately validates the JWT and extracts the user identity for protected routes.
- [ ] Angular route guards redirect unauthenticated users away from protected views.
