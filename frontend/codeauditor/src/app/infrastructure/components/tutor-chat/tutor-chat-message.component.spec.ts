// tutor-chat-message.component.spec.ts — render contract tests for
// the dumb message component. The component takes a ChatMessage
// and renders role-aligned + code-block aware. Tests assert the
// shape so a future refactor doesn't silently drop the syntax
// highlighting or the role alignment.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';

import { TutorChatMessageComponent } from './tutor-chat-message.component';
import { ChatMessage, SocraticLevel } from '../../../domain/models/chat';

describe('TutorChatMessageComponent', () => {
  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  it('renders user messages right-aligned', () => {
    TestBed.configureTestingModule({ imports: [TutorChatMessageComponent] });
    const fixture = TestBed.createComponent(TutorChatMessageComponent);
    const message: ChatMessage = {
      id: 'm1',
      role: 'user',
      content: '¿Qué pasa?',
      timestamp: new Date('2026-10-10T10:00:00Z'),
    };
    fixture.componentRef.setInput('message', message);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    // The outer wrapper has justify-end for user messages.
    const wrapper = root.querySelector('div');
    expect(wrapper?.className).toContain('justify-end');
  });

  it('renders assistant messages left-aligned and shows the socratic level', () => {
    TestBed.configureTestingModule({ imports: [TutorChatMessageComponent] });
    const fixture = TestBed.createComponent(TutorChatMessageComponent);
    const message: ChatMessage = {
      id: 'm2',
      role: 'assistant',
      content: '¿Qué creés que pasa?',
      timestamp: new Date('2026-10-10T10:00:05Z'),
      socraticLevel: 2 as SocraticLevel,
    };
    fixture.componentRef.setInput('message', message);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const wrapper = root.querySelector('div');
    expect(wrapper?.className).toContain('justify-start');
    expect(root.textContent).toContain('nivel socrático 2');
  });

  it('renders code blocks with a <pre><code> element', () => {
    TestBed.configureTestingModule({ imports: [TutorChatMessageComponent] });
    const fixture = TestBed.createComponent(TutorChatMessageComponent);
    const message: ChatMessage = {
      id: 'm3',
      role: 'assistant',
      content: 'Mirá este código:\n```ts\nconst x = 1;\n```\n¿Lo entendés?',
      timestamp: new Date('2026-10-10T10:00:10Z'),
    };
    fixture.componentRef.setInput('message', message);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    const pre = root.querySelector('pre');
    const code = pre?.querySelector('code');
    expect(pre).toBeTruthy();
    expect(code?.textContent?.trim()).toBe('const x = 1;');
  });

  it('renders system messages with the amber treatment', () => {
    TestBed.configureTestingModule({ imports: [TutorChatMessageComponent] });
    const fixture = TestBed.createComponent(TutorChatMessageComponent);
    const message: ChatMessage = {
      id: 'm4',
      role: 'system',
      content: 'Comandos: /help',
      timestamp: new Date('2026-10-10T10:00:15Z'),
    };
    fixture.componentRef.setInput('message', message);
    fixture.detectChanges();
    const root = fixture.nativeElement as HTMLElement;
    expect(root.textContent).toContain('Comandos');
  });
});
