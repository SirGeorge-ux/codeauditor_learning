import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HttpChallengeRepository } from './http-challenge.repository';
import { Challenge } from '../../domain/models/challenge';

// Mock global fetch
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const mockTokenProvider = {
  getToken: vi.fn(),
};

const mockChallenge: Challenge = {
  id: 'ch-sqli',
  title: 'SQL Injection',
  description: 'A vulnerable endpoint',
  difficulty: 'junior',
  category: 'security',
  language: 'typescript',
  repoUrl: 'https://github.com/example/vulnerable-api',
  code: 'const x = 1;',
  codeSmell: 'SQL Injection',
  status: 'available',
  createdAt: new Date('2025-01-01'),
};

const mockApiResponse = {
  id: 'ch-sqli',
  title: 'SQL Injection',
  description: 'A vulnerable endpoint',
  difficulty: 'junior',
  category: 'security',
  language: 'typescript',
  repo_url: 'https://github.com/example/vulnerable-api',
  code: 'const x = 1;',
  code_smell: 'SQL Injection',
  status: 'available',
  created_at: '2025-01-01T00:00:00.000Z',
};

describe('HttpChallengeRepository', () => {
  let repo: HttpChallengeRepository;

  beforeEach(() => {
    vi.clearAllMocks();
    mockTokenProvider.getToken.mockReturnValue('valid-token');
    repo = new HttpChallengeRepository(mockTokenProvider, 'http://localhost:8080/api/v1');
  });

  describe('getAll', () => {
    it('should return challenges on successful fetch', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => [mockApiResponse],
      });

      const result = await repo.getAll();

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('ch-sqli');
      expect(result[0].repoUrl).toBe('https://github.com/example/vulnerable-api');
      expect(result[0].codeSmell).toBe('SQL Injection');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges',
        expect.objectContaining({
          headers: { Authorization: 'Bearer valid-token' },
        }),
      );
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
    it('should return challenge on successful fetch', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => mockApiResponse,
      });

      const result = await repo.getById('ch-sqli');

      expect(result).not.toBeNull();
      expect(result!.id).toBe('ch-sqli');
      expect(result!.repoUrl).toBe('https://github.com/example/vulnerable-api');
      expect(result!.codeSmell).toBe('SQL Injection');
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/challenges/ch-sqli',
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
    it('should create a challenge and return it', async () => {
      const createResponse = {
        ...mockApiResponse,
        source_repo: 'owner/repo',
        user_id: 'user-1',
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => createResponse,
      });

      const input = {
        title: 'SQL Injection',
        description: 'A vulnerable endpoint',
        difficulty: 'junior' as const,
        category: 'security',
        language: 'typescript',
        repoUrl: 'https://github.com/example/vulnerable-api',
        sourceRepo: 'owner/repo',
        code: 'const x = 1;',
        codeSmell: 'SQL Injection',
      };

      const result = await repo.create(input);

      expect(result.id).toBe('ch-sqli');
      expect(result.sourceRepo).toBe('owner/repo');
      expect(result.repoUrl).toBe('https://github.com/example/vulnerable-api');
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

      // Verify the body was sent with snake_case mapping
      const callArgs = mockFetch.mock.calls[0];
      const body = JSON.parse(callArgs[1].body as string);
      expect(body.sourceRepo).toBe('owner/repo');
      expect(body.source_repo).toBeUndefined(); // should be camelCase in request
    });

    it('should map camelCase fields to snake_case in request body', async () => {
      const createResponse = {
        ...mockApiResponse,
        source_repo: 'owner/repo',
        code_smell: 'SQL Injection',
      };
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => createResponse,
      });

      const input = {
        title: 'Test',
        description: 'desc',
        difficulty: 'mid' as const,
        category: 'security',
        language: 'go',
        repoUrl: 'https://example.com',
        sourceRepo: 'owner/repo',
        code: 'code',
        codeSmell: 'SQL Injection',
      };

      await repo.create(input);

      const callArgs = mockFetch.mock.calls[0];
      const body = JSON.parse(callArgs[1].body as string);
      expect(body.repoUrl).toBe('https://example.com');
      expect(body.sourceRepo).toBe('owner/repo');
      expect(body.codeSmell).toBe('SQL Injection');
    });

    it('should throw on network error', async () => {
      const input = {
        title: 'SQL Injection',
        description: 'A vulnerable endpoint',
        difficulty: 'junior' as const,
        category: 'security',
        language: 'typescript',
        repoUrl: 'https://github.com/example/vulnerable-api',
        sourceRepo: 'owner/repo',
        code: 'const x = 1;',
        codeSmell: 'SQL Injection',
      };

      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await expect(repo.create(input)).rejects.toThrow('Network error');
    });

    it('should throw on non-OK response', async () => {
      const input = {
        title: 'SQL Injection',
        description: 'A vulnerable endpoint',
        difficulty: 'junior' as const,
        category: 'security',
        language: 'typescript',
        repoUrl: 'https://github.com/example/vulnerable-api',
        sourceRepo: 'owner/repo',
        code: 'const x = 1;',
        codeSmell: 'SQL Injection',
      };

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      await expect(repo.create(input)).rejects.toThrow('Failed to create challenge: 500');
    });

    it('should throw when no token is available', async () => {
      mockTokenProvider.getToken.mockReturnValue(null);

      const input = {
        title: 'SQL Injection',
        description: 'A vulnerable endpoint',
        difficulty: 'junior' as const,
        category: 'security',
        language: 'typescript',
        repoUrl: 'https://github.com/example/vulnerable-api',
        sourceRepo: 'owner/repo',
        code: 'const x = 1;',
        codeSmell: 'SQL Injection',
      };

      await expect(repo.create(input)).rejects.toThrow('Authentication required');
    });
  });
});