import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';

// Mock @supabase/supabase-js the same way auth.service.spec.ts does so
// importing AuthService here does not instantiate the real Supabase
// client (the class-field initializer in auth.service.ts creates a
// GoTrueClient at module load). Without this mock, this spec creates
// a second GoTrueClient and the auth.service.spec.ts vi.mock stops
// binding, producing the chronic 6/10 flake.
vi.mock('@supabase/supabase-js', () => ({
  createClient: () => ({
    auth: {
      signUp: vi.fn(),
      signInWithPassword: vi.fn(),
      signOut: vi.fn(),
      getSession: vi.fn(),
      getUser: vi.fn(),
      onAuthStateChange: vi.fn(() => ({
        data: { subscription: { unsubscribe: vi.fn() } },
      })),
    },
  }),
}));

import { ChallengeService } from './challenge.service';
import { ChallengeUseCase } from '../../application/challenge.use-case';
import { AuthService } from './auth.service';
import { Challenge } from '../../domain/models/challenge';
import { ChallengeRepository } from '../../domain/ports/challenge-repository.port';

// Test seam: after TestBed constructs the service, swap the useCase field
// for one wrapping the FakeChallengeRepository. Keeps the existing test
// assertion surface (mockRepo.create / getById / getAll) without forcing
// the service to accept a useCase constructor parameter.
type UseCaseHolder = { useCase: ChallengeUseCase };

// Inline fake for unit testing ChallengeService — replaces the
// deleted MockChallengeRepository schema adapter.
class FakeChallengeRepository {
  getAll = vi.fn<() => Promise<Challenge[]>>();
  getById = vi.fn<(id: string) => Promise<Challenge | null>>();
  create = vi.fn<(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>) => Promise<Challenge>>();
}

describe('ChallengeService', () => {
  let service: ChallengeService;
  let mockRepo: FakeChallengeRepository;

  beforeEach(() => {
    vi.clearAllMocks();
    mockRepo = new FakeChallengeRepository();
    const useCase = new ChallengeUseCase(mockRepo as unknown as ChallengeRepository);

    TestBed.configureTestingModule({
      providers: [
        ChallengeService,
        { provide: AuthService, useValue: { getAccessToken: () => 'test-token' } },
      ],
    });
    service = TestBed.inject(ChallengeService);
    // Test seam: inject the fake-repo-backed useCase.
    (service as unknown as UseCaseHolder).useCase = useCase;
  });

  describe('importChallenge', () => {
    it('should create challenge via use case and return ID', async () => {
      const created: Challenge = {
        id: 'ch-new',
        title: 'New Challenge',
        description: 'Imported file from a repository.',
        difficulty: 'mid',
        category: 'imported',
        language: 'go',
        code: 'package main',
        status: 'available',
        createdAt: new Date('2025-06-19T00:00:00.000Z'),
        createdBy: 'ggogsmic',
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
        origin: 'imported',
        sourceRepo: 'ggogsmic/academy-mic',
      };

      mockRepo.create.mockResolvedValue(created);
      mockRepo.getAll.mockResolvedValue([created]);

      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'New Challenge',
        description: 'Imported file from a repository.',
        difficulty: 'mid',
        category: 'imported',
        language: 'go',
        code: 'package main',
        createdBy: 'ggogsmic',
        sourceRepo: 'GgogsMIC/academy-mic',
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
        origin: 'imported',
      };

      const id = await service.importChallenge(input);

      expect(id).toBe('ch-new');
      expect(mockRepo.create).toHaveBeenCalledWith(input);
      expect(mockRepo.getAll).toHaveBeenCalled();
    });

    it('should throw if create fails', async () => {
      mockRepo.create.mockRejectedValue(new Error('Network error'));

      const input: Omit<Challenge, 'id' | 'createdAt' | 'status'> = {
        title: 'T',
        description: 'desc',
        difficulty: 'mid',
        category: 'imported',
        language: 'go',
        code: 'code',
        createdBy: 'ggogsmic',
        sourceRepo: 'owner/repo',
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
        origin: 'gogs',
      };

      await expect(service.importChallenge(input)).rejects.toThrow('Network error');
    });
  });

  describe('getChallenge', () => {
    it('should delegate to the use case repository', async () => {
      const challenge: Challenge = {
        id: 'ch-xss',
        title: 'Login Form',
        description: 'A user comment feature.',
        difficulty: 'junior',
        category: 'security',
        language: 'typescript',
        code: 'some code',
        status: 'available',
        createdAt: new Date('2025-01-02T00:00:00.000Z'),
        createdBy: 'curated',
        learningObjectives: [],
        hints: [],
        commonMistakes: [],
        estimatedTimeMinutes: 15,
        expectedFindings: [],
        testCases: [],
        linterRules: [],
        basePoints: 100,
        bonusPoints: 50,
        penaltyPerHint: 15,
        timeBonus: true,
        origin: 'curated',
      };

      mockRepo.getById.mockResolvedValue(challenge);

      const result = await service.getChallenge('ch-xss');

      expect(result).not.toBeNull();
      expect(result!.id).toBe('ch-xss');
      expect(mockRepo.getById).toHaveBeenCalledWith('ch-xss');
    });
  });
});
