# Design: Socratic AI Tutor Chat

## 1. Architecture Overview

The `tutor-chat` is a reactive, context-aware AI assistant utilizing Server-Sent Events (SSE) for low-latency streaming and a strict hexagonal architecture.

1. **Frontend**: A standalone Angular component (`tutor-chat-panel`) powered by `TutorChatUseCase`. Slash commands are intercepted client-side. Normal messages are sent to the backend via an HTTP POST request.
2. **Backend**: The `TutorService` receives the message, assembles a massive `ChatContext` by fetching data concurrently (user profile, language progress, short-term memory), and formats these as a sequence of `system` messages.
3. **Prompting**: The appropriate socratic prompt (`tutor-socratic-{0..3}.md`) is fetched using `mcp-pedagogical.get_socratic_prompt`.
4. **LLM Cascade**: The request is routed via the OpenRouter cascade (Groq -> DeepSeek) or MiniMax M3. Tool calls (e.g., `mcp-curriculum.get_topics`) are resolved server-side.
5. **Streaming**: The final text is streamed back to the frontend via SSE.
6. **Memory**: Every 10 messages, an async routine triggers `SummarizeSession` to update the persistent `LearningProfile`.

## 2. TutorService Design (Go, Hexagonal)

The logic resides in `backend/internal/core/services/tutor_service.go`.

```go
type TutorService struct {
    llmClient       ports.LLMClient
    userRepo        ports.UserRepository
    mcpClient       ports.MCPClient
    sessionCache    ports.CachePort // In-memory cache for 10-message window
}

// Interfaces / Contracts
type StreamChatRequest struct {
    UserID      string
    Message     string
    Route       string
    ChallengeID string
    Code        string
    Output      string
}

func (s *TutorService) StreamChat(ctx context.Context, req StreamChatRequest) (<-chan ports.SSEEvent, error)
func (s *TutorService) ProcessCommand(ctx context.Context, userID, command string) error
func (s *TutorService) SummarizeSession(ctx context.Context, sessionID string) error
```

**Responsibilities**:
- Assemble context from repositories.
- Manage auto-escalation state tracking.
- Handle server-side MCP tool execution natively before yielding tokens to the frontend.

## 3. ChatContext Assembly Pipeline

Before querying the LLM, the backend constructs the context pipeline. Instead of one massive JSON string, context is injected as distinct structured system messages to improve LLM attention:

1. **Identity**: Load `.atl/prompts/tutor-system.md`.
2. **Socratic Level**: Load `.atl/prompts/tutor-socratic-{level}.md` (level determined by `LearningProfile` or auto-escalation overrides).
3. **User & Language Context**: Formatted summaries of `rango_global`, streak, and `LanguageProgress` points/success rate.
4. **Location Context**: Active route (`req.Route`), current challenge ID, Monaco editor code (`req.Code`), and last sandbox output.
5. **History & Preferences**: User's `idioma`, `longitud_maxima_msg`, and recent blocks from the `LearningProfile`.

## 4. LLM Cascade Configuration

The tutor chat integrates with the existing LLM cascade logic (as defined in `docs/llm-strategy.md`), avoiding expensive models by default.

```yaml
# config/llm-cascade.yaml
  tutor_chat:
    cascade:
      - provider: openrouter
        model: llama-3.3-70b-versatile
        tier: 1
      - provider: openrouter
        model: deepseek-chat
        tier: 2
```
If the user's `UserProfile` has `use_minimax: true`, the cascade routes directly to MiniMax M3. All requests enforce a 5s timeout on the first token to guarantee UX responsiveness.

## 5. SSE Transport Design

We choose **SSE (Server-Sent Events)** over WebSockets because the communication is inherently unidirectional (request -> stream response) and aligns with existing `/api/v1/sse` patterns.

**Flow**:
1. Frontend makes HTTP POST to `/api/v1/tutor/chat` with `{ message, location, code }`.
2. Backend responds immediately with `200 OK` and a `text/event-stream` body.

**Event Types**:
- `event: token`, `data: {"text": "chunk"}`
- `event: status`, `data: {"status": "escalated", "level": 1}`
- `event: done`, `data: {}`
- `event: error`, `data: {"message": "Cascade failed"}`

## 6. Frontend Component Tree

Components are generated as standalone Angular 21 elements using the new control flow (`@if`, `@for`).

```text
frontend/codeauditor/src/app/infrastructure/components/tutor-chat/
├── tutor-chat-panel/        # Smart: Injects TutorChatUseCase, manages FAB/Sidebar state
├── tutor-chat-message/      # Dumb: Renders markdown, lucide-icons, and syntax highlighting
└── tutor-chat-input/        # Dumb: textarea with auto-resize, Enter to send
```

State is managed entirely by Angular Signals within `TutorChatUseCase`, ensuring zero `RxJS` overhead for state storage.

## 7. Command Parser

Slash commands (7 total) are parsed **client-side** in the `TutorChatUseCase`.

- **`/nivel [0-3]`**: Calls `PUT /api/v1/tutor/level`, updates local signal, bypasses LLM.
- **`/pista`**: Calls `POST /api/v1/challenges/{id}/hints`, appends backend response as a tutor message.
- **`/solucion`**: Calls backend to reveal solution (applying penalty), appends to chat.
- **`/reset`**: Calls `DELETE /api/v1/tutor/session`, clears local signals.
- **`/perfil`**: Calls `GET /api/v1/user/progress`, renders local widget.
- **`/diccionario <term>`**: Calls `GET /api/v1/dictionary/{term}`, renders locally.
- **`/help`**: Renders local static helper text.

## 8. Auto-escalation State Machine

The backend tracks session metrics per user per challenge.

**Triggers**:
- `failures_count >= 2` -> Level = `max(current - 1, 0)`
- `give_answer_count >= 3` -> Level = `max(current - 2, 0)`
- `time_on_challenge > 10m` && `code_unchanged` -> Emit special prompt to LLM to offer hint.
- `challenge_solved_no_hints` -> Level = `min(current + 1, 3)`

These state transitions override the baseline `LearningProfile.nivel_socratismo` temporarily for the session.

## 9. Memory Management

### Short-Term Memory
Since the project is single-user (`ggogsmic`), we use **In-Memory Go maps** (e.g., `sync.Map` or `go-cache`) keyed by `session_id` (derived from user + challenge). It holds the last 10 messages. On navigation to a new challenge, the frontend sends `/reset`, clearing the entry.

### Long-Term Memory
After an audit session completes, `SummarizeSession` is triggered asynchronously. It feeds the 10-message window to the LLM (Groq `qwen-2.5-coder-32b`) to generate a 2-3 sentence summary of blocks, learnings, and responses to socratic methods. This overwrites `LearningProfile.resumen` in PostgreSQL.

## 10. Trade-offs

**Decision**: SSE vs WebSockets
*Choice*: SSE
*Rationale*: Simpler to implement over existing HTTP endpoints. The frontend only needs to stream incoming tokens; it doesn't need to push stream data up.

**Decision**: Command parsing Client-side vs Server-side
*Choice*: Client-side
*Rationale*: Reduces unnecessary LLM roundtrips and saves compute. Standard UI commands like `/help` or `/perfil` map cleanly to existing endpoints without polluting the LLM context.

**Decision**: Session Storage In-Memory vs Redis
*Choice*: In-Memory
*Rationale*: The system targets a single user local environment. Introducing Redis for a 10-message rolling window violates the goal of minimal infrastructure dependencies unless we use Supabase/PG for it, but in-memory is fastest.

## Open Questions

- [ ] Does `mcp-pedagogical.get_socratic_prompt` require any pre-processing, or does it return the raw `.md` content directly?
- [ ] Should the user's `/pista` command intercept the point penalty silently, or prompt for confirmation via a UI dialog before execution?