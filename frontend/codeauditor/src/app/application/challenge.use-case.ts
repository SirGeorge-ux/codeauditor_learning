// ChallengeUseCase — application service orchestrating challenge operations.
//
// Responsibilities:
// 1. Load all available challenges
// 2. Select a specific challenge by ID
// 3. Create a new challenge (import from Gogs)
//
// Zero framework imports. Pure TypeScript.
import { Challenge } from '../domain/models/challenge';
import { ChallengeRepository } from '../domain/ports/challenge-repository.port';

export class ChallengeUseCase {
  constructor(private readonly repo: ChallengeRepository) {}

  async loadChallenges(): Promise<Challenge[]> {
    return this.repo.getAll();
  }

  async selectChallenge(id: string): Promise<Challenge | null> {
    return this.repo.getById(id);
  }

  async createChallenge(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>): Promise<Challenge> {
    return this.repo.create(input);
  }
}
