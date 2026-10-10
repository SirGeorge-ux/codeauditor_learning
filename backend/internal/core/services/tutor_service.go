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
	"strconv"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
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
// The flow (assembled here so a future maintainer reads it in one
// place) mirrors design.md §3:
//
//  1. Sanitize the user message at the boundary (max length).
//  2. Run EscalateIfNeeded against (user, challenge) → new level.
//  3. Build the ChatContext via assembleChatContext.
//  4. Append the user's message to the cache (LIFO; spec §Session-
//     Memory rolling-window).
//  5. Push a `status` event with the resolved level, then call
//     llmClient.StreamChat(…) which fires `token` events.
//  6. Push `done` (or `error`) and close the channel.
//  7. If the cache has 10 messages, call SummarizeSession async.
//
// The implementation runs the LLM stream in a goroutine and forwards
// every event back through ch; the HTTP handler drains ch and emits
// SSE wire frames per design.md §5.
func (s *TutorService) StreamChat(ctx context.Context, req ports.StreamChatRequest) (<-chan ports.StreamChatEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.llmClient == nil {
		return nil, fmt.Errorf("%w: llmClient not configured", ErrTutorUnavailable)
	}

	// 1. Sanitize the message. Empty → no-op stream that emits done.
	if strings.TrimSpace(req.Message) == "" {
		ch := make(chan ports.StreamChatEvent, 1)
		go func() {
			defer close(ch)
			ch <- ports.StreamChatEvent{Type: "done"}
		}()
		return ch, nil
	}

	ch := make(chan ports.StreamChatEvent, 16)
	sessionID := inferSessionID(req.UserID, req.ChallengeID)

	// Resolve the baseline level from LearningProfile (default 2 on
	// the wire), then apply the auto-escalation state machine.
	baseline := models.SocraticPure
	if s.profileRepo != nil {
		if profile, err := s.profileRepo.GetLearningProfile(ctx, req.UserID); err == nil {
			if l := models.SocraticLevel(profile.Preferencias.NivelSocratismo); l.Valid() {
				baseline = l
			}
		}
	}
	metricsSnapshot := s.escalation.getSnapshot(sessionKey{userID: req.UserID, challengeID: req.ChallengeID})
	escalated := EscalateIfNeeded(baseline, metricsSnapshot, time.Now().UTC())
	activeLevel := escalated.NewLevel

	go func() {
		defer close(ch)

		// 5a. Emit the `status` event first so the UI can lock the
		// header before any tokens land. The handler translates
		// this into SSE `event: status\ndata: ...\n\n`.
		statusEvent := ports.StreamChatEvent{
			Type:    "status",
			Content: fmt.Sprintf("level=%s offer_hint=%t", activeLevel.String(), escalated.ShouldEmitOfferHint),
		}
		if statusEvent.Level == nil {
			lvl := activeLevel
			statusEvent.Level = &lvl
		}
		select {
		case <-ctx.Done():
			return
		case ch <- statusEvent:
		}

		// If the auto-escalation state machine says "user is
		// stuck", emit a `status` event the UI can decode as
		// "¿Querés una pista?".
		if escalated.ShouldEmitOfferHint {
			select {
			case <-ctx.Done():
				return
			case ch <- ports.StreamChatEvent{
				Type:    "status",
				Content: "offer_hint",
				Level:   levelPtr(activeLevel),
			}:
			}
		}

		// 3. Build the ChatContext for the LLM.
		messages, err := assembleChatContext(ctx, s.profileRepo, s.cache, s.prompts, req, activeLevel)
		if err != nil {
			select {
			case <-ctx.Done():
			case ch <- ports.StreamChatEvent{
				Type:    "error",
				Content: err.Error(),
			}:
			}
			return
		}

		// 4. Cache the user's message so the next LLM call has
		// rolling-window history.
		if s.cache != nil {
			s.cache.Set(ctx, sessionID, models.ChatMessage{
				Role:      models.ChatRoleUser,
				Content:   req.Message,
				Timestamp: time.Now().UTC(),
			})
		}

		// 5b. Drive the cascade. Each token the LLM emits is
		// forwarded 1:1 onto ch. The callback returns an error
		// when the SSE writer has back-pressure (broken pipe);
		// we propagate to stop the cascade cleanly.
		opts := ports.StreamChatOptions{
			PreferredProvider: req.PreferredProvider,
			Timeout:          5 * time.Second,
		}
		llmErr := s.llmClient.StreamChat(ctx, messages, opts, func(ev ports.LLMStreamEvent) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			out := ports.StreamChatEvent{
				Type:    ev.Type,
				Content: ev.Content,
			}
			if ev.Level != nil {
				lvl := *ev.Level
				out.Level = &lvl
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- out:
				return nil
			}
		})
		if llmErr != nil {
			select {
			case <-ctx.Done():
				return
			case ch <- ports.StreamChatEvent{
				Type:    "error",
				Content: llmErr.Error(),
			}:
			}
			return
		}

		// 6. Done event after the cascade completes; closes the
		// stream. The HTTP handler maps this to SSE `event: done`.
		select {
		case <-ctx.Done():
			return
		case ch <- ports.StreamChatEvent{Type: "done"}:
		}

		// 7. Background summarization every 10 messages.
		snap := s.cache.Snapshot(ctx, sessionID)
		if len(snap) >= 10 {
			go func(uid, sess string) {
				_ = s.SummarizeSession(context.Background(), uid, sess)
			}(req.UserID, sessionID)
		}
	}()
	return ch, nil
}

// levelPtr is a tiny helper to grab the active level's address for
// the SSE event. Avoids creating a temp variable in two places.
func levelPtr(level models.SocraticLevel) *models.SocraticLevel {
	l := level
	return &l
}

// ProcessCommand is the single entry for the 7 slash commands
// (/nivel, /pista, /solucion, /reset, /perfil, /diccionario, /help).
// PR 3 (frontend) is documented to parse them client-side; this
// server-side method is the durable backend path used when the
// frontend sends the bare command string in a StreamChat request, or
// for tests that bypass the UI layer.
//
// The dispatch table below maps the canonical command verb (already
// extracted + lowercased by canonicalCommand) to its handler. Each
// handler is side-effect-only on this layer; /pista, /solucion,
// /perfil, /diccionario and /help bridge to MCP tools the PR 3
// frontend will also call — the server path is here for tests and
// future non-UI callers.
func (s *TutorService) ProcessCommand(ctx context.Context, userID, command string) error {
	verb := canonicalCommand(command)
	body := trimSlash(command)
	switch verb {
	case "":
		return fmt.Errorf("%w: empty command", ErrTutorCommandUnknown)
	case "nivel":
		return s.handleNivel(ctx, userID, body)
	case "pista":
		s.escalation.RecordHintRequest(userID, inferChallengeFromBody(body))
		return nil
	case "solucion":
		s.escalation.RecordGiveUp(userID, inferChallengeFromBody(body))
		return nil
	case "reset":
		// /reset without a challenge id clears all the user's
		// sessions in both the escalation tracker AND the cache.
		challenge := inferChallengeFromBody(body)
		if challenge == "" || challenge == "reset" {
			challenge = ""
		}
		s.escalation.ResetSession(userID, challenge)
		if s.cache != nil {
			if challenge == "" {
				for _, key := range s.cache.Keys(ctx) {
					if strings.HasPrefix(key, userID) {
						s.cache.Reset(ctx, key)
					}
				}
			} else {
				s.cache.Reset(ctx, inferSessionID(userID, challenge))
			}
		}
		return nil
	case "perfil":
		return nil
	case "diccionario":
		return nil
	case "help":
		return nil
	}
	return fmt.Errorf("%w: %s", ErrTutorCommandUnknown, verb)
}

// handleNivel parses the trailing token and updates the
// LearningProfile.NivelSocratismo. Out-of-range values are rejected
// with ErrTutorCommandUnknown so the caller can surface a friendly
// "Nivel inválido. Usá /nivel 0-3" message (spec scenario "Invalid
// command"). The command body is "/nivel 0|1|2|3" — we parse the
// first whitespace-separated token after the verb.
func (s *TutorService) handleNivel(ctx context.Context, userID, body string) error {
	fields := strings.Fields(body)
	if len(fields) < 2 {
		return fmt.Errorf("%w: /nivel expects a value in [0..3]", ErrTutorCommandUnknown)
	}
	level, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("%w: /nivel expects an integer, got %q", ErrTutorCommandUnknown, fields[1])
	}
	if level < 0 || level > 3 {
		return fmt.Errorf("%w: /nivel level must be in [0..3], got %d", ErrTutorCommandUnknown, level)
	}
	profile, err := s.profileRepo.GetLearningProfile(ctx, userID)
	if err != nil {
		return fmt.Errorf("tutor /nivel: load profile: %w", err)
	}
	profile.Preferencias.NivelSocratismo = level
	profile.UltimaActualizacion = time.Now().UTC()
	if err := s.profileRepo.UpdateLearningProfile(ctx, profile); err != nil {
		return fmt.Errorf("tutor /nivel: persist profile: %w", err)
	}
	return nil
}

// inferChallengeFromBody devuelve la "challenge id" que el usuario
// tenía en mente al ejecutar el comando. Sin un campo dedicado en
// el wire shape, hacemos una extracción heurística: el último
// token del cuerpo. Es un stub razonable que el handler refina
// cuando el frontend envía un payload enriquecido.
func inferChallengeFromBody(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// inferSessionID compone la sessionID cache-friendly = userID:challengeID.
// El cache service usa este patrón.
func inferSessionID(userID, challengeID string) string {
	if challengeID == "" {
		return userID
	}
	return userID + ":" + challengeID
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
	// Trim leading whitespace. Doing this before the slash strip
	// keeps "   /nivel" working the same as "/nivel".
	for len(out) > 0 && (out[0] == ' ' || out[0] == '\t') {
		out = out[1:]
	}
	// Strip a single leading "/" if present.
	if len(out) > 0 && out[0] == '/' {
		out = out[1:]
	}
	// Trim trailing whitespace (rare on chat input, but cheap).
	for len(out) > 0 && (out[len(out)-1] == ' ' || out[len(out)-1] == '\t' || out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return out
}
