package services

import (
	"context"
	"database/sql"
	"errors"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// ErrChallengeNotFound is returned when a challenge lookup fails.
var ErrChallengeNotFound = errors.New("challenge not found")

// ChallengeService retrieves challenges from PostgreSQL.
type ChallengeService struct {
	db *sql.DB
}

// NewChallengeService creates a new ChallengeService.
func NewChallengeService(db *sql.DB) *ChallengeService {
	return &ChallengeService{db: db}
}

// GetAll returns all challenges with status='available', ordered by created_at DESC.
func (s *ChallengeService) GetAll(ctx context.Context) ([]models.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at
		 FROM public.challenges
		 WHERE status = 'available'
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var c models.Challenge
		if err := rows.Scan(&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Category, &c.Language, &c.RepoURL, &c.Code, &c.CodeSmell, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, rows.Err()
}

// GetByID returns a single challenge by ID, or ErrChallengeNotFound.
func (s *ChallengeService) GetByID(ctx context.Context, id string) (models.Challenge, error) {
	var c models.Challenge
	err := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available'`,
		id,
	).Scan(&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Category, &c.Language, &c.RepoURL, &c.Code, &c.CodeSmell, &c.Status, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ErrChallengeNotFound
		}
		return models.Challenge{}, err
	}
	return c, nil
}