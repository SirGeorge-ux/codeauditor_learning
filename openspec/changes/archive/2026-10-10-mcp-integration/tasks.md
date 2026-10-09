# Tasks: MCP Integration — Gogs Proxy + Repo Browser

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~750–900 |
| 400-line budget risk | High |
| 800-line review budget (D2) | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (backend) → PR 2 (frontend) |
| Delivery strategy | ask-always |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Backend Gogs proxy — client, handler, routes, tests | PR 1 | Base: main. Independent backend slice, verifiable via curl. |
| 2 | Frontend integration — service, McpPage, temp challenge flow, tests | PR 2 | Depends on PR 1 routes. Tests with mocked backend. |

## Phase 1: Backend Gogs Proxy

- [x] 1.1 Create `backend/internal/infrastructure/driven/gogs/gogs_client.go` — `GogsClient` struct, `NewGogsClient(baseURL, token)`, `ListRepos(ctx)`, `GetFileContents(ctx, owner, repo, ref, path)`, `Repo`/`FileContent` types, 1 MB size cap, token sanitized from logs
- [x] 1.2 Create `backend/internal/infrastructure/driven/gogs/gogs_client_test.go` — `httptest` server: success paths, 404, 1 MB limit, unreachable, auth failure (REQ-GP-002/003/004/005)
- [x] 1.3 Create `backend/internal/infrastructure/driving/handlers/gogs_handler.go` — `GogsHandler` struct, `ListRepos(w, r)`, `GetFile(w, r)`, request body parsing, structured JSON errors (REQ-GP-001/004)
- [x] 1.4 Create `backend/internal/infrastructure/driving/handlers/gogs_handler_test.go` — `httptest` + Chi router: valid requests, missing fields, token not exposed in response, error codes, auth middleware integration (REQ-GP-006)
- [x] 1.5 Modify `backend/cmd/api/main.go` — add `GOGS_BASE_URL`/`GOGS_TOKEN` env vars, init `GogsClient`, wire `GogsHandler` under `/api/v1` auth group (~15 lines)

## Phase 2: Frontend GogsService

- [x] 2.1 Create `frontend/codeauditor/src/app/infrastructure/services/gogs.service.ts` — `@Injectable` with `listRepos(): Observable<GogsRepo[]>` and `fetchFile(owner, repo, branch, path): Observable<GogsFileResponse>`, typed interfaces (REQ-RB-006)
- [x] 2.2 Create `frontend/codeauditor/src/app/infrastructure/services/gogs.service.spec.ts` — Vitest `HttpClientTestingController`: verify HTTP method, URL, body for both methods
- [x] 2.3 Modify `frontend/codeauditor/src/app/infrastructure/services/challenge.service.ts` — add `addTempChallenge(challenge: Challenge): string` storing in local `Map<string, Challenge>`; update `getChallenge(id)` to check temp map first

## Phase 3: McpPage + Temp Challenge Flow

- [x] 3.1 Rewrite `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.ts` — inject `GogsService`/`Router`/`ChallengeService`; `repos` signal; repo card list; file path input with branch; loading/error states; retry button (REQ-RB-001/004/005)
- [x] 3.2 Wire temp challenge creation — on "Auditar" click: build `Challenge` with `tempId`, `difficulty=mid`, `category=imported`, language from backend response, `codeSmell=pending-analysis`; store via `addTempChallenge()`; navigate to `/dojo/:tempId` (REQ-RB-002/003)
- [x] 3.3 Create `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.spec.ts` — Vitest `TestBed` with mock `GogsService`: repo list renders, file input, navigation triggers, loading/error states