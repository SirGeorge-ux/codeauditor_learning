// McpTokenProvider — wires the MCP_TOKEN_PROVIDER injection token to
// the live AuthService. The MCPClientAdapter (and the new
// HttpTutorChatAdapter in PR 3) depend on MCP_TOKEN_PROVIDER to stamp
// the Authorization header on every request; without this factory
// registered, the adapter fails to bootstrap.
//
// The factory is intentionally placed in the infrastructure layer
// (not the domain layer) because it references the concrete
// AuthService Angular injectable. The TokenProvider interface itself
// stays in mcp-client.adapter.ts so other adapters (the existing
// HttpDictionaryRepository pattern, the new HttpTutorChatAdapter)
// can depend on the contract without pulling in this factory.
//
// The factory's return value is a thin closure that delegates to
// AuthService.getAccessToken() on every call — the auth service
// owns the Supabase session signal, so reading through it always
// returns the freshest token without us needing to subscribe.
import { inject } from '@angular/core';
import { AuthService } from './auth.service';
import { TokenProvider } from '../adapters/mcp-client.adapter';

/**
 * Factory used in app.config.ts to register MCP_TOKEN_PROVIDER.
 *
 * Usage:
 *   ```ts
 *   providers: [
 *     ...,
 *     { provide: MCP_TOKEN_PROVIDER, useFactory: mcpTokenProvider },
 *   ]
 *   ```
 *
 * The factory runs at injector construction time; the closure it
 * returns is the TokenProvider injected by every consumer.
 */
export function mcpTokenProvider(): TokenProvider {
  const auth = inject(AuthService);
  return {
    getToken: (): string | null => auth.getAccessToken(),
  };
}
