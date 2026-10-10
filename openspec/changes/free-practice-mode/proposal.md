# Proposal: Free Practice Mode (S3)

## Intent

**What:** On-demand challenge generator at `/practice/free` — the user picks a language, topic, and F–S level; the system generates a validated challenge via the S2 MCP tools and opens it in the shared dojo flow.

**Why:** Curated mode is capped at 8 static challenges. AGENTS.md §0 defines free practice as "la IA genera ejercicios on-demand según el lenguaje y nivel" — without it users stall once curated content is exhausted.

**For whom:** a single-user dev (`ggogsmic`) practicing a specific language at a specific F–S level.

## Scope

### In Scope (MVP)
- Seed curriculum `.atl/curricula/typescript.md` (NEW, 10–15 topics, F–S ordered) as the canonical curricula-format example.
- Replace the `practice-page` STUB with the real flow: language → topic → level (F–S) → generated challenge.
- Frontend `PracticeUseCase` (`application/practice.use-case.ts`) orchestrating MCP calls via the existing `MCPClientPort`.
- Un-stub `ChallengeService.GenerateChallenge` and drive it through `mcp-pedagogical` (`get_learning_objective`, `generate_story_context`, `validate_solution` loop) + `mcp-challenges.validate_challenge`.
- Rango F–S selection (model already in `domain/models/language-progress.ts`).
- Per-user daily generation cap (default 20/day) to bound Groq usage.

### Out of Scope
- Curricula for languages other than TypeScript (post-S3 via `add-curriculum-topic.harness.md`).
- Voice input, in-chat code execution.
- Code Health integration (S4).
- LLM cascade *generalization* (config-driven multi-provider routing — S5). S3 adds only the structured generation call.
- Curriculum authoring UI (harness + Git only; editor is S6+).
- Updating `codeauditor-free-practice-generator/SKILL.md` (deferred follow-up).

## Capabilities

### New Capabilities
- `free-practice`: on-demand challenge generation from versioned curriculum + F–S level, with validation and daily cap.

### Modified Capabilities
- `challenges`: `ChallengeService.GenerateChallenge` moves from stub to real implementation (generation + persist `origin="generated"`).

## Approach

Frontend: selector → `PracticeUseCase` → `mcp-curriculum.get_topics`/`get_topic` → `mcp-pedagogical.get_learning_objective` + `generate_story_context` → `mcp-challenges.generate_challenge` → challenge opens in the dojo flow. Backend: un-stub `GenerateChallenge` to call the existing `llm_cascade` (Groq `qwen-2.5-coder-32b` JSON-mode primary, DeepSeek fallback) + `validate_solution` loop (max 3 attempts) + `validate_challenge`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `frontend/.../components/practice/` | Modified/New | Replace STUB; add language/topic/level selectors |
| `frontend/.../application/practice.use-case.ts` | New | Orchestrates MCP calls |
| `frontend/.../domain/` | Reuse | `challenge.ts`, `mcp-client.port.ts` already exist |
| `backend/internal/core/services/challenge_service.go` | Modified | Un-stub `GenerateChallenge` |
| `backend/internal/infrastructure/driven/llm_cascade/` | Modified | Add JSON-mode structured completion |
| `.atl/curricula/typescript.md` | New | Seed 10–15 topics |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| `GenerateChallenge` is a stub (`ErrChallengeGenerateNotImplemented`) | High | Un-stub in S3; add structured `Complete` to cascade (today `LLMClientPort` is chat-only `StreamChat`) |
| LLM cost/abuse (10–30s per challenge) | Med | Per-user daily cap (default 20) |
| Low seed topic quality drags the S3 demo | Med | Source from S1 curated challenges + `curricula-format.md` example |
| Sandbox runs unsafe generated code | Low | Existing flags `--cap-drop=ALL --network=none --read-only` |

## Rollback Plan

Additive: revert `practice-page` + `practice.use-case.ts`; delete `.atl/curricula/typescript.md` (nothing else depends on it). If the `GenerateChallenge` un-stub is isolated to `challenge_service.go`, revert that commit; the tool stays registered and returns a clean error.

## Dependencies

- `mcp-monolith` (S2): `mcp-curriculum`, `mcp-pedagogical`, `mcp-sandbox`, `mcp-challenges` tools + `MCPClientAdapter`.
- `tutor-chat` (S2): backend `llm_cascade` + `LLMClientPort`.
- `challenge-rebuild` (S1): v2 `Challenge` model (`expectedFindings`, `testCases`, `hints`).

## Success Criteria

- [ ] `/practice/free` renders the real flow (not STUB), F–S selector functional
- [ ] `mcp-challenges.generate_challenge` returns a validated challenge for a TypeScript topic
- [ ] Generated challenge opens in dojo and runs in sandbox
- [ ] Daily cap enforced (≥20 requests rejected)
- [ ] `make validate` passes; unit test per touched area

## Open Questions

1. **Generation backend ownership:** `ChallengeService.GenerateChallenge` is a stub deferred to "S5". S3 ships nothing usable without un-stubbing it. Confirm S3 owns the minimal un-stub + a JSON-mode `Complete` on the cascade (recommended), or defer real generation to S5 and ship frontend+seed only.
2. **Chat panel sharing:** reuse the S2 tutor-chat panel in `/practice/free` (already wired via the STUB) vs. a separate challenge panel. Recommendation: keep the shared panel, no `tutor-chat` spec changes.
