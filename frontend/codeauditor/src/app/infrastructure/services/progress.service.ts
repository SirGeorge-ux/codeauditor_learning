// ProgressService — Angular injectable service wrapping ProgressUseCase.
//
// Creates HttpProgressRepository with AuthService token, exposes reactive
// signals from ProgressUseCase. Follows the ChallengeService pattern.
import { Injectable, inject, computed } from '@angular/core';
import { ProgressUseCase } from '../../application/progress.use-case';
import { HttpProgressRepository } from '../repositories/http-progress.repository';
import { AuthService } from './auth.service';
import { LanguageProgress } from '../../domain/models/language-progress';
import { LearningProfile, Preferencias, EstiloAprendizaje } from '../../domain/models/learning-profile';

@Injectable({ providedIn: 'root' })
export class ProgressService {
  private useCase: ProgressUseCase;

  readonly languages = computed(() => this.useCase.languages());
  readonly rangoGlobal = computed(() => this.useCase.rangoGlobal());
  readonly learningProfile = computed(() => this.useCase.learningProfile());
  readonly loading = computed(() => this.useCase.loading());
  readonly error = computed(() => this.useCase.error());

  constructor() {
    const authService = inject(AuthService);
    const repo = new HttpProgressRepository({
      getToken: () => authService.getAccessToken(),
      getUserId: () => authService.userSignal()?.id ?? null,
    });
    this.useCase = new ProgressUseCase(repo);
  }

  async loadProgress(): Promise<void> {
    return this.useCase.loadProgress();
  }

  async loadLearningProfile(): Promise<void> {
    return this.useCase.loadLearningProfile();
  }

  async saveLearningProfile(
    preferencias?: Partial<Preferencias>,
    estiloAprendizaje?: Partial<EstiloAprendizaje>,
  ): Promise<void> {
    return this.useCase.saveLearningProfile(preferencias, estiloAprendizaje);
  }

  updatePreferencias(preferencias: Preferencias): void {
    this.useCase.updatePreferencias(preferencias);
  }

  updateEstilo(estiloAprendizaje: EstiloAprendizaje): void {
    this.useCase.updateEstilo(estiloAprendizaje);
  }
}