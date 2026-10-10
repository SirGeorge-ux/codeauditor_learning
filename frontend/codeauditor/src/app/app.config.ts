import { ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient } from '@angular/common/http';

import { routes } from './app.routes';
import { MCP_TOKEN_PROVIDER } from './infrastructure/adapters/mcp-client.adapter';
import { mcpTokenProvider } from './infrastructure/services/mcp-token.provider';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes),
    provideHttpClient(),
    // MCP_TOKEN_PROVIDER — wires the AuthService-backed token
    // resolver so the MCPClientAdapter (mcp-monolith PR 5) and the
    // HttpTutorChatAdapter (tutor-chat PR 3) can stamp the
    // Authorization header on every HTTP+SSE call. Without this
    // provider, both adapters fail to bootstrap.
    { provide: MCP_TOKEN_PROVIDER, useFactory: mcpTokenProvider },
  ],
};
