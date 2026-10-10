// practice-page.component.spec.ts — minimal smoke test for the
// practice page stub. The full S3 implementation will replace
// this; for now we only assert the page renders and the chat
// panel mounts.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { signal } from '@angular/core';

import { PracticePageComponent } from './practice-page.component';
import { TutorChatService } from '../../services/tutor-chat.service';
import { ChatMessage, SocraticLevel } from '../../../domain/models/chat';

function makeStubService(): TutorChatService {
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
  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  it('renders the placeholder and mounts the chat panel', () => {
    TestBed.configureTestingModule({
      imports: [PracticePageComponent],
      providers: [provideRouter([]), { provide: TutorChatService, useValue: makeStubService() }],
    });
    const fixture = TestBed.createComponent(PracticePageComponent);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.textContent).toContain('Modo práctica libre');
    expect(root.querySelector('[data-testid="tutor-chat-panel"]')).toBeTruthy();
  });
});
