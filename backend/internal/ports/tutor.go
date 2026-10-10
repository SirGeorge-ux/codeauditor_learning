package ports

import (
	"context"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// TutorServicePort is the hexagonal contract the HTTP handler (and
// future MCP tutor tools) will consume. The concrete implementation
// lives in core/services/tutor_service.go (PR 2); this port lets
// driving adapters depend on the behavior without importing the
// service implementation.
//
// StreamChat is the primary path: a request enters the service, the
// service assembles a ChatContext, dispatches to the LLM cascade, and
// emits SSE-shaped events back through the channel. The handler
// drains the channel and pushes each event onto the wire.
//
// ProcessCommand handles the 7 slash commands (spec §7) server-side
// BEFORE the LLM is invoked — commands like /nivel 0 need to update
// state, not consume LLM tokens.
//
// SummarizeSession runs after every 10 messages (or audit completion)
// and persists a 2–3 sentence profile update via
// mcp-user.update_learning_profile.
type TutorServicePort interface {
	// StreamChat returns a channel of SSE-shaped events. Events are
	// {Type, Content, Level?} with Type in {"token","status","done","error"}.
	// The channel is closed by the implementation when the stream ends
	// or ctx is canceled.
	StreamChat(ctx context.Context, req StreamChatRequest) (<-chan StreamChatEvent, error)

	// ProcessCommand handles a slash command (eg. "/nivel 0",
	// "/reset", "/perfil"). Returns an error if the command is unknown
	// or fails its side-effect (eg. update_learning_profile reject).
	ProcessCommand(ctx context.Context, userID, command string) error

	// SummarizeSession rolls up the last 10 messages into a 2–3 sentence
	// profile entry and persists via mcp-user.update_learning_profile.
	SummarizeSession(ctx context.Context, userID, sessionID string) error
}

// StreamChatRequest is the input envelope for StreamChat. The handler
// stamps UserID from the JWT; the rest comes from the chat panel +
// current Monaco editor + last sandbox output.
//
// Two key design notes:
//   - Route + ChallengeID + Code + Output are all optional. A message
//     from /dashboard has Route="/" and empty ChallengeID.
//   - Code is sanitized for prompt-injection patterns at the boundary
//     (matches the mcp-pedagogical guard) so a malicious code blob
//     cannot escape into the system prompt.
//   - PreferredProvider is the user-side opt-in to MiniMax M3 (e.g.
//     "minimax"). Empty value tells the cascade to walk its ranking.
type StreamChatRequest struct {
	UserID            string
	Message           string
	Route             string
	ChallengeID       string
	Code              string
	Output            string
	PreferredProvider string
}

// StreamChatEvent is the SSE-shaped event the handler streams back to
// the frontend. The event types mirror design.md §5:
//
//	"token"  — incremental LLM token (Content holds the delta)
//	"status" — escalation event (Level is set, Content is human label)
//	"done"   — terminal event; stream complete
//	"error"  — terminal event; cascade failed or validation rejected
type StreamChatEvent struct {
	Type    string                // "token" | "status" | "done" | "error"
	Content string                // token text or status label
	Level   *models.SocraticLevel // only set on "status" events
}

// CachePort is the in-memory short-term memory contract the tutor
// service uses to keep the last 10 messages per (user, session) tuple.
// Implementations live in infrastructure/driven/session_cache.go (PR 1
// task 1.7).
//
// The port is keyed by sessionID because each challenge gets its own
// 10-message window (spec "Window resets on challenge change"). The
// frontend signals a reset via /reset or by navigating challenges.
type CachePort interface {
	// Get returns the message addressed by sessionID. Returns ok=false
	// when the key is unknown.
	Get(ctx context.Context, sessionID string) (models.ChatMessage, bool)

	// Set stores the message at sessionID. The implementation enforces
	// a 10-message cap per session, evicting the oldest on overflow.
	Set(ctx context.Context, sessionID string, msg models.ChatMessage)

	// Snapshot returns the current 10-message window in insertion
	// order (oldest first). Returns an empty slice when the session
	// is unknown or empty.
	Snapshot(ctx context.Context, sessionID string) []models.ChatMessage

	// Keys enumerates all active sessions. Used by the SSE handler
	// to support admin tooling (eg. /api/v1/tutor/sessions).
	Keys(ctx context.Context) []string

	// Reset clears the 10-message window for sessionID. Idempotent on
	// an unknown key.
	Reset(ctx context.Context, sessionID string)
}

// StreamChatOptions tunes LLMClientPort.StreamChat. The cascade impl
// (PR 2) honors PreferredProvider when set; otherwise it walks
// Groq → DeepSeek V3 → MiniMax per docs/llm-strategy.md.
type StreamChatOptions struct {
	// PreferredProvider short-circuits the cascade when set. Allowed
	// values: "groq" | "deepseek" | "minimax". Empty string means
	// "use the cascade ranking".
	PreferredProvider string
	// Timeout caps the first-token wait. Defaults to 5s if zero
	// (matches spec.md#SSE-Streaming fallback to DeepSeek).
	Timeout time.Duration
	// MaxTokens caps the response length; 0 leaves it to the model
	// default. The service layer clamps to the user's
	// longitud_maxima_msg.
	MaxTokens int
}

// LLMStreamEvent is the typed event yielded by LLMClientPort.StreamChat's
// callback. The shape aligns with StreamChatEvent so the service
// layer can forward events 1:1 onto the SSE channel without
// re-shaping.
type LLMStreamEvent struct {
	Type    string // "token" | "status" | "done" | "error"
	Content string
	Level   *models.SocraticLevel
}

// LLMClientPort is the multi-provider abstraction over the cascade. It
// accepts []ChatMessage because the tutor prompt is structured — the
// service emits 5 distinct system messages (identity, level, user,
// language, location, history, preferences) rather than concatenating
// one prompt blob.
//
// This port is DELIBERATELY a sibling of OllamaClient (not an
// extension). OllamaClient is the audit path's narrow contract — a
// single provider, a single-model. The tutor cascade is the opposite:
// multi-provider, multi-model, structured prompt. Mixing them in one
// port would tangle unrelated trajectories.
//
// The cascade concrete impl lands in PR 2; for now every consumer is
// satisfied via this interface alone.
type LLMClientPort interface {
	// StreamChat sends messages to the cascade and yields LLMStreamEvent
	// to onEvent for each emit. The callback returns an error to
	// signal back-pressure; StreamChat must propagate it and stop
	// emitting further events. Returns the terminal error (or nil
	// when the stream ends cleanly with Type == "done").
	StreamChat(ctx context.Context, messages []models.ChatMessage, opts StreamChatOptions, onEvent func(LLMStreamEvent) error) error
}
