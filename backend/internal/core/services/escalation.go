package services

import (
	"strings"
	"sync"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// sessionKey is the (user, challenge) tuple that scopes the
// auto-escalation state machine. It matches the spec "state is
// keyed by (userID, challengeID)" invariant from design.md §8.
type sessionKey struct {
	userID      string
	challengeID string
}

// EscalationMetrics tracks the live counters used by
// EscalateIfNeeded. All fields are value types so an empty
// EscalationMetrics{} is a valid zero-state.
type EscalationMetrics struct {
	// Failures counts test-case failures on the same challenge.
	Failures int
	// GiveUpCounts counts /solucion requests on the same challenge.
	GiveUpCounts int
	// LastUserMessage is the timestamp of the most recent user
	// message on this (user, challenge). Reset by every Set step.
	LastUserMessage time.Time
	// SolvedNoHints records whether the user solved the challenge
	// without asking for any hints. Cleared on /reset.
	SolvedNoHints bool
	// HintsUsed counts /pista calls. Used by the "solved AND
	// hintsUsed == 0" trigger to escalate the level up.
	HintsUsed int
}

// EscalationTracker holds the per-session metrics. It is NOT a port
// — there is no out-of-process persistence requirement; the tracker
// is in-memory and survives only within a process. Each TutorService
// owns its tracker.
type EscalationTracker struct {
	mu      sync.Mutex
	metrics map[sessionKey]*EscalationMetrics
}

// NewEscalationTracker returns an empty tracker. The constructor
// initializes the map so callers can call any method without first
// triggering a nil-map lazy initialization.
func NewEscalationTracker() *EscalationTracker {
	return &EscalationTracker{
		metrics: make(map[sessionKey]*EscalationMetrics),
	}
}

// escalateKey returns the inner metrics pointer, creating an empty
// record if needed. Caller must hold t.mu.
func (t *EscalationTracker) escalateKey(k sessionKey) *EscalationMetrics {
	if m, ok := t.metrics[k]; ok {
		return m
	}
	m := &EscalationMetrics{}
	t.metrics[k] = m
	return m
}

// getSnapshot returns a defensive copy of the metrics for the given
// key, or the zero value if the key is unknown. Callers should use
// this when they want to react to a read-only observation (e.g. a
// handler-side debug endpoint).
func (t *EscalationTracker) getSnapshot(k sessionKey) EscalationMetrics {
	t.mu.Lock()
	defer t.mu.Unlock()
	m, ok := t.metrics[k]
	if !ok {
		return EscalationMetrics{}
	}
	return *m
}

// RecordFailure increments the failure counter for the session.
// Called by the SSE handler whenever the underlying audit flow reports
// a test failure on the active challenge.
func (t *EscalationTracker) RecordFailure(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := sessionKey{userID: userID, challengeID: challengeID}
	m := t.escalateKey(k)
	m.Failures++
	m.LastUserMessage = time.Now().UTC()
}

// RecordGiveUp increments the give-up counter. Called by /solucion.
func (t *EscalationTracker) RecordGiveUp(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := sessionKey{userID: userID, challengeID: challengeID}
	m := t.escalateKey(k)
	m.GiveUpCounts++
	m.LastUserMessage = time.Now().UTC()
}

// RecordHintRequest increments the hints-used counter. Called by /pista.
func (t *EscalationTracker) RecordHintRequest(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := sessionKey{userID: userID, challengeID: challengeID}
	m := t.escalateKey(k)
	m.HintsUsed++
	m.LastUserMessage = time.Now().UTC()
}

// RecordUserMessage stamps the last-message timestamp. The "stuck >
// 10m" trigger compares against this.
func (t *EscalationTracker) RecordUserMessage(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := sessionKey{userID: userID, challengeID: challengeID}
	m := t.escalateKey(k)
	m.LastUserMessage = time.Now().UTC()
}

// MarkSolvedNoHints flags the session as solved without hints. The
// "solved AND hintsUsed==0 AND level<3" trigger consumes it.
func (t *EscalationTracker) MarkSolvedNoHints(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := sessionKey{userID: userID, challengeID: challengeID}
	m := t.escalateKey(k)
	m.SolvedNoHints = true
	m.LastUserMessage = time.Now().UTC()
}

// ResetSession wipes the metrics for the (user, challenge) pair.
// Called by /reset.
func (t *EscalationTracker) ResetSession(userID, challengeID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.metrics, sessionKey{userID: userID, challengeID: challengeID})
}

// EscalateResult is the outcome of one EscalateIfNeeded call.
//
//   - NewLevel is the level the LLM should use for the next call.
//   - ShouldEmitOfferHint is true when the trigger says "ask the user
//     if they want a hint" — the handler turns that into a
//     `status: offer_hint` SSE event with the fixed prompt
//     "¿Querés una pista?".
//   - LevelChanged is true when NewLevel differs from the input level
//     — lets the handler decide whether to fire a `status:
//     escalated` event for the UI badge.
type EscalateResult struct {
	NewLevel            models.SocraticLevel
	ShouldEmitOfferHint bool
	LevelChanged        bool
}

// stuckThreshold caps the "no code change for >10m → offer hint"
// trigger. The value comes from design.md §8 ("time_on_challenge >
// 10m && code_unchanged → emit special prompt"). PR 2 uses time since
// last user message as a proxy for "no progress" since code-unchanged
// detection belongs to the audit flow, not the chat layer.
const stuckThreshold = 10 * time.Minute

// EscalateIfNeeded applies the spec's auto-escalation rules
// (spec.md#Auto-Escalation) to the supplied baseline level, producing
// the level the next LLM call should use. It is a pure function over
// (current level, metrics, now): no I/O, no logging, no side effects.
//
// Triggers (priority order — first match wins):
//
//  1. failures ≥ 2  AND level > 0 → level - 1   (escalate: more guided)
//  2. give_up ≥ 3   AND level > 0 → level - 1   (genuinely stuck: be more direct)
//  3. now - LastUserMessage > stuckThreshold → keep level, emit hint
//  4. solved_no_hints AND hints_used == 0 AND level < 3 → level + 1
//     (more questioning next time)
//
// The function is intentionally stateless beyond its metrics input so
// it can be unit-tested without constructing a TutorService.
func EscalateIfNeeded(baseline models.SocraticLevel, m EscalationMetrics, now time.Time) EscalateResult {
	level := baseline
	if !level.Valid() {
		level = models.SocraticGuided
	}

	// Rule 1: two failures → escalate down one (more direct).
	if m.Failures >= 2 && level > models.SocraticDirect {
		newLevel := level - 1
		return EscalateResult{NewLevel: newLevel, LevelChanged: newLevel != baseline}
	}

	// Rule 2: three /solucion calls → escalate down one.
	if m.GiveUpCounts >= 3 && level > models.SocraticDirect {
		newLevel := level - 1
		return EscalateResult{NewLevel: newLevel, LevelChanged: newLevel != baseline}
	}

	// Rule 3: stuck > 10m without progress → offer a hint, keep
	// level. We only fire when the metrics actually track a last-
	// message timestamp; fresh sessions don't qualify.
	if !m.LastUserMessage.IsZero() && now.Sub(m.LastUserMessage) > stuckThreshold {
		return EscalateResult{
			NewLevel:            level,
			ShouldEmitOfferHint: true,
			LevelChanged:        level != baseline,
		}
	}

	// Rule 4: solved without hints → escalate up one (more
	// questioning next time).
	if m.SolvedNoHints && m.HintsUsed == 0 && level < models.SocraticDiscovery {
		newLevel := level + 1
		return EscalateResult{NewLevel: newLevel, LevelChanged: newLevel != baseline}
	}

	return EscalateResult{NewLevel: level, LevelChanged: level != baseline}
}

// canonicalCommand extracts the verb from a slash command string
// ("/nivel 0" → "nivel"). It trims whitespace and is shared between
// ProcessCommand (server-side) and any future test that wants to
// assert the same logic via the application boundary.
func canonicalCommand(command string) string {
	body := trimSlash(command)
	if body == "" {
		return ""
	}
	for i, r := range body {
		if r == ' ' || r == '\t' || r == '\n' {
			return strings.ToLower(body[:i])
		}
	}
	return strings.ToLower(body)
}
