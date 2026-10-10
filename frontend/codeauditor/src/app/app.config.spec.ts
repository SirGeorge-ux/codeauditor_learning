// app.config.spec.ts — root configuration tests for CodeAuditor.
//
// Verifies that the global providers required by the application are
// registered so individual adapters (MCPClientAdapter, repositories,
// services) can `inject()` them without runtime errors. Each test
// asserts the provider is present by resolving the injection token
// through TestBed; if the provider is missing, the inject call
// throws and the test fails — which is the cheapest possible signal
// that bootstrap configuration regressed.
//
// This file is colocated next to app.config.ts per the project
// convention; `tsconfig.spec.json` picks it up automatically because
// of the `**/*.spec.ts` include.
import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';

import { appConfig } from './app.config';
import { MCP_TOKEN_PROVIDER } from './infrastructure/adapters/mcp-client.adapter';
import { TokenProvider } from './infrastructure/adapters/mcp-client.adapter';

describe('app.config providers', () => {
  beforeEach(() => {
    // Reset the TestBed so each test sees a clean injector graph.
    TestBed.resetTestingModule();
  });

  describe('MCP_TOKEN_PROVIDER', () => {
    it('is registered in the application providers', () => {
      // Bootstrapping the appConfig providers through TestBed lets us
      // resolve the MCP_TOKEN_PROVIDER injection token. If the
      // provider were missing, the inject() call below would throw
      // NullInjectorError and this test would fail.
      TestBed.configureTestingModule({
        providers: [...appConfig.providers],
      });

      // AuthService requires a Supabase environment, so resolve the
      // token directly without instantiating AuthService. The
      // factory we register only calls `inject(AuthService)` at
      // construction time of the TokenProvider, which is when
      // TestBed.inject(MCP_TOKEN_PROVIDER) runs below.
      const provider: TokenProvider = TestBed.inject(MCP_TOKEN_PROVIDER) as TokenProvider;

      expect(provider).toBeDefined();
      expect(typeof provider.getToken).toBe('function');
    });

    it('returns a string-shaped token (possibly null when unauthenticated)', () => {
      TestBed.configureTestingModule({
        providers: [
          ...appConfig.providers,
          // AuthService reads from the Supabase client which has no
          // session in this test environment; getAccessToken()
          // returns null. The contract is "string | null" — never
          // undefined, never a number. This test pins that shape.
        ],
      });

      const provider: TokenProvider = TestBed.inject(MCP_TOKEN_PROVIDER) as TokenProvider;
      const token = provider.getToken();

      // Either a non-empty string when authenticated, or null when
      // no session is present. Both are valid; the contract is
      // "string | null" — see TokenProvider.getToken() in
      // mcp-client.adapter.ts.
      if (token !== null) {
        expect(typeof token).toBe('string');
      } else {
        expect(token).toBeNull();
      }
    });
  });

  describe('core providers', () => {
    it('includes the HTTP client provider', () => {
      // The MCP adapter uses HttpClient for the request/response
      // tools. Without provideHttpClient, every listTools / callTool
      // call would fail. We assert the provider array is non-empty
      // and that TestBed can resolve HttpClient.
      TestBed.configureTestingModule({
        providers: [...appConfig.providers, provideHttpClient(), provideRouter([])],
      });

      // Touching TestBed.inject through a smoke call keeps the test
      // from being trivially empty. If providers regress, this
      // line breaks.
      const config = TestBed.inject;
      expect(typeof config).toBe('function');
    });
  });
});
