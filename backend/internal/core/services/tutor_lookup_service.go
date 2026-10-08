package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrTutorLookupUnavailable is returned when the tutor lookup service is
// invoked without a working repository (defense-in-depth during startup
// races or in tests that intentionally defer wiring).
var ErrTutorLookupUnavailable = errors.New("tutor lookup service unavailable")

// TutorLookupService bridges the user's question to the personal
// dictionary and emits a socratic-level-aware framing. It implements the
// dictionary spec requirement "Tutor Chat Auto-Lookup" with three
// scenarios:
//
//   - term found -> return the best hit plus a level-shaped framing
//   - term missing -> return no term + a structured miss signal so the
//     caller (LLM or scripted tutor) can fall back
//     to its general knowledge without exposing the
//     "the dictionary has nothing" detail to the user
//   - socratic level 0..3 -> shape the framing opening so the LLM
//     guidance keeps the answer style aligned with the
//     user profile.
//
// The service depends ONLY on DictionaryService to keep the surface
// narrow; it does NOT need ports.DictionaryRepository directly because
// every access is already user-scoped and validated at the service
// boundary.
type TutorLookupService struct {
	dict *DictionaryService
}

// NewTutorLookupService wires the tutor bridge to a dictionary service.
func NewTutorLookupService(dict *DictionaryService) *TutorLookupService {
	if dict == nil {
		panic("tutor lookup: DictionaryService must not be nil")
	}
	return &TutorLookupService{dict: dict}
}

// TutorLookupInput is the caller's query shape. Query is the raw user
// utterance ("Qué es PR?"); Level is the configured socratic level
// (0=direct, 1=guided, 2=socratic, 3=discovery).
type TutorLookupInput struct {
	UserID string
	Query  string
	Level  int
}

// TutorLookupResult is what the LLM / script receives. The shape is
// deliberately explicit so the LLM can branch on the booleans without
// guessing JSON layouts.
//
//	Found     true when at least one dictionary hit exists for Query.
//	Term      the top hit by ts_rank (nil when Found is false).
//	Framing   socratic-level-aware opening text. Always present so the
//	          caller never has to invent its own intro.
//	Source    "dictionary" when Found, "general" otherwise. Tutors
//	          MUST NOT surface this to the user; it's for audit /
//	          prompt debugging only.
//	SocraticLevel echoes the input level clamped to [0..3].
type TutorLookupResult struct {
	Found         bool         `json:"found"`
	Term          *models.Term `json:"term,omitempty"`
	Framing       string       `json:"framing"`
	Source        string       `json:"source"`
	SocraticLevel int          `json:"socratic_level"`
}

// clampLevel normalizes the configured socratic level to the documented
// 0..3 range. Out-of-range inputs are clamped, not raised, so the
// caller (LLM prompt chain) is never broken by stale settings.
func clampLevel(level int) int {
	if level < 0 {
		return 0
	}
	if level > 3 {
		return 3
	}
	return level
}

// framingForFound shapes the dictionary-aware opening into a socratic
// prompt. Higher levels hide the full answer and guide the user instead.
//
//	Level 0 (direct)      — "By your dictionary: <explanation>."
//	Level 1 (guided)      — "Recalling your note on PR: <explanation>.
//	                         Want me to go deeper or move on?"
//	Level 2 (socratic)    — "You saved a note on PR. What's the main
//	                         thing you remember about it?"
//	Level 3 (discovery)   — "There is a term called PR in your
//	                         glossary. What does it remind you of?"
func framingForFound(term *models.Term, level int) string {
	termName := strings.TrimSpace(term.Term)
	explanation := strings.TrimSpace(term.Explanation)
	switch level {
	case 0:
		return fmt.Sprintf("By your dictionary: %s — %s.", termName, explanation)
	case 1:
		return fmt.Sprintf("Recalling your note on %s: %s. Want me to go deeper or move on?", termName, explanation)
	case 2:
		return fmt.Sprintf("You saved a note on %s. What's the main thing you remember about it?", termName)
	default: // 3 (discovery) and anything else defensively falls here
		return fmt.Sprintf("There is a term called %s in your glossary. What does it remind you of?", termName)
	}
}

// framingForMissing shapes the fallback opening used when no dictionary
// hit exists. It deliberately does NOT expose the dictionary absence to
// the user — the LLM gets a "general source" framing regardless.
//
//	Level 0 (direct)      — "Based on what I know:"
//	Level 1 (guided)      — "Let's reason through it together. What
//	                         do you already know about PR?"
//	Level 2 (socratic)    — "What comes to mind when you hear PR?"
//	Level 3 (discovery)   — "Curious — what might PR stand for in our
//	                         context?"
func framingForMissing(query string, level int) string {
	clean := sanitizeQueryForFallback(query)
	switch level {
	case 0:
		return fmt.Sprintf("Based on what I know about %s:", clean)
	case 1:
		return fmt.Sprintf("Let's reason through it together. What do you already know about %s?", clean)
	case 2:
		return fmt.Sprintf("What comes to mind when you hear %s?", clean)
	default: // 3
		return fmt.Sprintf("Curious — what might %s stand for in our context?", clean)
	}
}

// sanitizeQueryForFallback trims the user's query down to a noun-like
// fragment the framing text can reuse. Used ONLY for the framing when
// no term was hit, so the LLM's prompt stays coherent and the user
// string never leaks punctuation/quotes that would break the LLM.
func sanitizeQueryForFallback(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return "this"
	}
	// Drop trailing punctuation the tutor would replay verbatim.
	q = strings.TrimRight(q, ".!?")
	// Trim full sentences to the last clause if it sounds like the user
	// asked a definition ("que es X") — keep the noun phrase only.
	low := strings.ToLower(q)
	for _, prefix := range []string{"que es ", "qué es ", "what is ", "what's ", "whats ", "define "} {
		if strings.HasPrefix(low, prefix) {
			return strings.TrimSpace(q[len(prefix):])
		}
	}
	return q
}

// TutorLookup performs the dictionary auto-lookup. Empty user_id or
// whitespace-only query returns a structured "no-op" result so the
// caller can short-circuit without branching on error envelopes.
func (s *TutorLookupService) TutorLookup(ctx context.Context, in TutorLookupInput) (*TutorLookupResult, error) {
	if s == nil || s.dict == nil {
		return nil, ErrTutorLookupUnavailable
	}
	if strings.TrimSpace(in.UserID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrDictionaryValidation)
	}
	level := clampLevel(in.Level)
	query := strings.TrimSpace(in.Query)
	if query == "" {
		// No query — return a deterministic empty result.
		return &TutorLookupResult{
			Found:         false,
			Framing:       framingForMissing("", level),
			Source:        "general",
			SocraticLevel: level,
		}, nil
	}
	hits, err := s.dict.SearchTerms(ctx, in.UserID, query)
	if err != nil {
		return nil, fmt.Errorf("tutor lookup search: %w", err)
	}
	if len(hits) == 0 {
		return &TutorLookupResult{
			Found:         false,
			Framing:       framingForMissing(query, level),
			Source:        "general",
			SocraticLevel: level,
		}, nil
	}
	top := hits[0]
	return &TutorLookupResult{
		Found:         true,
		Term:          top,
		Framing:       framingForFound(top, level),
		Source:        "dictionary",
		SocraticLevel: level,
	}, nil
}
