// HttpChallengeRepository — infrastructure adapter that fetches challenges from the backend API.
//
// Uses native fetch with manual Authorization header (consistent with AuthService pattern).
// Implements ChallengeRepository port interface.
import { Challenge } from '../../domain/models/challenge';
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
      return Array.isArray(data) ? data.map(mapSnakeToCamel) : [];
    } catch {
      return [];
    }
  }

  async getById(id: string): Promise<Challenge | null> {
    try {
      const token = this.tokenProvider.getToken();
      if (!token) return null;

      const resp = await fetch(`${this.baseUrl}/challenges/${id}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (resp.status === 404) return null;
      if (!resp.ok) return null;
      const data = await resp.json();
      return mapSnakeToCamel(data);
    } catch {
      return null;
    }
  }
}

// Map snake_case API fields to camelCase domain model fields.
function mapSnakeToCamel(obj: Record<string, unknown>): Challenge {
  return {
    id: obj.id as string,
    title: obj.title as string,
    description: obj.description as string,
    difficulty: obj.difficulty as Challenge['difficulty'],
    category: obj.category as string,
    language: obj.language as string,
    repoUrl: (obj.repo_url ?? obj.repoUrl) as string,
    code: obj.code as string,
    codeSmell: (obj.code_smell ?? obj.codeSmell) as string,
    status: obj.status as Challenge['status'],
    createdAt: new Date(obj.created_at as string ?? obj.createdAt as string),
  };
}