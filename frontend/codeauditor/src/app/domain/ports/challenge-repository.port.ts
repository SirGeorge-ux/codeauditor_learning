// ChallengeRepository — port interface for challenge data access.
//
// Implemented by infrastructure layer (mock or real backend).
// Zero framework imports. Pure TypeScript interface.
import { Challenge } from '../models/challenge';

export interface ChallengeRepository {
  getAll(): Promise<Challenge[]>;
  getById(id: string): Promise<Challenge | null>;
}
