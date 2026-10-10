package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// maxChatContextCodeBytes caps the per-request code blob at 2KB so a
// pathological Monaco payload can't blow up the LLM context window.
// Matches the audit session cache's truncation boundary (the spec
// mentions 2KB explicitly as the comparison value).
const maxChatContextCodeBytes = 2048

// assembleChatContext loads the user profile + learning profile and
// stitches together the 5-system-message ChatContext that TutorService
// passes to LLMClientPort.StreamChat. The shape mirrors design.md §3:
//
//	1. Identity    — tutor-system.md
//	2. Socratic    — tutor-socratic-{N}.md, N = active level
//	3. UserSummary — last LearningProfile.Preferencias.Resumen (or empty)
//	4. Location    — active route, challenge, truncated code, output
//	5. History     — last 10 cache messages + idioma + max msg length
//
// The fifth message also carries the user's idioma preference so the
// LLM respects spec scenario "Response respects language preference"
// (spec.md#Chat-Restrictions).
func assembleChatContext(
	ctx context.Context,
	repo ports.UserProgressRepository,
	cache ports.CachePort,
	prompts *TutorPrompts,
	req ports.StreamChatRequest,
	level models.SocraticLevel,
) ([]models.ChatMessage, error) {
	language := normalizeLanguage(req.Route)
	profile, err := repo.GetLearningProfile(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("tutor context: load profile: %w", err)
	}
	hist := []models.ChatMessage{}
	if cache != nil {
		hist = cache.Snapshot(ctx, req.UserID+":"+req.ChallengeID)
	}

	msgs := make([]models.ChatMessage, 0, 6+len(hist))

	// 1. Identity
	msgs = append(msgs, models.ChatMessage{
		Role:    models.ChatRoleSystem,
		Content: prompts.System,
	})

	// 2. Socratic level
	msgs = append(msgs, models.ChatMessage{
		Role:    models.ChatRoleSystem,
		Content: prompts.SocraticPromptFor(level),
	})

	// 3. User summary (last persisted Resumen on LearningProfile)
	summary := strings.TrimSpace(profile.Preferencias.Resumen)
	if summary == "" {
		summary = "Sin resumen previo en el perfil de aprendizaje."
	}
	msgs = append(msgs, models.ChatMessage{
		Role: models.ChatRoleSystem,
		Content: fmt.Sprintf(
			"User summary (id=%s, rango global=%s, racha=%d): %s",
			req.UserID, profile.Preferencias.Idioma, 0, summary,
		),
	})

	// 4. Location + language
	code := req.Code
	if len(code) > maxChatContextCodeBytes {
		code = code[:maxChatContextCodeBytes] + "\n// …(truncated)…"
	}
	locText := fmt.Sprintf(
		"Location: route=%s challenge=%s language=%s\nCode (%d chars):\n```\n%s\n```\nSandbox output:\n%s",
		req.Route, req.ChallengeID, language, len(code), code, req.Output,
	)
	msgs = append(msgs, models.ChatMessage{
		Role:    models.ChatRoleSystem,
		Content: locText,
	})

	// 5. History + preferences (idioma + max-message length)
	maxLen := profile.Preferencias.LongitudMaximaMsg
	if maxLen <= 0 {
		maxLen = 200
	}
	idioma := profile.Preferencias.Idioma
	if idioma == "" {
		idioma = "es"
	}
	historyText := buildHistoryText(hist)
	prefsText := fmt.Sprintf(
		"Preferences: idioma=%s nivel_socratismo=%d longitud_maxima_msg=%d",
		idioma, int(level), maxLen,
	)
	msgs = append(msgs, models.ChatMessage{
		Role:    models.ChatRoleSystem,
		Content: historyText + "\n" + prefsText,
	})

	// The user's actual message travels as the last user-role
	// message — the LLM treats it as the prompt to respond to.
	timestamp := time.Now().UTC()
	msgs = append(msgs, models.ChatMessage{
		Role:      models.ChatRoleUser,
		Content:   req.Message,
		Timestamp: timestamp,
	})
	return msgs, nil
}

// normalizeLanguage pulls a language hint from the request. The
// StreamChatRequest doesn't currently carry a `Language` field (PR 1's
// signature), so we default to Spanish until the wire shape lands.
// Keeping the helper centralizes the "where does the language come
// from" question for a future PR 3 reviewer.
func normalizeLanguage(route string) string {
	// Future: read from req.Language when the wire shape grows it.
	_ = route
	return "es"
}

// buildHistoryText renders the rolling window as a flat transcript
// the LLM can ingest. We keep timestamps out of the wire (the LLM
// doesn't need wall-clock); only role+content are encoded.
func buildHistoryText(hist []models.ChatMessage) string {
	if len(hist) == 0 {
		return "History: (empty)"
	}
	var b strings.Builder
	b.WriteString("History (most recent):\n")
	for _, m := range hist {
		role := string(m.Role)
		if role == "" {
			role = "user"
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", role, m.Content))
	}
	return strings.TrimRight(b.String(), "\n")
}
