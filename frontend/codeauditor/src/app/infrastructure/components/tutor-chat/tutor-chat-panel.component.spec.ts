// tutor-chat-panel.component.spec.ts — smart component tests for
// the tutor chat panel. The panel wires a TutorChatService
// (injectable) and forwards input events to the use case.
//
// Test strategy:
//   - Stub TutorChatService with a fake that exposes the same
//     public surface (signals + sendMessage + handleCommand).
//   - Assert the panel renders without crashing and that the
//     (send) output routes to the service.sendMessage method.
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';

import { TutorChatPanelComponent } from './tutor-chat-panel.component';
import { TutorChatService } from '../../services/tutor-chat.service';
import { ChatMessage, SocraticLevel } from '../../../domain/models/chat';

// Minimal stub of TutorChatService. Mirrors the public surface
// the panel + template consume.
function makeFakeService(): TutorChatService {
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
    sendMessage: vi.fn(async () => undefined),
    handleCommand: vi.fn(async () => undefined),
    registerNavigateHook: vi.fn(),
  };
  return stub as unknown as TutorChatService;
}

describe('TutorChatPanelComponent', () => {
  let fakeService: TutorChatService;
  let fixture: import('@angular/core/testing').ComponentFixture<TutorChatPanelComponent>;

  beforeEach(async () => {
    TestBed.resetTestingModule();
    fakeService = makeFakeService();
    await TestBed.configureTestingModule({
      imports: [TutorChatPanelComponent],
      providers: [{ provide: TutorChatService, useValue: fakeService }],
    }).compileComponents();

    fixture = TestBed.createComponent(TutorChatPanelComponent);
    fixture.componentRef.setInput('routeContext', { route: '/dojo/ch-x', challengeId: 'ch-x' });
    fixture.detectChanges();
  });

  it('renders the panel shell', () => {
    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[data-testid="tutor-chat-panel"]')).toBeTruthy();
    expect(root.textContent).toContain('Tutor');
  });

  it('shows the welcome state when messages is empty', () => {
    const root = fixture.nativeElement as HTMLElement;
    expect(root.textContent).toContain('Soy tu tutor socrático');
  });

  it('passes the streaming signal to the input child', () => {
    const root = fixture.nativeElement as HTMLElement;
    const textarea: HTMLTextAreaElement | null = root.querySelector('textarea');
    expect(textarea).toBeTruthy();
    // streaming defaults to false so the input is enabled.
    expect(textarea?.disabled).toBe(false);
  });

  it('calls service.sendMessage when the input child emits (send)', () => {
    const sendSpy = vi.spyOn(fakeService, 'sendMessage');
    // The panel is a smart orchestrator; it forwards (send) from
    // tutor-chat-input to service.sendMessage. We invoke the
    // public onSend method directly to assert the wiring.
    const component = fixture.componentInstance;
    component.onSend('hola');
    expect(sendSpy).toHaveBeenCalledWith(
      'hola',
      expect.objectContaining({ route: '/dojo/ch-x', challengeId: 'ch-x' }),
    );
  });

  it('calls service.handleCommand when the input child emits (command)', () => {
    const cmdSpy = vi.spyOn(fakeService, 'handleCommand');
    const component = fixture.componentInstance;
    component.onCommand('/help');
    expect(cmdSpy).toHaveBeenCalledWith('/help');
  });

  it('hides the panel host element when hidden input is true', () => {
    fixture.componentRef.setInput('hidden', true);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    // The host element (the <app-tutor-chat-panel> itself)
    // has the .hidden class binding.
    expect(root.classList.contains('hidden')).toBe(true);
  });
});
