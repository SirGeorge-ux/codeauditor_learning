package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrUserValidation is returned by UserService when an input fails
// per-field rules (e.g. empty user_id). The driving layer translates this
// to ErrInvalidParams (-32602) so the LLM sees actionable feedback.
var ErrUserValidation = errors.New("user input validation failed")

// UserService owns the read path for the authenticated user's profile
// row. PR 3 ships only GetProfile — the minimal method the mcp-user
// get_profile tool needs. Future spec changes (display name edits,
// notification prefs) will add UpdateProfile here, but they land with
// their own MCP tool review so the surface stays small and reviewable.
//
// Hexagonal contract: this type imports ONLY core/domain/models, ports,
// and the standard library. No SQL, no HTTP, no infrastructure SDKs.
type UserService struct {
	repo ports.UserProfileRepository
}

// NewUserService constructs a UserService backed by the given port.
func NewUserService(repo ports.UserProfileRepository) *UserService {
	return &UserService{repo: repo}
}

// GetProfile returns the profile row for userID. Returns
// ErrUserValidation when userID is empty (defense in depth — the tool
// layer also rejects this, but the service refuses to call the repo
// with garbage input). Returns ports.ErrUserProfileNotFound when no row
// matches.
func (s *UserService) GetProfile(ctx context.Context, userID string) (*models.UserProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrUserValidation)
	}
	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profile, nil
}
