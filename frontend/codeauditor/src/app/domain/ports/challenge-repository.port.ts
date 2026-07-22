// ChallengeRepository — port interface for challenge data access.
//
// Implemented by the infrastructure layer (HTTP adapter).
// Zero framework imports. Pure TypeScript interface.
import { Challenge } from '../models/challenge';

export interface ChallengeRepository {
  getAll(): Promise<Challenge[]>;
  getById(id: string): Promise<Challenge | null>;
  create(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>): Promise<Challenge>;
}
