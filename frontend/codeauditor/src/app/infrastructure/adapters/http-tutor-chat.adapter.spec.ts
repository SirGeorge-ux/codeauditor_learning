// http-tutor-chat.adapter.spec.ts — unit tests for the SSE
// HttpTutorChatAdapter. The adapter is the only piece that knows
// the concrete wire shape of POST /api/v1/tutor/chat; the use case
// depends on TutorChatPort (interface) and never imports this
// class directly.
//
// Test strategy:
//   - Use vi.stubGlobal('fetch', mockFetch) so we can return
//     canned SSE streams without a real HTTP server. Matches the
//     MCPClientAdapter pattern.
//   - Drive the port's streamChat() and assert each event type
//     surfaces through the Observable correctly.
//   - Verify the Authorization header is stamped from the token
//     parameter.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { NgZone } from '@angular/core';

import { HttpTutorChatAdapter } from './http-tutor-chat.adapter';
import { SSEEvent, StreamChatRequest } from '../../domain/models/chat';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

function sseResponse(frames: string[]): Response {
  const encoder = new TextEncoder();
  const stream = new ReadableStream<Uint8Array>({
    start(controller: ReadableStreamDefaultController<Uint8Array>) {
      for (const frame of frames) {
        controller.enqueue(encoder.encode(frame));
      }
      controller.close();
    },
  });
  return {
    ok: true,
    status: 200,
    body: stream,
  } as unknown as Response;
}

describe('HttpTutorChatAdapter', () => {
  let adapter: HttpTutorChatAdapter;
  const token = 'jwt-abc';

  beforeEach(() => {
    TestBed.resetTestingModule();
    TestBed.configureTestingModule({
      providers: [
        HttpTutorChatAdapter,
        {
          provide: NgZone,
          useValue: {
            run: (fn: () => unknown) => fn(),
            runOutsideAngular: (fn: () => unknown) => fn(),
          },
        },
      ],
    });
    vi.clearAllMocks();
    adapter = TestBed.inject(HttpTutorChatAdapter);
  });

  it('POSTs to /api/v1/tutor/chat with the request body and Bearer auth', async () => {
    mockFetch.mockResolvedValueOnce(
      sseResponse(['event: status\ndata: {"level":2,"status":"thinking"}\n\n']),
    );

    const req: StreamChatRequest = {
      message: 'hola',
      route: '/dojo/ch-sqli',
      challengeId: 'ch-sqli',
      codeSnippet: 'const x = 1;',
      output: 'audit-ok',
      preferredProvider: 'auto',
    };

    let received: SSEEvent | undefined;
    await new Promise<void>((resolve) => {
      adapter.streamChat(req, token).subscribe({
        next: (e) => {
          received = e;
        },
        error: () => undefined,
        complete: () => resolve(),
      });
    });

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/v1/tutor/chat');
    expect(init.method).toBe('POST');
    expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer jwt-abc');
    expect((init.headers as Record<string, string>)['Content-Type']).toContain('application/json');
    expect((init.headers as Record<string, string>)['Accept']).toBe('text/event-stream');
    const body = JSON.parse(init.body as string);
    expect(body).toEqual({
      message: 'hola',
      route: '/dojo/ch-sqli',
      challengeId: 'ch-sqli',
      codeSnippet: 'const x = 1;',
      output: 'audit-ok',
      preferredProvider: 'auto',
    });
    expect(received).toEqual({ type: 'status', data: { level: 2, status: 'thinking' } });
  });

  it('emits each parsed SSE frame in order until done', async () => {
    mockFetch.mockResolvedValueOnce(
      sseResponse([
        'event: status\ndata: {"level":1,"status":"thinking"}\n\n',
        'event: token\ndata: {"text":"a"}\n\n',
        'event: token\ndata: {"text":"b"}\n\n',
        'event: done\ndata: {}\n\n',
      ]),
    );

    const collected: SSEEvent[] = [];
    await new Promise<void>((resolve, reject) => {
      adapter.streamChat({ message: 'm', route: '/' }, token).subscribe({
        next: (e) => collected.push(e),
        error: reject,
        complete: () => resolve(),
      });
    });

    expect(collected.map((e) => e.type)).toEqual(['status', 'token', 'token', 'done']);
  });

  it('emits an error event when the server sends event: error', async () => {
    mockFetch.mockResolvedValueOnce(
      sseResponse(['event: error\ndata: {"message":"Cascade failed"}\n\n']),
    );

    const collected: SSEEvent[] = [];
    await new Promise<void>((resolve) => {
      adapter.streamChat({ message: 'm', route: '/' }, token).subscribe({
        next: (e) => collected.push(e),
        error: () => undefined,
        complete: () => resolve(),
      });
    });

    expect(collected).toHaveLength(1);
    expect(collected[0].type).toBe('error');
    if (collected[0].type === 'error') {
      expect(collected[0].data.message).toBe('Cascade failed');
    }
  });

  it('emits an Observable error on non-2xx HTTP status', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
    });

    let received: unknown;
    await new Promise<void>((resolve) => {
      adapter.streamChat({ message: 'm', route: '/' }, token).subscribe({
        next: () => undefined,
        error: (err: unknown) => {
          received = err;
          resolve();
        },
        complete: () => resolve(),
      });
    });

    expect(received).toBeDefined();
    expect((received as { status: number }).status).toBe(401);
  });

  it('skips malformed SSE frames', async () => {
    mockFetch.mockResolvedValueOnce(
      sseResponse(['event: token\ndata: not-json\n\n', 'event: token\ndata: {"text":"x"}\n\n']),
    );

    const collected: SSEEvent[] = [];
    await new Promise<void>((resolve, reject) => {
      adapter.streamChat({ message: 'm', route: '/' }, token).subscribe({
        next: (e) => collected.push(e),
        error: reject,
        complete: () => resolve(),
      });
    });

    expect(collected).toHaveLength(1);
    expect(collected[0].type).toBe('token');
  });
});
