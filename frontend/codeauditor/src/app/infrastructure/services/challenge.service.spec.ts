import { describe, it, expect, vi, beforeEach } from 'vitest';

import { ChallengeService } from './challenge.service';
import { ChallengeUseCase } from '../../application/challenge.use-case';
import { Challenge } from '../../domain/models/challenge';

// Manual DI — no TestBed needed. Uses optional constructor parameter.
class MockChallengeRepository {
  getAll = vi.fn<() => Promise<Challenge[]>>();
  getById = vi.fn<(id: string) => Promise<Challenge | null>>();
  create = vi.fn<(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>) => Promise<Challenge>>();
}

describe('ChallengeService', () => {
  let service: ChallengeService;
  let mockRepo: MockChallengeRepository;

  beforeEach(() => {
    vi.clearAllMocks();
    mockRepo = new MockChallengeRepository();
    const useCase = new ChallengeUseCase(mockRepo as any);
    service = new ChallengeService(useCase);
  });

  describe('importChallenge', () => {
    it('should create challenge via use case and return ID', async () => {
      const created: Challenge = {
        id: 'ch-new',
        title: 'New Challenge',
        description: 'Imported from Gogs',
        difficulty: 'mid',
        category: 'imported',
        language: 'go',
        repoUrl: '',
        sourceRepo: 'ggogsmic/academy-mic',
        code: 'package main',
        codeSmell: 'pending-analysis',
        status: 'available',
        createdAt: new Date('2025-06-19T00:00:00.000Z'),
      };

      mockRepo.create.mockResolvedValue(created);
      mockRepo.getAll.mockResolvedValue([created]);

      const input = {
        title: 'New Challenge',
        description: 'Imported from Gogs',
        difficulty: 'mid' as const,
        category: 'imported',
        language: 'go',
        repoUrl: '',
        sourceRepo: 'GgogsMIC/academy-mic',
        code: 'package main',
        codeSmell: 'pending-analysis',
      };

      const id = await service.importChallenge(input);

      expect(id).toBe('ch-new');
      expect(mockRepo.create).toHaveBeenCalledWith(input);
      expect(mockRepo.getAll).toHaveBeenCalled();
    });

    it('should throw if create fails', async () => {
      mockRepo.create.mockRejectedValue(new Error('Network error'));

      const input = {
        title: 'New Challenge',
        description: 'desc',
        difficulty: 'mid' as const,
        category: 'imported',
        language: 'go',
        repoUrl: '',
        sourceRepo: 'owner/repo',
        code: 'code',
        codeSmell: 'smell',
      };

      await expect(service.importChallenge(input)).rejects.toThrow('Network error');
    });
  });

  describe('getChallenge', () => {
    it('should delegate to the use case repository', async () => {
      const challenge: Challenge = {
        id: 'ch-xss',
        title: 'XSS',
        description: 'Cross-site scripting',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        repoUrl: 'https://github.com/example/social-app',
        sourceRepo: undefined,
        code: 'some code',
        codeSmell: 'Cross-Site Scripting',
        status: 'available',
        createdAt: new Date('2025-01-02T00:00:00.000Z'),
      };

      mockRepo.getById.mockResolvedValue(challenge);

      const result = await service.getChallenge('ch-xss');

      expect(result).not.toBeNull();
      expect(result!.id).toBe('ch-xss');
      expect(mockRepo.getById).toHaveBeenCalledWith('ch-xss');
    });
  });
});
