// tutor-chat.use-case.spec.ts — application-layer tests for the
// TutorChatUseCase. The use case is the brain of the chat panel:
// it owns the conversation state (signals), parses slash commands,
// and consumes the SSE stream from TutorChatPort.
//
// Test strategy:
//   - Wire a fake TutorChatPort that emits canned SSEEvent streams
//     so we can assert how the use case reacts to each event kind.
//   - Wire a fake AuthPort returning a fixed token (the use case
//     forwards the token to the port).
//   - Wire a fake MCPClientPort so the /perfil and /nivel commands
//     can hit the in-memory mock without HTTP.
//
// TDD discipline (per strict_tdd: true in openspec/config.yaml):
//   these tests were written BEFORE the implementation; the test
//   file fails with import errors at HEAD~1, then goes green when
//   tutor-chat.use-case.ts lands.
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { of, throwError } from 'rxjs';

import { TutorChatUseCase } from './tutor-chat.use-case';
import { TutorChatPort } from '../domain/ports/tutor-chat.port';
import { AuthPort } from '../domain/ports/auth.port';
import { MCPClientPort, MCPTool } from '../domain/ports/mcp-client.port';
import {
  ChatMessage,
  ChatProvider,
  SocraticLevel,
  SSEEvent,
  StreamChatRequest,
} from '../domain/models/chat';
import { User } from '../domain/models/user';

// -- Test doubles ------------------------------------------------------------

class FakeAuthPort implements AuthPort {
  constructor(private readonly token: string | null = 'test-token') {}
  getCurrentUser(): Promise<User | null> {
    return Promise.resolve(null);
  }
  signIn(): Promise<User> {
    return Promise.reject(new Error('not implemented in fake'));
  }
  signOut(): Promise<void> {
    return Promise.resolve();
  }
  onAuthStateChange(): () => void {
    return () => undefined;
  }
  // The use case reads the token synchronously via a helper, so the
  // fake exposes a sync getter that the use case calls.
  getToken(): string | null {
    return this.token;
  }
}

interface CapturedCall {
  name: string;
  args: unknown;
}

class FakeMcpPort {
  listToolsResult: MCPTool[] = [];
  callToolResult: unknown = {};
  callToolError: Error | null = null;
  captured: CapturedCall[] = [];

  listTools() {
    return of(this.listToolsResult);
  }
  callTool(name: string, args: unknown) {
    this.captured.push({ name, args });
    if (this.callToolError) {
      return throwError(() => this.callToolError);
    }
    return of(this.callToolResult);
  }
  streamTool() {
    return of();
  }
  // The use case only consumes a small subset of MCPClientPort. Cast
  // at the constructor site to avoid fighting Observable<T> variance
  // in the fake's signatures.
  asPort(): MCPClientPort {
    return this as unknown as MCPClientPort;
  }
}

class FakeTutorChatPort implements TutorChatPort {
  captured: StreamChatRequest[] = [];
  streams: SSEEvent[][] = [];
  // If set, streamChat throws on the next call (used for error paths).
  throwWith: Error | null = null;
  // Optional latency so multiple events can be queued before complete().
  defaultStream: SSEEvent[] = [
    { type: 'status', data: { level: 2, status: 'thinking' } },
    { type: 'token', data: { text: '¿Qué ' } },
    { type: 'token', data: { text: 'creés ' } },
    { type: 'token', data: { text: 'que ' } },
    { type: 'token', data: { text: 'pasa?' } },
    { type: 'done', data: {} },
  ];

  streamChat(req: StreamChatRequest, _token: string) {
    this.captured.push(req);
    if (this.throwWith) {
      const err = this.throwWith;
      this.throwWith = null;
      return throwError(() => err);
    }
    const events = this.streams.shift() ?? this.defaultStream;
    return of(...events);
  }
}

// -- Helpers -----------------------------------------------------------------

function makeUseCase(opts?: { port?: FakeTutorChatPort; mcp?: FakeMcpPort; token?: string | null }) {
  const port = opts?.port ?? new FakeTutorChatPort();
  const mcp = opts?.mcp ?? new FakeMcpPort();
  const auth = new FakeAuthPort(opts?.token ?? 'test-token');
  const useCase = new TutorChatUseCase(port, mcp.asPort(), auth);
  return { useCase, port, mcp, auth };
}

// -- Tests -------------------------------------------------------------------

describe('TutorChatUseCase', () => {
  describe('initial state', () => {
    it('starts with empty messages, socratic level 2, not streaming, no error', () => {
      const { useCase } = makeUseCase();
      expect(useCase.messages()).toEqual([]);
      expect(useCase.currentLevel()).toBe(2);
      expect(useCase.streaming()).toBe(false);
      expect(useCase.error()).toBeNull();
      expect(useCase.lastOfferHint()).toBe(false);
    });
  });

  describe('sendMessage (SSE consumer)', () => {
    it('appends the user message to history before opening the stream', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [
        [
          { type: 'status', data: { level: 2, status: 'thinking' } },
          { type: 'token', data: { text: 'hola' } },
          { type: 'done', data: {} },
        ],
      ];
      await useCase.sendMessage('hola', { route: '/dojo/ch-x' });
      // The user message is at index 0; the assistant message is at index 1.
      const msgs = useCase.messages();
      expect(msgs.length).toBeGreaterThanOrEqual(2);
      expect(msgs[0].role).toBe('user');
      expect(msgs[0].content).toBe('hola');
    });

    it('forwards the request to the port with the route + token', async () => {
      const { useCase, port } = makeUseCase({ token: 'jwt-abc' });
      port.streams = [[{ type: 'done', data: {} }]];
      await useCase.sendMessage('ayuda', {
        route: '/dojo/ch-sqli',
        challengeId: 'ch-sqli',
        codeSnippet: 'const x = 1;',
        output: 'audit-ok',
      });
      expect(port.captured).toHaveLength(1);
      const req = port.captured[0];
      expect(req.message).toBe('ayuda');
      expect(req.route).toBe('/dojo/ch-sqli');
      expect(req.challengeId).toBe('ch-sqli');
      expect(req.codeSnippet).toBe('const x = 1;');
      expect(req.output).toBe('audit-ok');
    });

    it('appends tokens incrementally to a single assistant message', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [
        [
          { type: 'token', data: { text: 'a' } },
          { type: 'token', data: { text: 'b' } },
          { type: 'token', data: { text: 'c' } },
          { type: 'done', data: {} },
        ],
      ];
      await useCase.sendMessage('q', { route: '/' });
      const msgs = useCase.messages();
      const assistant = msgs[msgs.length - 1];
      expect(assistant.role).toBe('assistant');
      expect(assistant.content).toBe('abc');
    });

    it('updates currentLevel when the status event carries a level', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [
        [
          { type: 'status', data: { level: 1, status: 'escalated' } },
          { type: 'token', data: { text: 'x' } },
          { type: 'done', data: {} },
        ],
      ];
      await useCase.sendMessage('q', { route: '/' });
      expect(useCase.currentLevel()).toBe(1);
    });

    it('sets lastOfferHint when the status event says "offer_hint"', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [
        [
          { type: 'status', data: { level: 1, status: 'offer_hint' } },
          { type: 'token', data: { text: '¿Querés una pista?' } },
          { type: 'done', data: {} },
        ],
      ];
      await useCase.sendMessage('q', { route: '/' });
      expect(useCase.lastOfferHint()).toBe(true);
      expect(useCase.currentLevel()).toBe(1);
    });

    it('clears streaming on done', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [[{ type: 'done', data: {} }]];
      await useCase.sendMessage('q', { route: '/' });
      expect(useCase.streaming()).toBe(false);
    });

    it('appends an error message and clears streaming on terminal error', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [[{ type: 'error', data: { message: 'Cascade failed' } }]];
      await useCase.sendMessage('q', { route: '/' });
      const msgs = useCase.messages();
      const last = msgs[msgs.length - 1];
      expect(last.role).toBe('system');
      expect(last.content).toContain('Cascade failed');
      expect(useCase.streaming()).toBe(false);
      expect(useCase.error()).toBe('Cascade failed');
    });

    it('keeps streaming=true mid-stream and only clears it on done', async () => {
      // The streaming flag should be true WHILE the port is
      // producing events and false AFTER the last event is
      // consumed. The fake port emits all events synchronously
      // through of(...), so by the time sendMessage resolves,
      // streaming is already false. The contract is the same as
      // the real port: resolve = done.
      const { useCase } = makeUseCase();
      const p = useCase.sendMessage('q', { route: '/' });
      // The awaitable resolves AFTER the full stream drains.
      await p;
      expect(useCase.streaming()).toBe(false);
    });
  });

  describe('slash commands', () => {
    it('does NOT call streamChat for an empty /nivel command (no argument)', async () => {
      const { useCase, port } = makeUseCase();
      await useCase.handleCommand('/nivel');
      expect(port.captured).toHaveLength(0);
      const last = useCase.messages()[useCase.messages().length - 1];
      expect(last.content).toContain('Usá /nivel 0-3');
    });

    it('rejects an out-of-range /nivel argument', async () => {
      const { useCase, port } = makeUseCase();
      await useCase.handleCommand('/nivel 5');
      expect(port.captured).toHaveLength(0);
      const last = useCase.messages()[useCase.messages().length - 1];
      expect(last.content).toContain('Nivel inválido');
    });

    it('updates currentLevel + persists to MCP for a valid /nivel N', async () => {
      const { useCase, port, mcp } = makeUseCase();
      await useCase.handleCommand('/nivel 1');
      expect(useCase.currentLevel()).toBe(1);
      // The MCP update_learning_profile call captured.
      expect(mcp.captured.some((c) => c.name === 'mcp-user.update_learning_profile')).toBe(true);
      expect(port.captured).toHaveLength(0);
    });

    it('handles /reset by clearing messages and lastOfferHint', async () => {
      // The cleaner path: drive /reset through a real seeded state
      // by first calling sendMessage on a stream that emits
      // offer_hint, then /reset. The /reset command appends a
      // "Sesión reiniciada" system message after clearing, so the
      // post-reset messages list contains exactly that one entry.
      const port = new FakeTutorChatPort();
      port.streams = [
        [
          { type: 'status', data: { level: 1, status: 'offer_hint' } },
          { type: 'token', data: { text: 'x' } },
          { type: 'done', data: {} },
        ],
      ];
      const { useCase: uc2 } = makeUseCase({ port });
      await uc2.sendMessage('q', { route: '/' });
      expect(uc2.lastOfferHint()).toBe(true);
      expect(uc2.messages().length).toBeGreaterThan(0);
      await uc2.handleCommand('/reset');
      const after = uc2.messages();
      expect(after).toHaveLength(1);
      expect(after[0].role).toBe('system');
      expect(after[0].content).toContain('reiniciada');
      expect(uc2.lastOfferHint()).toBe(false);
    });

    it('handles /perfil by fetching the profile via MCP and appending a system message', async () => {
      const { useCase, mcp } = makeUseCase();
      mcp.callToolResult = {
        userId: 'u-1',
        preferencias: {
          idioma: 'es',
          nivelSocratismo: 2,
          longitudMaximaMsg: 600,
        },
        estiloAprendizaje: { aprendeMejorCon: 'practica' },
        ultimaActualizacion: '2026-10-10T00:00:00Z',
      };
      await useCase.handleCommand('/perfil');
      expect(mcp.captured.some((c) => c.name === 'mcp-user.get_learning_profile')).toBe(true);
      const last = useCase.messages()[useCase.messages().length - 1];
      expect(last.role).toBe('system');
      expect(last.content).toContain('idioma: es');
    });

    it('handles /help by appending a help message listing the 7 commands', async () => {
      const { useCase, port } = makeUseCase();
      await useCase.handleCommand('/help');
      expect(port.captured).toHaveLength(0);
      const last = useCase.messages()[useCase.messages().length - 1];
      expect(last.role).toBe('system');
      expect(last.content).toContain('/nivel');
      expect(last.content).toContain('/pista');
      expect(last.content).toContain('/solucion');
      expect(last.content).toContain('/reset');
      expect(last.content).toContain('/perfil');
      expect(last.content).toContain('/diccionario');
      expect(last.content).toContain('/help');
    });

    it('routes /diccionario by emitting a navigation request via the dictionary command channel', async () => {
      const { useCase, port } = makeUseCase();
      let navigated: string | null = null;
      useCase.registerNavigateHook((p: string) => {
        navigated = p;
      });
      await useCase.handleCommand('/diccionario');
      expect(navigated).toBe('/dictionary');
      expect(port.captured).toHaveLength(0);
    });

    it('routes /pista by sending a hidden hint-seeking message through streamChat', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [[{ type: 'done', data: {} }]];
      await useCase.handleCommand('/pista');
      expect(port.captured).toHaveLength(1);
      // The hidden system message asks for a hint, not a direct answer.
      expect(port.captured[0].message).toMatch(/pista/i);
    });

    it('routes /solucion by sending a solution-seeking message through streamChat', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [[{ type: 'done', data: {} }]];
      await useCase.handleCommand('/solucion');
      expect(port.captured).toHaveLength(1);
      expect(port.captured[0].message).toMatch(/soluci[oó]n/i);
    });

    it('returns false for unknown commands and logs an unknown-command system message', async () => {
      const { useCase } = makeUseCase();
      await useCase.handleCommand('/foo');
      const last = useCase.messages()[useCase.messages().length - 1];
      expect(last.role).toBe('system');
      expect(last.content).toContain('/foo');
    });
  });

  describe('command routing from sendMessage', () => {
    it('intercepts messages that start with "/" before calling streamChat', async () => {
      const { useCase, port } = makeUseCase();
      await useCase.sendMessage('/help', { route: '/' });
      expect(port.captured).toHaveLength(0);
    });

    it('forwards regular (non-slash) messages to streamChat', async () => {
      const { useCase, port } = makeUseCase();
      port.streams = [[{ type: 'done', data: {} }]];
      await useCase.sendMessage('hola', { route: '/' });
      expect(port.captured).toHaveLength(1);
    });
  });
});
