package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrChallengeNotFound is returned when a challenge lookup fails.
var ErrChallengeNotFound = errors.New("challenge not found")

// ErrInvalidDifficulty is returned when a challenge has an invalid difficulty level.
var ErrInvalidDifficulty = errors.New("invalid difficulty: must be one of junior, mid, senior, architect")

// validDifficulties contains allowed difficulty values.
var validDifficulties = map[string]bool{
	"junior":    true,
	"mid":       true,
	"senior":    true,
	"architect": true,
}

// ChallengeService retrieves and creates challenges from PostgreSQL.
type ChallengeService struct {
	db *sql.DB
}

// NewChallengeService creates a new ChallengeService.
func NewChallengeService(db *sql.DB) *ChallengeService {
	return &ChallengeService{db: db}
}

// generateUUID generates a version 4 UUID using crypto/rand.
func generateUUID() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// Create inserts a new challenge or returns the existing one if source_repo+user_id already exists.
// Returns (challenge, created, nil) where created is true for new inserts and false for dedup hits.
func (s *ChallengeService) Create(ctx context.Context, input models.CreateChallengeInput, userID string) (*models.Challenge, bool, error) {
	// Validate difficulty
	if !validDifficulties[input.Difficulty] {
		return nil, false, ErrInvalidDifficulty
	}

	// Normalize source_repo for dedup
	normalizedRepo := strings.ToLower(strings.TrimSpace(input.SourceRepo))

	// Check for duplicate
	var existing models.Challenge
	var existingUserIDNull sql.NullString
	var existingSourceRepoNull sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`,
		normalizedRepo, userID,
	).Scan(&existing.ID, &existing.Title, &existing.Description, &existing.Difficulty, &existing.Category, &existing.Language, &existing.RepoURL, &existing.Code, &existing.CodeSmell, &existing.Status, &existing.CreatedAt, &existingUserIDNull, &existingSourceRepoNull)

	if err == nil {
		// Found existing challenge (dedup hit)
		if existingUserIDNull.Valid {
			existing.UserID = &existingUserIDNull.String
		}
		if existingSourceRepoNull.Valid {
			existing.SourceRepo = existingSourceRepoNull.String
		}
		return &existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("checking for duplicate challenge: %w", err)
	}

	// No duplicate — insert new challenge
	id, err := generateUUID()
	if err != nil {
		return nil, false, fmt.Errorf("generating UUID: %w", err)
	}

	status := "available"
	var createdAt time.Time

	err = s.db.QueryRowContext(ctx,
		`INSERT INTO public.challenges (id, title, description, difficulty, category, language, repo_url, source_repo, code, code_smell, status, user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING created_at`,
		id, input.Title, input.Description, input.Difficulty, input.Category, input.Language, input.RepoURL, normalizedRepo, input.Code, input.CodeSmell, status, userID,
	).Scan(&createdAt)

	if err != nil {
		return nil, false, fmt.Errorf("inserting challenge: %w", err)
	}

	return &models.Challenge{
		ID:          id,
		Title:       input.Title,
		Description: input.Description,
		Difficulty:  input.Difficulty,
		Category:    input.Category,
		Language:    input.Language,
		RepoURL:     input.RepoURL,
		Code:        input.Code,
		CodeSmell:   input.CodeSmell,
		Status:      status,
		CreatedAt:   createdAt,
		UserID:      &userID,
		SourceRepo:  normalizedRepo,
	}, true, nil
}

// GetAll returns all challenges visible to the given user: seeds (NULL user_id) plus user-owned.
func (s *ChallengeService) GetAll(ctx context.Context, userID string) ([]models.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var c models.Challenge
		var userIDNull sql.NullString
		var sourceRepoNull sql.NullString
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Category, &c.Language, &c.RepoURL, &c.Code, &c.CodeSmell, &c.Status, &c.CreatedAt, &userIDNull, &sourceRepoNull); err != nil {
			return nil, err
		}
		if userIDNull.Valid {
			c.UserID = &userIDNull.String
		}
		if sourceRepoNull.Valid {
			c.SourceRepo = sourceRepoNull.String
		}
		challenges = append(challenges, c)
	}
	return challenges, rows.Err()
}

// GetByID returns a single challenge by ID visible to the given user, or ErrChallengeNotFound.
func (s *ChallengeService) GetByID(ctx context.Context, id string, userID string) (models.Challenge, error) {
	var c models.Challenge
	var userIDNull sql.NullString
	var sourceRepoNull sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at, user_id, source_repo
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`,
		id, userID,
	).Scan(&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Category, &c.Language, &c.RepoURL, &c.Code, &c.CodeSmell, &c.Status, &c.CreatedAt, &userIDNull, &sourceRepoNull)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ErrChallengeNotFound
		}
		return models.Challenge{}, err
	}
	if userIDNull.Valid {
		c.UserID = &userIDNull.String
	}
	if sourceRepoNull.Valid {
		c.SourceRepo = sourceRepoNull.String
	}
	return c, nil
}