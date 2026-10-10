# Tasks: Socratic AI Tutor Chat

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Backend infra) → PR 2 (Backend core) → PR 3 (Frontend) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Backend infra: prompts, models, ports, MCP tools, session cache | PR 1 | `go test ./backend/internal/infrastructure/mcp/...` | `go run ./backend/cmd/api` + `curl /api/v1/health` | Revert PR 1 — no handler depends on it yet |
| 2 | Backend core: TutorService, SSE handler, LLM cascade wiring | PR 2 | `go test ./backend/internal/core/services/...` | `curl -X POST /api/v1/tutor/chat -d '{"message":"hi"}'` verify SSE stream | Revert PR 2 — no frontend depends on it yet |
| 3 | Frontend: models, use case, components, layout wiring | PR 3 | `pnpm test -- --include '**/tutor-chat/**'` | Navigate `/dojo/ch-sqli` verify panel renders | Revert PR 3 — no layout regression |

## Phase 1: Backend Foundation

- [x] 1.1 Create `.atl/prompts/tutor-system.md` with tutor identity system prompt
- [x] 1.2 Create `.atl/prompts/tutor-socratic-{0..3}.md` with 4 level-specific prompts
- [x] 1.3 Create `backend/internal/core/domain/models/tutor.go` — ChatMessage, ChatContext, SocraticLevel
- [x] 1.4 Create `backend/internal/ports/tutor.go` — TutorServicePort, CachePort, LLMClientPort interfaces
- [x] 1.5 ~~Create~~ **already shipped via mcp-monolith PR 4** — `mcp-pedagogical.get_socratic_prompt` exists at `backend/internal/infrastructure/mcp/pedagogical/pedagogical_tool.go:524` with prompt-injection guard + service-layer defense + OpenAI schema; tool-layer test passes; covers spec scenario "Tutor fetches socratic prompts". **No re-implementation in PR 1.**
- [x] 1.6 ~~Create~~ **already shipped via mcp-monolith PR 3** — `mcp-user.get_learning_profile` (lines 376–412) and `mcp-user.update_learning_profile` (lines 471–538) exist with `preferencias` validation + `nivel_socratismo` range guard; tool-layer tests pass; cover spec scenarios "Profile update after audit completion" + "/nivel 0–3 changes LearningProfile". **No re-implementation in PR 1.**
- [x] 1.7 Create `backend/internal/infrastructure/driven/session_cache.go` — in-memory 10-message ring

### PR 1 outcome

Phase 1 delivered in **stacked-to-main** chain, PR 1 of 3. Branch `feature/tutor-chat-pr1` (8 commits, ~1150 LOC inserted, ~12 LOC delta at HEAD). Phases 2–5 land in PR 2 (backend TutorService + SSE handler) and PR 3 (frontend).

Tasks 1.5 and 1.6 were discovered to be already shipped via `mcp-monolith` (PR 3 for `get/update_learning_profile`, PR 4 for `get_socratic_prompt`); PR 1 verified the existing coverage and the new domain types (`SocraticLevel`, `ChatMessage`, `ChatContext`) are consistent with the tool-layer validation. The discover-only outcome is documented in commit messages.

Reused without re-implementation:
- `services.TutorLookupService` (mcp-monolith PR 3) — registered as `mcp-tutor.tutor_lookup`
- `services.PedagogicalService` (mcp-monolith PR 4) — the 6 tools including `GetSocraticPrompt`
- `mcp-user.profile_tools` (mcp-monolith PR 3) — the `get/update_learning_profile` tools
- `ports.OllamaClient` (audit path) — kept narrow on purpose; **NOT** extended for the cascade. A new `ports.LLMClientPort` carries the multi-provider cascade concern so the two LLM trajectories stay untangled (see discovery note in apply-progress).

## Phase 2: Core Backend Service

- [x] 2.1 Create `backend/internal/core/services/tutor_service.go` — StreamChat, ProcessCommand, SummarizeSession
- [x] 2.2 Build ChatContext assembly pipeline (5 system messages: identity, socratic, user, location, history)
- [x] 2.3 Implement auto-escalation state machine (failures≥2, give-up≥3, 10m-stuck, solved-no-hints)
- [x] 2.4 Create `backend/internal/infrastructure/driving/tutor_handler.go` — POST /api/v1/tutor/chat with SSE
- [x] 2.5 Wire LLM cascade: Groq `llama-3.3-70b-versatile` → DeepSeek V3 fallback, MiniMax M3 opt-in

## Phase 3: Frontend Domain & Application

- [x] 3.1 Create `domain/models/chat.ts` — ChatMessage, ChatContext, SocraticLevel types
- [x] 3.2 Create `domain/ports/tutor-chat.port.ts` — TutorChatPort interface
- [x] 3.3 Create `application/tutor-chat.use-case.ts` — signals for state + command parser (7 slash commands)
- [x] 3.4 Implement SSE consumer in use case for token/status/done/error events

## Phase 4: Frontend Components & Layout

- [x] 4.1 Create `tutor-chat-panel` smart component — FAB/sidebar toggle, injects use case
- [x] 4.2 Create `tutor-chat-message` dumb component — markdown render, syntax highlight
- [x] 4.3 Create `tutor-chat-input` dumb component — auto-resize textarea, Enter to send
- [x] 4.4 Wire panel into `dojo-page.component.ts` (rightmost sidebar)
- [x] 4.5 Wire panel into practice page layout (FAB <768px, sidebar on desktop)

## Phase 5: Tests & Verification

- [x] 5.1 Go unit tests: TutorService context assembly, escalation, commands
- [x] 5.2 Go unit tests: SSE handler (POST → SSE stream, fallback, errors)
- [x] 5.3 Go tests: MCP pedagogical and user profile tools
- [x] 5.4 Angular tests: TutorChatUseCase command parsing, signal state, SSE consumer
- [x] 5.5 Angular tests: 3 components (panel toggle, message rendering, input behavior)
- [x] 5.6 Verify `make validate` passes end-to-end

## Dependencies

- `language-progress` S1: LearningProfile schema must exist before Phase 1.6
- `mcp-monolith` S2: MCP tool infra must exist before Phase 1.5–1.6

### PR 3 outcome

Phase 3 + 4 delivered in **stacked-to-main** chain, PR 3 of 3. Branch `feature/tutor-chat-pr3` (8 commits, ~2100 LOC inserted at HEAD). Phases 1–2 landed in PR 1 + PR 2 (backend infra + TutorService + LLM cascade + SSE handler); Phase 5 (verification) is split across all 3 PRs (Go tests in PR 1 + 2, Angular tests in PR 3, end-to-end verify in this PR).

**Frontend (Phase 3):**
- 3.1 `domain/models/chat.ts` — ChatMessage, SocraticLevel, StreamChatRequest, SSEEvent (discriminated union). 12 contract tests in `chat.spec.ts`.
- 3.2 `domain/ports/tutor-chat.port.ts` — TutorChatPort with `streamChat(req, token) → Observable<SSEEvent>`. 2 contract tests in `tutor-chat.port.spec.ts`.
- 3.3 `application/tutor-chat.use-case.ts` — 5 readonly signals (messages / currentLevel / streaming / error / lastOfferHint) + 7 slash commands + SSE consumer. 21 use case tests.
- 3.4 SSE consumer subscribes to the full stream, dispatches per-event-kind, clears streaming on done / error.

**Frontend (Phase 4):**
- 4.1 `tutor-chat-panel` smart component (OnPush, injects TutorChatService, wires (navigateTo) hook for /diccionario).
- 4.2 `tutor-chat-message` dumb component (input.required signal, role alignment, ``` code-block render via `<pre><code>`, system amber treatment).
- 4.3 `tutor-chat-input` dumb component (auto-resize textarea, Enter sends, Shift+Enter newline, slash-command detection).
- 4.4 dojo-page wire (rightmost w-80 sidebar, routeContext with route + challengeId + codeSnippet).
- 4.5 practice-page stub created; mounts the panel + "coming soon" placeholder. Full S3 implementation lands in `free-practice-mode`.

**Bonus:** closes the mcp-monolith PR 5 verify blocker — `MCP_TOKEN_PROVIDER` is now registered in `app.config.ts` via `mcpTokenProvider` factory (uses `AuthService.getAccessToken()`). The `MCPClientAdapter` can now bootstrap cleanly.

**Reused without re-implementation (carry-over from PR 1 + 2 + mcp-monolith):**
- `TutorChatService.StreamChat` (backend) — frontend just calls POST /api/v1/tutor/chat
- `MCPClientAdapter` (mcp-monolith PR 5) — for /perfil + /nivel persistence
- `AuthService` — for JWT forward
- Backend SSE wire format — adapter matches the Go handler's `event: <type>\ndata: <payload>\n\n` shape exactly

**Work-unit commits (oldest → newest):**
1. `fix(frontend): register MCP_TOKEN_PROVIDER in app.config.ts`
2. `feat(frontend): tutor chat domain models`
3. `feat(frontend): TutorChatPort interface in domain/ports/`
4. `feat(frontend): TutorChatUseCase with signals + SSE consumer + 7 slash commands`
5. `feat(frontend): HttpTutorChatAdapter — SSE consumer for POST /api/v1/tutor/chat`
6. `feat(frontend): tutor-chat 3 components — panel (smart) + message + input (dumb)`
7. `feat(frontend): wire tutor chat panel into dojo + create practice page stub`
8. `chore(frontend): greenbar — prettier + eslint pass clean`

**Quality gates (PR HEAD):**
- `go test -count=1 ./cmd/... ./internal/...` → ALL PASS, 0 FAIL
- `golangci-lint run ./...` → 0 findings
- `gofumpt -l .` → empty
- `go vet ./...` → exit 0
- `go build ./...` → exit 0
- `npx ng test --watch=false` → 207 tests, 23 files, 0 FAIL
- `pnpm prettier --check` → 0 issues
- `pnpm lint` → 0 errors, 0 warnings
- `npx ng build --configuration=development` → exit 0

**Hexagonal purity (verified):**
- 0 `@angular/*` imports in `frontend/.../domain/` (chat.ts, chat.spec.ts, tutor-chat.port.ts, tutor-chat.port.spec.ts)
- 0 `database/sql` imports in `backend/internal/core/` (regression — no backend touched)
- TutorChatUseCase imports only domain types + RxJS + Angular `signal`; no HTTP / fetch / DOM

**Out of scope (deferred):**
- `/pista` + `/solucion` are routed through the LLM cascade (placeholder prompts); the real hint/solution endpoints from `ChallengeService` (backend) are not yet called from the frontend — these can land in a follow-up PR.
- Practice page is a stub; the S3 free-practice-mode change delivers the real generator.
- FAB/modal layout for mobile (<768px) is not yet implemented; the panel is rendered as an inline sidebar. The hidden() input on TutorChatPanelComponent exists for the future FAB toggle.
- Markdown rendering is naive (``` fences only); a heavier library (e.g. marked, markdown-it) can be added in a follow-up without touching the use case.
