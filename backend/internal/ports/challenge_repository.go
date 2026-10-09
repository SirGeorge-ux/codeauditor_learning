package ports

import (
	"context"
	"errors"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrChallengeNotFound is returned by ChallengeRepository operations when
// the requested challenge is not visible to the supplied user (either the
// id does not exist, the row is not 'available', or user_id does not match
// the requester and the row is not a public seed). Service / driving
// layers translate this into a 404 Not Found without leaking internal
// SQL details.
var ErrChallengeNotFound = errors.New("challenge not found")

// ChallengeRepository is the driven (secondary) port that persists the
// public.challenges rows the curated-mode Dojo reads and the mcp-challenges
// tools mutate. Implementations live in infrastructure/driven (e.g.
// Supabase). The service layer is free to assume "if it came back, the
// caller is authorized" because every adapter implementation is expected
// to scope reads with the supplied userID at the SQL layer (mirroring
// the RLS contract the dictionary repository already uses).
//
// The port surface is deliberately minimal — exactly the queries the
// current ChallengeService issues. New adapter-side capabilities
// (analytics, search, tags) land in their own port method with their own
// scenario tests; this file does not grow a god interface.
type ChallengeRepository interface {
	// FindBySourceRepoAndUser is the dedup probe used by Create. Returns
	// ErrChallengeNotFound when no row matches the (lowercased
	// source_repo, user_id, status='available') tuple. The returned
	// Challenge carries every v1 + v2 column except the solution
	// fields, matching the historical dedup response shape.
	FindBySourceRepoAndUser(ctx context.Context, sourceRepo, userID string) (models.Challenge, error)

	// Insert persists a new challenge. The caller pre-populates the
	// Challenge model (id, all v1 + v2 fields, normalized source_repo)
	// and the adapter performs the INSERT...RETURNING created_at. The
	// returned time.Time is the database-assigned created_at; the
	// caller echoes it back in the API response.
	Insert(ctx context.Context, c *models.Challenge) (createdAt time.Time, err error)

	// ListVisibleToUser returns every 'available' challenge visible to
	// the supplied userID (public seeds where user_id IS NULL plus the
	// caller's own rows), ordered by created_at DESC. Solution fields
	// are NOT included. The empty-result case is not an error.
	ListVisibleToUser(ctx context.Context, userID string) ([]models.Challenge, error)

	// FindByIDVisibleToUser returns a single 'available' challenge by
	// id, scoped to the supplied userID. Solution fields are NOT
	// included. Returns ErrChallengeNotFound on miss.
	FindByIDVisibleToUser(ctx context.Context, id, userID string) (models.Challenge, error)

	// FindByIDWithSolutionVisibleToUser is the same lookup as
	// FindByIDVisibleToUser but also populates SolutionCode and
	// SolutionExplanation. Use this only when the caller is
	// authorized to see the solution (tutor context or reveal flag).
	// Returns ErrChallengeNotFound on miss.
	FindByIDWithSolutionVisibleToUser(ctx context.Context, id, userID string) (models.Challenge, error)
}
