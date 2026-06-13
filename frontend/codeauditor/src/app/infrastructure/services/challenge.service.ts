// ChallengeService — Angular injectable service orchestrating challenge use case.
//
// Exposes reactive signals for challenges list, selected challenge, and loading state.
// Uses ChallengeUseCase (application layer) which delegates to MockChallengeRepository.
// Also manages temporary challenges imported from Gogs repos (in-memory, not persisted).
import { Injectable, signal } from '@angular/core';
import { ChallengeUseCase } from '../../application/challenge.use-case';
import { MockChallengeRepository } from '../repositories/mock-challenge.repository';
import { Challenge } from '../../domain/models/challenge';

@Injectable({ providedIn: 'root' })
export class ChallengeService {
  private useCase = new ChallengeUseCase(new MockChallengeRepository());
  private tempChallenges = new Map<string, Challenge>();

  challengesSignal = signal<Challenge[]>([]);
  selectedChallengeSignal = signal<Challenge | null>(null);
  loadingSignal = signal(false);

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