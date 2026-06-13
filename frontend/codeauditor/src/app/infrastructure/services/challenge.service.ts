// ChallengeService — Angular injectable service orchestrating challenge use case.
//
// Exposes reactive signals for challenges list, selected challenge, and loading state.
// Uses ChallengeUseCase (application layer) which delegates to MockChallengeRepository.
import { Injectable, signal } from "@angular/core";
import { ChallengeUseCase } from "../../application/challenge.use-case";
import { MockChallengeRepository } from "../repositories/mock-challenge.repository";
import { Challenge } from "../../domain/models/challenge";

@Injectable({ providedIn: "root" })
export class ChallengeService {
  private useCase = new ChallengeUseCase(new MockChallengeRepository());

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
      const challenge = await this.useCase.selectChallenge(id);
      this.selectedChallengeSignal.set(challenge);
    } finally {
      this.loadingSignal.set(false);
    }
  }

  async getChallenge(id: string): Promise<Challenge | null> {
    return this.useCase.selectChallenge(id);
  }
}