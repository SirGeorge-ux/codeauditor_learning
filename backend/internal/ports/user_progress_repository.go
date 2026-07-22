package ports

import (
	"context"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// UserProgressRepository is the driven (secondary) port that persists
// per-language progress and the learning profile. Implementations live in
// infrastructure/driven (e.g. Supabase).
//
// Contract notes:
//   - GetLanguageProgress MUST upsert a default row (rango="F",
//     rangoCodeHealth="Junior", puntos=0) when the (userID, language) pair
//     does not exist yet, and return the upserted row.
//   - GetLearningProfile MUST upsert a default profile on first access.
type UserProgressRepository interface {
	// GetLanguageProgress returns the progress row for (userID, language),
	// creating a default row if the pair does not exist.
	GetLanguageProgress(ctx context.Context, userID, language string) (*models.LanguageProgress, error)

	// UpdateLanguageProgress persists the given progress row (upsert by
	// (user_id, language)).
	UpdateLanguageProgress(ctx context.Context, progress *models.LanguageProgress) error

	// GetAllLanguageProgress returns all progress rows for the given user.
	GetAllLanguageProgress(ctx context.Context, userID string) ([]*models.LanguageProgress, error)

	// GetLearningProfile returns the learning profile for userID, creating
	// a default profile if none exists.
	GetLearningProfile(ctx context.Context, userID string) (*models.LearningProfile, error)

	// UpdateLearningProfile persists the given learning profile.
	UpdateLearningProfile(ctx context.Context, profile *models.LearningProfile) error
}
