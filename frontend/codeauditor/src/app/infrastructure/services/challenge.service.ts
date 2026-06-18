// ChallengeService — Angular injectable service orchestrating challenge use case.
//
// Exposes reactive signals for challenges list, selected challenge, and loading state.
// Uses ChallengeUseCase (application layer) which delegates to HttpChallengeRepository.
// No more tempChallenges in-memory — imports are persisted via the backend.
import { Injectable, inject, signal } from '@angular/core';
import { ChallengeUseCase } from '../../application/challenge.use-case';
import { HttpChallengeRepository } from '../repositories/http-challenge.repository';
import { AuthService } from './auth.service';
import { Challenge } from '../../domain/models/challenge';

@Injectable({ providedIn: 'root' })
export class ChallengeService {
  private useCase: ChallengeUseCase;

  challengesSignal = signal<Challenge[]>([]);
  selectedChallengeSignal = signal<Challenge | null>(null);
  loadingSignal = signal(false);

  constructor(useCase?: ChallengeUseCase) {
    if (useCase) {
      this.useCase = useCase;
    } else {
      const authService = inject(AuthService);
      const repo = new HttpChallengeRepository({
        getToken: () => authService.getAccessToken(),
      });
      this.useCase = new ChallengeUseCase(repo);
    }
  }

  async loadChallenges(): Promise<void> {
    this.loadingSignal.set(true);
    try {
      const challenges = await this.useCase.loadChallenges();
      this.challengesSignal.set(challenges);
    } finally {
      this.loadingSignal.set(false);
    }
  }

  async selectChallenge(id: string): Promise<void> {
    this.loadingSignal.set(true);
    try {
      const challenge = await this.getChallenge(id);
      this.selectedChallengeSignal.set(challenge);
    } finally {
      this.loadingSignal.set(false);
    }
  }

  async getChallenge(id: string): Promise<Challenge | null> {
    return this.useCase.selectChallenge(id);
  }

  async importChallenge(input: Omit<Challenge, 'id' | 'createdAt' | 'status'>): Promise<string> {
    const challenge = await this.useCase.createChallenge(input);
    await this.loadChallenges();
    return challenge.id;
  }
}