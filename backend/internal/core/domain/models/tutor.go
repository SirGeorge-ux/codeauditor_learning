package models

import (
	"strconv"
	"time"
)

// SocraticLevel is the tutor's pedagogical level (0=direct … 3=discovery).
// The numeric values align with the JSON tag on Preferencias.NivelSocratismo
// (learning_profile.go) so an integer can travel across JSON without any
// translation layer. Constants with names document the call-site intent so
// the auto-escalation state machine (PR 2) does not pass arbitrary ints.
//
// Per the tutor-chat spec (spec.md#Socratic-Levels) + design.md §2, the
// default level is derived from the user's rank and may be overridden by
// auto-escalation triggers (failures≥2, give-up≥3, stuck>10m, solved
// without hints). Out-of-range values are rejected by Valid() so an
// LLM caller cannot invent a fifth level through an MCP tool call.
type SocraticLevel int

// Canonical level constants. The numeric values match the JSON wire shape
// of Preferencias.NivelSocratismo so a SocraticLevel can travel across
// JSON without transcoding.
const (
	// SocraticDirect: name the code smell + give a 1–3 line directional
	// snippet. Reserved for B–A / Senior / 2+ failures.
	SocraticDirect SocraticLevel = 0
	// SocraticGuided: one question + one hint. Reserved for D–C / Mid.
	SocraticGuided SocraticLevel = 1
	// SocraticPure: questions only. Reserved for F–E / Junior (the
	// LearningProfile default).
	SocraticPure SocraticLevel = 2
	// SocraticDiscovery: breadcrumbs + confrontation. Reserved for
	// S / Architect / solved-without-hints.
	SocraticDiscovery SocraticLevel = 3
)

// Valid reports whether the level is in the configured 0..3 range.
// Used by auto-escalation (PR 2) and the update_learning_profile MCP
// tool (already enforces this rule).
func (l SocraticLevel) Valid() bool {
	return l >= SocraticDirect && l <= SocraticDiscovery
}

// String returns the canonical label of the level. The label is the
// wire shape for the SSE `status` event emitted when auto-escalation
// adjusts the level. An out-of-range level returns "unknown" so the
// auto-escalation state machine can log anomalies without inventing a
// lookalike label.
func (l SocraticLevel) String() string {
	switch l {
	case SocraticDirect:
		return "direct"
	case SocraticGuided:
		return "guided"
	case SocraticPure:
		return "socratic"
	case SocraticDiscovery:
		return "discovery"
	default:
		return "unknown"
	}
}

// MarshalJSON serializes the level as the underlying integer so the
// shape matches Preferencias.NivelSocratismo (also `int`). Decoder in
// the frontend + tutors' tool layer expect a number, not a string.
func (l SocraticLevel) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(l))), nil
}

// UnmarshalJSON parses the integer wire shape. Out-of-range values are
// accepted at parse time; Valid() is the gate that rejects them at
// the auto-escalation boundary.
func (l *SocraticLevel) UnmarshalJSON(data []byte) error {
	v, err := strconv.Atoi(string(data))
	if err != nil {
		return err
	}
	*l = SocraticLevel(v)
	return nil
}

// ChatRole identifies the author of a ChatMessage. The three canonical
// roles are the only sender types the tutor chat can produce — system
// messages come from ChatContext assembly (PR 2), user from the
// frontend, tutor from the LLM.
type ChatRole string

const (
	ChatRoleUser   ChatRole = "user"
	ChatRoleTutor  ChatRole = "tutor"
	ChatRoleSystem ChatRole = "system"
)

// ChatMessage is a single entry in the tutor chat's 10-message rolling
// window. The Level field is set only on tutor messages so the auto-
// escalation state machine (PR 2) can correlate responses to the level
// that produced them.
//
// JSON tags are aligned with the SSE wire shape consumed by the Angular
// frontend (PR 3) — timestamp is RFC3339 by default via time.Time.
type ChatMessage struct {
	Role      ChatRole       `json:"role"`
	Content   string         `json:"content"`
	Level     *SocraticLevel `json:"level,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// ChallengeLocationState is the snapshot of the user's challenge at
// the moment they sent a tutor message. Code is sanitized at the
// boundary by the prompt-injection guard (matches the patterns from
// mcp-pedagogical.get_socratic_prompt) so a malicious code blob cannot
// take over the system prompt.
type ChallengeLocationState struct {
	ChallengeID   string `json:"challengeId,omitempty"`
	Code          string `json:"code,omitempty"`
	SandboxOutput string `json:"sandboxOutput,omitempty"`
}

// ChatLocationContext is the user's navigational + challenge position.
// Route is the active Angular route (eg "/dojo/ch-sqli"). Challenge is
// nil when the chat is opened outside a challenge (eg. on the
// dashboard).
type ChatLocationContext struct {
	Route     string                  `json:"route"`
	Challenge *ChallengeLocationState `json:"challenge,omitempty"`
}

// ChatUserContext is the minimal user identity the LLM needs to
// personalize responses without leaking PII. The full profile is held
// by mcp-user.get_profile; this is the in-context projection.
type ChatUserContext struct {
	UserID     string `json:"userId"`
	GlobalRank string `json:"globalRank"`
	StreakDays int    `json:"streakDays"`
}

// ChatLanguageContext is the per-language learning state. Rank is the
// F-S or Junior-Architect label per the gamification spec. SuccessRate
// is a float in [0, 1].
type ChatLanguageContext struct {
	Language    string  `json:"language"`
	Rank        string  `json:"rank"`
	Points      int     `json:"points"`
	SuccessRate float64 `json:"successRate"`
}

// ChatHistoryContext is the recent tutor interaction surface used by
// the LLM to keep continuity: hints the user already saw, and the
// code smells that historically trip them up.
//
// Source fields on the production side come from mcp-user.get_learning_profile
// (PerfilAprendizaje.bloqueos_tipicos, profile.resumen).
type ChatHistoryContext struct {
	RecentHints     []string `json:"recentHints,omitempty"`
	TypicalBlockers []string `json:"typicalBlockers,omitempty"`
}

// ChatPreferencesContext mirrors the slice of LearningProfile that
// governs every LLM call: idioma determines the response language,
// SocraticLevel is the active level (after auto-escalation overrides),
// MaxMessageLength caps the response tokens (PR 2 wires this into the
// truncation prompt).
type ChatPreferencesContext struct {
	Language         string        `json:"language"`
	SocraticLevel    SocraticLevel `json:"socraticLevel"`
	MaxMessageLength int           `json:"maxMessageLength"`
}

// ChatContext is the snapshot the tutor service assembles before every
// LLM call (per spec.md#ChatContext-Assembly). Each field corresponds
// to one of the 5 system messages the backend injects:
// user → language → location → history → preferences.
//
// All fields are value types (no nil-pointer traps on zero value) so
// the JSON encoder round-trips cleanly even when the user is in a
// pre-challenge route.
type ChatContext struct {
	User        ChatUserContext        `json:"user"`
	Language    ChatLanguageContext    `json:"language"`
	Location    ChatLocationContext    `json:"location"`
	History     ChatHistoryContext     `json:"history"`
	Preferences ChatPreferencesContext `json:"preferences"`
}
