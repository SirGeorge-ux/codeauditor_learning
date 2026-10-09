// Tests for the MCPClientAdapter — the Angular infrastructure adapter that
// lets the application layer talk to the Go MCP monolith over HTTP+SSE.
//
// Contract (mirrors openspec/changes/mcp-monolith/specs/mcp/spec.md
// Requirement: MCPClientAdapter):
//   - listTools(): GET /mcp/tools/list → Observable<MCPTool[]>
//   - callTool<T>(name, args): POST /mcp/tools/call → Observable<T>
//   - streamTool<T>(name, args): GET /mcp/events → Observable<T> (SSE)
//
// The adapter lives in the infrastructure layer and is the only piece
// that imports @angular/common/http or uses native fetch. The
// MCPClientPort interface it implements is a pure-TS contract (no
// @angular/* imports) so use cases can be wired in unit tests against a
// fake MCPClientPort without HTTP mocks.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import {
  HttpTestingController,
  provideHttpClientTesting,
} from '@angular/common/http/testing';
import { HttpErrorResponse } from '@angular/common/http';

import { MCPClientAdapter } from './mcp-client.adapter';
import { MCPTool } from '../../domain/ports/mcp-client.port';
import { environment } from '../../../environments/environment';

// Mock global fetch — streamTool uses native fetch + ReadableStream for
// SSE so HttpClientTestingController can't intercept it (matching the
// AuditService SSE pattern). The adapter delegates streamTool to fetch
// because Angular HttpClient does not expose an SSE stream.
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

const tokenProvider = {
  getToken: vi.fn().mockReturnValue('valid-token'),
};

// MCPTool used as a typed alias in the next/error callbacks below.
type Tool = MCPTool;
type ToolResult = { status: string; result?: { ok: boolean } };

describe('MCPClientAdapter', () => {
  let adapter: MCPClientAdapter;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    vi.clearAllMocks();
    tokenProvider.getToken.mockReturnValue('valid-token');

    TestBed.configureTestingModule({
      providers: [
        MCPClientAdapter,
        { provide: 'MCP_TOKEN_PROVIDER', useValue: tokenProvider },
        provideHttpClient(withInterceptorsFromDi()),
        provideHttpClientTesting(),
      ],
    });

    adapter = TestBed.inject(MCPClientAdapter);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
    mockFetch.mockReset();
  });

  describe('listTools', () => {
    it('issues GET /mcp/tools/list with Bearer auth and returns mapped MCPTool[]', () => {
      const wireTools = [
        {
          name: 'mcp-pedagogical.get_socratic_prompt',
          description: 'Returns the next socratic prompt.',
          parameters: {
            type: 'object',
            properties: {
              level: { type: 'integer', description: 'Socratic level 0..3' },
            },
            required: ['level'],
          },
        },
        {
          name: 'mcp-sandbox.list_languages',
          description: 'Returns the language catalog.',
          parameters: { type: 'object', properties: {}, required: [] },
        },
      ];

      let received: Tool[] | undefined;
      adapter.listTools().subscribe({
        next: (tools: Tool[]) => {
          received = tools;
        },
        error: () => expect.unreachable('listTools should succeed'),
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/list`);
      expect(req.request.method).toBe('GET');
      expect(req.request.headers.get('Authorization')).toBe('Bearer valid-token');
      req.flush(wireTools);

      expect(received).toBeDefined();
      expect(received).toHaveLength(2);
      expect(received![0].name).toBe('mcp-pedagogical.get_socratic_prompt');
      expect(received![0].description).toContain('socratic');
      expect(received![0].parameters).toMatchObject({ type: 'object' });
      expect(received![1].name).toBe('mcp-sandbox.list_languages');
    });

    it('propagates 401 as Observable error', () => {
      let receivedError: HttpErrorResponse | undefined;
      adapter.listTools().subscribe({
        next: () => expect.unreachable('listTools should fail'),
        error: (err: HttpErrorResponse) => {
          receivedError = err;
        },
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/list`);
      req.flush('unauthorized', { status: 401, statusText: 'Unauthorized' });

      expect(receivedError).toBeDefined();
      expect(receivedError!.status).toBe(401);
    });

    it('omits Authorization header when token provider returns null', () => {
      tokenProvider.getToken.mockReturnValue(null);

      adapter.listTools().subscribe({
        next: () => undefined,
        error: () => expect.unreachable('listTools should succeed'),
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/list`);
      expect(req.request.headers.has('Authorization')).toBe(false);
      req.flush([]);
    });
  });

  describe('callTool', () => {
    it('issues POST /mcp/tools/call with JSON-RPC 2.0 envelope and returns Observable<T>', () => {
      const args: { level: number; language: string; topic_id: string } = {
        level: 2,
        language: 'typescript',
        topic_id: 'inheritance',
      };
      const toolResult = {
        prompt: '¿Qué pasaría si...?',
        level: 2,
        language: 'typescript',
      };

      let received: typeof toolResult | undefined;
      adapter.callTool<typeof toolResult>('mcp-pedagogical.get_socratic_prompt', args).subscribe({
        next: (result: typeof toolResult) => {
          received = result;
        },
        error: () => expect.unreachable('callTool should succeed'),
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/call`);
      expect(req.request.method).toBe('POST');
      const body = req.request.body as { jsonrpc: string; id: string; method: string; params: { name: string; arguments: unknown } };
      expect(body.jsonrpc).toBe('2.0');
      expect(body.method).toBe('tools/call');
      expect(body.params.name).toBe('mcp-pedagogical.get_socratic_prompt');
      expect(body.params.arguments).toEqual(args);
      expect(typeof body.id).toBe('string');
      expect(req.request.headers.get('Authorization')).toBe('Bearer valid-token');
      expect(req.request.headers.get('Content-Type')).toContain('application/json');
      req.flush(toolResult);

      expect(received).toEqual(toolResult);
    });

    it('returns the .result field when the response is a JSON-RPC envelope', () => {
      const wire = {
        jsonrpc: '2.0',
        id: 'req-1',
        result: { ok: true, count: 7 },
      };

      let received: { ok: boolean; count: number } | undefined;
      adapter.callTool<{ ok: boolean; count: number }>('mcp-dictionary.list_terms', {}).subscribe({
        next: (result: { ok: boolean; count: number }) => {
          received = result;
        },
        error: () => expect.unreachable('callTool should succeed'),
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/call`);
      req.flush(wire);

      expect(received).toEqual({ ok: true, count: 7 });
    });

    it('serializes null args as {}', () => {
      adapter.callTool('mcp-sandbox.healthcheck', null).subscribe({
        next: () => undefined,
        error: () => expect.unreachable('callTool should succeed'),
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/call`);
      const body = req.request.body as { params: { arguments: unknown } };
      expect(body.params.arguments).toEqual({});
      req.flush({ status: 'ok' });
    });

    it('propagates 5xx as Observable error', () => {
      let receivedError: HttpErrorResponse | undefined;
      adapter.callTool('mcp-pedagogical.get_socratic_prompt', { level: 0 }).subscribe({
        next: () => expect.unreachable('callTool should fail'),
        error: (err: HttpErrorResponse) => {
          receivedError = err;
        },
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/mcp/tools/call`);
      req.flush('boom', { status: 500, statusText: 'Internal Server Error' });

      expect(receivedError).toBeDefined();
      expect(receivedError!.status).toBe(500);
    });
  });

  describe('streamTool', () => {
    // Helper: build an SSE Response whose body yields the supplied frames
    // then closes. Each frame is a fully-formed SSE event block.
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

    it('opens SSE GET to /mcp/events with tool name + args in the URL', async () => {
      const args = { language: 'typescript', code: 'const x = 1;' };

      mockFetch.mockResolvedValueOnce(
        sseResponse([
          'event: message\ndata: {"status":"ready"}\n\n',
          'event: message\ndata: {"status":"completed","result":{"ok":true}}\n\n',
        ]),
      );

      const collected: ToolResult[] = [];
      await new Promise<void>((resolve, reject) => {
        adapter.streamTool<ToolResult>('mcp-sandbox.run_code', args).subscribe({
          next: (event: ToolResult) => collected.push(event),
          error: reject,
          complete: () => resolve(),
        });
      });

      expect(mockFetch).toHaveBeenCalledTimes(1);
      const url = mockFetch.mock.calls[0][0] as string;
      const init = mockFetch.mock.calls[0][1] as RequestInit;
      expect(url).toContain('/mcp/events');
      expect(url).toContain('tool=mcp-sandbox.run_code');
      expect((init.headers as Record<string, string>)['Authorization']).toBe('Bearer valid-token');
      expect(init.signal).toBeInstanceOf(AbortSignal);

      expect(collected).toEqual([
        { status: 'ready' },
        { status: 'completed', result: { ok: true } },
      ]);
    });

    it('completes cleanly when the stream closes without a trailing event', async () => {
      mockFetch.mockResolvedValueOnce(
        sseResponse(['event: ping\ndata: {"status":"alive"}\n\n']),
      );

      let completed = false;
      await new Promise<void>((resolve, reject) => {
        adapter.streamTool('mcp-sandbox.healthcheck', {}).subscribe({
          next: () => undefined,
          error: reject,
          complete: () => {
            completed = true;
            resolve();
          },
        });
      });

      expect(completed).toBe(true);
    });

    it('emits an error when fetch rejects', async () => {
      mockFetch.mockRejectedValueOnce(new Error('network down'));

      let received: Error | undefined;
      await new Promise<void>((resolve) => {
        adapter.streamTool('mcp-sandbox.healthcheck', {}).subscribe({
          next: () => undefined,
          error: (err: Error) => {
            received = err;
            resolve();
          },
          complete: () => resolve(),
        });
      });

      expect(received).toBeDefined();
      expect(received!.message).toBe('network down');
    });

    it('emits an error when the SSE response is non-OK', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 502,
        statusText: 'Bad Gateway',
      });

      let received: Error | undefined;
      await new Promise<void>((resolve) => {
        adapter.streamTool('mcp-sandbox.healthcheck', {}).subscribe({
          next: () => undefined,
          error: (err: Error) => {
            received = err;
            resolve();
          },
          complete: () => resolve(),
        });
      });

      expect(received).toBeDefined();
      // The adapter surfaces non-OK responses with status + statusText so
      // the caller can branch on the HTTP code without inspecting the
      // raw Response object.
      expect((received as unknown as { status: number }).status).toBe(502);
    });

    it('skips malformed SSE frames without breaking the stream', async () => {
      mockFetch.mockResolvedValueOnce(
        sseResponse([
          'event: message\ndata: not-json\n\n',
          'event: message\ndata: {"status":"ready"}\n\n',
        ]),
      );

      const collected: ToolResult[] = [];
      await new Promise<void>((resolve, reject) => {
        adapter.streamTool<ToolResult>('mcp-pedagogical.get_socratic_prompt', {}).subscribe({
          next: (event: ToolResult) => collected.push(event),
          error: reject,
          complete: () => resolve(),
        });
      });

      expect(collected).toEqual([{ status: 'ready' }]);
    });
  });

  describe('MCPTool contract shape (regression guard)', () => {
    it('exposes the MCPTool shape that the spec requires (name + description + parameters)', () => {
      // Compile-time regression guard — if any of these fields disappear
      // from MCPTool, the test file fails to compile, which is the
      // cheapest possible signal that the contract broke.
      const sample: MCPTool = {
        name: 'mcp-x.y',
        description: 'desc',
        parameters: { type: 'object' },
      };
      expect(sample.name).toBe('mcp-x.y');
      expect(sample.description).toBe('desc');
      expect(sample.parameters).toMatchObject({ type: 'object' });
    });
  });
});