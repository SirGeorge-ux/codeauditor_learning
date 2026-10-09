# Archive Report: mcp-monolith (S2)

- **Change**: mcp-monolith
- **Archived**: 2026-10-10 → `openspec/changes/archive/2026-10-10-mcp-monolith/`
- **Artifact store**: openspec (report additionally persisted to Engram per orchestrator instruction, topic `sdd/mcp-monolith/archive-report`)
- **Closed by**: sdd-archive sub-agent (orchestrator launch, final-state handoff)

## Final-State Authority

Sources ranked per the Archive Final-State Handoff protocol:

1. **Persisted tasks artifact** (`tasks.md`) — completion visibility.
2. **Orchestrator launch prompt final-state facts** (most recent account): all 5 PRs of the change merged to master; PR 2 at `1f5d2f5` (+ amendment `9fc9564`, chore `1a6b6595` merged at `5e8bd53`), PR 3 at `baafa8c`, PR 4 at `8009ead`, PR 5 at `00a1b1a` (master HEAD); total 33 tools across 8 groups.
3. **Intermediate snapshots** (lower rank, valid history only): apply-progress engram #757 (6 revisions), verify-report engram #759 (PR 3), verify decisions #769/#776 (PR 3/PR 4), #781 (PR 5 — PASS-WITH-WARNINGS, 0 CRITICAL).

## Task Completion Gate — Exceptional Reconciliation (exact reason)

The persisted `tasks.md` showed Phase 2 tasks 2.1–2.3 unchecked at archive time. The gate's stale-checkbox clause was exercised:

- The orchestrator launch prompt **explicitly asserted** final state: "PR 2 (mcp-sandbox + mcp-challenges — 9 tools) — merged at `1f5d2f5`" and routed this archive.
- apply-progress revision history (#757, rev 0) proves 2.1 (RED: `run_code` structural security-flag constraint + SecurityProfile runtime check), 2.2 (4 `mcp-sandbox` tools), 2.3 (5 `mcp-challenges` tools) were delivered with unit tests (40 PASS) and HTTP transport smoke tests.
- Repository evidence: tool files + tests exist under `backend/internal/infrastructure/mcp/{sandbox,challenges}/`; merge commits `1f5d2f5`/`5e8bd53`; final verify #781 confirmed all 33 tools registered and reachable, backend + 145 frontend tests green.

Result: tasks.md L35–37 flipped `[ ]` → `[x]` for 2.1/2.2/2.3. No work was fabricated — the reconciliation is backed by the evidence above. Archive is therefore marked **intentional-with-warnings (stale-checkbox reconciliation)**.

## Reconciliation Audit (32→33, 7→8) — mandated by orchestrator, verified against reality

The 33rd tool (`mcp-tutor.tutor_lookup`, 8th group) is registered by the archived `2026-10-08-dictionary` change (owns `TutorLookupService`; `wiring.go` header documents "tutor (1) lookup (shipped via the dictionary change)"). Decision per orchestrator + verify #781: **update the spec to match reality (33/8); do NOT remove the 33rd tool** (its removal would regress the tutor LLM).

### `specs/mcp/spec.md`
| Line | Change |
|------|--------|
| 5 | headline: "32 tools across 7 groups" → "33 tools across 8 groups" |
| 44–46 | Tool Registry scenario GIVEN/THEN: 32/7 → 33/8 |
| 196 | inventory prose: "exactly 32 tools across 7 groups" → "exactly 33 tools across 8 groups" |
| 207 | added row `\| mcp-tutor \| tutor_lookup \| 1 \|` (inventory total 33 = 4+5+6+4+4+6+3+1) |
| 213 | scenario assertion: "exactly 32 tools" → "exactly 33 tools" |
| 217 | deferred-groups GIVEN: lists all 8 included groups (mcp-tutor added); deferred `mcp-inspiration` + `mcp-code_health` unchanged |
| 221 | added Note: 33rd-tool ownership (dictionary change, tutor LLM dependency) |

### `proposal.md`
| Line | Change |
|------|--------|
| 5 | headline 32/7 → 33/8 |
| 42 | group enumeration: added `tutor/` |
| 49 | "remaining ~27" → "~28" (33 − 5 = 28; PRs 2–4 landed 9+10+9) |
| 60 | "7 group sub-packages" → "8 group sub-packages (dictionary + tutor shipped; …)" |
| 61 | "(5 tools registered; 32 by PR 5)" → "33 by PR 5" |
| 93 | success criterion "returns 32 tools" → "returns 33 tools" |

### `design.md`
| Line | Change |
|------|--------|
| 7 | "32 tools across 7 groups" → "33 tools across 8 groups" |
| 17 | "same 32-tool registry" → "same 33-tool registry" |
| 190 | counts 32/7 → 33/8; attribution fixed: tutor lookup "(tutor-chat change)" → "(registered by the archived `2026-10-08-dictionary` change)" (matches tasks.md reconciliation + wiring.go + verify evidence) |
| 201 | added table row `\| mcp-tutor \| tutor_lookup \| TutorLookupService (over DictionaryService), LLMClient \|` |

### `tasks.md`
| Line | Change |
|------|--------|
| 35–37 | Phase 2 tasks 2.1/2.2/2.3 `[ ]` → `[x]` (stale-checkbox reconciliation, see gate section) |
| 63 | reconciliation note tense: "reads" → "read … at the time this note was written" (preserves the historical record: count = 33 not 32) |

## Verification Status at Close

- engram #781 (PR 5, final): **PASS-WITH-WARNINGS, 0 CRITICAL**. WARNINGs at close: (1) spec count stale 32 vs 33 — **fixed by this archive**; (2) `MCP_TOKEN_PROVIDER` missing from frontend `app.config.ts` — belongs to `tutor-chat` change, NOT mcp-monolith — handed off; (3) PR 5 size 1,378 LOC (3.4× budget) — informational, auto-chain policy accepted.
- engram #759 (PR 3): PASS. #769/#776 (PR 3/PR 4 verifies): PASS, 0 CRITICAL.
- No CRITICAL issues in any verify report → archive permitted.
- Wiring pinned by `TestBuildMCPRegistry_HTTP_GET_MCPToolsList_Reports33` (`backend/cmd/api/wiring_test.go`).

## Deliverable at Close

- **33 MCP tools / 8 groups**: sandbox 4, challenges 5, user 6, curriculum 4, dictionary 4 (via dictionary change), pedagogical 6, external_docs 3, tutor 1 (via dictionary change). Single Go binary, HTTP+SSE (JSON-RPC 2.0). Angular `MCPClientAdapter` complete. **S2 deliverable closed.**

## Spec Sync (mechanical)

- Main spec `openspec/specs/mcp/spec.md` did not exist → created by mechanical shell `cp` of the reconciled delta spec (never Read→Write). Verbatim `diff -r` source vs destination: **empty (exit 0)** — byte-identical. File mode 644 matching repo convention.

## Archive Move (mechanical, readback-verified)

- `openspec/changes/mcp-monolith/` → `openspec/changes/archive/2026-10-10-mcp-monolith/` via `git mv`. Pre-move recursive snapshot (`cp -R` to mktemp dir) vs archived folder: verbatim `diff -r` **empty (exit 0)** — byte-identical.
- Archive contains: `proposal.md`, `specs/mcp/spec.md`, `design.md`, `tasks.md` (0 unchecked implementation tasks). Active `openspec/changes/` no longer contains mcp-monolith.

## Engram Observations Read (traceability)

- Full reads: #757 (apply-progress, incl. 5-revision history), #759 (verify-report topic, PR 3 PASS), #781 (PR 5 verify decision).
- Search-preview hits: #769, #776, #777, #780, #783, #770, #775.

## Out-of-Scope Handoff (tracked for future changes, NOT fixed here)

1. `MCP_TOKEN_PROVIDER` not in frontend `app.config.ts` — belongs to `tutor-chat` change (S2), needs provider registration when tutor-chat ships.
2. Frontend prettier/eslint debt: 8 files + 1 error `challenge.service.ts:20` (`@angular-eslint/prefer-inject`) + 3 unused-var warnings (`progress.use-case.ts:12`, `progress.service.ts:9-10`).
3. `dojo-page.component.ts` pre-existing import-path bug (broken since commit `3ef8f32`, 2026-07-22, s1 challenge-rebuild); breaks `npx ng build`. Out of scope.
4. `core/services/{challenge,audit_history}_service.go` still hold `*sql.DB` directly (chore PR fixed only `audit_service.go`). Out of scope.
5. Housekeeping: 4 stale `feature/multi-lang-sandbox-oleada*` branches + `feature/auto-categorize-imports` + `feature/mcpkit` open — possible separate housekeeping change.
6. Pre-existing orphan changes in `openspec/changes/`: `mcp-integration` (to archive) + `multi-lang-sandbox-oleada1/` (to remove) — noted in AGENTS.md; untouched by this archive.

## Intentional-With-Warnings Marking

Archive marked **intentional-with-warnings** solely for the stale-checkbox reconciliation (Phase 2). No CRITICAL verification issues existed; no partial archive; no code modified. Hard constraints honored: no code files touched, no PR branches (`feature/mcp-monolith-pr2..-pr5`) modified, `make` not installed.