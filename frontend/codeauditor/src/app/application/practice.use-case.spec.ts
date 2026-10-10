// practice.use-case.spec.ts — application-layer tests for the Free
// Practice Mode use case.
//
// Tests cover (per design.md §PracticeUseCase):
//   - Signal transitions (language / topic / level / loading / error /
//     remainingToday / generatedChallenge).
//   - Command flow: generateChallenge() calls
//     `mcp-challenges.generate_challenge` with the right args.
//   - Daily-cap enforcement: the 21st generation in a UTC day is
//     rejected without calling the backend; the UI surfaces a clear
//     error and the counter stays at 20.
//   - localStorage persistence + UTC-midnight reset: a count stored
//     under yesterday's key is ignored after midnight; the counter
//     shows 20 remaining.
//
// Test doubles:
//   - FakeMcp: an in-memory MCPClientPort that records each callTool
//     invocation and lets the test script the result.
//   - FakeStorage: a Map-backed localStorage stub.
//   - AuthPort: a fixed-user stub (userId is on the User, not the
//     AuthPort surface — the use case calls getCurrentUser() to stamp
//     the daily-count key).
//
// RED state at HEAD~1: this file imports `PracticeUseCase` from a
// non-existent module, so vitest fails to load the suite.
import { describe, it, expect, beforeEach } from 'vitest';
import { of, throwError } from 'rxjs';

import { PracticeUseCase } from './practice.use-case';
import { MCPClientPort, MCPTool } from '../domain/ports/mcp-client.port';
import { Challenge } from '../domain/models/challenge';
import { CurriculumTopic } from '../domain/models/curriculum';

// -- Test doubles ------------------------------------------------------------

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
  // Test introspection helpers (NOT part of the Storage interface).
  raw(): Record<string, string> {
    return Object.fromEntries(this.map);
  }
}

interface CapturedCall {
  name: string;
  args: unknown;
}

class FakeMcp implements MCPClientPort {
  captured: CapturedCall[] = [];
  // default to a successful challenge so happy-path tests don't have to script it
  challengeResult: Challenge | null = makeChallengeFixture();
  challengeError: Error | null = null;
  topicsResult: CurriculumTopic[] = [
    { id: 'inheritance', title: 'Inheritance', difficulty: 'D', description: 'OOP basics' },
    { id: 'generics', title: 'Generics', difficulty: 'C', description: 'Type parameters' },
  ];

  listTools() {
    return of([] as MCPTool[]);
  }
  callTool<T = unknown>(name: string, args: unknown) {
    this.captured.push({ name, args });
    if (name === 'mcp-challenges.generate_challenge') {
      if (this.challengeError) {
        return throwError(() => this.challengeError);
      }
      return of(this.challengeResult as unknown as T);
    }
    if (name === 'mcp-curriculum.get_topics') {
      return of(this.topicsResult as unknown as T);
    }
    return of({} as T);
  }
  streamTool<T = unknown>(_name: string, _args: unknown) {
    return of({} as T);
  }
}

function makeChallengeFixture(): Challenge {
  return {
    id: 'gen-1',
    title: 'Generated challenge',
    description: 'A description without spoilers.',
    difficulty: 'mid',
    category: 'oop',
    language: 'typescript',
    code: '// challenge code',
    status: 'available',
    createdAt: new Date('2026-10-10'),
    createdBy: 'user-1',
    learningObjectives: ['Learn inheritance'],
    hints: [
      { level: 1, content: 'hint 1', costPoints: 0 },
      { level: 2, content: 'hint 2', costPoints: 5 },
      { level: 3, content: 'hint 3', costPoints: 10 },
    ],
    commonMistakes: ['common mistake'],
    estimatedTimeMinutes: 30,
    expectedFindings: [
      {
        severity: 'medium',
        category: 'design',
        message: 'Tight coupling',
        evidence: '...',
        suggestedFix: '...',
      },
      {
        severity: 'low',
        category: 'style',
        message: 'Magic numbers',
        evidence: '...',
        suggestedFix: '...',
      },
    ],
    testCases: [
      {
        name: 'smoke',
        input: {},
        expectedOutput: 'ok',
        weight: 1,
      },
    ],
    linterRules: [],
    basePoints: 100,
    bonusPoints: 20,
    penaltyPerHint: 5,
    timeBonus: true,
    origin: 'generated',
    generatedBy: 'free-practice-v1',
    solutionCode: '// solution',
    solutionExplanation: '...',
  };
}

// utcDate returns a YYYY-MM-DD string for a Date in UTC. Mirrors the
// helper inside the use case so tests can pin the "today" boundary.
function utcDate(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, '0');
  const day = String(d.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// -- Test setup --------------------------------------------------------------

describe('PracticeUseCase', () => {
  let mcp: FakeMcp;
  let storage: FakeStorage;
  let useCase: PracticeUseCase;

  beforeEach(() => {
    mcp = new FakeMcp();
    storage = new FakeStorage();
    useCase = new PracticeUseCase({
      mcp,
      storage,
      // Pin "now" to a fixed UTC instant so the cap math is reproducible.
      now: () => new Date('2026-10-10T12:00:00Z'),
    });
  });

  describe('signal initial state', () => {
    it('starts with empty selectors and 20 remaining generations', () => {
      expect(useCase.language()).toBe('');
      expect(useCase.topic()).toBe('');
      expect(useCase.level()).toBe('F');
      expect(useCase.loading()).toBe(false);
      expect(useCase.error()).toBeNull();
      expect(useCase.remainingToday()).toBe(20);
      expect(useCase.generatedChallenge()).toBeNull();
    });
  });

  describe('setLanguage / setTopic / setLevel', () => {
    it('updates the corresponding signals', () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      expect(useCase.language()).toBe('typescript');
      expect(useCase.topic()).toBe('inheritance');
      expect(useCase.level()).toBe('D');
    });

    it('ignores unknown F..S levels on setLevel', () => {
      useCase.setLevel('Z');
      expect(useCase.level()).toBe('F');
    });
  });

  describe('loadTopics', () => {
    it('calls mcp-curriculum.get_topics and stores the result', async () => {
      await useCase.loadTopics('typescript');
      expect(useCase.topics()).toHaveLength(2);
      expect(useCase.topics()[0].id).toBe('inheritance');
      expect(mcp.captured).toContainEqual({
        name: 'mcp-curriculum.get_topics',
        args: { language: 'typescript' },
      });
    });
  });

  describe('generateChallenge (happy path)', () => {
    it('calls mcp-challenges.generate_challenge with the right args', async () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      await useCase.generateChallenge();
      const genCall = mcp.captured.find((c) => c.name === 'mcp-challenges.generate_challenge');
      expect(genCall).toBeDefined();
      expect(genCall?.args).toEqual({
        language: 'typescript',
        topicId: 'inheritance',
        level: 'D',
      });
    });

    it('sets generatedChallenge signal to the returned Challenge', async () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      await useCase.generateChallenge();
      expect(useCase.generatedChallenge()?.id).toBe('gen-1');
      expect(useCase.generatedChallenge()?.origin).toBe('generated');
    });

    it('decrements remainingToday on success and persists to localStorage', async () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      await useCase.generateChallenge();
      expect(useCase.remainingToday()).toBe(19);
      const key = `practice.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`;
      expect(storage.getItem(key)).toBe('1');
    });

    it('toggles loading signal across the call', async () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      const inFlight = useCase.generateChallenge();
      // The loading signal is set before the awaited body resolves;
      // vitest 4 microtask ordering makes this snapshot safe.
      expect(useCase.loading()).toBe(true);
      await inFlight;
      expect(useCase.loading()).toBe(false);
    });
  });

  describe('generateChallenge (validation guards)', () => {
    it('rejects when language is empty', async () => {
      await useCase.generateChallenge();
      expect(useCase.error()).toMatch(/language/i);
      expect(useCase.loading()).toBe(false);
      expect(mcp.captured).toHaveLength(0);
    });

    it('rejects when topic is empty', async () => {
      useCase.setLanguage('typescript');
      await useCase.generateChallenge();
      expect(useCase.error()).toMatch(/topic/i);
      expect(mcp.captured).toHaveLength(0);
    });

    it('rejects when level is not in the F..S band', async () => {
      // The public setter already filters invalid F..S values, so the
      // level signal stays at the default ('F') when an invalid value
      // is passed. The validateArgs() guard is then a defense-in-depth
      // check; we assert the setter's gate here.
      useCase.setLevel('Z');
      expect(useCase.level()).toBe('F');
      useCase.setLevel('1');
      expect(useCase.level()).toBe('F');
      // And a valid level is accepted.
      useCase.setLevel('D');
      expect(useCase.level()).toBe('D');
    });
  });

  describe('daily cap', () => {
    it('rejects the 21st generation without calling the backend', async () => {
      // Pre-fill the localStorage count to 20 (today's UTC date).
      const key = `practice.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`;
      storage.setItem(key, '20');
      // The use case needs to read the persisted count on construction,
      // so build one that does so against this storage.
      const capped = new PracticeUseCase({
        mcp,
        storage,
        now: () => new Date('2026-10-10T12:00:00Z'),
      });
      expect(capped.remainingToday()).toBe(0);
      capped.setLanguage('typescript');
      capped.setTopic('inheritance');
      capped.setLevel('D');
      await capped.generateChallenge();
      expect(capped.error()).toMatch(/Daily limit/i);
      expect(capped.generatedChallenge()).toBeNull();
      expect(
        mcp.captured.find((c) => c.name === 'mcp-challenges.generate_challenge'),
      ).toBeUndefined();
    });

    it('surfaces the daily cap via remainingToday signal', () => {
      const key = `practice.dailyCount.${utcDate(new Date('2026-10-10T12:00:00Z'))}`;
      storage.setItem(key, '15');
      const uc = new PracticeUseCase({
        mcp,
        storage,
        now: () => new Date('2026-10-10T12:00:00Z'),
      });
      expect(uc.remainingToday()).toBe(5);
    });
  });

  describe('UTC midnight reset', () => {
    it('treats yesterday count as irrelevant for today', () => {
      // Yesterday: 20 generations done. Today is a fresh day.
      const yesterday = utcDate(new Date('2026-10-09T23:59:59Z'));
      storage.setItem(`practice.dailyCount.${yesterday}`, '20');
      const uc = new PracticeUseCase({
        mcp,
        storage,
        now: () => new Date('2026-10-10T00:00:01Z'),
      });
      expect(uc.remainingToday()).toBe(20);
    });
  });

  describe('backend error surfacing', () => {
    it('maps an MCP error into the error signal without reloading', async () => {
      mcp.challengeError = new Error('backend 500');
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      await useCase.generateChallenge();
      expect(useCase.error()).toMatch(/backend 500/i);
      expect(useCase.loading()).toBe(false);
      // The cap should NOT increment when the backend rejected the
      // attempt — only successful generations burn the budget.
      expect(useCase.remainingToday()).toBe(20);
      expect(storage.raw()).toEqual({});
    });

    it('surfaces network errors via Observable errors', async () => {
      mcp.challengeError = new Error('connection refused');
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      await useCase.generateChallenge();
      expect(useCase.error()).toMatch(/connection refused/i);
    });
  });

  describe('clearError', () => {
    it('resets the error signal', async () => {
      useCase.setLanguage('typescript');
      useCase.setTopic('inheritance');
      useCase.setLevel('D');
      mcp.challengeError = new Error('boom');
      await useCase.generateChallenge();
      expect(useCase.error()).toBeTruthy();
      useCase.clearError();
      expect(useCase.error()).toBeNull();
    });
  });
});
