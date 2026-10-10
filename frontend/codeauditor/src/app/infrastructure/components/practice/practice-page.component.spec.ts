// practice-page.component.spec.ts — page-level tests for the Free
// Practice Mode UI. Mocks PracticeService directly so the test does
// not depend on the MCP client / AuthService wiring.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { signal } from '@angular/core';

import { PracticePageComponent } from './practice-page.component';
import { PracticeService } from '../../services/practice.service';
import { TutorChatService } from '../../services/tutor-chat.service';
import { MCP_TOKEN_PROVIDER } from '../../adapters/mcp-client.adapter';
import { Challenge } from '../../../domain/models/challenge';
import { ChatMessage, SocraticLevel } from '../../../domain/models/chat';
import { CurriculumTopic } from '../../../domain/models/curriculum';

// Stub PracticeService — drives the page signals directly.
class StubPracticeService {
  language = signal<string>('');
  topic = signal<string>('');
  level = signal<string>('F');
  topics = signal<CurriculumTopic[]>([]);
  loading = signal<boolean>(false);
  error = signal<string | null>(null);
  generatedChallenge = signal<Challenge | null>(null);
  remainingToday = signal<number>(20);
  canGenerate = signal<boolean>(false);
  dailyCount = signal<number>(0);

  levels = ['F', 'E', 'D', 'C', 'B', 'A', 'S'];

  setLanguage(value: string): void {
    this.language.set(value);
    this.topic.set('');
    this.topics.set([
      { id: 'inheritance', title: 'Inheritance', difficulty: 'D', description: 'OOP basics' },
    ]);
  }
  setTopic(value: string): void {
    this.topic.set(value);
    this.canGenerate.set(this.language().length > 0 && value.length > 0);
  }
  setLevel(value: string): void {
    this.level.set(value);
  }
  async loadTopics(_language: string): Promise<void> {
    this.topics.set([
      { id: 'inheritance', title: 'Inheritance', difficulty: 'D', description: 'OOP basics' },
    ]);
    return Promise.resolve();
  }
  async generateChallenge(): Promise<Challenge | null> {
    this.loading.set(true);
    const challenge: Challenge = {
      id: 'gen-99',
      title: 'Refactor the inheritance chain',
      description: 'Tight coupling.',
      difficulty: 'mid',
      category: 'oop',
      language: this.language() || 'typescript',
      code: 'class Animal {}',
      status: 'available',
      createdAt: new Date('2026-10-10'),
      createdBy: 'u-1',
      learningObjectives: [],
      hints: [],
      commonMistakes: [],
      estimatedTimeMinutes: 30,
      expectedFindings: [],
      testCases: [],
      linterRules: [],
      basePoints: 100,
      bonusPoints: 20,
      penaltyPerHint: 5,
      timeBonus: true,
      origin: 'generated',
      generatedBy: 'free-practice-v1',
    };
    this.loading.set(false);
    this.generatedChallenge.set(challenge);
    return Promise.resolve(challenge);
  }
  clearError(): void {
    this.error.set(null);
  }
  resetGeneratedChallenge(): void {
    this.generatedChallenge.set(null);
  }
}

function makeStubTutorChatService(): TutorChatService {
  const messages = signal<ChatMessage[]>([]);
  const currentLevel = signal<SocraticLevel>(2);
  const streaming = signal<boolean>(false);
  const error = signal<string | null>(null);
  const lastOfferHint = signal<boolean>(false);
  const stub: Partial<TutorChatService> = {
    messages: messages.asReadonly(),
    currentLevel: currentLevel.asReadonly(),
    streaming: streaming.asReadonly(),
    error: error.asReadonly(),
    lastOfferHint: lastOfferHint.asReadonly(),
    sendMessage: async () => undefined,
    handleCommand: async () => undefined,
    registerNavigateHook: () => undefined,
  };
  return stub as unknown as TutorChatService;
}

describe('PracticePageComponent', () => {
  let practice: StubPracticeService;

  beforeEach(() => {
    TestBed.resetTestingModule();
    practice = new StubPracticeService();
  });

  function configure(): void {
    TestBed.configureTestingModule({
      imports: [PracticePageComponent],
      providers: [
        provideRouter([]),
        { provide: TutorChatService, useValue: makeStubTutorChatService() },
        { provide: PracticeService, useValue: practice },
        // Phase 5 (free-practice-mode): the production
        // PracticeService injects MCPClientAdapter which reads
        // MCP_TOKEN_PROVIDER. We stub both services via useValue,
        // so neither reaches the provider at runtime — but
        // registering a static stub here means any future edit
        // that touches a child that asks for MCP_TOKEN_PROVIDER
        // (not just the stub) will not silently leak the
        // AuthService chain. Belt-and-suspenders against the
        // practice-vs-auth.service.spec.ts flake.
        {
          provide: MCP_TOKEN_PROVIDER,
          useValue: { getToken: (): string | null => 'fake-token' },
        },
      ],
    });
  }

  it('renders the four-section layout (language / topic / level / generated-challenge)', () => {
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('app-language-selector')).toBeTruthy();
    expect(root.querySelector('app-topic-list')).toBeTruthy();
    expect(root.querySelector('app-level-selector')).toBeTruthy();
    expect(root.querySelector('app-generated-challenge')).toBeTruthy();
  });

  it('mounts the tutor chat panel', () => {
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="tutor-chat-panel"]')).toBeTruthy();
  });

  it('shows the daily-cap notice when remainingToday is 0', () => {
    practice.remainingToday.set(0);
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="practice-daily-cap-notice"]')).toBeTruthy();
  });

  it('surfaces service errors in the page banner', () => {
    practice.error.set('backend boom');
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const banner = root.querySelector('[data-testid="practice-error-banner"]');
    expect(banner).toBeTruthy();
    expect(banner?.textContent).toMatch(/backend boom/i);
  });

  it('hides the error banner after clearError()', () => {
    practice.error.set('something');
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    fixture.componentInstance.onClearError();
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="practice-error-banner"]')).toBeFalsy();
  });

  it('renders language options in the language-selector', () => {
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const langRow = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="language-selector-row"][data-id="typescript"]',
    );
    expect(langRow).toBeTruthy();
  });

  it('renders topic rows after language selection', () => {
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    fixture.componentInstance.onLanguageChange('typescript');
    fixture.detectChanges();
    const topicRows = (fixture.nativeElement as HTMLElement).querySelectorAll(
      '[data-testid="topic-list-row"]',
    );
    expect(topicRows.length).toBeGreaterThan(0);
  });

  it('disables the Generate button while loading', () => {
    practice.loading.set(true);
    configure();
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const btn = (fixture.nativeElement as HTMLElement).querySelector(
      '[data-testid="practice-generate-button"]',
    ) as HTMLButtonElement | null;
    expect(btn).toBeTruthy();
    expect(btn?.disabled).toBe(true);
  });
});
