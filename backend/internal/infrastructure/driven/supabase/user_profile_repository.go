package supabase

// SupabaseUserProfileRepository is the Supabase/PostgreSQL adapter for the
// UserProfileRepository port. It owns all SQL for the `public.usuarios`
// row the mcp-user.get_profile tool reads.
//
// Connection sharing: same db handle as the other Supabase adapters; main.go
// supplies it once at startup.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time guard.
var _ ports.UserProfileRepository = (*SupabaseUserProfileRepository)(nil)

// SupabaseUserProfileRepository is the Supabase adapter for the
// UserProfileRepository port.
type SupabaseUserProfileRepository struct {
	db *sql.DB
}

// NewSupabaseUserProfileRepository constructs the adapter backed by the
// given database/sql connection.
func NewSupabaseUserProfileRepository(db *sql.DB) *SupabaseUserProfileRepository {
	return &SupabaseUserProfileRepository{db: db}
}

// GetProfile returns the public.usuarios row for userID. Returns
// ports.ErrUserProfileNotFound when the row does not exist (DB no
// rows), mirroring the same upward-sentinel contract
// SupabaseDictionaryRepository uses.
func (r *SupabaseUserProfileRepository) GetProfile(ctx context.Context, userID string) (*models.UserProfile, error) {
	const query = `SELECT id, email, COALESCE(display_name, ''), racha_dias, puntos_maestria,
		rango_actual, ultimo_intento_valido, created_at, updated_at
		FROM public.usuarios
		WHERE id = $1`

	var profile models.UserProfile
	var ultimoIntento sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&profile.ID,
		&profile.Email,
		&profile.DisplayName,
		&profile.RachaDias,
		&profile.PuntosMaestria,
		&profile.RangoActual,
		&ultimoIntento,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ports.ErrUserProfileNotFound
		}
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	if ultimoIntento.Valid {
		t := ultimoIntento.Time
		profile.UltimoIntento = &t
	}
	return &profile, nil
}
