// mcp-token.provider.spec.ts — unit tests for the mcp-token provider
// helpers. The pure `makeMcpTokenProvider` factory is tested
// directly with a stub AuthService so the test never instantiates
// the real Supabase client (which performs network I/O at
// construction time and would pollute the auth.service mocks).
import { describe, it, expect, vi } from 'vitest';

// Mock @supabase/supabase-js so the AuthService import (used as a
// type only) does not load the real module + create a real
// GoTrueClient at module-evaluation time. Required to keep the
// auth.service.spec.ts vi.mock binding across the full suite.
vi.mock('@supabase/supabase-js', () => ({
  createClient: () => ({
    auth: {
      signUp: vi.fn(),
      signInWithPassword: vi.fn(),
      signOut: vi.fn(),
      getSession: vi.fn(),
      getUser: vi.fn(),
      onAuthStateChange: vi.fn(() => ({
        data: { subscription: { unsubscribe: vi.fn() } },
      })),
    },
  }),
}));

import { makeMcpTokenProvider, mcpTokenProvider } from './mcp-token.provider';
import { AuthService } from './auth.service';
import { TokenProvider } from '../adapters/mcp-client.adapter';

const makeAuthStub = (token: string | null): Pick<AuthService, 'getAccessToken'> => ({
  getAccessToken: (): string | null => token,
});

describe('mcpTokenProvider', () => {
  describe('makeMcpTokenProvider (pure factory)', () => {
    it('returns a TokenProvider whose getToken delegates to AuthService.getAccessToken', () => {
      const provider = makeMcpTokenProvider(makeAuthStub('jwt-abc'));
      expect(provider.getToken()).toBe('jwt-abc');
    });

    it('returns null when AuthService reports no session', () => {
      const provider = makeMcpTokenProvider(makeAuthStub(null));
      expect(provider.getToken()).toBeNull();
    });

    it('reads the token on every call so the latest session is reflected', () => {
      let current: string | null = 'first';
      const provider = makeMcpTokenProvider({
        getAccessToken: (): string | null => current,
      });
      expect(provider.getToken()).toBe('first');
      current = 'second';
      expect(provider.getToken()).toBe('second');
    });
  });

  describe('mcpTokenProvider (Angular DI factory)', () => {
    it('is a function suitable for useFactory in app.config.ts', () => {
      // The DI factory is the same function reference registered
      // in app.config.ts. We can't call it outside an injection
      // context, but we can assert its shape and that the pure
      // helper it depends on works.
      expect(typeof mcpTokenProvider).toBe('function');
      const pure = makeMcpTokenProvider(makeAuthStub('x'));
      const _typed: TokenProvider = pure;
      expect(_typed.getToken()).toBe('x');
    });
  });
});
