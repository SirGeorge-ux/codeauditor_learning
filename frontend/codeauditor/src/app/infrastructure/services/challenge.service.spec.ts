import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock Angular's inject to return a mock AuthService before importing ChallengeService
const mockGetAccessToken = vi.fn();

vi.mock('@angular/core', async () => {
  const actual = await vi.importActual('@angular/core');
  return {
    ...actual,
    inject: () => ({
      getAccessToken: mockGetAccessToken,
    }),
  };
});

// Mock fetch before importing anything that uses it
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Import AFTER mocks are set up
import { ChallengeService } from './challenge.service';
import { Challenge } from '../../domain/models/challenge';

const mockChallenge: Challenge = {
  id: 'ch-sqli',
  title: 'SQL Injection',
  description: 'Vulnerable login endpoint',
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
  id: 'ch-xss',
  title: 'XSS',
  description: 'Cross-site scripting',
  difficulty: 'junior',
  category: 'security',
  language: 'typescript',
  repo_url: 'https://github.com/example/social-app',
  code: 'some code',
  code_smell: 'Cross-Site Scripting',
  status: 'available',
  created_at: '2025-01-02T00:00:00.000Z',
};

describe('ChallengeService', () => {
  let service: ChallengeService;

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAccessToken.mockReturnValue('valid-token');
    service = new ChallengeService();
  });

  describe('tempChallenges', () => {
    it('should return temp challenge when present', async () => {
      const tempId = service.addTempChallenge({ ...mockChallenge, id: 'temp-123' });
      const result = await service.getChallenge(tempId);

      expect(result).not.toBeNull();
      expect(result!.id).toBe(tempId);
    });

    it('should not call HTTP when temp challenge is available', async () => {
      const tempId = service.addTempChallenge({ ...mockChallenge, id: 'temp-456' });
      await service.getChallenge(tempId);

      // fetch should NOT have been called since temp challenge was found
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it('should delegate to HTTP repo when no temp challenge matches', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => mockApiResponse,
      });

      const result = await service.getChallenge('ch-xss');

      // Should have called fetch since no temp challenge matched
      expect(mockFetch).toHaveBeenCalledTimes(1);
      expect(result).not.toBeNull();
      expect(result!.id).toBe('ch-xss');
    });

    it('should preserve temp challenge data', async () => {
      const tempId = service.addTempChallenge({
        ...mockChallenge,
        title: 'Custom Challenge',
      });
      const result = await service.getChallenge(tempId);

      expect(result!.title).toBe('Custom Challenge');
    });

    it('should generate unique IDs for temp challenges', () => {
      let counter = 0;
      const originalNow = Date.now;
      Date.now = () => 1000 + counter++;

      try {
        const id1 = service.addTempChallenge(mockChallenge);
        const id2 = service.addTempChallenge(mockChallenge);

        expect(id1).not.toBe(id2);
        expect(id1).toContain('temp-');
        expect(id2).toContain('temp-');
      } finally {
        Date.now = originalNow;
      }
    });
  });
});