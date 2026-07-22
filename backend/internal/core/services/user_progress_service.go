package services

import (
	"context"
	"sort"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// MaxCompletedChallengeIDs is the FIFO cap for the anti-gaming dedup array.
const MaxCompletedChallengeIDs = 500

// rangoOrdinals maps a free-practice F-S range to its ordinal position.
var rangoOrdinals = map[string]int{
	"F": 0, "E": 1, "D": 2, "C": 3, "B": 4, "A": 5, "S": 6,
}

// ordinalRangos is the reverse lookup (index == ordinal).
var ordinalRangos = []string{"F", "E", "D", "C", "B", "A", "S"}

// UserProgressService handles per-language mastery tracking, range
// calculation (F-S / Junior-Architect), anti-gaming dedup, tasa_exito, and
// global rank (median of per-language F-S ranges).
//
// Hexagonal contract: this type imports ONLY core/domain/models, ports, and
// the standard library. No SQL, no HTTP, no infrastructure SDKs.
type UserProgressService struct {
	repo ports.UserProgressRepository
}

// NewUserProgressService constructs a UserProgressService backed by the given
// repository port.
func NewUserProgressService(repo ports.UserProgressRepository) *UserProgressService {
	return &UserProgressService{repo: repo}
}

// RecordAuditCompletion updates the LanguageProgress for (userID, language)
// after an audit session attempts a challenge.
//
// The completed flag distinguishes a successful audit from a failed one, mirroring
// the spec's AuditSession.status field:
//
//   - completed == true  (status="completed"): full scoring path applies.
//     Anti-gaming dedup: if challengeID is already in CompletedChallengeIDs,
//     score is NOT added and ChallengesCompletados is NOT incremented — only
//     ChallengesIntentados increments. First-time completions append challengeID
//     to the dedup array (FIFO, capped at MaxCompletedChallengeIDs entries) and
//     append learningObjectives (deduplicated) to TopicsDominados.
//   - completed == false (status="failed"): only challenges_intentados increments
//     and ultimo_completado is stamped. puntos, challenges_completados, topics_dominados,
//     and the dedup array are left untouched. A failed attempt does NOT poison
//     dedup, so a subsequent success on the same challengeID still earns full credit.
//
// Always: challenges_intentados += 1, tasa_exito is recomputed, ultimo_completado = NOW(),
// and rango/rango_code_health are recomputed (no-ops when puntos is unchanged).
func (s *UserProgressService) RecordAuditCompletion(
	ctx context.Context,
	userID, language, challengeID string,
	score int,
	learningObjectives []string,
	completed bool,
) error {
	progress, err := s.repo.GetLanguageProgress(ctx, userID, language)
	if err != nil {
		return err
	}

	// Normalize nil slices so the adapter can marshal them as JSONB [].
	if progress.TopicsDominados == nil {
		progress.TopicsDominados = []string{}
	}
	if progress.CompletedChallengeIDs == nil {
		progress.CompletedChallengeIDs = []string{}
	}

	// Every audit — successful or failed — counts as an attempt.
	progress.ChallengesIntentados++

	if completed {
		// Anti-gaming dedup: only the first completion of a challengeID contributes
		// puntos and bumps ChallengesCompletados.
		if !containsString(progress.CompletedChallengeIDs, challengeID) {
			progress.Puntos += score
			progress.ChallengesCompletados++
			progress.CompletedChallengeIDs = appendCappedFIFO(
				progress.CompletedChallengeIDs, challengeID, MaxCompletedChallengeIDs,
			)

			// Record learning objectives as dominados (deduplicated).
			for _, topic := range learningObjectives {
				if !containsString(progress.TopicsDominados, topic) {
					progress.TopicsDominados = append(progress.TopicsDominados, topic)
				}
			}
		}
	}
	// Failed audits (completed == false) intentionally bypass every credit path
	// above: no puntos, no ChallengesCompletados, no dedup entry, no topics.

	// tasa_exito with division-by-zero guard. Recomputed on every attempt since
	// challenges_intentados always changes.
	progress.TasaExito = tasaExito(progress.ChallengesCompletados, progress.ChallengesIntentados)

	// Stamp attempt time.
	now := time.Now().UTC()
	progress.UltimoCompletado = &now

	// Recalculate ranges from updated puntos (idempotent when puntos is unchanged).
	progress.Rango = models.RangoFromPuntosFree(progress.Puntos)
	progress.RangoCodeHealth = models.RangoFromPuntosCodeHealth(progress.PuntosCodeHealth)

	return s.repo.UpdateLanguageProgress(ctx, progress)
}

// GetGlobalRank returns the global range for a user, computed as the median of
// all per-language F-S ranges (mapped to ordinals, median taken, mapped back).
// Returns "Junior" when the user has zero language_progress rows.
func (s *UserProgressService) GetGlobalRank(ctx context.Context, userID string) (string, error) {
	progresses, err := s.repo.GetAllLanguageProgress(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(progresses) == 0 {
		return "Junior", nil
	}

	ordinals := make([]int, 0, len(progresses))
	for _, p := range progresses {
		ordinals = append(ordinals, rangoOrdinal(p.Rango))
	}
	return ordinalRango(medianOrdinal(ordinals)), nil
}

// GetAllLanguageProgress returns all per-language progress rows for the given
// user. Delegates to the repository port.
func (s *UserProgressService) GetAllLanguageProgress(ctx context.Context, userID string) ([]*models.LanguageProgress, error) {
	return s.repo.GetAllLanguageProgress(ctx, userID)
}

// GetLanguageProgress returns the progress row for (userID, language), upserting
// a default row if the pair does not exist yet. Delegates to the repository port.
func (s *UserProgressService) GetLanguageProgress(ctx context.Context, userID, language string) (*models.LanguageProgress, error) {
	return s.repo.GetLanguageProgress(ctx, userID, language)
}

// GetLearningProfile returns the learning profile for userID, creating a
// default profile if none exists. Delegates to the repository port.
func (s *UserProgressService) GetLearningProfile(ctx context.Context, userID string) (*models.LearningProfile, error) {
	return s.repo.GetLearningProfile(ctx, userID)
}

// UpdateLearningProfile persists the given learning profile. Delegates to the
// repository port.
func (s *UserProgressService) UpdateLearningProfile(ctx context.Context, profile *models.LearningProfile) error {
	return s.repo.UpdateLearningProfile(ctx, profile)
}

// UpdateLanguageProgress persists the given language progress row (upsert by
// (user_id, language)). Delegates to the repository port.
func (s *UserProgressService) UpdateLanguageProgress(ctx context.Context, progress *models.LanguageProgress) error {
	return s.repo.UpdateLanguageProgress(ctx, progress)
}

// --- pure helpers (no side effects) ---

// containsString reports whether v is present in s.
func containsString(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// appendCappedFIFO appends v to s, evicting the oldest entry when len(s) >= cap
// (FIFO). Returns the (possibly new) slice. Assumes cap > 0.
func appendCappedFIFO(s []string, v string, cap int) []string {
	if len(s) >= cap {
		// Drop the oldest (index 0) and append.
		s = append(s[1:], v)
		return s
	}
	return append(s, v)
}

// tasaExito returns the success rate as a float in [0,1]. Guards division by
// zero by returning 0 when attempted == 0.
func tasaExito(completados, intentados int) float64 {
	if intentados == 0 {
		return 0
	}
	return float64(completados) / float64(intentados)
}

// rangoOrdinal converts a F-S range string to its ordinal position. Returns 0
// (F) for unknown values, treating them as lowest.
func rangoOrdinal(r string) int {
	if o, ok := rangoOrdinals[r]; ok {
		return o
	}
	return 0
}

// ordinalRango converts an ordinal back to its F-S range string. Clamps to
// the valid range.
func ordinalRango(o int) string {
	if o < 0 {
		o = 0
	}
	if o >= len(ordinalRangos) {
		o = len(ordinalRangos) - 1
	}
	return ordinalRangos[o]
}

// medianOrdinal returns the median of a slice of ordinals. For even-length
// slices, the floor of the arithmetic mean is used (matches spec: median 3.5
// rounds down to 3). Returns 0 for empty input.
func medianOrdinal(values []int) int {
	if len(values) == 0 {
		return 0
	}
	cp := make([]int, len(values))
	copy(cp, values)
	sort.Ints(cp)

	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	// Even: average of two middle values floored.
	left := cp[n/2-1]
	right := cp[n/2]
	return (left + right) / 2
}
