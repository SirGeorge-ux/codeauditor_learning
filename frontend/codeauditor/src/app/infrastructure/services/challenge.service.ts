// ChallengeService — Angular injectable service orchestrating challenge use case.
//
// Exposes reactive signals for challenges list, selected challenge, and loading state.
// Uses ChallengeUseCase (application layer) which delegates to HttpChallengeRepository.
// Also manages temporary challenges imported from Gogs repos (in-memory, not persisted).
import { Injectable, inject, signal } from '@angular/core';
import { ChallengeUseCase } from '../../application/challenge.use-case';
import { HttpChallengeRepository } from '../repositories/http-challenge.repository';
import { AuthService } from './auth.service';
import { Challenge } from '../../domain/models/challenge';

@Injectable({ providedIn: 'root' })
export class ChallengeService {
  private useCase: ChallengeUseCase;
  private tempChallenges = new Map<string, Challenge>();

  challengesSignal = signal<Challenge[]>([]);
  selectedChallengeSignal = signal<Challenge | null>(null);
  loadingSignal = signal(false);

  constructor() {
    const authService = inject(AuthService);
    const repo = new HttpChallengeRepository({
      getToken: () => authService.getAccessToken(),
    });
    this.useCase = new ChallengeUseCase(repo);
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
    const temp = this.tempChallenges.get(id);
    if (temp) return temp;
    return this.useCase.selectChallenge(id);
  }

  addTempChallenge(challenge: Challenge): string {
    const id = `temp-${Date.now()}`;
    this.tempChallenges.set(id, { ...challenge, id });
    return id;
  }
}