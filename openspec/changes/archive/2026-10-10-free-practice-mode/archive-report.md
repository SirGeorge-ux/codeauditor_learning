# Archive Report: free-practice-mode

> **Status:** SUCCESS — clean archive, no reconciliation needed
> **Archive date:** 2026-10-10
> **Archived to:** `openspec/changes/archive/2026-10-10-free-practice-mode/`
> **Canonical spec:** `openspec/specs/free-practice/spec.md` (byte-identical to the delta, verified via `diff -r`)
> **Closed at commit:** `cc5e787` (master HEAD)

## Change Summary

`free-practice-mode` delivers the S3 Free Practice Mode capability: an on-demand
challenge generator at `/practice/free` where the user picks a language, a topic from
that language's curriculum, and an F–S level; the backend orchestrates the S2 MCP tools
(`mcp-curriculum`, `mcp-pedagogical`, `mcp-challenges`) plus the S2 LLM cascade into a
validated v2 `Challenge`, persists it with `origin="generated"`, and opens it in the
shared dojo flow. The two hard blockers resolved here are (1) un-stubbing
`ChallengeService.GenerateChallenge` and (2) adding a structured-JSON `Complete` method
to `LLMClientPort` (today chat-only `StreamChat`).

**PRs landed (stacked-to-main, 4 PRs total):**

| PR | Focus | Merged at | Notes |
|----|-------|-----------|-------|
| 1 | Backend cascade `LLMClientPort.Complete` | `e41ae23` | Additive non-streaming JSON-mode path; `StreamChat` untouched; 5s whole-response timeout; Groq `response_format` / DeepSeek OpenRouter-compat / MiniMax text+parser; provider fallthrough |
| 2 | Backend un-stub + curricula + `main.go` wiring | `a0cc492` | `GenerateChallenge` orchestration (topic → objective → story → LLM → parse → validate → sandbox gate → insert); `GenerateChallengeHint` extended with `TopicID`+`Level`; 4 nil-safe option funcs; `CURRICULA_BASE_DIR` env override; `.atl/curricula/typescript.md` seed; extended `mcp-challenges.generate_challenge` wire shape (`topicId`,`level`) |
| 3 | Frontend (domain + use case + 4 components + page) | `4b0b1a7` | `domain/models/curriculum.ts`; `application/practice.use-case.ts` (signals + daily cap + localStorage); 4 dumb standalone components; `practice-page` STUB replaced; canonical `/practice/free` route |
| 4 | Sandbox gate executes `testCases` + auth isolation (Phase 5) | `cc5e787` | `SandboxExecutor.ExecuteWithTests` + `TestCase`/`TestRunResult`/`TestOutcome`; Docker+Local drivers (TypeScript driver, other languages degrade cleanly); `GenerateChallenge` now runs `solutionCode`+`testCases` together (≤3 retry loop); `MCP_TOKEN_PROVIDER` TestBed stub isolates the practice specs from the `@supabase/supabase-js` chain |

**Tasks completed:** all of 1.1–1.10, 2.1–2.14, 3.1–3.9, 5.1–5.15. The Phase 4
verification tasks (4.1–4.13) are by design N/A from this change: automated gates
4.1–4.11 are green, and the manual smokes 4.12–4.13 are deferred to the user (they
require a live backend + LLM + browser). Tasks 5.16 (push + `--no-ff` merge) and 5.17
(post-merge re-verify) are the delivery steps that landed as PR 4's merge
(`cc5e787`). All implementation tasks (Phases 1, 2, 3, 5) are checked in the archived
`tasks.md`; the unchecked entries are verification/delivery gates only, so the Task
Completion Gate passes with no exceptional reconciliation.

## Spec Reconciliation Status

**NONE NEEDED** per the orchestrator's final-state fact: no existing capability spec is
modified; `free-practice` is a NEW capability, and the delta spec IS its full canonical
spec. The delta was synced to the canonical location via a mechanical `cp` + `diff -r` +
`mv` (never Read→Write), and the archive retains its own byte-identical copy (precedent:
`2026-10-10-tutor-chat`, `2026-10-10-mcp-monolith`).

**Delta-target discrepancy (documented, NOT silently resolved).** The delta spec's own
header declares it "lands at" TWO canonical targets — `openspec/specs/free-practice/spec.md`
(new) **and** `openspec/specs/challenges/spec.md` (modified) — and its `## MODIFIED
Requirements` section carries three requirements (`LLMClientPort.Complete Method`,
`ChallengeService.GenerateChallenge Is Wired`, `CURRICULA_BASE_DIR Environment Override`)
whose diff annotations point at `openspec/specs/challenges/spec.md` and
`openspec/specs/tutor/spec.md`. The canonical `openspec/specs/challenges/spec.md` exists
(21,322 bytes) and was **NOT** composed:

- Source A — spec artifact (authored 2026-10-10, in-repo): declares `challenges` is a
  MODIFIED capability and that the delta lands at `openspec/specs/challenges/spec.md`.
- Source B — orchestrator launch prompt (most recent final-state account): "Spec
  reconciliation: none needed (no modification to existing capability specs;
  `free-practice` is a new capability)".

The two statements disagree on whether `openspec/specs/challenges/spec.md` should have
been updated. Per the SDD Final-State Authority rule (unrankable contradictions are
recorded, never silently resolved), this archive records both and follows the
orchestrator's explicit mechanical instruction: the delta was synced VERBATIM to
`openspec/specs/free-practice/spec.md`, and `openspec/specs/challenges/spec.md` /
`openspec/specs/tutor/spec.md` were left unmodified. This is also consistent with the
directory-based spec routing (the only delta dir is `specs/free-practice/`). **Follow-up:**
a human/orchestrator should confirm whether the three `MODIFIED Requirements` sections
belong in the canonical `challenges`/`tutor` specs, and if so, compose them in a follow-up
chore.

## Mechanical Copy Audit

| Operation | Method | Readback |
|-----------|--------|----------|
| Delta spec → canonical | `cp` + `diff -r` + `mv` to `openspec/specs/free-practice/spec.md` | `diff -r` delta vs canonical: **empty** (byte-identical) |
| Change folder → archive | `git mv openspec/changes/free-practice-mode openspec/changes/archive/2026-10-10-free-practice-mode` | `diff -r` pre-move snapshot vs destination: **empty** (byte-identical) |

All copies/moves were native shell operations; zero file content passed through the
model's Read/Write path. The archive retains `proposal.md`, `design.md`, `tasks.md`,
`specs/free-practice/spec.md` (byte-identical to source) plus this additive
`archive-report.md` (excluded from the source/destination comparison).

## Final-State Verification (post-merge, precedence-ranked)

The `verify-report` (#813) was captured on master @ `4b0b1a7` — i.e. **before** PR 4 — and
therefore reports intermediate state. PR 4 (`cc5e787`) is the later, authoritative
account; where the two disagree the PR 4 outcome governs:

- **CRITICAL drift in `verify-report` #813 — RESOLVED by PR 4.** #813 flagged that the
  sandbox gate ran `solutionCode` WITHOUT `testCases` (spec "Solution-Validates Sandbox
  Gate" drift). PR 4 added `SandboxExecutor.ExecuteWithTests` and wired
  `ChallengeService.GenerateChallenge` to run `solutionCode` + `testCases` together with
  the ≤3 retry loop. **Final state: aligned with the spec.**
- **WARNING flake in `verify-report` #813 — MITIGATED by PR 4.** #813 saw 6
  `auth.service.spec.ts` failures at full parallel scale (practice specs transitively
  importing the real `AuthService` → `@supabase/supabase-js`). PR 4 isolated the practice
  specs via a `MCP_TOKEN_PROVIDER` TestBed stub; `ng test` ×3 → 29/29 files, 253/253
  tests, 0 FAIL. A pre-existing latent flake in `auth.service.spec.ts` remains as known
  debt (see follow-ups).

## Hexagonal Purity Check (final)

- `backend/internal/core/`: **0** `database/sql` imports (verified via quoted grep; a
  pre-existing doc comment match in `dictionary_service.go:29` is not an import).
- `frontend/.../domain/`: **0** `@angular/*` imports.

## Quality Gates (final, post-merge @ `cc5e787`)

All green per the PR 4 outcome (apply-progress #804):

- Backend: `go test ./...` (24 packages PASS), `gofumpt -l .` (empty), `golangci-lint run`
  (0 findings), `go vet ./...` (clean), `go build ./...` (clean).
- Frontend: `pnpm exec prettier --check` (clean), `pnpm exec eslint src/` (clean),
  `npx ng test --watch=false` ×3 (29/29 files, 253/253 tests, 0 FAIL each run),
  `npx ng build` (clean).
- One latent test-isolation flake in `auth.service.spec.ts` (pre-existing file,
  2026-06-14) passes in isolation (10/10) and pairwise but can fail at full parallel
  scale; documented as known debt.

## Risks / Follow-ups (carried forward)

1. **`mcp-curriculum.list_languages`** — replace the hardcoded `SUPPORTED_LANGUAGES` array
   in `practice-page.component.ts` with a curriculum-driven language list.
2. **Daily-cap key tenancy** — when multi-tenant arrives, prefix the daily-cap
   `localStorage` key (`practice.dailyCount.<UTC-date>`) with `userId`.
3. **`auth.service.spec.ts` × practice specs test-isolation flake** — investigate
   `vi.mock` behavior vs `@angular/build:unit-test`'s `vitest-mock-patch` (relative-import
   `vi.mock` is rejected; TestBed `provide:` is the working mechanism for module-level
   dependency mocking in Angular specs).
4. **Per-language test drivers** — only TypeScript is wired in PR 4 (`tsrunner.go`,
   `--experimental-strip-types`, requires Node 22+); go/python/ruby/php/lua/bash/perl
   return degraded mode (`TestsRun:0`, sentinel). Real drivers are a follow-up PR.
5. **Manual smoke deferred to user** — Phase 4 tasks 4.12 (`/practice/free` end-to-end
   generate → dojo → sandbox) and 4.13 (daily-cap UI rejection).

Additional documented discrepancies (non-blocking): the spec scenario "Curriculum-Driven
Topic Catalog" says "12 topics" while `.atl/curricula/typescript.md` ships 15; the task
context once referenced `specs/practice/spec.md` while the actual path is
`specs/free-practice/spec.md`.

## Engram Observations Read (traceability)

- `#813` — `sdd/free-practice-mode/verify-report` (intermediate: master @ `4b0b1a7`, pre-PR 4)
- `#804` — `sdd/free-practice-mode/apply-progress` (PR 4 LANDED; supersedes #813 for final state)
- `#808` — `free-practice-mode PR 2: orchestration + curriculum seed + cascade sharing`
- `#806` — `Pre-existing master debt (validate RED, comment match)`
- `#805` — `S3 PR 1: LLMClientPort.Complete contract decision`
- `#803` — `sdd/free-practice-mode/design`
- `#802` — `Free practice mode proposal (S3)`

## Git Record

- Archive commit: `chore(openspec): archive free-practice-mode change (S3 closed)` —
  canonical spec `openspec/specs/free-practice/spec.md`, archive folder move via `git mv`,
  and this `archive-report.md`.
- AGENTS.md commit: `docs(agents): mark free-practice-mode as archived + S3 done (snapshot 2026-10-10)`.
- Chain merges retained in history: `e41ae23` (PR 1), `a0cc492` (PR 2), `4b0b1a7` (PR 3),
  `cc5e787` (PR 4).
- Working tree clean after both commits.

The SDD cycle for `free-practice-mode` is COMPLETE: planned, implemented, verified, and
archived. S3 (Free Practice Mode) is now feature-complete AND archive-complete.
