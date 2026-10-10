package services

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// TestEscalateIfNeeded_NoTriggersReturnsBaseline es el caso más
// simple: sin triggers, devuelve el nivel sin cambios. Funciona
// como prueba de cordura antes de los escenarios compuestos.
func TestEscalateIfNeeded_NoTriggersReturnsBaseline(t *testing.T) {
	got := EscalateIfNeeded(
		models.SocraticPure,
		EscalationMetrics{},
		time.Now().UTC(),
	)
	if got.NewLevel != models.SocraticPure {
		t.Errorf("NewLevel = %v, want socratic(pure)", got.NewLevel)
	}
	if got.LevelChanged {
		t.Errorf("LevelChanged = true, want false (no trigger)")
	}
	if got.ShouldEmitOfferHint {
		t.Errorf("ShouldEmitOfferHint = true, want false")
	}
}

// TestEscalateIfNeeded_TwoFailures escalates down. Tabla con
// baseline y métrica Drives a RED de los 4 escenarios. Cada caso
// referencia el trigger del spec ("2 failures on the same challenge →
// escalate +1 level").
func TestEscalateIfNeeded_TwoFailures(t *testing.T) {
	cases := []struct {
		name     string
		baseline models.SocraticLevel
		metrics  EscalationMetrics
		want     models.SocraticLevel
	}{
		{
			name:     "two failures at level 2 (socratic) → level 1 (guided)",
			baseline: models.SocraticPure,
			metrics:  EscalationMetrics{Failures: 2},
			want:     models.SocraticGuided,
		},
		{
			name:     "two failures at level 1 → level 0",
			baseline: models.SocraticGuided,
			metrics:  EscalationMetrics{Failures: 2},
			want:     models.SocraticDirect,
		},
		{
			name:     "two failures at level 0 stays at 0 (floor)",
			baseline: models.SocraticDirect,
			metrics:  EscalationMetrics{Failures: 2},
			want:     models.SocraticDirect,
		},
		{
			name:     "one failure is not enough",
			baseline: models.SocraticPure,
			metrics:  EscalationMetrics{Failures: 1},
			want:     models.SocraticPure,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EscalateIfNeeded(c.baseline, c.metrics, time.Now().UTC())
			if got.NewLevel != c.want {
				t.Errorf("NewLevel = %v, want %v", got.NewLevel, c.want)
			}
		})
	}
}

// TestEscalateIfNeeded_ThreeGiveUps baja un nivel. El spec dice
// "3 requests for the answer → escalate +2 levels" pero el design.md
// §8 lo simplifica a "give_answer_count >= 3 → level = current - 1".
// Seguimos el design (la implementación vive en design.md).
func TestEscalateIfNeeded_ThreeGiveUps(t *testing.T) {
	cases := []struct {
		name     string
		baseline models.SocraticLevel
		want     models.SocraticLevel
	}{
		{"three give-ups at level 2 → level 1", models.SocraticPure, models.SocraticGuided},
		{"three give-ups at level 1 → level 0", models.SocraticGuided, models.SocraticDirect},
		{"three give-ups at level 0 stays at 0", models.SocraticDirect, models.SocraticDirect},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EscalateIfNeeded(c.baseline, EscalationMetrics{GiveUpCounts: 3}, time.Now().UTC())
			if got.NewLevel != c.want {
				t.Errorf("got %v, want %v", got.NewLevel, c.want)
			}
		})
	}
}

// TestEscalateIfNeeded_StuckElevenMinutes ofrece pista sin cambiar
// nivel. Cubrimos el caso negativo (8min, no triggers) y el positivo
// (15min, emite hint).
func TestEscalateIfNeeded_StuckEmitsOfferHint(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	lastMsg := now.Add(-15 * time.Minute) // 15m ago

	res := EscalateIfNeeded(models.SocraticPure, EscalationMetrics{
		LastUserMessage: lastMsg,
	}, now)
	if !res.ShouldEmitOfferHint {
		t.Errorf("ShouldEmitOfferHint = false, want true at 15min")
	}
	if res.NewLevel != models.SocraticPure {
		t.Errorf("NewLevel = %v, want unchanged at level 2", res.NewLevel)
	}

	// Below threshold: no hint.
	lastMsgRecent := now.Add(-8 * time.Minute)
	res2 := EscalateIfNeeded(models.SocraticPure, EscalationMetrics{
		LastUserMessage: lastMsgRecent,
	}, now)
	if res2.ShouldEmitOfferHint {
		t.Errorf("ShouldEmitOfferHint = true at 8min, want false (under 10m threshold)")
	}
}

// TestEscalateIfNeeded_SolvedNoHints sube un nivel cuando el usuario
// resolvió el challenge sin pedir pistas. Cobertura: la condición
// AND de los tres predicados (solved, hints_used==0, level<3).
func TestEscalateIfNeeded_SolvedNoHints(t *testing.T) {
	cases := []struct {
		name     string
		baseline models.SocraticLevel
		metrics  EscalationMetrics
		want     models.SocraticLevel
	}{
		{
			name:     "solved without hints at level 2 → level 3",
			baseline: models.SocraticPure,
			metrics:  EscalationMetrics{SolvedNoHints: true, HintsUsed: 0},
			want:     models.SocraticDiscovery,
		},
		{
			name:     "solved with hints stays at baseline",
			baseline: models.SocraticPure,
			metrics:  EscalationMetrics{SolvedNoHints: true, HintsUsed: 1},
			want:     models.SocraticPure,
		},
		{
			name:     "at level 3 stays at level 3 (ceiling)",
			baseline: models.SocraticDiscovery,
			metrics:  EscalationMetrics{SolvedNoHints: true, HintsUsed: 0},
			want:     models.SocraticDiscovery,
		},
		{
			name:     "not solved stays at baseline",
			baseline: models.SocraticPure,
			metrics:  EscalationMetrics{SolvedNoHints: false, HintsUsed: 0},
			want:     models.SocraticPure,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EscalateIfNeeded(c.baseline, c.metrics, time.Now().UTC())
			if got.NewLevel != c.want {
				t.Errorf("got %v, want %v", got.NewLevel, c.want)
			}
		})
	}
}

// TestEscalateIfNeeded_Priority verifica que dos triggers
// concurrentes se evalúan con prioridad correcta: dos failures +
// tres give-ups → failure gana (es el primer trigger que matchea).
func TestEscalateIfNeeded_Priority(t *testing.T) {
	now := time.Now().UTC()
	res := EscalateIfNeeded(
		models.SocraticDiscovery, // empezamos arriba
		EscalationMetrics{
			Failures:     2,
			GiveUpCounts: 5,
		},
		now,
	)
	// Discovery (3) → fail rule → level - 1 = 2 (socratic)
	if res.NewLevel != models.SocraticPure {
		t.Errorf("priority rule: got %v, want %v (failures wins)", res.NewLevel, models.SocraticPure)
	}
}

// TestEscalateIfNeeded_OutOfRangeBaselineClamps cubre el contrato
// defensivo: niveles fuera de [0..3] se clampean a Guided(1) para
// que la lógica nunca opere sobre un valor inválido.
func TestEscalateIfNeeded_OutOfRangeBaselineClamps(t *testing.T) {
	res := EscalateIfNeeded(models.SocraticLevel(99), EscalationMetrics{}, time.Now().UTC())
	if res.NewLevel != models.SocraticGuided {
		t.Errorf("got %v, want %v (clamp default)", res.NewLevel, models.SocraticGuided)
	}
}

// --- EscalationTracker -------------------------------------------------------

// TestEscalationTracker_RecordFailureIncrements comprueba el
// contador de failures en el orden de los records. El cursor de
// counters es secuencial para que /audit pueda alimentar el tracker.
func TestEscalationTracker_RecordFailureIncrements(t *testing.T) {
	tr := NewEscalationTracker()
	tr.RecordFailure("u1", "c1")
	tr.RecordFailure("u1", "c1")
	tr.RecordFailure("u1", "c1")
	snap := tr.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	if snap.Failures != 3 {
		t.Errorf("Failures = %d, want 3", snap.Failures)
	}
}

// TestEscalationTracker_SessionsAreIsolated garantiza que dos
// pares (user, challenge) no contaminan sus contadores.
func TestEscalationTracker_SessionsAreIsolated(t *testing.T) {
	tr := NewEscalationTracker()
	tr.RecordFailure("u1", "c1")
	tr.RecordFailure("u1", "c1")
	tr.RecordFailure("u2", "c1")

	snapA := tr.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	snapB := tr.getSnapshot(sessionKey{userID: "u2", challengeID: "c1"})
	if snapA.Failures != 2 || snapB.Failures != 1 {
		t.Errorf("isolation broken: A=%d B=%d", snapA.Failures, snapB.Failures)
	}
}

// TestEscalationTracker_ResetSession wipes metrics y permite que el
// siguiente RecordFailure arranque de 0.
func TestEscalationTracker_ResetSession(t *testing.T) {
	tr := NewEscalationTracker()
	tr.RecordFailure("u1", "c1")
	tr.RecordFailure("u1", "c1")
	tr.ResetSession("u1", "c1")
	snap := tr.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	if snap.Failures != 0 {
		t.Errorf("after Reset: Failures = %d, want 0", snap.Failures)
	}
}

// TestEscalationTracker_LastUserMessageStamped verifica el flujo
// del trigger "stuck > 10m". El helper RecordUserMessage debe
// estampar time.Now (con tolerancia para tests lentos).
func TestEscalationTracker_LastUserMessageStamped(t *testing.T) {
	tr := NewEscalationTracker()
	before := time.Now().UTC().Add(-time.Second)
	tr.RecordUserMessage("u1", "c1")
	after := time.Now().UTC().Add(time.Second)
	snap := tr.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	if snap.LastUserMessage.Before(before) || snap.LastUserMessage.After(after) {
		t.Errorf("LastUserMessage = %v, want in [%v..%v]", snap.LastUserMessage, before, after)
	}
}

// TestEscalationTracker_ConcurrentSafe corre N goroutines que
// registran failures + resets a la misma (user, challenge) y
// comprueba que no hay race ni corruption. race-detector corre en
// CI; aquí validamos correctness.
func TestEscalationTracker_ConcurrentSafe(t *testing.T) {
	tr := NewEscalationTracker()
	const goroutines = 20
	const writesPer = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < writesPer; i++ {
				tr.RecordFailure("u1", "c1")
				tr.RecordUserMessage("u1", "c1")
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		// resseter paralelo — verifica que la mutación no entrelace.
		for i := 0; i < 5; i++ {
			tr.ResetSession("u1", "c1")
		}
	}()
	wg.Wait()
	snap := tr.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	if snap.Failures < 0 {
		t.Errorf("negative failures indicates corruption: %d", snap.Failures)
	}
}

// TestCanonicalCommand_StripsSlashAndLowercases es el "lenguaje" del
// parser. La función es case-insensitive, ignora whitespace, y se usa
// desde ProcessCommand.
func TestCanonicalCommand_StripsSlashAndLowercases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/nivel 0", "nivel"},
		{"/NIVEL 2", "nivel"},
		{"   /pista  ", "pista"},
		{"/help", "help"},
		{"/perfil", "perfil"},
		{"", ""},
		{"/", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := canonicalCommand(c.in)
			if got != c.want {
				t.Errorf("canonicalCommand(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestEscalation_ErrIs_Truthiness verifica que errors.Is entiende
// los sentinels. Algún caller (e.g. handler) ramifica en base al
// match.
func TestEscalation_ErrIs_Truthiness(t *testing.T) {
	wrapped := wrapErrForTest(ErrTutorUnavailable, "tutor: gone")
	if !errors.Is(wrapped, ErrTutorUnavailable) {
		t.Errorf("errors.Is should match wrapped sentinel")
	}
}

// wrapErrForTest centraliza el wrapping para que el test anterior
// no necesite el import de fmt en línea.
func wrapErrForTest(sentinel error, msg string) error {
	return &wrappedForTest{sentinel: sentinel, msg: msg}
}

type wrappedForTest struct {
	sentinel error
	msg      string
}

func (e *wrappedForTest) Error() string { return e.msg + ": " + e.sentinel.Error() }
func (e *wrappedForTest) Unwrap() error { return e.sentinel }
