// practice.use-case.ts — application-layer orchestrator for the Free
// Practice Mode feature.
//
// Responsibilities (per openspec/changes/free-practice-mode/design.md
// §PracticeUseCase + specs/free-practice/spec.md):
//   1. Own the UI state for the selector trio
//      (language / topic / F..S level) as Angular signals.
//   2. Fetch the topic catalog for a language via
//      `mcp-curriculum.get_topics` (consumed by the topic-list
//      component).
//   3. Trigger on-demand challenge generation through
//      `mcp-challenges.generate_challenge` (the S3 wire shape).
//   4. Enforce the per-user soft cap of 20 generations per UTC day.
//      The count is persisted in localStorage under
//      `practice.dailyCount.<UTC-date>` so a page reload survives
//      across navigation. Counter resets at UTC midnight because
//      the key includes the UTC date — yesterday's key is simply
//      ignored.
//   5. Surface the most recent generated Challenge so the page can
//      render the dojo summary + open the dojo route.
//
// Hexagonal contract: this file imports only the domain port
// interfaces, the Angular `signal` primitive, and RxJS
// (`firstValueFrom`). Zero HTTP / fetch / DOM references. The
// `Storage` and clock are injected for testability; production code
// uses the defaults (window.localStorage + `() => new Date()`).
import { computed, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import { Challenge } from '../domain/models/challenge';
import {
  CurriculumTopic,
  F_S_LEVELS,
  GenerateChallengeArgs,
  isF_SLevel,
} from '../domain/models/curriculum';
import { MCPClientPort } from '../domain/ports/mcp-client.port';

// MCP tool names — the only place in the frontend that hard-codes
// them. Keep these in lockstep with
// backend/internal/infrastructure/mcp/challenges/challenges_tool.go
// and backend/internal/infrastructure/mcp/curriculum/curriculum_tool.go.
export const TOOL_GENERATE_CHALLENGE = 'mcp-challenges.generate_challenge';
export const TOOL_GET_TOPICS = 'mcp-curriculum.get_topics';

// Daily cap per UTC day. Mirrors the spec scenario "Per-User Daily
// Generation Cap" + AGENTS.md §0 gamification rules.
export const PRACTICE_DAILY_CAP = 20;

// localStorage key prefix. The full key is
// `practice.dailyCount.<UTC-date>` (e.g. `practice.dailyCount.2026-10-10`).
// Keeping the prefix exported lets tests assert on the key shape.
export const DAILY_COUNT_KEY_PREFIX = 'practice.dailyCount.';

// PracticeUseCaseOptions lets the constructor wire real adapters in
// production and fakes in tests. `storage` defaults to
// window.localStorage; `now` defaults to `() => new Date()`.
//
// Note: the spec scenario "Per-User Daily Generation Cap" is satisfied
// by the single-user app model (AGENTS.md §0 — `ggogsmic`). When the
// app moves to multi-tenant, prefix the localStorage key with the
// user id; the use case will need to receive an identity provider.
export interface PracticeUseCaseOptions {
  readonly mcp: MCPClientPort;
  readonly storage?: Storage;
  readonly now?: () => Date;
}

/**
 * PracticeUseCase is the single source of truth for the free-practice
 * UI. Components inject it through an Angular Injectable wrapper and
 * read its readonly signals; no other layer mutates state.
 */
export class PracticeUseCase {
  // Selector trio (private setters, public readonly views).
  private readonly _language = signal<string>('');
  private readonly _topic = signal<string>('');
  private readonly _level = signal<string>('F');

  // Topics catalog for the current language (loaded via
  // `mcp-curriculum.get_topics`).
  private readonly _topics = signal<CurriculumTopic[]>([]);

  // Lifecycle + UX signals.
  private readonly _loading = signal<boolean>(false);
  private readonly _error = signal<string | null>(null);

  // Latest generated challenge. Null until generateChallenge()
  // succeeds. The page renders the summary + "Open in dojo" button
  // when this is non-null.
  private readonly _generatedChallenge = signal<Challenge | null>(null);

  // Daily-count bookkeeping. We read it once at construction
  // (the user may have already exhausted today's budget before the
  // page mounted) and update it on every successful generation.
  private readonly _dailyCount = signal<number>(0);

  // Resolved wiring (defaults applied).
  private readonly storage: Storage;
  private readonly now: () => Date;

  // Public readonly signals — components bind to these via service
  // wrappers or computed views.
  readonly language = this._language.asReadonly();
  readonly topic = this._topic.asReadonly();
  readonly level = this._level.asReadonly();
  readonly topics = this._topics.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();
  readonly generatedChallenge = this._generatedChallenge.asReadonly();
  readonly dailyCount = this._dailyCount.asReadonly();
  readonly remainingToday = computed<number>(() =>
    Math.max(0, PRACTICE_DAILY_CAP - this._dailyCount()),
  );
  readonly canGenerate = computed<boolean>(
    () =>
      !this._loading() &&
      this._language().length > 0 &&
      this._topic().length > 0 &&
      isF_SLevel(this._level()) &&
      this.remainingToday() > 0,
  );

  // Parameter properties wire the required deps (mcp, auth) at the
  // constructor site. Storage + clock have safe defaults so the
  // production injection (infrastructure/practice.service.ts) can
  // omit them.
  constructor(private readonly opts: PracticeUseCaseOptions) {
    this.storage = opts.storage ?? defaultStorage();
    this.now = opts.now ?? (() => new Date());
    this._dailyCount.set(this.readPersistedCount());
  }

  // -- Selectors --------------------------------------------------------------

  setLanguage(value: string): void {
    this._language.set(value);
    // Reset topic when language changes — the catalog is per-language.
    this._topic.set('');
    this._topics.set([]);
  }

  setTopic(value: string): void {
    this._topic.set(value);
  }

  setLevel(value: string): void {
    if (isF_SLevel(value)) {
      this._level.set(value);
    }
  }

  // -- Topics ----------------------------------------------------------------

  // loadTopics fetches the topic catalog for a language. It runs once
  // per language change; the page can call it again to force-refresh.
  async loadTopics(language: string): Promise<void> {
    if (language.length === 0) {
      this._topics.set([]);
      return;
    }
    try {
      const topics = await firstValueFrom(
        this.opts.mcp.callTool<CurriculumTopic[]>(TOOL_GET_TOPICS, { language }),
      );
      this._topics.set(Array.isArray(topics) ? topics : []);
    } catch (e) {
      this.setError(this.errMsg(e, 'Could not load topics.'));
      this._topics.set([]);
    }
  }

  // -- generateChallenge -----------------------------------------------------

  async generateChallenge(): Promise<Challenge | null> {
    if (this._loading()) {
      return null;
    }
    const args = this.validateArgs();
    if (args === null) {
      return null;
    }
    if (this.remainingToday() <= 0) {
      this.setError('Daily limit reached. Try again after UTC midnight.');
      return null;
    }

    this._loading.set(true);
    this.setError(null);

    try {
      const challenge = await firstValueFrom(
        this.opts.mcp.callTool<Challenge>(TOOL_GENERATE_CHALLENGE, args),
      );
      this._generatedChallenge.set(challenge);
      // Burn one daily slot only on success — a server error shouldn't
      // cost the user a generation from the budget.
      this.bumpDailyCount();
      return challenge;
    } catch (e) {
      this.setError(this.errMsg(e, 'Challenge generation failed.'));
      this._generatedChallenge.set(null);
      return null;
    } finally {
      this._loading.set(false);
    }
  }

  // -- Helpers ---------------------------------------------------------------

  clearError(): void {
    this._error.set(null);
  }

  resetGeneratedChallenge(): void {
    this._generatedChallenge.set(null);
  }

  // The F..S band is part of the public surface so the level-selector
  // component can render the same ordered list.
  static readonly F_S_LEVELS = F_S_LEVELS;

  // -- Internals -------------------------------------------------------------

  // validateArgs returns the wire args or sets the error signal +
  // returns null. Centralizes the input checks so the happy-path body
  // of generateChallenge stays linear.
  private validateArgs(): GenerateChallengeArgs | null {
    const language = this._language().trim();
    const topic = this._topic().trim();
    const level = this._level();

    if (language.length === 0) {
      this.setError('Pick a language before generating a challenge.');
      return null;
    }
    if (topic.length === 0) {
      this.setError('Pick a topic before generating a challenge.');
      return null;
    }
    if (!isF_SLevel(level)) {
      this.setError(`Level "${level}" is not in the F..S band.`);
      return null;
    }
    return { language, topicId: topic, level };
  }

  // bumpDailyCount increments the in-memory signal + persists the new
  // count under today's UTC-date key. Reading happens lazily on
  // construction; writes happen immediately so a mid-session reload
  // preserves the running total.
  private bumpDailyCount(): void {
    const next = this._dailyCount() + 1;
    this._dailyCount.set(next);
    const key = this.dailyCountKey(this.now());
    try {
      this.storage.setItem(key, String(next));
    } catch {
      // localStorage may be unavailable (private mode, quota). The
      // in-memory count still drives the current session — we just
      // lose the cross-session persistence. The cap survives for the
      // remainder of the day in-tab.
    }
  }

  // readPersistedCount reads today's count from localStorage. A
  // missing or malformed value yields 0.
  private readPersistedCount(): number {
    const key = this.dailyCountKey(this.now());
    const raw = this.safeGet(key);
    if (raw === null) return 0;
    const parsed = Number.parseInt(raw, 10);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0;
  }

  // dailyCountKey is the storage key for today's count. Pinning the
  // UTC-date makes the counter auto-reset at UTC midnight without any
  // timer or event subscription — yesterday's key is irrelevant by
  // definition.
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

  private setError(msg: string | null): void {
    this._error.set(msg);
  }

  private errMsg(e: unknown, fallback: string): string {
    if (e instanceof Error && e.message.length > 0) return e.message;
    return fallback;
  }
}

// utcDateString formats a Date as YYYY-MM-DD in UTC. Used by both
// the use case (to build the daily-count key) and the spec tests
// (to assert the key shape).
export function utcDateString(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// defaultStorage returns window.localStorage when available, otherwise
// a no-op stub. The no-op keeps the use case functional in non-DOM
// test runners; the in-memory signal still drives the daily cap
// within a single session.
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
