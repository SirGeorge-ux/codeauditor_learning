// tutor-chat.service.ts — Angular infrastructure injectable that
// wires the TutorChatUseCase with the concrete HttpTutorChatAdapter,
// the MCPClientAdapter, and the AuthService. Components depend on
// the use case through this single Injectable so the dependency
// graph is one-deep: panel → service → use case.
import { Injectable, inject, computed } from '@angular/core';

import { TutorChatUseCase } from '../../application/tutor-chat.use-case';
import { HttpTutorChatAdapter } from '../adapters/http-tutor-chat.adapter';
import { MCPClientAdapter } from '../adapters/mcp-client.adapter';
import { AuthService } from './auth.service';
import { AuthPort } from '../../domain/ports/auth.port';

@Injectable({ providedIn: 'root' })
export class TutorChatService {
  private readonly useCase: TutorChatUseCase;

  // Signal re-exports so components read them through the
  // service rather than reaching into the use case directly.
  readonly messages = computed(() => this.useCase.messages());
  readonly currentLevel = computed(() => this.useCase.currentLevel());
  readonly streaming = computed(() => this.useCase.streaming());
  readonly error = computed(() => this.useCase.error());
  readonly lastOfferHint = computed(() => this.useCase.lastOfferHint());

  constructor() {
    const port = inject(HttpTutorChatAdapter);
    const mcp = inject(MCPClientAdapter);
    const auth = inject(AuthService);
    // AuthService exposes getAccessToken(); the use case's
    // readToken() helper looks for any object with a sync
    // getToken() method, so we adapt at the boundary.
    const tokenReader = {
      getToken: (): string | null => auth.getAccessToken(),
    };
    // The use case's third parameter is typed as AuthPort but
    // the readToken() helper uses duck typing on getToken();
    // cast through unknown to keep the type surface honest.
    this.useCase = new TutorChatUseCase(port, mcp, tokenReader as unknown as AuthPort);
  }

  // Pass-through methods. The use case's public surface is
  // intentionally narrow; the service just makes it injectable.
  sendMessage(
    text: string,
    ctx: { route: string; challengeId?: string; codeSnippet?: string; output?: string },
  ): Promise<void> {
    return this.useCase.sendMessage(text, ctx);
  }

  handleCommand(command: string): Promise<void> {
    return this.useCase.handleCommand(command);
  }

  registerNavigateHook(hook: (path: string) => void): void {
    this.useCase.registerNavigateHook(hook);
  }
}
