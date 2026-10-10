# Archive Report: tutor-chat

> **Status:** SUCCESS — clean archive, no reconciliation needed
> **Archive date:** 2026-10-10
> **Archived to:** `openspec/changes/archive/2026-10-10-tutor-chat/`
> **Canonical spec:** `openspec/specs/tutor/spec.md` (byte-identical copy, verified via `diff -r`)

## Change Summary

`tutor-chat` delivers the Socratic AI Tutor Chat capability for S2: a context-aware,
always-visible chat panel on `/dojo/*` and `/practice/*` with 4 socratic levels (0–3),
7 slash commands, 10-message session memory, LLM cascade streaming over SSE, and
persistent `LearningProfile` updates.

**Scope delivered (final state, at chain HEAD `372eed0`):**

| PR | Branch | Commits | Files | LOC | Merged at | Delivered |
|----|--------|---------|-------|-----|-----------|-----------|
| 1 | `feature/tutor-chat-pr1` | 9 | 13 | +1,555 / -7 | `863f845` | 5 `.atl/prompts/*.md`, `core/domain/models/tutor.go`, `ports/tutor.go` (TutorServicePort, CachePort, LLMClientPort), `driven/session_cache.go` |
| 2 | `feature/tutor-chat-pr2` | 9 | 22 | +4,145 / -12 | `5730db6` | `services.TutorService` (StreamChat, ProcessCommand, SummarizeSession, EscalateIfNeeded), `tutor_context.go` (5-system-message ChatContext assembly), `tutor_prompts.go`, `llm_cascade/{cascade,groq,deepseek,minimax}.go` (5s first-token timeout), `tutor_handler.go` (POST /api/v1/tutor/chat SSE), wiring |
| 3 | `feature/tutor-chat-pr3` | 9 | 26 | +2,544 / -16 | `372eed0` | `domain/models/chat.ts`, `domain/ports/tutor-chat.port.ts`, `application/tutor-chat.use-case.ts` (5 signals + 7 slash commands + SSE consumer), `http-tutor-chat.adapter.ts`, 3 standalone components, `dojo-page` rightmost w-80 sidebar, `practice-page` stub, `mcp-token.provider.ts` + `app.config.ts` registration |

**Totals:** 27 tasks across 5 phases; **8,244 LOC** (1,555 + 4,145 + 2,544) across the chain.

**Verification (final state):** 207/207 frontend tests + 1,070+ backend tests pass at chain
HEAD; `make validate` quality gates (gofumpt, golangci-lint, go vet, go build, prettier,
eslint, ng build) all clean per tasks.md PR 3 outcome section and the apply-progress
observation. No `verify-report` artifact exists for this change; the archive gates were
satisfied by the persisted tasks artifact (all checkboxes complete), the apply agent's
`archive_readiness: READY` handoff, and the launch prompt's final-state facts.

## Spec Reconciliation Status

**NONE NEEDED.** The PR 3 apply agent's `archive_readiness` field confirmed the spec is
consistent with reality — the status event includes the `level` field + `status` text,
token events include the `text` field, done is an empty object, error includes `message`;
the frontend SSE wire format in design.md §5 matches the Go handler's emit shape exactly.
No 32→33-style reconciliation was required. The four change artifacts
(proposal.md, specs/tutor/spec.md, design.md, tasks.md) were archived byte-identical,
unmodified. tasks.md checkmarks (`[x]` for all 27 tasks) were in place at commit
`3bb1b56 docs(tasks): mark Phase 3 + 4 + 5 complete + PR 3 outcome + all phases done`.

**Canonical spec composition:** `openspec/specs/tutor/` did not previously exist, so the
delta spec IS a full spec for a new domain. It was synced to the canonical location via a
mechanical `cp` + `diff -r` + `mv` (never Read→Write). The archive retains its own copy
(precedent set by `2026-10-10-mcp-monolith`). The spec contains **11 requirements**
(Chat Panel Visibility, Chat Panel UI, Socratic Levels, Auto-Escalation, Chat Commands,
ChatContext Assembly, SSE Streaming, Session Memory, Persistent LearningProfile Updates,
MCP Tool Integration, Chat Restrictions).

## Mechanical Copy Audit

| Operation | Method | Readback |
|-----------|--------|----------|
| Change folder → archive | `git mv openspec/changes/tutor-chat openspec/changes/archive/2026-10-10-tutor-chat` | `diff -r` snapshot vs destination: **empty** (byte-identical) |
| Delta spec → canonical | `cp` + `diff -r` + `mv` to `openspec/specs/tutor/spec.md` | `diff -r archive/.../specs/tutor/spec.md openspec/specs/tutor/spec.md`: **empty** (byte-identical) |

All copies/moves were native shell operations; zero file content passed through the
model's Read/Write path.

## Engram Observations Read (traceability)

- `#798` — `sdd/tutor-chat/apply-progress` (PR 1 + PR 2 + PR 3 narrative; supersedes #794)
- `#794` — tutor-chat PR 1 discovery (MCP tools 1.5/1.6 already shipped via mcp-monolith)

## Side Benefits at Archive Time

1. **`MCP_TOKEN_PROVIDER` blocker closed:** PR 3 registered the token provider in
   `app.config.ts` (via `mcpTokenProvider` factory using `AuthService.getAccessToken()`),
   resolving the mcp-monolith PR 5 verify blocker — `MCPClientAdapter` now bootstraps cleanly.
2. **mcp-integration housekeeping verified:** `openspec/changes/mcp-integration` was already
   archived at `openspec/changes/archive/2026-10-10-mcp-integration/` by the prior
   housekeeping chore; not re-archived.

## Known Out-of-Scope Handoffs (for S3 / S5)

1. **`practice-page` is a STUB** — full S3 work in `openspec/changes/free-practice-mode/` will replace it.
2. **`/pista` + `/solucion` use placeholder LLM prompts** — real challenge-hint + challenge-solution integration from `ChallengeService` deferred to a follow-up PR after S3.
3. **Backend `SummarizeSession` is a deterministic 2-sentence stub** (not LLM-driven) — the real LLM-driven summary lands with the S5 llm-cascade change.
4. **Mobile FAB layout (<768px) not implemented** — panel renders as an inline sidebar on all viewports; the `hidden()` input on `TutorChatPanelComponent` exists for the future FAB toggle.
5. **AGENTS.md §6 + §12 follow-up doc update** — mark tutor-chat as archived in the roadmap and specs table (the 2026-10-10 doc-update chore predates this archive).

## Git Record

- Archive commit: `chore(openspec): archive tutor-chat change` (this commit: archive folder
  move via `git mv`, canonical spec addition, archive-report.md).
- Chain merges retained in history: `863f845` (PR 1), `5730db6` (PR 2), `372eed0` (PR 3).
- Feature branches `feature/tutor-chat-pr{1,2,3}` retained locally as historical record (not modified).
- Working tree clean after commit.

The SDD cycle for `tutor-chat` is COMPLETE: planned, implemented, verified, and archived.
S2 (Dictionary + MCP base + Tutor Chat) is now feature-complete AND archive-complete.