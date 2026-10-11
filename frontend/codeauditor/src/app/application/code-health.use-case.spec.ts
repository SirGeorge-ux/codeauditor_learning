// code-health.use-case.spec.ts — application-layer tests for the
// CodeHealthUseCase (S4 — code-health-dashboard PR 3).
//
// Tests cover (per design.md §CodeHealthUseCase + REQ-CODE-HEALTH-003
// async lifecycle + REQ-CODE-HEALTH-011 daily cap):
//   - Signal transitions (analysis, loading, error, remainingToday).
//   - submitAnalysis(...) → analyzeRepo + poll loop with backoff 2s→30s
//     up to 60 polls (~5min cap).
//   - Daily-cap enforcement: 6th submission in a UTC day is rejected
//     without calling the port; error signal carries the daily-cap
//     message; counter stays at 5.
//   - localStorage persistence + UTC-midnight reset.
//   - Error propagation through the polling loop (failed → no further
//     polls).
//
// Test doubles:
//   - FakePort: in-memory CodeHealthPort with scripted responses per
//     analysis_id; lets the test advance the analysis lifecycle by
//     mutating the script between calls.
//   - FakeStorage: Map-backed localStorage stub.
//   - sleep: a no-wait clock so the polling tests don't block.
import { describe, it, expect, beforeEach, vi } from 'vitest';

import { CodeHealthUseCase } from './code-health.use-case';
import {
  AnalyzeRepoArgs,
  CodeHealthPort,
  GenerateHealthReportResult,
  GetAnalysisResult,
  GetRepoIssuesResult,
  SuppressIssueResult,
} from '../domain/ports/code-health.port';
import {
  CodeHealthAnalysis,
  CodeHealthIssue,
} from '../domain/models/code-health';

// -- Test doubles ------------------------------------------------------------

interface FakeScript {
  // Each `analyzeRepo` call returns the analysis_id here, then the
  // polling loop reads the same id off `getAnalysis` and advances
  // through these statuses until 'completed' or 'failed'.
  analysisId: string;
  // Status sequence the script returns per getAnalysis poll.
  getAnalysisSequence: readonly Partial<CodeHealthAnalysis>[];
  // getAnalysis invariant overrides (id, owner, name, etc.).
  fixedAnalysis: Partial<CodeHealthAnalysis>;
  // index pointer advanced each call.
  step: number;
}

class FakeStorage implements Storage {
  private readonly map = new Map<string, string>();
  get length(): number {
    return this.map.size;
  }
  clear(): void {
    this.map.clear();
  }
  key(index: number): string | null {
    return Array.from(this.map.keys())[index] ?? null;
  }
  getItem(key: string): string | null {
    return this.map.get(key) ?? null;
  }
  setItem(key: string, value: string): void {
    this.map.set(key, value);
  }
  removeItem(key: string): void {
    this.map.delete(key);
  }
  raw(): Record<string, string> {
    return Object.fromEntries(this.map);
  }
}

class FakePort implements CodeHealthPort {
  // Per-test scripted responses.
  get analysisScript(): FakeScript {
    if (this._script === null) {
      throw new Error('No script set; tests should assign port.script(...)');
    }
    return this._script;
  }
  private _script: FakeScript | null = null;

  script(s: FakeScript): void {
    this._script = s;
    s.step = 0;
  }

  // Track method calls for assertions.
  analyzeRepoCalls = 0;
  getAnalysisCalls = 0;
  getRepoIssuesCalls = 0;
  generateHealthReportCalls = 0;
  suppressIssueCalls = 0;
  // Per-call error override.
  analyzeRepoError: Error | null = null;
  getAnalysisError: Error | null = null;

  async analyzeRepo(_args: AnalyzeRepoArgs): Promise<{ analysis_id: string; status: 'pending' }> {
    this.analyzeRepoCalls += 1;
    if (this.analyzeRepoError !== null) {
      throw this.analyzeRepoError;
    }
    return {
      analysis_id: this.analysisScript.analysisId,
      status: 'pending',
    };
  }

  async getAnalysis(_id: string): Promise<GetAnalysisResult> {
    this.getAnalysisCalls += 1;
    if (this.getAnalysisError !== null) {
      throw this.getAnalysisError;
    }
    const script = this.analysisScript;
    const idx = script.step;
    script.step = Math.min(idx + 1, script.getAnalysisSequence.length - 1);
    return makeAnalysis({
      id: script.analysisId,
      ...script.fixedAnalysis,
      ...script.getAnalysisSequence[idx],
    });
  }

  async getRepoIssues(_args: {
    owner: string;
    name: string;
    source: 'gogs' | 'github';
    severity?: 'critical' | 'high' | 'medium' | 'low';
    limit?: number;
  }): Promise<GetRepoIssuesResult> {
    this.getRepoIssuesCalls += 1;
    return { issues: [] };
  }

  async generateHealthReport(_args: {
    analysis_id: string;
    format: 'md' | 'pdf';
  }): Promise<GenerateHealthReportResult> {
    this.generateHealthReportCalls += 1;
    return { content: '# report' };
  }

  async suppressIssue(_args: {
    issue_id: string;
    reason?: string;
  }): Promise<SuppressIssueResult> {
    this.suppressIssueCalls += 1;
    return {
      suppression_id: `s-${_args.issue_id}`,
      suppressed_at: new Date('2026-10-10T12:00:00Z'),
    };
  }
}

function makeAnalysis(partial: Partial<CodeHealthAnalysis>): CodeHealthAnalysis {
  return {
    id: 'a-default',
    owner: 'o',
    name: 'n',
    source: 'gogs',
    status: 'pending',
    health_score: 0,
    top_issues: [],
    languages: {},
    summary: '',
    created_at: new Date('2026-10-10T12:00:00Z'),
    ...partial,
  };
}

function utcDate(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// -- Tests -------------------------------------------------------------------

describe('CodeHealthUseCase', () => {
  let port: FakePort;
  let storage: FakeStorage;
  let sleepMock: ReturnType<typeof vi.fn>;
  let sleep: (ms: number) => Promise<void>;
  let now: () => Date;
  let useCase: CodeHealthUseCase;

  beforeEach(() => {
    port = new FakePort();
    storage = new FakeStorage();
    sleepMock = vi.fn(async () => undefined);
    sleep = sleepMock as unknown as (ms: number) => Promise<void>;
    now = () => new Date('2026-10-10T12:00:00Z');
    useCase = new CodeHealthUseCase({
      port,
      storage,
      now,
      sleep,
      // Default cap matches design.md §Daily cap (5/day).
      dailyCap: 5,
    });
  });

  describe('initial state', () => {
    it('starts with empty analysis + loading=false + 5 remaining today', () => {
      expect(useCase.analysis()).toBeNull();
      expect(useCase.loading()).toBe(false);
      expect(useCase.error()).toBeNull();
      expect(useCase.remainingToday()).toBe(5);
      expect(useCase.topIssues()).toEqual([]);
      expect(useCase.languages()).toEqual({});
    });
  });

  describe('submitAnalysis (happy path)', () => {
    it('completes when the analysis transitions to completed', async () => {
      port.script({
        analysisId: 'a-1',
        getAnalysisSequence: [
          { status: 'running' },
          { status: 'completed', health_score: 78, top_issues: [], languages: { go: 0.6 } },
        ],
        fixedAnalysis: { owner: 'ggogsmic', name: 'academy-mic', source: 'gogs' },
        step: 0,
      });
      const out = await useCase.submitAnalysis({
        owner: 'ggogsmic',
        name: 'academy-mic',
        source: 'gogs',
      });
      expect(out?.status).toBe('completed');
      expect(useCase.analysis()?.health_score).toBe(78);
      expect(useCase.loading()).toBe(false);
      expect(useCase.remainingToday()).toBe(4);
    });

    it('drives topIssues + languages as computed views on analysis', async () => {
      const issue: CodeHealthIssue = {
        id: 'i-1',
        rule_id: 'RULE-SEC-SECRET-001',
        category: 'security',
        severity: 'critical',
        message: 'm',
        file_path: 'x.ts',
        line_number: 1,
        evidence: 'e',
        suggested_fix: 'f',
        confidence: 1.0,
      };
      // Drive a completed analysis through the loop so the
      // computed signals reflect the final state.
      port.script({
        analysisId: 'a-2',
        getAnalysisSequence: [
          {
            status: 'completed',
            health_score: 50,
            top_issues: [issue],
            languages: { ts: 1.0 },
          },
        ],
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      expect(useCase.topIssues()).toEqual([issue]);
      expect(useCase.languages()).toEqual({ ts: 1.0 });
    });

    it('toggles loading signal across the call', async () => {
      port.script({
        analysisId: 'a-3',
        getAnalysisSequence: [{ status: 'completed' }],
        fixedAnalysis: {},
        step: 0,
      });
      const inFlight = useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      expect(useCase.loading()).toBe(true);
      await inFlight;
      expect(useCase.loading()).toBe(false);
    });

    it('persists the daily count under the UTC-date key', async () => {
      port.script({
        analysisId: 'a-4',
        getAnalysisSequence: [{ status: 'completed' }],
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      const key = `codeHealth.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`;
      expect(storage.getItem(key)).toBe('1');
    });
  });

  describe('polling backoff', () => {
    it('sleeps 2s on the first poll, then doubles up to 30s cap', async () => {
      port.script({
        analysisId: 'a-5',
        getAnalysisSequence: [
          { status: 'running' },
          { status: 'running' },
          { status: 'running' },
          { status: 'completed' },
        ],
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      // 3 sleep calls (one per intermediate poll): 2000, 4000, 8000ms.
      expect(sleepMock).toHaveBeenCalledTimes(3);
      expect(sleepMock).toHaveBeenNthCalledWith(1, 2000);
      expect(sleepMock).toHaveBeenNthCalledWith(2, 4000);
      expect(sleepMock).toHaveBeenNthCalledWith(3, 8000);
    });

    it('caps backoff at 30000ms', async () => {
      // 6 polls running + 1 completed = 7 polls, 6 sleeps between.
      const seq: Partial<CodeHealthAnalysis>[] = Array(6).fill({ status: 'running' });
      seq.push({ status: 'completed' });
      port.script({
        analysisId: 'a-6',
        getAnalysisSequence: seq,
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      // 6 sleep calls: 2s, 4s, 8s, 16s, 30s, 30s
      const calls = sleepMock.mock.calls.map((c: unknown[]) => c[0]);
      expect(calls[0]).toBe(2000);
      expect(calls[1]).toBe(4000);
      expect(calls[2]).toBe(8000);
      expect(calls[3]).toBe(16000);
      expect(calls[4]).toBe(30000);
      expect(calls[5]).toBe(30000);
    });
  });

  describe('max-poll cap', () => {
    it('aborts after 60 polls with a poll-cap error message', async () => {
      // 60 polls, all running — never reaches completed.
      const seq = Array(60).fill({ status: 'running' as const });
      // Add a 61st completed so a buggy implementation would resolve.
      seq.push({ status: 'completed' as const });
      port.script({
        analysisId: 'a-7',
        getAnalysisSequence: seq,
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      // getAnalysis called exactly 60 times (max-poll cap).
      expect(port.getAnalysisCalls).toBe(60);
      expect(useCase.analysis()?.status).toBe('running');
      expect(useCase.error()).toMatch(/poll/i);
    });
  });

  describe('failed analysis', () => {
    it('stops polling and surfaces the failure via the error signal', async () => {
      port.script({
        analysisId: 'a-8',
        getAnalysisSequence: [
          { status: 'running' },
          { status: 'failed', error_message: 'gogs down' },
        ],
        fixedAnalysis: {},
        step: 0,
      });
      const out = await useCase.submitAnalysis({
        owner: 'o',
        name: 'n',
        source: 'gogs',
      });
      expect(out?.status).toBe('failed');
      expect(port.getAnalysisCalls).toBe(2);
      expect(useCase.error()).toMatch(/gogs down/);
    });
  });

  describe('daily cap', () => {
    it('rejects the 6th submission without calling the port', async () => {
      storage.setItem(
        `codeHealth.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`,
        '5',
      );
      const capped = new CodeHealthUseCase({
        port,
        storage,
        now,
        sleep,
        dailyCap: 5,
      });
      const out = await capped.submitAnalysis({
        owner: 'o',
        name: 'n',
        source: 'gogs',
      });
      expect(out).toBeNull();
      expect(capped.error()).toMatch(/Daily limit/i);
      expect(capped.remainingToday()).toBe(0);
      expect(port.analyzeRepoCalls).toBe(0);
    });

    it('burns a slot only on success', async () => {
      port.script({
        analysisId: 'a-9',
        getAnalysisSequence: [{ status: 'failed', error_message: 'x' }],
        fixedAnalysis: {},
        step: 0,
      });
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      // Daily count was NOT bumped because the analysis failed — same
      // policy as PracticeUseCase (only successful generations burn
      // the budget).
      expect(useCase.remainingToday()).toBe(5);
      const key = `codeHealth.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`;
      expect(storage.getItem(key)).toBeNull();
    });
  });

  describe('UTC midnight reset', () => {
    it('treats yesterday count as irrelevant for today', async () => {
      const yesterdayKey = `codeHealth.dailyCount.${utcDate(new Date('2026-10-09T23:59:59Z'))}`;
      storage.setItem(yesterdayKey, '5');
      const uc = new CodeHealthUseCase({
        port,
        storage,
        now: () => new Date('2026-10-10T00:00:01Z'),
        sleep,
      });
      expect(uc.remainingToday()).toBe(5);
    });
  });

  describe('error propagation', () => {
    it('maps an MCP error into the error signal', async () => {
      port.analyzeRepoError = new Error('rate limited');
      const out = await useCase.submitAnalysis({
        owner: 'o',
        name: 'n',
        source: 'gogs',
      });
      expect(out).toBeNull();
      expect(useCase.error()).toMatch(/rate limited/i);
      expect(useCase.loading()).toBe(false);
      expect(port.analyzeRepoCalls).toBe(1);
    });
  });

  describe('clearError', () => {
    it('resets the error signal', async () => {
      port.analyzeRepoError = new Error('boom');
      await useCase.submitAnalysis({ owner: 'o', name: 'n', source: 'gogs' });
      expect(useCase.error()).toBeTruthy();
      useCase.clearError();
      expect(useCase.error()).toBeNull();
    });
  });
});
