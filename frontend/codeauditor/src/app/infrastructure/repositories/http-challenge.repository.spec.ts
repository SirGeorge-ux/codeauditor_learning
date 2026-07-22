import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HttpChallengeRepository } from './http-challenge.repository';
import { Challenge } from '../../domain/models/challenge';

// Mock global fetch
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const mockTokenProvider = {
  getToken: vi.fn(),
};

// V2 API response shape — camelCase at top level (matches Go json tags),
// snake_case inside nested JSONB sub-objects (matches DB JSONB format).
const mockApiResponse = {
  id: 'ch-sqli',
  title: 'Login Form',
  description: 'A login endpoint that processes credentials and returns a token.',
  difficulty: 'junior',
  category: 'security',
  language: 'typescript',
  code: 'const x = 1;',
  status: 'available',
  createdAt: '2025-01-01T00:00:00.000Z',
  createdBy: 'curated',

  learningObjectives: ['sanitized queries', 'parameterized SQL'],
  hints: [
    { level: 1, content: 'Look at how the username is interpolated.', cost_points: 0 },
    { level: 2, content: 'String concatenation in SQL is risky.', cost_points: 15 },
    { level: 3, content: 'Use parameterized queries.', cost_points: 20 },
  ],
  commonMistakes: ['trusting user input'],
  estimatedTimeMinutes: 15,
  expectedFindings: [
    {
      severity: 'high',
      category: 'security',
      message: 'SQL injection',
      evidence: 'line 3',
      suggested_fix: 'Use parameterized query',
    },
  ],
  testCases: [
    { name: 'rejects empty', input: '', expected_output: 'error', weight: 5 },
    { name: 'accepts valid', input: 'user', expected_output: 'ok', weight: 3 },
  ],
  linterRules: [{ type: 'no-eval', config: { severity: 'error' } }],
  basePoints: 100,
  bonusPoints: 50,
  penaltyPerHint: 15,
  timeBonus: true,
  origin: 'curated',
};

function expectedChallenge(): Challenge {
  return {
    id: 'ch-sqli',
    title: 'Login Form',
    description: 'A login endpoint that processes credentials and returns a token.',
    difficulty: 'junior',
    category: 'security',
    language: 'typescript',
    code: 'const x = 1;',
    status: 'available',
    createdAt: new Date('2025-01-01T00:00:00.000Z'),
    createdBy: 'curated',
    learningObjectives: ['sanitized queries', 'parameterized SQL'],
    hints: [
      { level: 1, content: 'Look at how the username is interpolated.', costPoints: 0 },
      { level: 2, content: 'String concatenation in SQL is risky.', costPoints: 15 },
      { level: 3, content: 'Use parameterized queries.', costPoints: 20 },
    ],
    commonMistakes: ['trusting user input'],
    estimatedTimeMinutes: 15,
    expectedFindings: [
      {
        severity: 'high',
        category: 'security',
        message: 'SQL injection',
        evidence: 'line 3',
        suggestedFix: 'Use parameterized query',
      },
    ],
    testCases: [
      { name: 'rejects empty', input: '', expectedOutput: 'error', weight: 5 },
      { name: 'accepts valid', input: 'user', expectedOutput: 'ok', weight: 3 },
    ],
    linterRules: [{ type: 'no-eval', config: { severity: 'error' } }],
    basePoints: 100,
    bonusPoints: 50,
    penaltyPerHint: 15,
    timeBonus: true,
    origin: 'curated',
  };
}

describe('HttpChallengeRepository', () => {
  let repo: HttpChallengeRepository;

  beforeEach(() => {
    vi.clearAllMocks();
    mockTokenProvider.getToken.mockReturnValue('valid-token');
    repo = new HttpChallengeRepository(mockTokenProvider, 'http://localhost:8080/api/v1');
  });

  describe('getAll', () => {
    it('should return challenges mapped from v2 API response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => [mockApiResponse],
      });

      const result = await repo.getAll();

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('ch-sqli');
      expect(result[0].hints).toHaveLength(3);
      expect(result[0].hints[1].costPoints).toBe(15); // snake_case cost_points → costPoints
      expect(result[0].testCases).toHaveLength(2);
      expect(result[0].testCases[0].expectedOutput).toBe('error'); // snake_case expected_output
      expect(result[0].expectedFindings[0].suggestedFix).toBe('Use parameterized query');
      expect(result[0].basePoints).toBe(100);
      expect(result[0].timeBonus).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges',
        expect.objectContaining({
          headers: { Authorization: 'Bearer valid-token' },
        }),
      );
    });

    it('should default missing v2 arrays to empty arrays', async () => {
      // Simulate a response with v2 fields missing (e.g., legacy rows)
      const minimalResponse = {
        id: 'ch-1',
        title: 'T',
        description: 'D',
        difficulty: 'junior',
        category: 'x',
        language: 'typescript',
        code: '',
        status: 'available',
        createdAt: '2025-01-01T00:00:00.000Z',
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => [minimalResponse],
      });

      const result = await repo.getAll();

      expect(result).toHaveLength(1);
      expect(result[0].hints).toEqual([]);
      expect(result[0].testCases).toEqual([]);
      expect(result[0].expectedFindings).toEqual([]);
      expect(result[0].linterRules).toEqual([]);
      expect(result[0].learningObjectives).toEqual([]);
      expect(result[0].basePoints).toBe(0);
      expect(result[0].timeBonus).toBe(false);
    });

    it('should not include solutionCode by default', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => [mockApiResponse],
      });

      const result = await repo.getAll();

      expect(result[0].solutionCode).toBeUndefined();
      expect(result[0].solutionExplanation).toBeUndefined();
    });

    it('should map solutionCode when reveal=true is set in API response', async () => {
      const withSolution = {
        ...mockApiResponse,
        solutionCode: 'SELECT * FROM users WHERE ...',
        solutionExplanation: 'Parameterize the query.',
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => withSolution,
      });

      const result = await repo.getById('ch-sqli', true);

      expect(result).not.toBeNull();
      expect(result!.solutionCode).toBe('SELECT * FROM users WHERE ...');
      expect(result!.solutionExplanation).toBe('Parameterize the query.');
    });

    it('should return empty array on network error', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const result = await repo.getAll();

      expect(result).toEqual([]);
    });

    it('should return empty array when no token', async () => {
      mockTokenProvider.getToken.mockReturnValue(null);

      const result = await repo.getAll();

      expect(result).toEqual([]);
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it('should return empty array on non-200 response', async () => {
      mockFetch.mockResolvedValueOnce({ ok: false, status: 500 });

      const result = await repo.getAll();

      expect(result).toEqual([]);
    });
  });

  describe('getById', () => {
    it('should return challenge mapped from v2 API response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => mockApiResponse,
      });

      const result = await repo.getById('ch-sqli');

      expect(result).not.toBeNull();
      expect(result).toEqual(expectedChallenge());
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges/ch-sqli',
        expect.objectContaining({
          headers: { Authorization: 'Bearer valid-token' },
        }),
      );
    });

    it('should append ?reveal=true when revealSolution flag set', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ ...mockApiResponse, solutionCode: 'sol' }),
      });

      await repo.getById('ch-sqli', true);

      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges/ch-sqli?reveal=true',
        expect.objectContaining({
          headers: { Authorization: 'Bearer valid-token' },
        }),
      );
    });

    it('should return null on 404', async () => {
      mockFetch.mockResolvedValueOnce({ ok: false, status: 404 });

      const result = await repo.getById('ch-nonexistent');

      expect(result).toBeNull();
    });

    it('should return null on network error', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const result = await repo.getById('ch-sqli');

      expect(result).toBeNull();
    });

    it('should return null when no token', async () => {
      mockTokenProvider.getToken.mockReturnValue(null);

      const result = await repo.getById('ch-sqli');

      expect(result).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
    });
  });

  describe('create', () => {
    it('should create a challenge and return it mapped to v2', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => mockApiResponse,
      });

      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'Login Form',
        description: 'A login endpoint that processes credentials and returns a token.',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        code: 'const x = 1;',
        createdBy: 'curated',
        learningObjectives: ['sanitized queries', 'parameterized SQL'],
        hints: [
          { level: 1, content: 'Look at how the username is interpolated.', costPoints: 0 },
          { level: 2, content: 'String concatenation in SQL is risky.', costPoints: 15 },
          { level: 3, content: 'Use parameterized queries.', costPoints: 20 },
        ],
        commonMistakes: ['trusting user input'],
        estimatedTimeMinutes: 15,
        expectedFindings: [
          {
            severity: 'high',
            category: 'security',
            message: 'SQL injection',
            evidence: 'line 3',
            suggestedFix: 'Use parameterized query',
          },
        ],
        testCases: [
          { name: 'rejects empty', input: '', expectedOutput: 'error', weight: 5 },
          { name: 'accepts valid', input: 'user', expectedOutput: 'ok', weight: 3 },
        ],
        linterRules: [{ type: 'no-eval', config: { severity: 'error' } }],
        basePoints: 100,
        bonusPoints: 50,
        penaltyPerHint: 15,
        timeBonus: true,
        origin: 'curated',
      };

      const result = await repo.create(input);

      expect(result.id).toBe('ch-sqli');
      expect(result.basePoints).toBe(100);
      expect(result.hints).toHaveLength(3);
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Authorization: 'Bearer valid-token',
          },
        }),
      );

      // Verify the body was sent with camelCase v2 fields
      const callArgs = mockFetch.mock.calls[0];
      const body = JSON.parse(callArgs[1].body as string);
      expect(body.basePoints).toBe(100);
      expect(body.timeBonus).toBe(true);
      expect(body.origin).toBe('curated');
      // v1 fields should not be sent
      expect(body.repoUrl).toBeUndefined();
      expect(body.codeSmell).toBeUndefined();
    });

    it('should throw on network error', async () => {
      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'T',
        description: 'D',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        code: '',
        createdBy: 'curated',
        learningObjectives: [],
        hints: [],
        commonMistakes: [],
        estimatedTimeMinutes: 0,
        expectedFindings: [],
        testCases: [],
        linterRules: [],
        basePoints: 0,
        bonusPoints: 0,
        penaltyPerHint: 0,
        timeBonus: false,
        origin: 'curated',
      };

      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await expect(repo.create(input)).rejects.toThrow('Network error');
    });

    it('should throw on non-OK response', async () => {
      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'T',
        description: 'D',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        code: '',
        createdBy: 'curated',
        learningObjectives: [],
        hints: [],
        commonMistakes: [],
        estimatedTimeMinutes: 0,
        expectedFindings: [],
        testCases: [],
        linterRules: [],
        basePoints: 0,
        bonusPoints: 0,
        penaltyPerHint: 0,
        timeBonus: false,
        origin: 'curated',
      };

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      await expect(repo.create(input)).rejects.toThrow('Failed to create challenge: 500');
    });

    it('should throw when no token is available', async () => {
      mockTokenProvider.getToken.mockReturnValue(null);

      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'T',
        description: 'D',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        code: '',
        createdBy: 'curated',
        learningObjectives: [],
        hints: [],
        commonMistakes: [],
        estimatedTimeMinutes: 0,
        expectedFindings: [],
        testCases: [],
        linterRules: [],
        basePoints: 0,
        bonusPoints: 0,
        penaltyPerHint: 0,
        timeBonus: false,
        origin: 'curated',
      };

      await expect(repo.create(input)).rejects.toThrow('Authentication required');
    });
  });
});
