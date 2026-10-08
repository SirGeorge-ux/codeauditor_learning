# Archive Report — mcpkit

**Change**: mcpkit — Extract MCP Runtime Library
**Archive date**: 2026-10-08
**Source-of-truth cycle**: SDD (openspec mode)
**Branch at archive**: `feature/mcpkit` (6 commits on top of `6f70721`)
**Archived to**: `openspec/changes/archive/2026-10-08-mcpkit/`

---

## 1. Cycle Outcome

**Status**: complete. SDD cycle closed.
**Mechanical copy contract**: satisfied (Step 2 and Step 3 `diff -r` readbacks both empty; no orphan temp files; no Read→Write byte routing).
**Main spec sync**: `openspec/specs/mcpkit/spec.md` created (main spec did not exist; mechanical shell copy of the delta spec; verbatim diff `-r` empty).
**Source-of-truth index updated**: `openspec/specs/` now reflects the new `mcpkit` capability.

---

## 2. Final State at Close (outranks intermediate snapshots)

Per the orchestrator's launch prompt and the persisted `tasks.md`:

- Branch: `feature/mcpkit` (created from `feature/dictionary` at `6f70721`).
- Commits on top of base (`6f70721`), in order:
  1. `306e2ce` — `refactor(mcp): extract runtime to internal/mcpkit`
  2. `a8ac3c1` — `refactor(mcp): drop dead re-exports from mcpkit`
  3. `0951635` — `docs(openspec): mark mcpkit apply tasks complete`
  4. `88a2e0e` — `docs(apply): record mcpkit apply-progress for orchestrator merge`
  5. `d615a9d` — `docs(openspec): add mcpkit design, spec, and verify-report artifacts`
  6. `95eaf60` — `docs(openspec): close mcpkit apply tasks 5.9 and 5.10`
- Composition: 2 refactor work-unit commits (Phase A mechanical move + Phase B re-export cleanup) + 4 docs commits (openspec hygiene).
- 0 CRITICAL findings, 0 blockers, 0 FAIL criteria.
- 8/8 Success Criteria PASS.
- 0 DEBT items remaining.
- 8/8 spec ADDED Requirements compliant; 24/24 Scenarios compliant.
- 0 changes committed to `openspec/changes/mcp-monolith/` or `openspec/changes/tutor-chat/` by this branch (working-tree noise on those folders is pre-existing and unrelated to this archive).

---

## 3. Archive of the Started

### 3.1 Artifacts moved to archive (`openspec/changes/archive/2026-10-08-mcpkit/`)

| Artifact | Status |
|---|---|
| `proposal.md` | ✅ present |
| `design.md` | ✅ present (committed at `d615a9d`) |
| `tasks.md` | ✅ present; 39/39 implementation tasks `[x]`; 4 unchecked lines are Out-of-Scope items (lines 93–98, prefaced with `~~`) — NOT implementation tasks per the orchestrator's prompt |
| `specs/mcpkit/spec.md` | ✅ present |
| `verify-report.md` | ✅ present |
| `apply-progress.md` | ✅ present (intermediate snapshot; informational) |
| `progress.md` | ✅ present |
| `grep-after-cleanup.txt` | ✅ present (Decision-2 grep evidence captured by the apply agent) |

### 3.2 Delta-spec → main-spec sync

| Domain | Action | Evidence |
|---|---|---|
| `mcpkit` | Created (new capability) | `openspec/specs/mcpkit/spec.md` written via mechanical shell `cp`+`mv` (skill §"If Main Spec Does NOT Exist" block); verbatim `diff -r` empty; no orphan temp file left in `openspec/specs/mcpkit/` (only `spec.md` present) |

### 3.3 Source-of-truth now reflects

- The `mcpkit` package location: `backend/internal/mcpkit/` (sibling of `internal/infrastructure/`).
- The `mcpctx` package location: `backend/internal/mcpkit/mcpctx/`.
- The public contract: `MCPTool`, `Registry`, `Dispatcher` (JSON-RPC 2.0), `Server`, `RateLimiter`, `mcpctx` helpers, six error-code constants, helper functions (`TruncateResult`, `SanitizeResult`, `SanitizeErrorMessage`, `IsRateLimitedError`, `NewRateLimitedError`), and tuning knobs (`DefaultRateLimit`, `DefaultRateWindow`, `MaxResponseBytes`).
- The four dead re-exports (`ContextWithUserID`, `UserIDFromContext`, `AuthUserIDKey`, `ErrInvalidParams`) are NOT in the `mcpkit` public surface; consumers reach them via `mcpkit/mcpctx`.
- The tool-group import contract: every tool group imports `internal/mcpkit` (or `internal/mcpkit/mcpctx`); no tool group imports `internal/infrastructure/mcp` for runtime symbols.
- The hexagonal direction: `mcpkit → infrastructure/driving/authmiddleware` is one-way and documented; no reverse imports introduced.

---

## 4. Carry-Forward Notes (out-of-scope from this change, recorded for the next cycle)

These were explicitly out of scope per `proposal.md` and `design.md` and are NOT in this archive's scope. They belong to follow-up changes:

- `mcpkit-interfaces` (or similar): canonicalize `RegistryRegistrar` / `MCPToolRegistrar` duplicates in `dictionary` and `tutor` tool groups. Decision 3 of `design.md` records the deferral.
- `mcp-monolith` archive (phases 2–5): update the parent spec's "mcp runtime" location reference from `infrastructure/mcp/` → `internal/mcpkit/`. This change's `design.md` (§"Tracking Note") documents the deferred one-line edit.
- `make validate` full-repo drift cleanup (pre-existing gofumpt drift in `internal/infrastructure/driven/sandbox/{dockersandbox,localsandbox}.go`, commit `39c35a9`, 2026-06-22): out of scope per `apply-progress.md`; predates this branch.
- Extract `authmiddleware.UserIDContextKey` to a leaf `authctx` package: bigger refactor, deferred.

These are recorded here for the next agent's continuity; they do not affect the closed cycle.

---

## 5. Source-of-Truth Artifacts

| Path | Role |
|---|---|
| `openspec/specs/mcpkit/spec.md` | New main spec — 8 ADDED Requirements, 24 Scenarios |
| `openspec/changes/archive/2026-10-08-mcpkit/proposal.md` | Archived proposal |
| `openspec/changes/archive/2026-10-08-mcpkit/design.md` | Archived design (4 decisions documented) |
| `openspec/changes/archive/2026-10-08-mcpkit/tasks.md` | Archived tasks (39/39 implementation tasks complete) |
| `openspec/changes/archive/2026-10-08-mcpkit/specs/mcpkit/spec.md` | Archived delta spec (identical to main spec) |
| `openspec/changes/archive/2026-10-08-mcpkit/verify-report.md` | Archived verify report (PASS WITH WARNINGS at verify time; all 3 warnings resolved before archive) |
| `openspec/changes/archive/2026-10-08-mcpkit/apply-progress.md` | Archived apply-progress (intermediate snapshot, informational) |
| `openspec/changes/archive/2026-10-08-mcpkit/progress.md` | Archived progress notes |
| `openspec/changes/archive/2026-10-08-mcpkit/grep-after-cleanup.txt` | Archived Decision-2 grep evidence (zero consumers of the four deleted re-exports) |
| `openspec/changes/archive/2026-10-08-mcpkit/archive-report.md` | This file (additive; not subject to source/destination diff readback) |

---

## 6. SDD Cycle Complete

The mcpkit change has been fully planned, implemented, verified, and archived. The mechanical move of the MCP runtime from `backend/internal/infrastructure/mcp/` to `backend/internal/mcpkit/` is preserved in git history (the two `refactor(mcp): …` work-unit commits). The main spec is in place. No CRITICAL or open items remain. Ready for the next change.

---

## 7. Mechanical Copy Evidence

### Step 2 — Spec sync (delta → main, main did not exist)

```
$ diff -r openspec/changes/mcpkit/specs/mcpkit/spec.md $temp_path
(empty)
```

Verbatim output: empty. Pass.

### Step 3 — Move to archive

```
$ diff -r $snapshot_root/source openspec/changes/archive/2026-10-08-mcpkit/
(empty)
```

Verbatim output: empty. Pass.

Both readbacks empty = byte-identity confirmed by shell; no model-mediated Read→Write byte routing occurred.

---

## 8. Snapshot Cross-Reference to Intermediate Sources

- `apply-progress.md` (2026-10-08, by the apply agent): accurate at the time it was written. Two stale "open" claims (tasks 5.9 and 5.10) were reconciled in commits `d615a9d` and `95eaf60` before this archive. Final state is authoritative.
- `verify-report.md` (PASS WITH WARNINGS, 2026-10-08): all 3 warnings (W1 untracked artifacts, W2 working-tree noise, W3 sandbox gofumpt drift) were either resolved before archive (W1 → `d615a9d`; W2/W3 out of this archive's scope by the orchestrator's launch prompt). Final state: 0 CRITICAL, 0 DEBT, 8/8 PASS.
- `proposal.md` and `design.md`: stable; final state matches.

No contradiction among sources at archive time.