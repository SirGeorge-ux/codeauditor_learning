// HttpChallengeRepository — infrastructure adapter that fetches challenges
// from the backend API.
//
// The backend serializes the v2 Challenge struct with camelCase JSON keys
// at the top level (e.g. `learningObjectives`, `testCases`, `basePoints`) and
// snake_case keys inside the nested JSONB sub-objects (e.g. `cost_points`,
// `expected_output`, `suggested_fix`) to match the DB JSONB format. This
// adapter maps the snake_case nested fields to camelCase and defaults any
// null/missing v2 arrays to `[]` so the domain layer can assume non-null.
//
// Uses native fetch with manual Authorization header (consistent with
// AuthService pattern). Implements ChallengeRepository port interface.
import {
  Challenge,
  Hint,
  ExpectedFinding,
  TestCase,
  LinterRule,
} from '../../domain/models/challenge';
import { ChallengeRepository } from '../../domain/ports/challenge-repository.port';

export interface TokenProvider {
  getToken(): string | null;
}

export class HttpChallengeRepository implements ChallengeRepository {
  private readonly baseUrl: string;
  private readonly tokenProvider: TokenProvider;

  constructor(tokenProvider: TokenProvider, baseUrl: string = '/api/v1') {
    this.baseUrl = baseUrl;
    this.tokenProvider = tokenProvider;
  }

  async getAll(): Promise<Challenge[]> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return [];

      const resp = await fetch(`${this.baseUrl}/challenges`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!resp.ok) return [];
      const data = await resp.json();
      return Array.isArray(data) ? data.map(mapApiToChallenge) : [];
    } catch {
      return [];
    }
  }

  async getById(id: string, revealSolution = false): Promise<Challenge | null> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return null;

      const path = `${this.baseUrl}/challenges/${encodeURIComponent(id)}`;
      const url = revealSolution ? `${path}?reveal=true` : path;

      const resp = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (resp.status === 404) return null;
      if (!resp.ok) return null;
      const data = await resp.json();
      return mapApiToChallenge(data);
    } catch {
      return null;
    }
  }

  async create(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>): Promise<Challenge> {
    const token = this.tokenProvider.getToken();
    if (!token) {
      throw new Error('Authentication required');
    }

    const response = await fetch(`${this.baseUrl}/challenges`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        title: input.title,
        description: input.description,
        difficulty: input.difficulty,
        category: input.category,
        language: input.language,
        sourceRepo: input.sourceRepo,
        code: input.code,
        // v2 fields — camelCase JSON keys match backend
        learningObjectives: input.learningObjectives,
        hints: input.hints,
        commonMistakes: input.commonMistakes,
        estimatedTimeMinutes: input.estimatedTimeMinutes,
        expectedFindings: input.expectedFindings,
        testCases: input.testCases,
        linterRules: input.linterRules,
        solutionCode: input.solutionCode,
        solutionExplanation: input.solutionExplanation,
        basePoints: input.basePoints,
        bonusPoints: input.bonusPoints,
        penaltyPerHint: input.penaltyPerHint,
        timeBonus: input.timeBonus,
        origin: input.origin,
        sourcePath: input.sourcePath,
        generatedBy: input.generatedBy,
        createdBy: input.createdBy,
      }),
    });

    if (!response.ok) {
      throw new Error(`Failed to create challenge: ${response.status}`);
    }

    const data = await response.json();
    return mapApiToChallenge(data);
  }
}

// Map a raw API JSON object into a Challenge domain entity.
// Defaults null/missing v2 arrays to `[]` and parses nested sub-objects.
function mapApiToChallenge(obj: Record<string, unknown>): Challenge {
  return {
    id: stringOrEmpty(obj['id']),
    title: stringOrEmpty(obj['title']),
    description: stringOrEmpty(obj['description']),
    difficulty: obj['difficulty'] as Challenge['difficulty'],
    category: stringOrEmpty(obj['category']),
    language: stringOrEmpty(obj['language']),
    code: stringOrEmpty(obj['code']),
    status: (obj['status'] as Challenge['status']) ?? 'available',
    createdAt: new Date((obj['createdAt'] ?? obj['created_at']) as string),
    createdBy: stringOrEmpty(obj['createdBy']),

    learningObjectives: arrayOfStrings(obj['learningObjectives']),
    hints: mapHints(obj['hints']),
    commonMistakes: arrayOfStrings(obj['commonMistakes']),
    estimatedTimeMinutes: numberOrZero(obj['estimatedTimeMinutes']),
    expectedFindings: mapExpectedFindings(obj['expectedFindings']),
    testCases: mapTestCases(obj['testCases']),
    linterRules: mapLinterRules(obj['linterRules']),
    basePoints: numberOrZero(obj['basePoints']),
    bonusPoints: numberOrZero(obj['bonusPoints']),
    penaltyPerHint: numberOrZero(obj['penaltyPerHint']),
    timeBonus: Boolean(obj['timeBonus']),
    origin: (obj['origin'] as Challenge['origin']) ?? 'curated',

    sourceRepo: optionalString(obj['sourceRepo']),
    sourcePath: optionalString(obj['sourcePath']),
    generatedBy: optionalString(obj['generatedBy']),

    solutionCode: optionalString(obj['solutionCode']),
    solutionExplanation: optionalString(obj['solutionExplanation']),
  };
}

function stringOrEmpty(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function optionalString(v: unknown): string | undefined {
  return typeof v === 'string' && v.length > 0 ? v : undefined;
}

function numberOrZero(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0;
}

function arrayOfStrings(v: unknown): string[] {
  return Array.isArray(v) ? v.map((x) => String(x)) : [];
}

function mapHints(v: unknown): Hint[] {
  if (!Array.isArray(v)) return [];
  return v.map((raw) => {
    const h = raw as Record<string, unknown>;
    return {
      level: numberOrZero(h['level']),
      content: stringOrEmpty(h['content']),
      costPoints: numberOrZero(h['cost_points'] ?? h['costPoints']),
    };
  });
}

function mapExpectedFindings(v: unknown): ExpectedFinding[] {
  if (!Array.isArray(v)) return [];
  return v.map((raw) => {
    const f = raw as Record<string, unknown>;
    return {
      severity: stringOrEmpty(f['severity']),
      category: stringOrEmpty(f['category']),
      message: stringOrEmpty(f['message']),
      evidence: stringOrEmpty(f['evidence']),
      suggestedFix: stringOrEmpty(f['suggested_fix'] ?? f['suggestedFix']),
    };
  });
}

function mapTestCases(v: unknown): TestCase[] {
  if (!Array.isArray(v)) return [];
  return v.map((raw) => {
    const t = raw as Record<string, unknown>;
    return {
      name: stringOrEmpty(t['name']),
      input: t['input'],
      expectedOutput: t['expected_output'] ?? t['expectedOutput'],
      weight: numberOrZero(t['weight']),
    };
  });
}

function mapLinterRules(v: unknown): LinterRule[] {
  if (!Array.isArray(v)) return [];
  return v.map((raw) => {
    const l = raw as Record<string, unknown>;
    return {
      type: stringOrEmpty(l['type']),
      config: (l['config'] as Record<string, unknown>) ?? {},
    };
  });
}
