// MCPClientAdapter — Angular infrastructure adapter for the MCP
// monolith (backend/internal/infrastructure/mcp/* tool groups served
// by backend/internal/mcpkit/server.go).
//
// Implements MCPClientPort with:
//   - listTools()  → GET  /mcp/tools/list  via HttpClient
//   - callTool()   → POST /mcp/tools/call  via HttpClient (JSON-RPC 2.0)
//   - streamTool() → GET  /mcp/events      via native fetch + SSE
//
// HttpClient drives the request/response pair; the SSE endpoint uses
// native fetch + ReadableStream because Angular's HttpClient does not
// expose a streaming response body. The same split lives in
// audit.service.ts for the audit SSE.
//
// Authorization: the Bearer JWT is pulled from the injected
// TokenProvider (consistent with HttpDictionaryRepository /
// HttpProgressRepository / HttpChallengeRepository). The Go server's
// mcpkit.Server.Router wires the JWT into the tool execution context
// via httpContextBridge so tools that read user_id (most mcp-user
// and mcp-dictionary tools) see the same identity the REST endpoints
// see.
//
// Defensive JSON-RPC unwrapping: the current Go dispatcher writes the
// raw tool result to the HTTP body. Future versions may wrap it in
// {jsonrpc, id, result}; the adapter unwraps both shapes without
// needing a migration.
import { Injectable, InjectionToken, NgZone, inject } from '@angular/core';
import { HttpClient, HttpErrorResponse, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

import { MCPTool, MCPClientPort } from '../../domain/ports/mcp-client.port';
import { environment } from '../../../environments/environment';

/**
 * TokenProvider is the minimal contract the adapter needs to stamp the
 * Authorization header. Matches the pattern in
 * infrastructure/repositories/http-dictionary.repository.ts and
 * http-progress.repository.ts so a single auth service can satisfy
 * every consumer.
 */
export interface TokenProvider {
  getToken(): string | null;
}

/**
 * Injection token for the TokenProvider. Wired in
 * app.config.ts (or wherever the root providers are declared) so the
 * adapter can `inject(MCP_TOKEN_PROVIDER)` without depending on a
 * concrete auth service.
 */
export const MCP_TOKEN_PROVIDER = new InjectionToken<TokenProvider>('MCP_TOKEN_PROVIDER');

/**
 * MCPClientAdapter is the only piece of the application that knows
 * the concrete HTTP+SSE shape of the MCP monolith. Use cases depend
 * on the MCPClientPort interface and never reference this class
 * directly.
 */
@Injectable({ providedIn: 'root' })
export class MCPClientAdapter implements MCPClientPort {
  private readonly http = inject(HttpClient);
  private readonly zone = inject(NgZone);
  private readonly tokenProvider = inject(MCP_TOKEN_PROVIDER, { optional: false }) as TokenProvider;
  private readonly baseUrl = `${environment.apiUrl}/mcp`;

  /**
   * Builds the standard Authorization header map. Returns an empty
   * object when the token is null so the request is sent without an
   * Authorization header (the Go auth middleware then returns 401,
   * which surfaces to the caller as Observable error).
   */
  private authHeaders(): Record<string, string> {
    const token = this.tokenProvider.getToken();
    if (!token) {
      return { 'Content-Type': 'application/json' };
    }
    return {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    };
  }

  /**
   * GET /mcp/tools/list → MCPTool[]. The wire shape matches
   * mcpkit.server.toolListItem verbatim; the adapter does NOT
   * re-shape the parameters object so the LLM-facing JSON Schema is
   * identical to what the server emits.
   */
  listTools(): Observable<MCPTool[]> {
    return this.http.get<MCPTool[]>(`${this.baseUrl}/tools/list`, {
      headers: this.authHeaders(),
    });
  }

  /**
   * POST /mcp/tools/call with a JSON-RPC 2.0 envelope. Unwraps a
   * `{jsonrpc, id, result}` response when present so callers always
   * receive the tool's raw result (defensive: current Go server
   * doesn't wrap, but a future release might).
   */
  callTool<T = unknown>(name: string, args: unknown): Observable<T> {
    const body = {
      jsonrpc: '2.0',
      id: this.newRequestID(),
      method: 'tools/call',
      params: {
        name,
        arguments: args ?? {},
      },
    };
    return new Observable<T>((observer) => {
      const sub = this.http
        .post<unknown>(`${this.baseUrl}/tools/call`, body, {
          headers: this.authHeaders(),
        })
        .subscribe({
          next: (raw: unknown) => {
            const unwrapped = unwrapJSONRPC<T>(raw);
            observer.next(unwrapped);
            observer.complete();
          },
          error: (err: unknown) => observer.error(err),
        });
      return () => sub.unsubscribe();
    });
  }

  /**
   * GET /mcp/events as Server-Sent Events. Each `data:` frame is
   * parsed as JSON and emitted to the subscriber; the connection is
   * aborted when the subscriber unsubscribes so the Go handler's
   * r.Context().Done() branch fires and the stream tears down.
   *
   * Malformed frames are silently skipped so a single bad event
   * never breaks the whole stream — this mirrors the AuditService
   * SSE consumer in infrastructure/services/audit.service.ts.
   */
  streamTool<T = unknown>(name: string, args: unknown): Observable<T> {
    return new Observable<T>((observer) => {
      const controller = new AbortController();
      const headers: Record<string, string> = {};
      const token = this.tokenProvider.getToken();
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }
      headers['Accept'] = 'text/event-stream';

      // The Go dispatcher doesn't yet take stream args in the query
      // string; we encode them so a future revision of mcpkit that
      // reads ?tool=&arguments=... can pick them up without a client
      // change. Args are JSON-encoded so a complex object survives.
      const params = new HttpParams()
        .set('tool', name)
        .set('arguments', JSON.stringify(args ?? {}));

      const url = `${this.baseUrl}/events?${params.toString()}`;

      // Run the fetch work outside Angular's zone so the resulting
      // stream emissions don't trigger excessive change detection on
      // every SSE frame. Re-enter the zone to emit so subscribers
      // that rely on signal-based UI can update. This mirrors the
      // AuditService SSE pattern.
      this.zone.runOutsideAngular(() => {
        fetch(url, { method: 'GET', headers, signal: controller.signal })
          .then(async (response: Response) => {
            if (!response.ok) {
              this.zone.run(() =>
                observer.error(
                  new SSEError(
                    response.status,
                    response.statusText,
                    `SSE failed: ${response.status} ${response.statusText}`,
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
                  // Drain any trailing frame in the buffer so callers
                  // see the last event before complete().
                  const trailing = buffer.trim();
                  if (trailing) {
                    const parsed = parseSSEFrame<T>(trailing);
                    if (parsed !== undefined) {
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
                  const parsed = parseSSEFrame<T>(frame);
                  if (parsed !== undefined) {
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
      });

      return () => controller.abort();
    });
  }

  /**
   * Generates a JSON-RPC 2.0 request id. Uses crypto.randomUUID when
   * available (modern browsers + jsdom 28); falls back to a
   * time-based string so older runtimes don't break.
   */
  private newRequestID(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return `req-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}

/**
 * unwrapJSONRPC strips the JSON-RPC envelope from a response body so
 * the caller always receives the tool's raw result. If the input is
 * not a JSON-RPC envelope (i.e. lacks both `jsonrpc` and `result`
 * fields), it is returned unchanged so the current Go dispatcher —
 * which writes the raw tool result directly — works without changes.
 */
function unwrapJSONRPC<T>(raw: unknown): T {
  if (
    raw !== null &&
    typeof raw === 'object' &&
    'jsonrpc' in raw &&
    'result' in (raw as Record<string, unknown>)
  ) {
    return (raw as unknown as { result: T }).result;
  }
  return raw as T;
}

/**
 * parseSSEFrame extracts the `data:` payload from a single SSE event
 * block and parses it as JSON. Returns undefined for frames that are
 * missing the `data:` prefix or whose payload isn't valid JSON —
 * callers should treat that as "skip" so a single bad frame doesn't
 * break the stream.
 */
function parseSSEFrame<T>(frame: string): T | undefined {
  const lines = frame.split('\n');
  const dataLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith('data: ')) {
      dataLines.push(line.slice('data: '.length));
    } else if (line.startsWith('data:')) {
      // Tolerate "data:" without the trailing space.
      dataLines.push(line.slice('data:'.length));
    }
  }
  if (dataLines.length === 0) {
    return undefined;
  }
  const payload = dataLines.join('\n');
  try {
    return JSON.parse(payload) as T;
  } catch {
    return undefined;
  }
}

/**
 * SSEError carries the HTTP status from a failed SSE handshake so
 * subscribers can branch on it (mirrors Angular's HttpErrorResponse
 * shape for the callTool path). The `.status` field is intentionally
 * an `unknown` cast in tests because we never want callers importing
 * the internal type — they just inspect `.status` and move on.
 */
class SSEError extends Error {
  readonly status: number;
  readonly statusText: string;
  constructor(status: number, statusText: string, message: string) {
    super(message);
    this.name = 'SSEError';
    this.status = status;
    this.statusText = statusText;
  }
}

// HttpErrorResponse is re-exported so test files can import the type
// alongside the adapter without reaching into @angular/common/http.
export type { HttpErrorResponse };
export { HttpParams };
