package ports

import "context"

// AuthValidator defines the contract for validating authentication tokens.
// It is a driven (secondary) port — the application use cases call it.
type AuthValidator interface {
	// ValidateToken checks whether the provided token is valid.
	// Returns nil if valid; returns an error if invalid or expired.
	ValidateToken(ctx context.Context, token string) error

	// UserIDFromToken extracts the user ID from a valid token.
	// The caller must ensure the token has already been validated.
	UserIDFromToken(token string) (string, error)
}