// chat.ts — tutor chat domain models.
//
// Pure TypeScript types + small helpers. Zero Angular imports. Mirrors
// the Go struct shape in
// backend/internal/core/domain/models/tutor.go and the SSE wire format
// emitted by backend/internal/infrastructure/driving/handlers/tutor_handler.go.
//
// The frontend never imports from Go code at runtime — these types are
// duplicated by hand so the boundary stays clean. Drift detection lives
// in chat.spec.ts (compile-time contract test) and the backend tutor
// handler tests (Go side).
//
// The wire shape in design.md §5 / spec.md#SSE-Streaming maps onto
// SSEEvent as a discriminated union on `type`:
//   - status: { level: SocraticLevel, status: string }
//   - token:  { text: string }
//   - done:   {}
//   - error:  { message: string }
// The Go handler emits `event: <type>\ndata: <payload>\n\n` so a
// browser EventSource (or our manual parser in
// infrastructure/adapters/http-tutor-chat.adapter.ts) reads `type`
// from the `event:` line and payload from the `data:` line.

/**
 * SocraticLevel is the 0..3 numeric literal that travels on the wire
 * (matches the Go SocraticLevel int and the JSON tag on
 * Preferencias.NivelSocratismo). Using a numeric literal type instead
 * of a plain `number` lets the use case reject bogus values at compile
 * time and the helper `isSocraticLevel` is the runtime gate.
 */
export type SocraticLevel = 0 | 1 | 2 | 3;

/**
 * Type-guard for SocraticLevel. Returns true only for the canonical
 * 0..3 integer values. Booleans, strings, and floats all return false.
 */
export function isSocraticLevel(value: unknown): value is SocraticLevel {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= 3;
}

/**
 * Clamp an arbitrary input to a SocraticLevel. Out-of-range numbers
 * collapse to the nearest boundary (negative → 0, > 3 → 3); non-numeric
 * input falls back to 2 (socratic-pure), which is the spec.md default
 * for F–E / Junior ranks.
 */
export function normalizeSocraticLevel(value: unknown): SocraticLevel {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 2;
  }
  if (value < 0) return 0;
  if (value > 3) return 3;
  // After the range clamps above, the integer is in 0..3; non-integer
  // values (e.g. 2.5) round to the nearest canonical level.
  return Math.round(value) as SocraticLevel;
}

/**
 * The role of a chat message author. Three canonical roles:
 *   - user:      text typed into the input box
 *   - assistant: tutor reply (LLM-generated or command echo)
 *   - system:    command echoes, /help, /perfil, etc.
 */
export type ChatRole = 'user' | 'assistant' | 'system';

/**
 * ChatMessage is a single entry in the conversation. The use case
 * generates the `id` (crypto.randomUUID or a monotonic counter) so
 * the panel can use it as a `@for` track key.
 *
 * The `socraticLevel` is attached to assistant messages so the panel
 * can show a small badge per message indicating which level produced
 * the reply. It mirrors the Go ChatMessage.Level *SocraticLevel field
 * (omitempty → undefined when not set).
 */
export interface ChatMessage {
  id: string;
  role: ChatRole;
  content: string;
  timestamp: Date;
  socraticLevel?: SocraticLevel;
}

/**
 * Provider for the LLM cascade. `auto` lets the backend run the
 * Groq → DeepSeek cascade; explicit values short-circuit to a single
 * provider. `minimax` is the premium opt-in (M3), which the backend
 * wires only when the user has use_minimax=true in their profile.
 */
export type ChatProvider = 'auto' | 'groq' | 'deepseek' | 'minimax';

/**
 * Wire shape the frontend POSTs to /api/v1/tutor/chat. The Go
 * handler decodes this exact struct (see
 * backend/internal/infrastructure/driving/handlers/tutor_handler.go
 * tutorChatRequest).
 */
export interface StreamChatRequest {
  message: string;
  route: string;
  challengeId?: string;
  codeSnippet?: string;
  output?: string;
  preferredProvider?: ChatProvider;
}

/**
 * SSEEvent is the discriminated union the TutorChatPort streams
 * back to the use case. The `type` field is the literal read from
 * the `event:` line of the SSE frame; the `data` field is the
 * JSON-decoded payload from the `data:` line.
 *
 * Field name is `data` (not `payload`) to match the existing
 * AuditEvent / SSE conventions in the codebase (see audit-event.ts).
 */
export type SSEEvent =
  | { type: 'status'; data: { level?: SocraticLevel; status: string } }
  | { type: 'token'; data: { text: string } }
  | { type: 'done'; data: Record<string, never> }
  | { type: 'error'; data: { message: string } };
