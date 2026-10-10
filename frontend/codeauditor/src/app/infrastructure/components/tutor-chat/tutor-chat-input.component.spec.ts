// tutor-chat-input.component.spec.ts — interaction tests for the
// dumb input component. Verifies the textarea behavior:
//   - Enter emits (send) for plain text
//   - Enter emits (command) for slash commands
//   - Shift+Enter inserts a newline (no event emitted)
//   - Disabled while streaming
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';

import { TutorChatInputComponent } from './tutor-chat-input.component';

describe('TutorChatInputComponent', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  it('emits (send) when Enter is pressed with plain text', () => {
    TestBed.configureTestingModule({ imports: [TutorChatInputComponent] });
    const fixture = TestBed.createComponent(TutorChatInputComponent);
    const component = fixture.componentInstance;

    let received: string | null = null;
    component.send.subscribe((s: string) => {
      received = s;
    });

    const textarea: HTMLTextAreaElement = fixture.nativeElement.querySelector('textarea');
    textarea.value = 'hola';
    textarea.dispatchEvent(new Event('input'));
    textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));

    expect(received).toBe('hola');
    expect(textarea.value).toBe('');
  });

  it('emits (command) when Enter is pressed with a slash command', () => {
    TestBed.configureTestingModule({ imports: [TutorChatInputComponent] });
    const fixture = TestBed.createComponent(TutorChatInputComponent);
    const component = fixture.componentInstance;

    let received: string | null = null;
    component.command.subscribe((c: string) => {
      received = c;
    });

    const textarea: HTMLTextAreaElement = fixture.nativeElement.querySelector('textarea');
    textarea.value = '/help';
    textarea.dispatchEvent(new Event('input'));
    textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));

    expect(received).toBe('/help');
  });

  it('does NOT emit on Shift+Enter (newline is the default)', () => {
    TestBed.configureTestingModule({ imports: [TutorChatInputComponent] });
    const fixture = TestBed.createComponent(TutorChatInputComponent);
    const component = fixture.componentInstance;

    let sendCalls = 0;
    let commandCalls = 0;
    component.send.subscribe(() => {
      sendCalls++;
    });
    component.command.subscribe(() => {
      commandCalls++;
    });

    const textarea: HTMLTextAreaElement = fixture.nativeElement.querySelector('textarea');
    textarea.value = 'línea 1';
    textarea.dispatchEvent(new Event('input'));
    textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true }));

    expect(sendCalls).toBe(0);
    expect(commandCalls).toBe(0);
  });

  it('disables the textarea + button while streaming', () => {
    TestBed.configureTestingModule({ imports: [TutorChatInputComponent] });
    const fixture = TestBed.createComponent(TutorChatInputComponent);
    fixture.componentRef.setInput('streaming', true);
    fixture.detectChanges();
    const textarea: HTMLTextAreaElement = fixture.nativeElement.querySelector('textarea');
    const button: HTMLButtonElement = fixture.nativeElement.querySelector(
      'button[type="submit"]',
    );
    expect(textarea.disabled).toBe(true);
    expect(button.disabled).toBe(true);
  });

  it('trims whitespace and ignores empty input', () => {
    TestBed.configureTestingModule({ imports: [TutorChatInputComponent] });
    const fixture = TestBed.createComponent(TutorChatInputComponent);
    const component = fixture.componentInstance;

    let received: string | null = null;
    component.send.subscribe((s: string) => {
      received = s;
    });

    const textarea: HTMLTextAreaElement = fixture.nativeElement.querySelector('textarea');
    textarea.value = '   ';
    textarea.dispatchEvent(new Event('input'));
    textarea.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }));

    expect(received).toBeNull();
  });
});
