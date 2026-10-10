// HttpTutorChatAdapter — Angular infrastructure adapter that
// streams tutor chat replies from POST /api/v1/tutor/chat.
//
// Implements TutorChatPort with a native fetch + ReadableStream
// SSE consumer (matches the AuditService and MCPClientAdapter
// pattern). Angular's HttpClient does not expose a streaming
// response body, so the SSE path goes through fetch directly.
//
// The token is passed in explicitly by the TutorChatService (the
// Angular injectable that wraps the use case). The use case reads
// the JWT from AuthService and forwards it; the adapter never
// reaches into the global injector for auth state. This keeps
// the adapter narrow and testable.
//
// Wire format (mirrors backend/internal/infrastructure/driving/handlers/tutor_handler.go):
//   - "event: <type>\ndata: <payload>\n\n" per frame
//   - types: status | token | done | error
//   - status payload: {"status":"<text>","level":<int>?}
//   - token payload:  {"text":"<chunk>"}
//   - done payload:   {}
//   - error payload:  {"message":"<text>"}
//
// The parser reads the `event:` line for the type and the
// `data:` line for the JSON payload, exactly like
// EventSource would.
import { Injectable, NgZone, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { TutorChatPort } from '../../domain/ports/tutor-chat.port';
import { SSEEvent, StreamChatRequest, normalizeSocraticLevel } from '../../domain/models/chat';

@Injectable({ providedIn: 'root' })
export class HttpTutorChatAdapter implements TutorChatPort {
  private readonly zone: NgZone = inject(NgZone);

  streamChat(req: StreamChatRequest, token: string): Observable<SSEEvent> {
    return new Observable<SSEEvent>((observer) => {
      const controller = new AbortController();
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      fetch('/api/v1/tutor/chat', {
        method: 'POST',
        headers,
        body: JSON.stringify(req),
        signal: controller.signal,
      })
        .then(async (response: Response) => {
          if (!response.ok) {
            this.zone.run(() =>
              observer.error(
                new TutorChatHTTPError(
                  response.status,
                  response.statusText,
                  `tutor chat failed: ${response.status} ${response.statusText}`,
                ),
              ),
            );
            return;
          }
          const reader = response.body?.getReader();
          if (!reader) {
            this.zone.run(() => observer.error(new Error('SSE: no response body')));
            return;
          }
          const decoder = new TextDecoder();
          let buffer = '';
          const pump = async (): Promise<void> => {
            try {
              const { done, value } = await reader.read();
              if (done) {
                // Drain trailing frame so callers see the last event
                // before complete().
                const trailing = buffer.trim();
                if (trailing) {
                  const parsed = parseSSEFrame(trailing);
                  if (parsed) {
                    this.zone.run(() => observer.next(parsed));
                  }
                }
                this.zone.run(() => observer.complete());
                return;
              }
              buffer += decoder.decode(value, { stream: true });
              // SSE events are delimited by a blank line. Split on
              // \n\n so partial frames stay in the buffer.
              const parts = buffer.split('\n\n');
              buffer = parts.pop() ?? '';
              for (const frame of parts) {
                const parsed = parseSSEFrame(frame);
                if (parsed) {
                  this.zone.run(() => observer.next(parsed));
                }
              }
              await pump();
            } catch (err) {
              this.zone.run(() => observer.error(err as Error));
            }
          };
          await pump();
        })
        .catch((err: unknown) => {
          if ((err as { name?: string })?.name !== 'AbortError') {
            this.zone.run(() => observer.error(err as Error));
          }
        });

      return () => controller.abort();
    });
  }
}

// parseSSEFrame extracts the event type + JSON payload from one
// SSE event block. Returns null for malformed frames so callers
// can skip without breaking the stream.
function parseSSEFrame(frame: string): SSEEvent | null {
  let eventType: string | null = null;
  const dataLines: string[] = [];
  for (const line of frame.split('\n')) {
    if (line.startsWith('event: ')) {
      eventType = line.slice('event: '.length).trim();
    } else if (line.startsWith('event:')) {
      eventType = line.slice('event:'.length).trim();
    } else if (line.startsWith('data: ')) {
      dataLines.push(line.slice('data: '.length));
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length));
    }
  }
  if (!eventType || dataLines.length === 0) {
    return null;
  }
  const payload = dataLines.join('\n');
  let parsed: unknown;
  try {
    parsed = JSON.parse(payload);
  } catch {
    return null;
  }
  return buildSSEEvent(eventType, parsed);
}

// buildSSEEvent maps a (type, payload) pair onto the SSEEvent
// discriminated union. Unknown event types are dropped so a future
// backend addition (e.g. "metric") doesn't crash the frontend.
function buildSSEEvent(type: string, payload: unknown): SSEEvent | null {
  switch (type) {
    case 'status':
      return {
        type: 'status',
        data: {
          level:
            typeof (payload as { level?: unknown })?.level === 'number'
              ? normalizeSocraticLevel((payload as { level: number }).level)
              : undefined,
          status: String((payload as { status?: unknown })?.status ?? ''),
        },
      };
    case 'token':
      return { type: 'token', data: { text: String((payload as { text?: unknown })?.text ?? '') } };
    case 'done':
      return { type: 'done', data: {} as Record<string, never> };
    case 'error':
      return {
        type: 'error',
        data: { message: String((payload as { message?: unknown })?.message ?? 'unknown error') },
      };
    default:
      return null;
  }
}

// TutorChatHTTPError carries the HTTP status from a failed SSE
// handshake so subscribers can branch on it (mirrors the
// SSEError pattern in MCPClientAdapter).
class TutorChatHTTPError extends Error {
  readonly status: number;
  readonly statusText: string;
  constructor(status: number, statusText: string, message: string) {
    super(message);
    this.name = 'TutorChatHTTPError';
    this.status = status;
    this.statusText = statusText;
  }
}
