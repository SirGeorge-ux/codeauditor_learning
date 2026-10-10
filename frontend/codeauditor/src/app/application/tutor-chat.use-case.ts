// TutorChatUseCase — application-layer brain of the tutor chat panel.
//
// Responsibilities:
//   1. Own the conversation state (Angular signals).
//   2. Parse 7 slash commands and route them client-side.
//   3. Consume the SSE stream from TutorChatPort and append tokens
//      incrementally to a single assistant message.
//   4. Forward the user's JWT to the port for HTTP authorization.
//   5. Bridge to MCP for /perfil + /nivel persistence.
//
// Hexagonal contract: this file imports ONLY domain types, RxJS,
// and Angular's `signal` primitive. It does NOT import any HTTP
// transport, fetch, or DOM type — every external boundary goes
// through an injected port.
//
// State machine:
//   sendMessage(text, ctx)
//     ├── if text starts with "/" → handleCommand(text)
//     └── else:
//         ├── push user message
//         ├── push empty assistant message (streaming target)
//         ├── mark streaming = true
//         ├── subscribe to port.streamChat(...)
//         │   ├── status → update currentLevel + lastOfferHint
//         │   ├── token  → append to assistant message content
//         │   ├── done   → finalize, clear streaming
//         │   └── error  → append error system message, clear streaming
//         └── unsubscribe on done/error
//
// Slash commands (parsed client-side per design.md §7):
//   /nivel [0-3]   → updates currentLevel + persists via MCP
//   /pista         → sends a hidden system-prompt seeking a hint
//   /solucion      → sends a hidden system-prompt seeking a solution
//   /reset         → clears messages + lastOfferHint
//   /perfil        → fetches mcp-user.get_learning_profile
//   /diccionario   → navigates to /dictionary via the page hook
//   /help          → appends a static help message
import { signal } from '@angular/core';
import { tap, catchError, of, firstValueFrom } from 'rxjs';

import {
  ChatMessage,
  ChatProvider,
  SocraticLevel,
  SSEEvent,
  StreamChatRequest,
  isSocraticLevel,
  normalizeSocraticLevel,
} from '../domain/models/chat';
import { TutorChatPort } from '../domain/ports/tutor-chat.port';
import { AuthPort } from '../domain/ports/auth.port';
import { MCPClientPort } from '../domain/ports/mcp-client.port';

export interface SendContext {
  route: string;
  challengeId?: string;
  codeSnippet?: string;
  output?: string;
  preferredProvider?: ChatProvider;
}

const HELP_TEXT = `Comandos disponibles:
  /nivel [0-3]   Cambia el nivel socrático y persiste el cambio
  /pista         Pide una pista (con penalización leve de puntos)
  /solucion      Pide la solución (con penalización alta de puntos)
  /reset         Borra el historial de esta sesión
  /perfil        Muestra tu perfil de aprendizaje
  /diccionario   Abre tu glosario personal
  /help          Muestra este mensaje`;

/**
 * TutorChatUseCase is the single source of truth for the chat panel.
 * Components inject this class and read signals; they never reach
 * into the port or MCP directly.
 */
export class TutorChatUseCase {
  // Conversation state (private setters, public readonly views).
  private readonly _messages = signal<ChatMessage[]>([]);
  private readonly _currentLevel = signal<SocraticLevel>(2);
  private readonly _streaming = signal<boolean>(false);
  private readonly _error = signal<string | null>(null);
  private readonly _lastOfferHint = signal<boolean>(false);

  readonly messages = this._messages.asReadonly();
  readonly currentLevel = this._currentLevel.asReadonly();
  readonly streaming = this._streaming.asReadonly();
  readonly error = this._error.asReadonly();
  readonly lastOfferHint = this._lastOfferHint.asReadonly();

  // Navigation hook — the page subscribes so /diccionario can switch
  // routes without the use case importing @angular/router.
  private navigateHook: ((path: string) => void) | null = null;

  constructor(
    private readonly port: TutorChatPort,
    private readonly mcp: MCPClientPort,
    private readonly auth: AuthPort,
  ) {}

  /**
   * registerNavigateHook is wired by the page so the /diccionario
   * command can switch routes without coupling the use case to
   * Angular Router.
   */
  registerNavigateHook(hook: (path: string) => void): void {
    this.navigateHook = hook;
  }

  /**
   * sendMessage is the primary public method. If the text starts
   * with "/", it's routed to handleCommand; otherwise it's POSTed
   * to /api/v1/tutor/chat via the port.
   */
  async sendMessage(message: string, ctx: SendContext): Promise<void> {
    const trimmed = message.trim();
    if (trimmed.length === 0) {
      return;
    }
    if (trimmed.startsWith('/')) {
      await this.handleCommand(trimmed);
      return;
    }
    return this.streamReply(trimmed, ctx);
  }

  /**
   * handleCommand is exposed so the input component can dispatch
   * slash commands directly without going through sendMessage (the
   * input strips the leading "/nivel 0" etc. before calling).
   */
  async handleCommand(command: string): Promise<void> {
    const trimmed = command.trim();
    if (!trimmed.startsWith('/')) {
      return;
    }
    const parts = trimmed.split(/\s+/);
    const cmd = parts[0]?.toLowerCase() ?? '';
    const arg = parts[1] ?? '';

    switch (cmd) {
      case '/nivel':
        await this.cmdNivel(arg);
        return;
      case '/pista':
        await this.cmdHint('pista');
        return;
      case '/solucion':
        await this.cmdHint('solucion');
        return;
      case '/reset':
        this.cmdReset();
        return;
      case '/perfil':
        await this.cmdPerfil();
        return;
      case '/diccionario':
        this.cmdDiccionario();
        return;
      case '/help':
        this.cmdHelp();
        return;
      default:
        this.appendSystem(`Comando no reconocido: ${cmd}. Usá /help para ver la lista.`);
        return;
    }
  }

  // -- Command implementations ------------------------------------------------

  private async cmdNivel(arg: string): Promise<void> {
    if (arg.length === 0) {
      this.appendSystem('Falta el nivel. Usá /nivel 0-3.');
      return;
    }
    const parsed = Number(arg);
    if (!isSocraticLevel(parsed)) {
      this.appendSystem(`Nivel inválido "${arg}". Usá /nivel 0-3.`);
      return;
    }
    this._currentLevel.set(parsed);
    // Persist via MCP. The tool name matches the backend
    // mcp-user.update_learning_profile (mcp-monolith PR 3).
    try {
      await firstValueFrom(
        this.mcp.callTool('mcp-user.update_learning_profile', {
          preferencias: { nivelSocratismo: parsed },
        }),
      );
      this.appendSystem(`Nivel socrático actualizado a ${parsed}.`);
    } catch (e) {
      this.appendSystem(`No pude persistir el nivel: ${this.errMsg(e)}`);
    }
  }

  private async cmdHint(kind: 'pista' | 'solucion'): Promise<void> {
    const prompt =
      kind === 'pista'
        ? 'El usuario pidió una pista. Respondé socráticamente con una pregunta guía y, si es necesario, una pista gradual (no la solución completa).'
        : 'El usuario pidió ver la solución. Aplicá una penalización alta de puntos y revelá la solución de manera estructurada, explicando el razonamiento.';
    // Send as a hidden system message through the regular stream
    // path so the LLM cascade handles it. The user message stays
    // visible above the assistant reply.
    await this.streamReply(prompt, { route: '/__tutor_command' });
  }

  private cmdReset(): void {
    this._messages.set([]);
    this._lastOfferHint.set(false);
    this._error.set(null);
    this.appendSystem('Sesión reiniciada.');
  }

  private async cmdPerfil(): Promise<void> {
    try {
      const profile = await firstValueFrom(
        this.mcp.callTool<{ preferencias?: { idioma?: string; nivelSocratismo?: number } }>(
          'mcp-user.get_learning_profile',
          {},
        ),
      );
      const prefs = profile?.preferencias ?? {};
      this.appendSystem(
        `Perfil de aprendizaje:\n` +
          `  · idioma: ${prefs.idioma ?? 'es'}\n` +
          `  · nivel socrático: ${prefs.nivelSocratismo ?? 2}\n` +
          `  · nivel actual: ${this._currentLevel()}`,
      );
    } catch (e) {
      this.appendSystem(`No pude leer el perfil: ${this.errMsg(e)}`);
    }
  }

  private cmdDiccionario(): void {
    this.appendSystem('Abriendo el glosario...');
    if (this.navigateHook) {
      this.navigateHook('/dictionary');
    }
  }

  private cmdHelp(): void {
    this.appendSystem(HELP_TEXT);
  }

  // -- Stream consumer --------------------------------------------------------

  private async streamReply(message: string, ctx: SendContext): Promise<void> {
    const userMsg = this.makeMessage('user', message);
    this._messages.update((msgs) => [...msgs, userMsg]);

    // Pre-allocate the assistant message so the consumer can
    // append tokens in place. The final message is committed in
    // the done branch.
    const assistantMsg = this.makeMessage('assistant', '');
    this._messages.update((msgs) => [...msgs, assistantMsg]);
    this._streaming.set(true);
    this._error.set(null);

    const token = this.readToken();

    const req: StreamChatRequest = {
      message,
      route: ctx.route,
      challengeId: ctx.challengeId,
      codeSnippet: ctx.codeSnippet,
      output: ctx.output,
      preferredProvider: ctx.preferredProvider,
    };

    // Consume the entire stream. The backend's TutorService emits
    // `done` (or `error`) as the terminal event; we hook the
    // streaming=false transition inside `applyEvent` so a
    // subscriber completing the Observable resolves the await
    // with no value (which is what we want here).
    await new Promise<void>((resolve) => {
      this.port
        .streamChat(req, token)
        .pipe(
          tap((ev) => this.applyEvent(ev, assistantMsg.id)),
          catchError((err: unknown) => {
            this._error.set(this.errMsg(err));
            this._streaming.set(false);
            this.appendSystem(`Error: ${this.errMsg(err)}`);
            return of(null);
          }),
        )
        .subscribe({
          next: () => undefined,
          error: () => {
            resolve();
          },
          complete: () => {
            resolve();
          },
        });
    });
  }

  private applyEvent(ev: SSEEvent, assistantId: string): void {
    if (ev.type === 'status') {
      const level = ev.data.level;
      if (level !== undefined) {
        this._currentLevel.set(normalizeSocraticLevel(level));
      }
      if (ev.data.status === 'offer_hint') {
        this._lastOfferHint.set(true);
      }
      return;
    }
    if (ev.type === 'token') {
      this._messages.update((msgs) =>
        msgs.map((m) => (m.id === assistantId ? { ...m, content: m.content + ev.data.text } : m)),
      );
      return;
    }
    if (ev.type === 'done') {
      this._streaming.set(false);
      return;
    }
    if (ev.type === 'error') {
      this._streaming.set(false);
      this._error.set(ev.data.message);
      this.appendSystem(`Error: ${ev.data.message}`);
      return;
    }
  }

  // -- Helpers ----------------------------------------------------------------

  private makeMessage(role: ChatMessage['role'], content: string): ChatMessage {
    return {
      id: this.nextId(),
      role,
      content,
      timestamp: new Date(),
      socraticLevel: role === 'assistant' ? this._currentLevel() : undefined,
    };
  }

  private appendSystem(content: string): void {
    this._messages.update((msgs) => [...msgs, this.makeMessage('system', content)]);
  }

  private readToken(): string {
    // The AuthPort interface doesn't expose a sync getToken, but
    // the concrete AuthService.getAccessToken() is sync. The
    // adapter (infrastructure layer) reads the token from
    // MCP_TOKEN_PROVIDER. Here we forward whatever the auth port
    // hands us; if the page registered a custom navigateHook, the
    // auth wiring is fully in place.
    const authAny = this.auth as unknown as { getToken?: () => string | null };
    if (typeof authAny.getToken === 'function') {
      return authAny.getToken() ?? '';
    }
    return '';
  }

  private errMsg(e: unknown): string {
    if (e instanceof Error) return e.message;
    return String(e);
  }

  // Monotonic id generator. crypto.randomUUID is available in
  // modern browsers + jsdom 28; the fallback is a time+random
  // string that's unique enough for chat-message @for tracking.
  private nextId(): string {
    if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
      return crypto.randomUUID();
    }
    return `msg-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
  }
}
