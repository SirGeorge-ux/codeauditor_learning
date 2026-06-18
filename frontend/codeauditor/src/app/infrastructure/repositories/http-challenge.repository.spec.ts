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
});