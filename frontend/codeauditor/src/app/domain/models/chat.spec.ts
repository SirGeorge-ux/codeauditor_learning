// chat.spec.ts — compile-time contract tests for the tutor chat
// domain models. These tests don't exercise runtime behavior
// (the models are pure interfaces) — they pin the shape so a
// future refactor that drops a field or renames a key fails the
// test file at compile time. That's the cheapest possible signal
// that the contract drifted from the Go backend
// (backend/internal/core/domain/models/tutor.go).
import { describe, it, expect } from 'vitest';

import {
  ChatMessage,
  SocraticLevel,
  StreamChatRequest,
  SSEEvent,
  isSocraticLevel,
  normalizeSocraticLevel,
} from './chat';

describe('chat domain models', () => {
  describe('SocraticLevel', () => {
    it('accepts the canonical 0..3 numeric literal values', () => {
      const cases: SocraticLevel[] = [0, 1, 2, 3];
      for (const level of cases) {
        expect(isSocraticLevel(level)).toBe(true);
      }
    });

    it('rejects out-of-range and non-integer values', () => {
      expect(isSocraticLevel(-1)).toBe(false);
      expect(isSocraticLevel(4)).toBe(false);
      expect(isSocraticLevel(2.5)).toBe(false);
      expect(isSocraticLevel('2')).toBe(false);
      expect(isSocraticLevel(null)).toBe(false);
      expect(isSocraticLevel(undefined)).toBe(false);
    });

    it('normalizeSocraticLevel clamps out-of-range numbers to the nearest boundary', () => {
      expect(normalizeSocraticLevel(-3)).toBe(0);
      expect(normalizeSocraticLevel(0)).toBe(0);
      expect(normalizeSocraticLevel(3)).toBe(3);
      expect(normalizeSocraticLevel(99)).toBe(3);
    });

    it('normalizeSocraticLevel falls back to 2 (the socratic-pure default) for non-numeric input', () => {
      expect(normalizeSocraticLevel('1' as unknown as number)).toBe(2);
      expect(normalizeSocraticLevel(null as unknown as number)).toBe(2);
      expect(normalizeSocraticLevel(undefined as unknown as number)).toBe(2);
    });
  });

  describe('ChatMessage', () => {
    it('accepts the canonical role/content/timestamp shape with optional socratic level', () => {
      const user: ChatMessage = {
        id: 'msg-1',
        role: 'user',
        content: '¿Por qué este código tiene SQL injection?',
        timestamp: new Date('2026-10-10T12:00:00Z'),
      };
      expect(user.role).toBe('user');
      expect(user.content).toMatch(/SQL injection/);
      expect(user.timestamp.toISOString()).toBe('2026-10-10T12:00:00.000Z');
      expect(user.socraticLevel).toBeUndefined();

      const assistant: ChatMessage = {
        id: 'msg-2',
        role: 'assistant',
        content: '¿Qué pasaría si el usuario ingresara comillas en el campo?',
        timestamp: new Date('2026-10-10T12:00:05Z'),
        socraticLevel: 2,
      };
      expect(assistant.socraticLevel).toBe(2);
    });

    it('accepts system messages (used for slash-command echoes)', () => {
      const system: ChatMessage = {
        id: 'msg-3',
        role: 'system',
        content: 'Comandos disponibles: /nivel, /pista, /solucion, /reset, /perfil, /diccionario, /help',
        timestamp: new Date('2026-10-10T12:00:10Z'),
      };
      expect(system.role).toBe('system');
    });
  });

  describe('StreamChatRequest', () => {
    it('accepts the wire shape the backend POST /api/v1/tutor/chat expects', () => {
      const req: StreamChatRequest = {
        message: '¿Qué línea es la peligrosa?',
        route: '/dojo/ch-sqli',
        challengeId: 'ch-sqli',
        codeSnippet: "const q = 'SELECT * FROM users WHERE id = ' + userId;",
        output: 'audit-ok',
        preferredProvider: 'auto',
      };
      expect(req.message).toBeTruthy();
      expect(req.route).toBe('/dojo/ch-sqli');
      expect(req.preferredProvider).toBe('auto');
    });

    it('allows every preferredProvider value, including minimax opt-in', () => {
      const cases: Array<StreamChatRequest['preferredProvider']> = [
        'auto',
        'groq',
        'deepseek',
        'minimax',
      ];
      for (const preferredProvider of cases) {
        const req: StreamChatRequest = { message: 'x', route: '/', preferredProvider };
        expect(req.preferredProvider).toBe(preferredProvider);
      }
    });
  });

  describe('SSEEvent', () => {
    it('exposes a status event with level + status payload', () => {
      const ev: SSEEvent = { type: 'status', data: { level: 2, status: 'thinking' } };
      if (ev.type === 'status') {
        expect(ev.data.level).toBe(2);
        expect(ev.data.status).toBe('thinking');
      }
    });

    it('exposes a token event with text payload', () => {
      const ev: SSEEvent = { type: 'token', data: { text: '¿Qué ' } };
      if (ev.type === 'token') {
        expect(ev.data.text).toBe('¿Qué ');
      }
    });

    it('exposes a done event with empty payload', () => {
      const ev: SSEEvent = { type: 'done', data: {} };
      if (ev.type === 'done') {
        expect(ev.data).toEqual({});
      }
    });

    it('exposes an error event with message payload', () => {
      const ev: SSEEvent = { type: 'error', data: { message: 'Cascade failed' } };
      if (ev.type === 'error') {
        expect(ev.data.message).toBe('Cascade failed');
      }
    });
  });
});
