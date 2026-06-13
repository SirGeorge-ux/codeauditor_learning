# Proposal: mcp-integration

## Intent

Close the gap between static mock challenges and real code repositories. Users currently browse 8 mock challenges with no connection to actual repositories. This change introduces a backend-proxy Gogs integration so users can select real files from Gogs, create a temporary challenge, and audit it in the existing Dojo pipeline.

## Scope

### In Scope (MVP — Slice 1)
- Backend GogsClient (`ListRepos`, `GetFileContents`) at `internal/infrastructure/driven/gogs/`
- Backend GogsHandler with `GET /api/v1/gogs/repos` and `POST /api/v1/gogs/file`
- Wire new routes under `/api/v1/gogs/*` behind existing auth middleware
- Frontend GogsService at `infrastructure/services/gogs.service.ts`
- Rewrite `McpPageComponent` to list repos + file browser
- On file select, create a temp `Challenge` and navigate to `/dojo/:tempId`

### Out of Scope
- Challenge persistence in Postgres (deferred — temp challenges only)
- Metadata editing (title, description, difficulty) after import
- Auto-smell-detection / auto-categorization
- Gogs webhook / push notifications
- Gogs write operations (create repo, push code)

## Capabilities

### New Capabilities
- `gogs-proxy`: Backend Gogs API proxy with authenticated token, exposing repo list and file contents without exposing the token to the frontend.
- `repo-browser`: Frontend UI for browsing Gogs repositories and files, selecting a file to import as a temporary challenge.

### Modified Capabilities
- `challenge-engine`: Extend `ChallengeRepository` or `ChallengeService` to support creating transient/temporary challenges from imported Gogs files, and routing to `/dojo/:tempId`.

## Approach

Backend proxy pattern: the Go server holds the Gogs API token (env var) and forwards requests. The frontend never sees the token. The GogsClient implements a small driven port (secondary port) if needed, or lives purely in infrastructure. The handler returns repo metadata and file contents as JSON. The frontend `McpPageComponent` uses a `GogsService` to list repos, drill into files, and upon selection constructs a temporary `Challenge` object (in-memory) and routes to the existing Dojo page with a synthetic `tempId`. The Dojo page already supports `:id` routing.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/driven/gogs/` | New | GogsClient with `ListRepos`, `GetFileContents` |
| `backend/internal/infrastructure/driving/handlers/gogs_handler.go` | New | HTTP handler for `/api/v1/gogs/repos` and `/api/v1/gogs/file` |
| `backend/cmd/api/main.go` | Modified | Wire Gogs routes under auth middleware; read `GOGS_TOKEN` and `GOGS_URL` env vars |
| `frontend/src/app/infrastructure/services/gogs.service.ts` | New | Angular service wrapping Gogs API calls |
| `frontend/src/app/infrastructure/components/mcp/mcp-page.component.ts` | Modified | Rewrite to repo list + file browser + file selection flow |
| `frontend/src/app/domain/ports/challenge-repository.port.ts` | Modified | Add `createTemp(challenge: Challenge): Promise<Challenge>` or equivalent |
| `frontend/src/app/infrastructure/services/challenge.service.ts` | Modified | Support storing and retrieving temp challenges by synthetic `tempId` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Gogs token accidentally logged or exposed | Low | Never return token in responses; use `GOGS_TOKEN` env var only; sanitize logs |
| Gogs instance unreachable | Med | Return structured HTTP errors; frontend shows retry / fallback message |
| Large file fetched from Gogs crashes SSE or Dojo | Med | Cap file size at 1 MB in GogsClient; reject with `413` if exceeded |
| Auth middleware bypass on new routes | Low | Add routes inside existing `/api/v1` group that already applies `AuthMiddleware` |
| Temp challenge state lost on refresh | High (by design) | Acceptable for MVP; document as known limitation; persistence is out of scope |

## Rollback Plan

1. Remove Gogs route registration from `main.go`.
2. Delete `backend/internal/infrastructure/driven/gogs/` and `gogs_handler.go`.
3. Revert `McpPageComponent` to previous placeholder (or keep a simple static version).
4. Remove `GogsService` and revert `ChallengeService` changes.
5. Revert `challenge-repository.port.ts` if modified.

## Dependencies

- Running Gogs instance accessible from the backend
- `GOGS_URL` and `GOGS_TOKEN` environment variables configured
- Existing Supabase JWT auth middleware (already in place)

## Success Criteria

- [ ] `GET /api/v1/gogs/repos` returns a JSON list of repositories when called with a valid JWT
- [ ] `POST /api/v1/gogs/file` returns file contents for a given `owner/repo/path` when called with a valid JWT
- [ ] The frontend never sends or receives the Gogs API token
- [ ] `McpPageComponent` displays a list of repos and allows drilling into files
- [ ] Selecting a file creates a temporary `Challenge` and navigates to `/dojo/:tempId`
- [ ] The Dojo page renders the imported code in the Monaco editor and the ContextPanel shows the file path
- [ ] The audit pipeline (`POST /api/v1/audit`) works on the imported code exactly as it does on mock challenges
