// tutor-chat.port.spec.ts — compile-time contract tests for the
// TutorChatPort domain interface. Mirrors the chat.spec.ts pattern:
// a fake implementation is enough to prove the interface shape is
// usable; runtime behavior is tested in the use-case spec
// (application/tutor-chat.use-case.spec.ts).
import { describe, it, expect } from 'vitest';
import { Observable } from 'rxjs';

import { TutorChatPort } from './tutor-chat.port';
import { SSEEvent, StreamChatRequest } from '../models/chat';

describe('TutorChatPort', () => {
  it('accepts a fake implementation that satisfies the interface', () => {
    const fake: TutorChatPort = {
      streamChat(_req: StreamChatRequest, _token: string): Observable<SSEEvent> {
        return new Observable<SSEEvent>((observer) => {
          observer.next({ type: 'status', data: { level: 2, status: 'thinking' } });
          observer.next({ type: 'token', data: { text: 'hola' } });
          observer.next({ type: 'done', data: {} });
          observer.complete();
        });
      },
    };
    expect(typeof fake.streamChat).toBe('function');
  });

  it('emits all four event kinds through the stream', () => {
    const fake: TutorChatPort = {
      streamChat(_req: StreamChatRequest, _token: string): Observable<SSEEvent> {
        return new Observable<SSEEvent>((observer) => {
          observer.next({ type: 'status', data: { level: 1, status: 'escalated' } });
          observer.next({ type: 'token', data: { text: 'a' } });
          observer.next({ type: 'error', data: { message: 'x' } });
          observer.next({ type: 'done', data: {} });
          observer.complete();
        });
      },
    };

    const received: SSEEvent[] = [];
    fake.streamChat({ message: 'm', route: '/' }, 'tok').subscribe({
      next: (e) => received.push(e),
    });

    expect(received.map((e) => e.type)).toEqual(['status', 'token', 'error', 'done']);
  });
});
