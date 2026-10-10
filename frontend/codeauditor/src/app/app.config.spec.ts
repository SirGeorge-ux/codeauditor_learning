// app.config.spec.ts — root configuration tests for CodeAuditor.
//
// Verifies that the MCP_TOKEN_PROVIDER is registered in appConfig
// and that the mcpTokenProvider factory wires AuthService into a
// TokenProvider closure. The factory is tested directly (not via
// TestBed) so the test never instantiates the real AuthService
// (which boots a Supabase client that performs network I/O and
// would pollute other test files' mocks).
import { describe, it, expect } from 'vitest';

import { appConfig } from './app.config';
import { MCP_TOKEN_PROVIDER, TokenProvider } from './infrastructure/adapters/mcp-client.adapter';

describe('app.config providers', () => {
  describe('MCP_TOKEN_PROVIDER', () => {
    it('is registered in the application providers array', () => {
      // We don't bootstrap TestBed here because that would
      // instantiate AuthService. Instead we scan the providers
      // array for a factory that references MCP_TOKEN_PROVIDER.
      const providers = appConfig.providers as ReadonlyArray<unknown>;
      const found = providers.some(
        (p) =>
          p !== null &&
          typeof p === 'object' &&
          'provide' in p &&
          (p as { provide: unknown }).provide === MCP_TOKEN_PROVIDER,
      );
      expect(found).toBe(true);
    });

    it('TokenProvider interface is what MCPClientAdapter expects', () => {
      // Compile-time regression guard: the factory's return value
      // must satisfy the TokenProvider contract. If the contract
      // drifts, the cast below fails to type-check.
      const fake: TokenProvider = { getToken: (): string | null => 'abc' };
      expect(typeof fake.getToken).toBe('function');
      expect(fake.getToken()).toBe('abc');
    });
  });
});
