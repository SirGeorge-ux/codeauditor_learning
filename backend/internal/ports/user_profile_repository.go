package ports

import (
	"context"
	"errors"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrUserProfileNotFound is returned by UserProfileRepository.GetProfile
// when no row matches the supplied userID. The driving layer translates
// this into the structured "user not found" envelope so the LLM can
// branch on a single line.
var ErrUserProfileNotFound = errors.New("user profile not found")

// UserProfileRepository is the driven (secondary) port that persists the
// authenticated user's profile row in public.usuarios. Implementations live
// in infrastructure/driven (e.g. Supabase).
//
// Contract notes:
//   - GetProfile returns the full UserProfile row (id, email, display_name,
//     racha_dias, puntos_maestria, rango_actual, ultimo_intento_valido,
//     created_at, updated_at).
//   - GetProfile is the only method the mcp-user get_profile tool needs
//     today; the underlying user table also stores password/email-token
//     state that the Auth adapter owns separately. We deliberately do
//     NOT expose UpdateProfile / DeleteProfile here because every spec
//     change to user-record shape lands with its own MCP tool — keeping
//     the port surface small makes the future review unit-coherent.
type UserProfileRepository interface {
	// GetProfile returns the profile row for userID. Returns
	// ErrUserProfileNotFound when no row matches userID.
	GetProfile(ctx context.Context, userID string) (*models.UserProfile, error)
}
