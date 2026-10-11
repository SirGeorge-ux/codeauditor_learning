// code-health.use-case.ts — application-layer orchestrator for the S4
// code-health dashboard (PR 3 of `code-health-dashboard`).
//
// Responsibilities (per openspec/changes/code-health-dashboard/design.md
// §CodeHealthUseCase + specs/code-health/spec.md + REQ-CODE-HEALTH-003/011):
//   1. Own the dashboard's state as Angular signals (`analysis`,
//      `loading`, `error`, `remainingToday` + computed
//      `topIssues`/`languages`).
//   2. Submit a new analysis via `mcp-code-health.analyze_repo`,
//      then poll `mcp-code-health.get_analysis` with exponential
//      backoff (2s → 30s) until `completed | failed | max-poll cap`.
//   3. Enforce the per-user soft cap of 5 analyses per UTC day,
//      tracked client-side in localStorage under
//      `codeHealth.dailyCount.<UTC-date>`. The count auto-resets at
//      UTC midnight because the key embeds the date.
//   4. Expose convenience methods for the page + table components:
//      `loadRepoIssues`, `generateHealthReport`, `suppressIssue`.
//
// Hexagonal contract: this file imports only the domain port, the
// Angular signal primitive, and `firstValueFrom` (for the
// Observable-typed MCP port). Zero HTTP / fetch / DOM references.
// `Storage` / `clock` / `sleep` are injected for testability;
// production code uses the safe defaults.
import { computed, signal } from '@angular/core';

import {
  AnalyzeRepoArgs,
  CodeHealthPort,
  GenerateHealthReportArgs,
  GetRepoIssuesArgs,
  SuppressIssueArgs,
} from '../domain/ports/code-health.port';
import { CodeHealthAnalysis, CodeHealthIssue } from '../domain/models/code-health';
import { MCPClientPort } from '../domain/ports/mcp-client.port';

// Tool-name constants — single source of truth in the application
// layer (mirrors practice.use-case.ts: TOOL_* exports + payload keys).
export const TOOL_ANALYZE_REPO = 'mcp-code-health.analyze_repo';
export const TOOL_GET_ANALYSIS = 'mcp-code-health.get_analysis';

// Daily-cap constant + localStorage key prefix. The full key is
// `codeHealth.dailyCount.<UTC-date>` (e.g. `codeHealth.dailyCount.2026-10-10`).
// Mirrors the spec REQ-CODE-HEALTH-011 scenario "Per-user 5 analyses
// / UTC day".
export const CODE_HEALTH_DAILY_CAP = 5;
export const DAILY_COUNT_KEY_PREFIX = 'codeHealth.dailyCount.';

// Polling parameters — design.md §Async job lifecycle + REQ-CODE-HEALTH-003.
// Initial delay 2s, doubles up to 30s cap, max 60 polls (~5min ceiling).
const POLL_INITIAL_MS = 2000;
const POLL_MAX_MS = 30000;
const POLL_MAX_ITERATIONS = 60;

/**
 * CodeHealthUseCaseOptions lets the constructor wire real adapters in
 * production and fakes in tests. `storage`, `now`, `sleep`, and
 * `dailyCap` have safe defaults so the production injection
 * (infrastructure/services/code-health.service.ts) can omit them.
 */
export interface CodeHealthUseCaseOptions {
  readonly port: CodeHealthPort;
  readonly storage?: Storage;
  readonly now?: () => Date;
  readonly sleep?: (ms: number) => Promise<void>;
  readonly dailyCap?: number;
}

/**
 * CodeHealthUseCase is the single source of truth for the dashboard
 * UI. Components read its readonly signals; the only mutation path is
 * the public commands (`submitAnalysis`, `suppressIssue`, etc.).
 */
export class CodeHealthUseCase {
  // Internal mutable signals (private so nothing mutates them from
  // outside the class).
  private readonly _analysis = signal<CodeHealthAnalysis | null>(null);
  private readonly _loading = signal<boolean>(false);
  private readonly _error = signal<string | null>(null);
  private readonly _dailyCount = signal<number>(0);

  // Resolved wiring.
  private readonly port: CodeHealthPort;
  private readonly storage: Storage;
  private readonly now: () => Date;
  private readonly sleep: (ms: number) => Promise<void>;
  private readonly dailyCap: number;
  // Tracks the in-flight analysis so the user can cancel (not yet
  // surfaced in the UI; the polling loop also respects a "stopped"
  // flag for component teardown).
  private currentAnalysisId: string | null = null;
  private stopped = false;

  // Public readonly signals — components bind to these.
  readonly analysis = this._analysis.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();
  readonly remainingToday = computed<number>(() => Math.max(0, this.dailyCap - this._dailyCount()));
  readonly topIssues = computed<CodeHealthIssue[]>(() => {
    const a = this._analysis();
    return a ? [...a.top_issues] : [];
  });
  readonly languages = computed<Readonly<Record<string, number>>>(() => {
    const a = this._analysis();
    return a ? a.languages : {};
  });

  constructor(opts: CodeHealthUseCaseOptions) {
    this.port = opts.port;
    this.storage = opts.storage ?? defaultStorage();
    this.now = opts.now ?? (() => new Date());
    this.sleep = opts.sleep ?? defaultSleep();
    this.dailyCap = opts.dailyCap ?? CODE_HEALTH_DAILY_CAP;
    this._dailyCount.set(this.readPersistedCount());
  }

  // -- Submit ----------------------------------------------------------------

  /**
   * submitAnalysis enforces the daily cap, invokes
   * `mcp-code-health.analyze_repo`, then polls `get_analysis` with
   * exponential backoff until `completed` | `failed` | the 60-poll
   * ceiling. Returns the final analysis (or null if the cap was
   * exceeded at the boundary).
   */
  async submitAnalysis(args: AnalyzeRepoArgs): Promise<CodeHealthAnalysis | null> {
    if (this.remainingToday() <= 0) {
      this._error.set('Daily limit reached, resets at 00:00 UTC. Try again after midnight.');
      return null;
    }

    this._loading.set(true);
    this._error.set(null);
    this.stopped = false;

    let analysisId: string;
    try {
      const submit = await this.port.analyzeRepo(args);
      analysisId = submit.analysis_id;
    } catch (e) {
      this._loading.set(false);
      this._error.set(this.errMsg(e, 'Could not submit analysis.'));
      return null;
    }

    this.currentAnalysisId = analysisId;

    const finalAnalysis = await this.pollUntilTerminal(analysisId);
    this._loading.set(false);

    if (finalAnalysis !== null && finalAnalysis.status === 'completed') {
      // Burn a daily slot only on successful completion — a failed
      // analysis shouldn't cost the user from the budget (same
      // policy as PracticeUseCase.generateChallenge).
      this.bumpDailyCount();
      return finalAnalysis;
    }
    if (finalAnalysis === null) {
      // poll cap reached — surface a friendly message.
      this._error.set('Analysis did not complete in time; try again or check the server.');
    } else if (finalAnalysis.status === 'failed') {
      this._error.set(finalAnalysis.error_message ?? 'Analysis failed; review and retry.');
    }
    return finalAnalysis;
  }

  /**
   * pollAnalysis lets the page re-issue a poll for a known analysis_id
   * (e.g. on a manual retry). It does NOT bump the daily count.
   */
  async pollAnalysis(analysisId: string): Promise<CodeHealthAnalysis | null> {
    this._loading.set(true);
    this._error.set(null);
    this.currentAnalysisId = analysisId;
    this.stopped = false;
    const a = await this.pollUntilTerminal(analysisId);
    this._loading.set(false);
    return a;
  }

  // -- Polling core ----------------------------------------------------------

  private async pollUntilTerminal(analysisId: string): Promise<CodeHealthAnalysis | null> {
    let backoffMs = POLL_INITIAL_MS;
    let last: CodeHealthAnalysis | null = null;
    let exhausted = false;
    for (let i = 0; i < POLL_MAX_ITERATIONS; i += 1) {
      if (this.stopped) {
        return last;
      }
      try {
        const a = await this.port.getAnalysis(analysisId);
        last = a;
        this._analysis.set(a);
        if (a.status === 'completed' || a.status === 'failed') {
          return a;
        }
      } catch (e) {
        this._error.set(this.errMsg(e, 'Polling the analysis failed.'));
        return last;
      }
      // Only sleep if we're going to iterate again.
      if (i < POLL_MAX_ITERATIONS - 1) {
        await this.sleep(backoffMs);
        backoffMs = Math.min(backoffMs * 2, POLL_MAX_MS);
      } else {
        exhausted = true;
      }
    }
    if (exhausted) {
      this._error.set(
        `Analysis did not complete after ${POLL_MAX_ITERATIONS} polls; check the server or retry.`,
      );
    }
    return last;
  }

  // -- Repo-issue + report + suppress helpers --------------------------------

  async loadRepoIssues(args: GetRepoIssuesArgs): Promise<CodeHealthIssue[]> {
    const out = await this.port.getRepoIssues(args);
    return [...out.issues];
  }

  async generateHealthReport(args: GenerateHealthReportArgs): Promise<string | null> {
    try {
      const out = await this.port.generateHealthReport(args);
      return out.content ?? null;
    } catch (e) {
      this._error.set(this.errMsg(e, 'Report generation failed.'));
      return null;
    }
  }

  async suppressIssue(args: SuppressIssueArgs): Promise<boolean> {
    try {
      await this.port.suppressIssue(args);
      return true;
    } catch (e) {
      this._error.set(this.errMsg(e, 'Suppression failed.'));
      return false;
    }
  }

  clearError(): void {
    this._error.set(null);
  }

  /**
   * stopPolling cancels the in-flight loop. The page calls this in
   * `ngOnDestroy` so a navigation doesn't leak the polling closure.
   */
  stopPolling(): void {
    this.stopped = true;
  }

  // -- Daily-cap bookkeeping --------------------------------------------------

  private bumpDailyCount(): void {
    const next = this._dailyCount() + 1;
    this._dailyCount.set(next);
    const key = this.dailyCountKey(this.now());
    try {
      this.storage.setItem(key, String(next));
    } catch {
      // localStorage may be unavailable (private mode, quota). The
      // in-memory count still drives the current session — we just
      // lose the cross-session persistence (mirrors PracticeUseCase).
    }
  }

  private readPersistedCount(): number {
    const key = this.dailyCountKey(this.now());
    const raw = this.safeGet(key);
    if (raw === null) return 0;
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0;
  }

  dailyCountKey(date: Date): string {
    return DAILY_COUNT_KEY_PREFIX + utcDateString(date);
  }

  private safeGet(key: string): string | null {
    try {
      return this.storage.getItem(key);
    } catch {
      return null;
    }
  }

  private errMsg(e: unknown, fallback: string): string {
    if (e instanceof Error && e.message.length > 0) return e.message;
    return fallback;
  }
}

// utcDateString formats a Date as YYYY-MM-DD in UTC. Same helper the
// PracticeUseCase exports, duplicated here so each use case stands
// alone (the application layer forbids cross-use-case imports).
export function utcDateString(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// defaultStorage returns window.localStorage when available, otherwise
// a no-op stub (mirrors PracticeUseCase so the daily cap survives
// in non-DOM test runners).
function defaultStorage(): Storage {
  if (typeof window !== 'undefined' && typeof window.localStorage !== 'undefined') {
    return window.localStorage;
  }
  const noop = (): void => undefined;
  return {
    get length() {
      return 0;
    },
    clear: noop,
    getItem: () => null,
    key: () => null,
    removeItem: noop,
    setItem: noop,
  };
}

// defaultSleep returns a real setTimeout-based sleep. Tests inject a
// no-wait stub so the polling tests don't block.
function defaultSleep(): (ms: number) => Promise<void> {
  return (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));
}

// DailyCapExceededException is reserved for the future S5 server-side
// cap; today the use case surfaces a friendly message via the error
// signal and returns null. The exported class makes the shape
// discoverable for tests + future wiring.
export class DailyCapExceededException extends Error {
  constructor(message: string = 'Daily limit reached.') {
    super(message);
    this.name = 'DailyCapExceededException';
  }
}

// Re-export so the use case appears as a single source of truth for
// the application layer; the page reads MCPClientPort only via the
// injected repository.
export type { MCPClientPort };
