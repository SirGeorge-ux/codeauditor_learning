package supabase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time check: SupabaseAuthAdapter implements ports.AuthValidator
var _ ports.AuthValidator = (*SupabaseAuthAdapter)(nil)

// SupabaseAuthAdapter validates Supabase JWT tokens locally.
type SupabaseAuthAdapter struct {
	jwtSecret string
}

// NewSupabaseAuthAdapter creates a new SupabaseAuthAdapter.
func NewSupabaseAuthAdapter(jwtSecret string) *SupabaseAuthAdapter {
	return &SupabaseAuthAdapter{jwtSecret: jwtSecret}
}

// ValidateToken parses and validates a JWT token locally using the secret.
// Returns nil if valid, error if invalid or expired.
func (a *SupabaseAuthAdapter) ValidateToken(ctx context.Context, token string) error {
	if token == "" {
		return errors.New("empty token")
	}

	// Strip "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		return errors.New("missing token")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		// Validate signing method - Supabase uses HS256
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(a.jwtSecret), nil
	})

	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	if !parsed.Valid {
		return errors.New("token validation failed")
	}

	return nil
}

// UserIDFromToken extracts the user ID (sub claim) from a validated JWT.
// Returns empty string and error if token is invalid or sub claim is missing.
func (a *SupabaseAuthAdapter) UserIDFromToken(token string) (string, error) {
	// Strip "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	if token == "" {
		return "", errors.New("missing token")
	}

	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(a.jwtSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims type")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing sub claim")
	}

	return sub, nil
}