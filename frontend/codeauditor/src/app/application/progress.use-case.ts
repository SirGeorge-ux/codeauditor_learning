// ProgressUseCase — application service orchestrating language progress and
// learning profile operations.
//
// Responsibilities:
// 1. Load all language progress + global rank
// 2. Load the learning profile
// 3. Save learning profile changes
//
// Uses signals for reactive state. Delegates to ProgressRepository port.
// Zero framework imports for the logic; uses Angular's signal() for reactivity.
import { signal, computed } from '@angular/core';
import { LanguageProgress, AllProgressResponse } from '../domain/models/language-progress';
import { LearningProfile, Preferencias, EstiloAprendizaje } from '../domain/models/learning-profile';
import { ProgressRepository } from '../domain/ports/progress-repository.port';

export class ProgressUseCase {
  // Reactive state
  private readonly _progress = signal<AllProgressResponse>({ rangoGlobal: 'F', languages: [] });
  private readonly _learningProfile = signal<LearningProfile | null>(null);
  private readonly _loading = signal(false);
  private readonly _error = signal<string | null>(null);

  // Public readonly signals
  readonly progress = this._progress.asReadonly();
  readonly languages = computed(() => this._progress().languages);
  readonly rangoGlobal = computed(() => this._progress().rangoGlobal);
  readonly learningProfile = this._learningProfile.asReadonly();
  readonly loading = this._loading.asReadonly();
  readonly error = this._error.asReadonly();

  constructor(private readonly repo: ProgressRepository) {}

  async loadProgress(): Promise<void> {
    this._loading.set(true);
    this._error.set(null);
    try {
      const result = await this.repo.getAllLanguageProgress();
      this._progress.set(result);
    } catch (e) {
      this._error.set(e instanceof Error ? e.message : 'Failed to load progress');
    } finally {
      this._loading.set(false);
    }
  }

  async loadLearningProfile(): Promise<void> {
    this._loading.set(true);
    this._error.set(null);
    try {
      const profile = await this.repo.getLearningProfile();
      this._learningProfile.set(profile);
    } catch (e) {
      this._error.set(e instanceof Error ? e.message : 'Failed to load learning profile');
    } finally {
      this._loading.set(false);
    }
  }

  async saveLearningProfile(
    preferencias?: Partial<Preferencias>,
    estiloAprendizaje?: Partial<EstiloAprendizaje>,
  ): Promise<void> {
    this._loading.set(true);
    this._error.set(null);
    try {
      const updated = await this.repo.updateLearningProfile(preferencias, estiloAprendizaje);
      if (updated) {
        this._learningProfile.set(updated);
      }
    } catch (e) {
      this._error.set(e instanceof Error ? e.message : 'Failed to save learning profile');
    } finally {
      this._loading.set(false);
    }
  }

  // Optimistic local updates for the learning profile — these do NOT call the
  // API. They update the signal in-place so the UI re-renders immediately.
  // Call saveLearningProfile() to persist.
  updatePreferencias(preferencias: Preferencias): void {
    const current = this._learningProfile();
    if (!current) return;
    this._learningProfile.set({ ...current, preferencias });
  }

  updateEstilo(estiloAprendizaje: EstiloAprendizaje): void {
    const current = this._learningProfile();
    if (!current) return;
    this._learningProfile.set({ ...current, estiloAprendizaje });
  }
}