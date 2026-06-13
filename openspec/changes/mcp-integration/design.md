# Design: mcp-integration

## Technical Approach

Implement the Gogs backend proxy as a hexagonal **driven adapter** (`GogsClient`), following the exact same pattern as `supabase/supabase_client.go`. The adapter lives at `backend/internal/infrastructure/driven/gogs/`, reads `GOGS_TOKEN` and `GOGS_BASE_URL` from environment variables, and exposes `ListRepos()` and `GetFileContents()` methods. These are wired through a Chi handler (`GogsHandler`) under the existing JWT-protected `/api/v1` route group — no new auth middleware needed.

The frontend introduces `GogsService` (a thin Angular injectable service using `HttpClient`), following the same pattern as `AuditService`. The `McpPageComponent` is rewritten from its current stub into a smart component that: (1) loads the repo list on init, (2) renders clickable repo cards, (3) shows a file path input after repo selection, and (4) calls `GogsService.fetchFile()` to retrieve raw code. On success, the service creates a temporary `Challenge` object (in-memory, no persistence) and navigates to the existing `/dojo/:id` route via Angular Router.

No new hexagonal ports are needed — `GogsClient` is a concrete driven adapter called directly from the handler, matching how `SupabaseClient` is used in `AuthHandler`. The existing `Challenge` domain model and `ChallengeRepository` interface remain unchanged. Temp challenges use the full `Challenge` interface with `difficulty=mid`, `category=imported`, `status=available`, and `codeSmell=pending-analysis` as defaults.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| Gogs token storage | Frontend env vs Backend env | Frontend env exposes token in browser; backend env keeps it server-side only | **Backend env** (`GOGS_TOKEN` in Go, never sent to client) |
| Hexagonal port for Gogs | Add `ports.GogsRepository` interface vs Concrete driven adapter | Port adds abstraction for future Gogs alternatives; concrete is simpler and matches existing `SupabaseClient` pattern | **Concrete driven adapter** — no port until a second provider exists (YAGNI) |
| File content transfer | Raw text in JSON vs Base64 | Raw text can break JSON; base64 is safe but adds decode step. Gogs API returns base64 natively. | **Base64** — matches Gogs API convention, decoded on frontend |
| Temp challenge storage | Route param vs Service signal vs SessionStorage | Route param exposes code in URL; signal is clean but lost on refresh; SessionStorage persists refresh | **Service signal** — simplest for MVP, matches existing `ChallengeService.selectedChallengeSignal` pattern |
| Language inference | Backend detection vs File extension mapping vs User selection | Backend detection requires running tools; user selection adds friction; extension mapping is deterministic and fast | **File extension mapping** (`.go`→`go`, `.ts`→`typescript`, `.js`→`javascript`, `.py`→`python`) |

## Data Flow

```
 McpPageComponent                    GogsService               Backend API                GogsClient
 ────────────────                    ───────────               ───────────                ─────────
       │                                  │                         │                         │
       │ ngOnInit()                       │                         │                         │
       │──loadRepos()────────────────────▶│                         │                         │
       │                                  │──GET /api/v1/gogs/repos▶│                         │
       │                                  │                         │──GET /user/repos───────▶│
       │                                  │                         │◀───[repo list]──────────│
       │                                  │◀───JSON repo array──────│                         │
       │◀──repos$ signal──────────────────│                         │                         │
       │                                  │                         │                         │
       │ [user clicks repo, enters path]  │                         │                         │
       │──fetchFile(repo, path)──────────▶│                         │                         │
       │                                  │──POST /api/v1/gogs/file▶│                         │
       │                                  │  {owner,repo,branch,path}                         │
       │                                  │                         │──GET contents/:path─────▶│
       │                                  │                         │◀───{content:base64}─────│
       │                                  │◀───JSON {content,lang}──│                         │
       │                                  │                         │                         │
       │  build temp Challenge            │                         │                         │
       │  router.navigate('/dojo/'+id)    │                         │                         │
       │                                  │                         │                         │
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `backend/internal/infrastructure/driven/gogs/gogs_client.go` | **Create** | Concrete adapter: `NewGogsClient(baseURL, token)`, `ListRepos()`, `GetFileContents(owner, repo, ref, path)`. Uses `net/http`, reads token from env. |
| `backend/internal/infrastructure/driving/handlers/gogs_handler.go` | **Create** | Chi handler struct: `NewGogsHandler(client)`, `ListRepos(w, r)`, `GetFile(w, r)`. Parses JSON body for file requests, returns structured errors. |
| `backend/cmd/api/main.go` | **Modify** | Initialize `GogsClient` (env vars: `GOGS_BASE_URL`, `GOGS_TOKEN`), create `GogsHandler`, wire routes under `/api/v1` auth group. ~10 lines added. |
| `frontend/codeauditor/src/app/infrastructure/services/gogs.service.ts` | **Create** | Angular `@Injectable`: `listRepos()`, `fetchFile(owner, repo, branch, path)`. Uses Angular `HttpClient`. Returns Observables. |
| `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.ts` | **Modify** | Rewrite stub: inject `GogsService` + `Router`, `repos$` signal, repo selection state, file path input, loading/error states, "Auditar" button that calls fetchFile → builds temp Challenge → navigates to `/dojo/:tempId`. |
| `frontend/codeauditor/src/app/infrastructure/services/challenge.service.ts` | **Modify** | Add `addTempChallenge(challenge: Challenge): string` method — stores in local `Map<string, Challenge>` alongside mock challenges, returns the ID. Modify `getChallenge(id)` to check temp map first. |

## Interfaces / Contracts

### Go — GogsClient

```go
type GogsClient struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

type Repo struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    FullName    string `json:"full_name"`
    Description string `json:"description"`
    Private     bool   `json:"private"`
    CloneURL    string `json:"clone_url"`
    DefaultBranch string `json:"default_branch"`
}

type FileContent struct {
    Name     string `json:"name"`
    Path     string `json:"path"`
    Content  string `json:"content"`  // base64-encoded
    Encoding string `json:"encoding"` // "base64"
    Size     int64  `json:"size"`
}

func NewGogsClient(baseURL, token string) *GogsClient
func (c *GogsClient) ListRepos(ctx context.Context) ([]Repo, error)
func (c *GogsClient) GetFileContents(ctx context.Context, owner, repo, ref, path string) (*FileContent, error)
```

### Go — Handler request/response

```go
type GetFileRequest struct {
    Owner  string `json:"owner"`
    Repo   string `json:"repo"`
    Branch string `json:"branch"`
    Path   string `json:"path"`
}

type GetFileResponse struct {
    Owner    string `json:"owner"`
    Repo     string `json:"repo"`
    Branch   string `json:"branch"`
    Path     string `json:"path"`
    Content  string `json:"content"`  // base64
    Encoding string `json:"encoding"`
    Language string `json:"language"` // inferred from extension
    Size     int64  `json:"size"`
}
```

### TypeScript — GogsService response types

```typescript
export interface GogsRepo {
  id: number;
  name: string;
  full_name: string;
  description: string;
  private: boolean;
  clone_url: string;
  default_branch: string;
}

export interface GogsFileResponse {
  owner: string;
  repo: string;
  branch: string;
  path: string;
  content: string;   // base64
  encoding: string;  // "base64"
  language: string;
  size: number;
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| `GogsClient` | `ListRepos`, `GetFileContents` with real Gogs instance | Integration test with env var token — skip if `GOGS_TOKEN` not set |
| `GogsClient` | Error handling (bad URL, timeout, 404) | Mock HTTP server with `httptest` |
| `GogsHandler` | Request parsing, auth check, error responses | `httptest` with Chi router |
| `GogsService` | HTTP calls return correct types | Vitest with `HttpClientTestingController` |
| `McpPageComponent` | Repo list renders, file input works, navigation triggers | Vitest with `TestBed`, mock `GogsService` |

## Migration / Rollout

No migration needed. The `GogsClient` reads new env vars (`GOGS_BASE_URL`, `GOGS_TOKEN`) — if they're not set, the backend starts normally but Gogs endpoints return 503. The frontend McpPage is feature-independent from the existing Dashboard and Dojo — rolling back means reverting the McpPage rewrite and GogsService, zero impact on mock challenge flow.

## Open Questions

- **Branch selection UX**: MVP uses the repo's `default_branch`. A branch picker (dropdown after repo selection) adds complexity. Defer to phase 2.
- **File tree browsing**: MVP uses a text input for file path. A full file tree (recursive Gogs API calls) is deferred. The path input is fast and sufficient for targeted audits.
