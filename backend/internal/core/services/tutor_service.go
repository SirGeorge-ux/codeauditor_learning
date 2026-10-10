// Package services — tutor-chat core logic (PR 2 of the tutor-chat
// change). TutorService is the hexagonal center of the /api/v1/tutor/chat
// endpoint: it assembles the 5-system-message ChatContext per
// design.md §3, drives the auto-escalation state machine
// (design.md §8), and pumps LLM tokens back through the handler via a
// channel compatible with ports.StreamChatEvent.
//
// PR 1 shipped the supporting types (models.ChatMessage,
// models.SocraticLevel, ports.TutorServicePort) and the in-memory
// SessionCache that backs CachePort. PR 2 layers the behavior on top.
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrTutorUnavailable is the sentinel for "the TutorService could not
// be reached" conditions: missing wiring, disabled cascades, or a
// feature flag turning the chat off. The handler maps this to a 503
// envelope so the frontend can show a non-confusing error.
var ErrTutorUnavailable = errors.New("tutor service unavailable")

// ErrTutorCascadeFailed is the sentinel the cascade layer surfaces when
// every configured provider times out or returns a non-recoverable
// error. The handler translates this into an SSE `error` event so the
// frontend can surface a one-liner to the user.
var ErrTutorCascadeFailed = errors.New("tutor cascade failed")

// ErrTutorCommandUnknown is returned by ProcessCommand when the slash
// command is not in the seven-command whitelist (/nivel, /pista,
// /solucion, /reset, /perfil, /diccionario, /help).
var ErrTutorCommandUnknown = errors.New("tutor command unknown")

// StreamChatInput is the application's view of the chat request. The
// HTTP handler converts its JSON body into this struct before calling
// StreamChat; the application layer NEVER imports ports.StreamChatRequest
// directly so the service stays decoupled from the wire shape.
type StreamChatInput struct {
	UserID      string
	Message     string
	Route       string
	ChallengeID string
	Code        string
	Output      string
	Language    string
	Preferred   string // empty = use cascade; "minimax" = opt-in to M3
}

// StreamChatOutput is what TutorService.StreamChat returns to the HTTP
// handler. Events is the SSE channel; events fire as the LLM cascade
// produces tokens or state transitions. The channel is closed by the
// implementation when the stream ends or ctx is canceled.
type StreamChatOutput struct {
	Events <-chan ports.StreamChatEvent
}

// TutorService is the production implementation of
// ports.TutorServicePort. It depends on:
//   - LLMClientPort   — the cascade adapter (PR 2 ships the cascade)
//   - CachePort       — short-term 10-msg window (SessionCache, PR 1)
//   - UserProgressRepository — long-term LearningProfile, exposed by
//     mcp-monolith so SummarizeSession can call UpdateLearningProfile
//   - *TutorPrompts   — 5 .atl/prompts/*.md files, loaded at startup
//   - *EscalationTracker — per-(user,challenge) in-memory escalation
//     state (design.md §8) — owned by TutorService, not a port, because
//     it has no out-of-process persistence requirement
type TutorService struct {
	llmClient   ports.LLMClientPort
	cache       ports.CachePort
	profileRepo ports.UserProgressRepository
	prompts     *TutorPrompts
	escalation  *EscalationTracker
}

// NewTutorService wires the service. All collaborators are required;
// the constructor panics on a nil dependency so a wiring mistake
// surfaces at process start, not at first request.
func NewTutorService(
	llm ports.LLMClientPort,
	cache ports.CachePort,
	repo ports.UserProgressRepository,
	prompts *TutorPrompts,
) *TutorService {
	if llm == nil {
		panic("tutor service: LLMClientPort must not be nil")
	}
	if cache == nil {
		panic("tutor service: CachePort must not be nil")
	}
	if repo == nil {
		panic("tutor service: UserProgressRepository must not be nil")
	}
	if prompts == nil {
		panic("tutor service: TutorPrompts must not be nil")
	}
	return &TutorService{
		llmClient:   llm,
		cache:       cache,
		profileRepo: repo,
		prompts:     prompts,
		escalation:  NewEscalationTracker(),
	}
}

// Compile-time guard: TutorService must satisfy ports.TutorServicePort
// so signature drift surfaces at compile time, not at the handler.
var _ ports.TutorServicePort = (*TutorService)(nil)

// StreamChat is the primary entry. PR 1 declared the channel
// signature; PR 2 fills it.
//
// The implementation shape (assembled here so a future maintainer
// reads the wire in one place):
//
//  1. Sanitize the user message at the boundary (max length + the
//     same injection patterns the mcp-pedagogical guard matches).
//  2. Run EscalateIfNeeded against (user, challenge) → new level.
//  3. Build the ChatContext via assembleChatContext (separate file).
//  4. Append the user's message to the cache (LIFO; spec §Session-
//     Memory rolling-window).
//  5. Push a `status` event with the resolved level, then call
//     llmClient.StreamChat(…) which fires `token` events.
//  6. Push `done` (or `error`) and close the channel.
//  7. If the cache has 10 messages and the user is paused, call
//     SummarizeSession in a background goroutine.
//
// The placeholder body emits a single `done` event so the channel
// satisfies the contract in pre-implementation tests; the production
// wire is in the next commit.
func (s *TutorService) StreamChat(ctx context.Context, req ports.StreamChatRequest) (<-chan ports.StreamChatEvent, error) {
	ch := make(chan ports.StreamChatEvent, 8)
	if err := ctx.Err(); err != nil {
		close(ch)
		return nil, err
	}
	if s.llmClient == nil {
		close(ch)
		return nil, fmt.Errorf("%w: llmClient not configured", ErrTutorUnavailable)
	}
	go func() {
		// Placeholder close so the channel satisfies the contract.
		// The next commit replaces this with the real cascade pipe.
		defer close(ch)
		ch <- ports.StreamChatEvent{Type: "done"}
	}()
	return ch, nil
}

// ProcessCommand is the single entry for the 7 slash commands
// (/nivel, /pista, /solucion, /reset, /perfil, /diccionario, /help).
// PR 3 (frontend) is documented to parse them client-side; this
// server-side method is the durable backend path used when the
// frontend sends the bare command string in a StreamChat request, or
// for tests that bypass the UI layer.
func (s *TutorService) ProcessCommand(_ context.Context, _ string, command string) error {
	trimmed := trimSlash(command)
	if trimmed == "" {
		return fmt.Errorf("%w: empty command", ErrTutorCommandUnknown)
	}
	// Temporary dispatch: every non-empty command is "unknown" so
	// the test for the unknown-branch is RED during the placeholder
	// phase. The next commit installs the real dispatcher.
	return fmt.Errorf("%w: %s", ErrTutorCommandUnknown, trimmed)
}

// SummarizeSession rolls the last 10 cache messages into a 2-3
// sentence LearningProfile note and persists via UpdateLearningProfile.
// The summary text is deterministic for the PR 2 placeholder — the
// real LLM-backed summarizer lands in PR 3 (S5 LLM cascade) per the
// spec. The shape must already be a 2-3 sentence note so the
// UpdateLearningProfile side-effect is observable in tests.
func (s *TutorService) SummarizeSession(ctx context.Context, userID, sessionID string) error {
	if userID == "" {
		return fmt.Errorf("%w: userID is required", ErrTutorUnavailable)
	}
	if sessionID == "" {
		return fmt.Errorf("%w: sessionID is required", ErrTutorUnavailable)
	}
	profile, err := s.profileRepo.GetLearningProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("tutor summarize: load profile: %w", err)
	}
	// Deterministic stub note so the side-effect test stays green.
	// PR 3 (S5) replaces this with the LLM-built note.
	profile.Preferencias.Resumen = "Sesión de práctica reciente. Sin resumen adicional en esta entrega."
	profile.UltimaActualizacion = time.Now().UTC()
	if err := s.profileRepo.UpdateLearningProfile(ctx, profile); err != nil {
		return fmt.Errorf("tutor summarize: persist profile: %w", err)
	}
	return nil
}

// trimSlash strips a leading "/" and surrounding whitespace. Public
// helper so the ProcessCommand test can lock the parser behavior
// without going through the SSE handler.
func trimSlash(command string) string {
	out := command
	if len(out) > 0 && out[0] == '/' {
		out = out[1:]
	}
	// Light trim without importing strings — keeps the helper
	// allocation-free for the hot path.
	for len(out) > 0 && (out[0] == ' ' || out[0] == '\t') {
		out = out[1:]
	}
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t' || out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return out
}
