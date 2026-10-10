// mcp-token.provider.ts — wires the MCP_TOKEN_PROVIDER injection
// token to the live AuthService. The MCPClientAdapter (and the new
// HttpTutorChatAdapter in PR 3) depend on MCP_TOKEN_PROVIDER to
// stamp the Authorization header on every request; without this
// factory registered, the adapter fails to bootstrap.
//
// The factory is intentionally placed in the infrastructure layer
// (not the domain layer) because it references the concrete
// AuthService Angular injectable. The TokenProvider interface
// itself stays in mcp-client.adapter.ts so other adapters (the
// existing HttpDictionaryRepository pattern, the new
// HttpTutorChatAdapter) can depend on the contract without
// pulling in this factory.
//
// The factory's return value is a thin closure that delegates to
// AuthService.getAccessToken() on every call — the auth service
// owns the Supabase session signal, so reading through it always
// returns the freshest token without us needing to subscribe.
import { inject } from '@angular/core';
import { AuthService } from './auth.service';
import { TokenProvider } from '../adapters/mcp-client.adapter';

/**
 * Pure factory: takes an AuthService-shaped object and returns a
 * TokenProvider. Pure function so it can be unit-tested without
 * bootstrapping Angular's DI.
 */
export function makeMcpTokenProvider(
  auth: Pick<AuthService, 'getAccessToken'>,
): TokenProvider {
  return {
    getToken: (): string | null => auth.getAccessToken(),
  };
}

/**
 * Angular DI factory used in app.config.ts to register
 * MCP_TOKEN_PROVIDER. Resolves AuthService from the injector.
 *
 * Usage:
 *   ```ts
 *   providers: [
 *     ...,
 *     { provide: MCP_TOKEN_PROVIDER, useFactory: mcpTokenProvider },
 *   ]
 *   ```
 */
export function mcpTokenProvider(): TokenProvider {
  const auth = inject(AuthService);
  return makeMcpTokenProvider(auth);
}
