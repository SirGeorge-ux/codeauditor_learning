// TutorChatPort — domain port for the socratic tutor chat stream.
//
// The application layer (TutorChatUseCase) depends on this interface;
// the infrastructure layer provides the HTTP+SSE implementation
// (infrastructure/adapters/http-tutor-chat.adapter.ts).
//
// Hexagonal contract: this file imports ONLY rxjs + the chat domain
// models. No @angular/*, no fetch, no DOM types. Tests can wire a
// fake TutorChatPort without HTTP mocks or zone dance — the use case
// is the unit under test, not the transport.
//
// `streamChat` mirrors the backend POST /api/v1/tutor/chat handler
// in backend/internal/infrastructure/driving/handlers/tutor_handler.go.
// The handler responds with `text/event-stream` carrying the four
// event kinds (status | token | done | error); the adapter
// implements this contract by parsing the SSE wire format and
// emitting SSEEvent values.
import { Observable } from 'rxjs';

import { SSEEvent, StreamChatRequest } from '../models/chat';

/**
 * TutorChatPort is the application-facing contract for streaming
 * chat replies from the socratic tutor backend.
 */
export interface TutorChatPort {
  /**
   * Open a Server-Sent Events stream against POST /api/v1/tutor/chat.
   *
   * The token is the user's Supabase JWT; the adapter stamps it as
   * the Authorization header. Passing it explicitly (instead of
   * pulling from MCP_TOKEN_PROVIDER inside the adapter) keeps the
   * port pure — the use case decides which token to forward and the
   * adapter never reaches into the global injector.
   *
   * The Observable emits SSEEvent values as they arrive. The stream
   * completes when the server sends `event: done` (success) or
   * `event: error` (terminal cascade failure). The subscriber can
   * cancel mid-stream and the adapter must abort the underlying
   * fetch so the Go handler's r.Context().Done() branch fires.
   */
  streamChat(req: StreamChatRequest, token: string): Observable<SSEEvent>;
}
