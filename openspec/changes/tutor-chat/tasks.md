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

- [ ] 2.1 Create `backend/internal/core/services/tutor_service.go` — StreamChat, ProcessCommand, SummarizeSession
- [ ] 2.2 Build ChatContext assembly pipeline (5 system messages: identity, socratic, user, location, history)
- [ ] 2.3 Implement auto-escalation state machine (failures≥2, give-up≥3, 10m-stuck, solved-no-hints)
- [ ] 2.4 Create `backend/internal/infrastructure/driving/tutor_handler.go` — POST /api/v1/tutor/chat with SSE
- [ ] 2.5 Wire LLM cascade: Groq `llama-3.3-70b-versatile` → DeepSeek V3 fallback, MiniMax M3 opt-in

## Phase 3: Frontend Domain & Application

- [ ] 3.1 Create `domain/models/chat.ts` — ChatMessage, ChatContext, SocraticLevel types
- [ ] 3.2 Create `domain/ports/tutor-chat.port.ts` — TutorChatPort interface
- [ ] 3.3 Create `application/tutor-chat.use-case.ts` — signals for state + command parser (7 slash commands)
- [ ] 3.4 Implement SSE consumer in use case for token/status/done/error events

## Phase 4: Frontend Components & Layout

- [ ] 4.1 Create `tutor-chat-panel` smart component — FAB/sidebar toggle, injects use case
- [ ] 4.2 Create `tutor-chat-message` dumb component — markdown render, syntax highlight
- [ ] 4.3 Create `tutor-chat-input` dumb component — auto-resize textarea, Enter to send
- [ ] 4.4 Wire panel into `dojo-page.component.ts` (rightmost sidebar)
- [ ] 4.5 Wire panel into practice page layout (FAB <768px, sidebar on desktop)

## Phase 5: Tests & Verification

- [ ] 5.1 Go unit tests: TutorService context assembly, escalation, commands
- [ ] 5.2 Go unit tests: SSE handler (POST → SSE stream, fallback, errors)
- [ ] 5.3 Go tests: MCP pedagogical and user profile tools
- [ ] 5.4 Angular tests: TutorChatUseCase command parsing, signal state, SSE consumer
- [ ] 5.5 Angular tests: 3 components (panel toggle, message rendering, input behavior)
- [ ] 5.6 Verify `make validate` passes end-to-end

## Dependencies

- `language-progress` S1: LearningProfile schema must exist before Phase 1.6
- `mcp-monolith` S2: MCP tool infra must exist before Phase 1.5–1.6
