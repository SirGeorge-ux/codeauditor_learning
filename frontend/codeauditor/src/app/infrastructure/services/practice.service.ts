// practice.service.ts — Angular injectable that wires PracticeUseCase
// with the concrete MCPClientAdapter + AuthService. Components depend
// on this single Injectable and read reactive signals.
//
// Pattern matches TutorChatService + DictionaryService: signal
// exposure + adapter wiring in the constructor; the use case is
// kept framework-free so the application layer can be unit-tested
// without Angular (see practice.use-case.spec.ts).
import { Injectable, inject, computed } from '@angular/core';

import { Challenge } from '../../domain/models/challenge';
import { CurriculumTopic } from '../../domain/models/curriculum';
import {
  PracticeUseCase,
  PRACTICE_DAILY_CAP,
  DAILY_COUNT_KEY_PREFIX,
  TOOL_GENERATE_CHALLENGE,
  TOOL_GET_TOPICS,
} from '../../application/practice.use-case';
import { MCPClientAdapter } from '../adapters/mcp-client.adapter';

@Injectable({ providedIn: 'root' })
export class PracticeService {
  private readonly useCase: PracticeUseCase;

  // Public signals — components bind to these via readonly computed
  // views so the service exposes a stable surface even if the use case
  // refactors its internal signal names.
  readonly language = computed(() => this.useCase.language());
  readonly topic = computed(() => this.useCase.topic());
  readonly level = computed(() => this.useCase.level());
  readonly topics = computed(() => this.useCase.topics());
  readonly loading = computed(() => this.useCase.loading());
  readonly error = computed(() => this.useCase.error());
  readonly generatedChallenge = computed(() => this.useCase.generatedChallenge());
  readonly remainingToday = computed(() => this.useCase.remainingToday());
  readonly canGenerate = computed(() => this.useCase.canGenerate());
  readonly dailyCount = computed(() => this.useCase.dailyCount());

  constructor() {
    const mcp = inject(MCPClientAdapter);
    this.useCase = new PracticeUseCase({ mcp });
  }

  // The page calls these methods; the service is a thin wrapper that
  // keeps the construction wiring in one place.
  setLanguage(value: string): void {
    this.useCase.setLanguage(value);
  }

  setTopic(value: string): void {
    this.useCase.setTopic(value);
  }

  setLevel(value: string): void {
    this.useCase.setLevel(value);
  }

  async loadTopics(language: string): Promise<void> {
    return this.useCase.loadTopics(language);
  }

  async generateChallenge(): Promise<Challenge | null> {
    return this.useCase.generateChallenge();
  }

  clearError(): void {
    this.useCase.clearError();
  }

  resetGeneratedChallenge(): void {
    this.useCase.resetGeneratedChallenge();
  }

  // The F..S band is exported so the level-selector component can
  // render the same ordered list as the use case's validation logic.
  readonly levels = PracticeUseCase.F_S_LEVELS;

  // Re-export the cap + prefix so tests / docs have a stable surface.
  static readonly PRACTICE_DAILY_CAP = PRACTICE_DAILY_CAP;
  static readonly DAILY_COUNT_KEY_PREFIX = DAILY_COUNT_KEY_PREFIX;
  static readonly TOOL_GENERATE_CHALLENGE = TOOL_GENERATE_CHALLENGE;
  static readonly TOOL_GET_TOPICS = TOOL_GET_TOPICS;
}

// Re-export CurriculumTopic so callers don't reach across the
// hexagonal boundary to import the domain model directly.
export type { CurriculumTopic };
