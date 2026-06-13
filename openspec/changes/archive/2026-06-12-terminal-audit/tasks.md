# Tasks: terminal-audit

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~380–500 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | ask-always |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Full feature — backend SSE + frontend audit wiring | PR 1 | All phases included; within budget |

## Phase 1: Backend Domain Models

- [x] 1.1 Create `backend/internal/core/domain/models/audit_models.go` with `AuditRequest`, `AuditEvent`, `AuditResult` structs (no external imports)

## Phase 2: Backend Local Sandbox

- [x] 2.1 Create `backend/internal/infrastructure/driven/sandbox/localsandbox.go` implementing `ports.SandboxExecutor`
  - TypeScript → `npx eslint --format=unix --stdin`
  - Go → `go vet` (temp file)
  - Configurable timeout (default 30s)
  - stdout/stderr pipes via `exec.CommandContext`

## Phase 3: Backend Application Service

- [x] 3.1 Create `backend/internal/core/services/audit_service.go` orchestrating sandbox execution → SSE writer

## Phase 4: Backend SSE Writer + Handler

- [x] 4.1 Create `backend/internal/infrastructure/driving/handlers/sse_handler.go` — `SSEWriter` adapter over `http.ResponseWriter`
- [x] 4.2 Create `backend/internal/infrastructure/driving/handlers/audit_handler.go` — `POST /api/v1/audit` handler, sets `Content-Type: text/event-stream`, streams events as `data: {...}\n\n`
- [x] 4.3 Modify `backend/cmd/api/main.go` — register `/api/v1/audit` route, exclude from global 30s timeout middleware

## Phase 5: Frontend Domain Model

- [x] 5.1 Create `frontend/codeauditor/src/app/domain/models/audit-event.ts` — `AuditEvent` interface (`type`, `data`, `timestamp`), zero Angular imports
- [x] 5.2 Update `frontend/codeauditor/src/app/domain/models/index.ts` — add export for `AuditEvent`

## Phase 6: Frontend AuditService

- [x] 6.1 Create `frontend/codeauditor/src/app/infrastructure/services/audit.service.ts` — `runAudit(code, language, challengeId)` returning `Observable<AuditEvent>`, uses `fetch()` + `ReadableStream` SSE parser
- [x] 6.2 Update `frontend/codeauditor/src/app/infrastructure/services/index.ts` — add export for `AuditService`

## Phase 7: Frontend UI

- [x] 7.1 Modify `terminal-panel.component.ts` — add `write(data: string)` and `clear()` public methods (already present in existing implementation)
- [x] 7.2 Modify `dojo-page.component.ts` — add "Auditar" button (disabled when no challenge selected), wire `AuditService`, clear terminal on new audit, handle loading/error states

## Phase 8: Verify

- [x] 8.1 Build frontend: `npx ng build` — successful
- [x] 8.2 Build backend: `go build ./cmd/api/` — successful

## Implementation Order

Backend phases first (1→4) to establish the API contract, then frontend (5→7). Verification (8) runs last to catch integration issues.