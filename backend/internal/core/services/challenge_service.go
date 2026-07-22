package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
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

// ErrInvalidExpectedFindings is returned when expected_findings count is outside 1-3.
var ErrInvalidExpectedFindings = errors.New("expected_findings must have 1-3 items")

// ErrInvalidTestCases is returned when test_cases count is outside 2-4.
var ErrInvalidTestCases = errors.New("test_cases must have 2-4 items")

// ErrInvalidHintsCount is returned when hints count is not exactly 3.
var ErrInvalidHintsCount = errors.New("exactly 3 hints required")

// ErrSpoilerInDescription is returned when the description contains spoiler keywords.
var ErrSpoilerInDescription = errors.New("description contains spoiler content")

// validDifficulties contains allowed difficulty values.
var validDifficulties = map[string]bool{
	"junior":    true,
	"mid":       true,
	"senior":    true,
	"architect": true,
}

// spoilerKeywords is a deny-list of smell/vulnerability names that MUST NOT
// appear in a challenge description. The check is case-insensitive.
var spoilerKeywords = []string{
	"sql injection", "xss", "cross-site scripting", "god function",
	"callback hell", "prop mutation", "dead code", "silent failure",
	"poor naming", "n+1", "race condition", "memory leak",
}

// validateChallengeV2 checks v2 model constraints on a CreateChallengeInput.
// It MUST be called before persisting a new challenge.
func validateChallengeV2(input models.CreateChallengeInput) error {
	// expected_findings: 1-3 items
	if len(input.ExpectedFindings) < 1 || len(input.ExpectedFindings) > 3 {
		return ErrInvalidExpectedFindings
	}
	// test_cases: 2-4 items
	if len(input.TestCases) < 2 || len(input.TestCases) > 4 {
		return ErrInvalidTestCases
	}
	// hints: exactly 3 items
	if len(input.Hints) != 3 {
		return ErrInvalidHintsCount
	}
	// description: no spoiler keywords (case-insensitive)
	lowerDesc := strings.ToLower(input.Description)
	for _, kw := range spoilerKeywords {
		if strings.Contains(lowerDesc, kw) {
			return ErrSpoilerInDescription
		}
	}
	return nil
}

// IsValidationError returns true if the error is one of the known challenge
// validation errors. Used by the HTTP handler to decide 400 vs 500.
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidDifficulty) ||
		errors.Is(err, ErrInvalidExpectedFindings) ||
		errors.Is(err, ErrInvalidTestCases) ||
		errors.Is(err, ErrInvalidHintsCount) ||
		errors.Is(err, ErrSpoilerInDescription)
}

// challengeSelectColumns lists the v1 + v2 columns (excluding solution fields).
const challengeSelectColumns = `id, title, description, difficulty, category, language,
	repo_url, code, code_smell, status, created_at, user_id, source_repo,
	learning_objectives, hints, common_mistakes, estimated_time_minutes,
	expected_findings, test_cases, linter_rules,
	base_points, bonus_points, penalty_per_hint, time_bonus,
	origin, source_path, generated_by, created_by`

// challengeSelectColumnsWithSolution adds solution_code and solution_explanation.
const challengeSelectColumnsWithSolution = challengeSelectColumns + `,
	solution_code, solution_explanation`

// ChallengeSelectColumns exposes the column list for test matching from other packages.
func ChallengeSelectColumns() string {
	return challengeSelectColumns
}

// ChallengeSelectColumnsWithSolution exposes the column list (with solution) for test matching.
func ChallengeSelectColumnsWithSolution() string {
	return challengeSelectColumnsWithSolution
}

// rowScanner is implemented by *sql.Rows and *sql.Row.
type rowScanner interface {
	Scan(dest ...any) error
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

// unmarshalJSONB parses a JSONB column scanned as []byte into the target.
// Nil or empty bytes leave the target at its zero value.
func unmarshalJSONB(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

// marshalJSONB converts a Go value to JSON bytes suitable for JSONB insertion.
// Nil slices/interfaces are serialized as "[]" to match DB defaults.
func marshalJSONB(v any) []byte {
	if v == nil {
		return []byte("[]")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	// json.Marshal produces "null" for nil slices — convert to "[]" for JSONB
	if string(data) == "null" {
		return []byte("[]")
	}
	return data
}

// scanChallenge reads all columns from a sql row into a Challenge struct.
// When includeSolution is true, solution_code and solution_explanation are
// also scanned (they must be the last two columns in the SELECT).
func scanChallenge(row rowScanner, includeSolution bool) (models.Challenge, error) {
	var c models.Challenge

	var repoURLNull, codeSmellNull, userIDNull, sourceRepoNull sql.NullString
	var originNull, sourcePathNull, generatedByNull, createdByNull sql.NullString
	var estimatedTimeMinutes, basePoints, bonusPoints, penaltyPerHint sql.NullInt64
	var timeBonus sql.NullBool

	var learningObjJSON, hintsJSON, commonMistakesJSON []byte
	var expectedFindingsJSON, testCasesJSON, linterRulesJSON []byte

	dest := []any{
		&c.ID, &c.Title, &c.Description, &c.Difficulty, &c.Category, &c.Language,
		&repoURLNull, &c.Code, &codeSmellNull, &c.Status, &c.CreatedAt,
		&userIDNull, &sourceRepoNull,
		&learningObjJSON, &hintsJSON, &commonMistakesJSON, &estimatedTimeMinutes,
		&expectedFindingsJSON, &testCasesJSON, &linterRulesJSON,
		&basePoints, &bonusPoints, &penaltyPerHint, &timeBonus,
		&originNull, &sourcePathNull, &generatedByNull, &createdByNull,
	}

	var solutionCodeNull, solutionExplanationNull sql.NullString
	if includeSolution {
		dest = append(dest, &solutionCodeNull, &solutionExplanationNull)
	}

	if err := row.Scan(dest...); err != nil {
		return c, err
	}

	// Nullable v1 fields
	if repoURLNull.Valid {
		c.RepoURL = repoURLNull.String
	}
	if codeSmellNull.Valid {
		c.CodeSmell = codeSmellNull.String
	}
	if userIDNull.Valid {
		uid := userIDNull.String
		c.UserID = &uid
	}
	if sourceRepoNull.Valid {
		c.SourceRepo = sourceRepoNull.String
	}

	// Nullable v2 scalar fields
	if estimatedTimeMinutes.Valid {
		c.EstimatedTimeMinutes = int(estimatedTimeMinutes.Int64)
	}
	if basePoints.Valid {
		c.BasePoints = int(basePoints.Int64)
	}
	if bonusPoints.Valid {
		c.BonusPoints = int(bonusPoints.Int64)
	}
	if penaltyPerHint.Valid {
		c.PenaltyPerHint = int(penaltyPerHint.Int64)
	}
	if timeBonus.Valid {
		c.TimeBonus = timeBonus.Bool
	}

	// Nullable v2 text fields
	if originNull.Valid {
		c.Origin = originNull.String
	}
	if sourcePathNull.Valid {
		c.SourcePath = sourcePathNull.String
	}
	if generatedByNull.Valid {
		c.GeneratedBy = generatedByNull.String
	}
	if createdByNull.Valid {
		c.CreatedBy = createdByNull.String
	}

	// Parse JSONB arrays
	if err := unmarshalJSONB(learningObjJSON, &c.LearningObjectives); err != nil {
		return c, fmt.Errorf("parsing learning_objectives: %w", err)
	}
	if err := unmarshalJSONB(hintsJSON, &c.Hints); err != nil {
		return c, fmt.Errorf("parsing hints: %w", err)
	}
	if err := unmarshalJSONB(commonMistakesJSON, &c.CommonMistakes); err != nil {
		return c, fmt.Errorf("parsing common_mistakes: %w", err)
	}
	if err := unmarshalJSONB(expectedFindingsJSON, &c.ExpectedFindings); err != nil {
		return c, fmt.Errorf("parsing expected_findings: %w", err)
	}
	if err := unmarshalJSONB(testCasesJSON, &c.TestCases); err != nil {
		return c, fmt.Errorf("parsing test_cases: %w", err)
	}
	if err := unmarshalJSONB(linterRulesJSON, &c.LinterRules); err != nil {
		return c, fmt.Errorf("parsing linter_rules: %w", err)
	}

	// Solution fields (only when explicitly requested)
	if includeSolution {
		if solutionCodeNull.Valid {
			c.SolutionCode = solutionCodeNull.String
		}
		if solutionExplanationNull.Valid {
			c.SolutionExplanation = solutionExplanationNull.String
		}
	}

	return c, nil
}

// Create inserts a new challenge or returns the existing one if source_repo+user_id already exists.
// Returns (challenge, created, nil) where created is true for new inserts and false for dedup hits.
func (s *ChallengeService) Create(ctx context.Context, input models.CreateChallengeInput, userID string) (*models.Challenge, bool, error) {
	// Validate difficulty
	if !validDifficulties[input.Difficulty] {
		return nil, false, ErrInvalidDifficulty
	}

	// Validate v2 model constraints (expected_findings, test_cases, hints, no spoilers)
	if err := validateChallengeV2(input); err != nil {
		return nil, false, err
	}

	// Normalize source_repo for dedup
	normalizedRepo := strings.ToLower(strings.TrimSpace(input.SourceRepo))

	// Check for duplicate — selects v2 columns so the returned challenge is complete
	dedupRow := s.db.QueryRowContext(ctx,
		`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`,
		normalizedRepo, userID,
	)

	c, scanErr := scanChallenge(dedupRow, false)
	if scanErr == nil {
		// Found existing challenge (dedup hit)
		return &c, false, nil
	}
	if !errors.Is(scanErr, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("checking for duplicate challenge: %w", scanErr)
	}

	// No duplicate — insert new challenge with v2 columns
	id, err := generateUUID()
	if err != nil {
		return nil, false, fmt.Errorf("generating UUID: %w", err)
	}

	status := "available"
	var createdAt time.Time

	err = s.db.QueryRowContext(ctx,
		`INSERT INTO public.challenges (
			id, title, description, difficulty, category, language, repo_url, source_repo, code, code_smell, status, user_id,
			learning_objectives, hints, common_mistakes, estimated_time_minutes,
			expected_findings, test_cases, linter_rules,
			solution_code, solution_explanation,
			base_points, bonus_points, penalty_per_hint, time_bonus,
			origin, source_path, generated_by, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		RETURNING created_at`,
		id, input.Title, input.Description, input.Difficulty, input.Category, input.Language,
		input.RepoURL, normalizedRepo, input.Code, input.CodeSmell, status, userID,
		marshalJSONB(input.LearningObjectives), marshalJSONB(input.Hints), marshalJSONB(input.CommonMistakes),
		input.EstimatedTimeMinutes, marshalJSONB(input.ExpectedFindings),
		marshalJSONB(input.TestCases), marshalJSONB(input.LinterRules),
		input.SolutionCode, input.SolutionExplanation,
		input.BasePoints, input.BonusPoints, input.PenaltyPerHint, input.TimeBonus,
		input.Origin, input.SourcePath, input.GeneratedBy, input.CreatedBy,
	).Scan(&createdAt)

	if err != nil {
		return nil, false, fmt.Errorf("inserting challenge: %w", err)
	}

	return &models.Challenge{
		ID:                   id,
		Title:                input.Title,
		Description:           input.Description,
		Difficulty:            input.Difficulty,
		Category:              input.Category,
		Language:              input.Language,
		RepoURL:               input.RepoURL,
		Code:                  input.Code,
		CodeSmell:             input.CodeSmell,
		Status:                status,
		CreatedAt:             createdAt,
		UserID:                &userID,
		SourceRepo:            normalizedRepo,
		LearningObjectives:    input.LearningObjectives,
		Hints:                 input.Hints,
		CommonMistakes:        input.CommonMistakes,
		EstimatedTimeMinutes:  input.EstimatedTimeMinutes,
		ExpectedFindings:      input.ExpectedFindings,
		TestCases:             input.TestCases,
		LinterRules:           input.LinterRules,
		SolutionCode:          input.SolutionCode,
		SolutionExplanation:   input.SolutionExplanation,
		BasePoints:            input.BasePoints,
		BonusPoints:           input.BonusPoints,
		PenaltyPerHint:        input.PenaltyPerHint,
		TimeBonus:             input.TimeBonus,
		Origin:                input.Origin,
		SourcePath:            input.SourcePath,
		GeneratedBy:           input.GeneratedBy,
		CreatedBy:             input.CreatedBy,
	}, true, nil
}

// GetAll returns all challenges visible to the given user: seeds (NULL user_id) plus user-owned.
// Solution fields (solution_code, solution_explanation) are NOT included in the response.
func (s *ChallengeService) GetAll(ctx context.Context, userID string) ([]models.Challenge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+challengeSelectColumns+`
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
		c, err := scanChallenge(rows, false)
		if err != nil {
			return nil, err
		}
		challenges = append(challenges, c)
	}
	return challenges, rows.Err()
}

// GetByID returns a single challenge by ID visible to the given user, or ErrChallengeNotFound.
// Solution fields are NOT included — use GetChallengeWithDetails for the full challenge.
func (s *ChallengeService) GetByID(ctx context.Context, id string, userID string) (models.Challenge, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+challengeSelectColumns+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`,
		id, userID,
	)
	c, err := scanChallenge(row, false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ErrChallengeNotFound
		}
		return models.Challenge{}, err
	}
	return c, nil
}

// GetChallengeWithDetails returns a challenge including solution_code and
// solution_explanation. Use this only when the caller is authorized to see
// the solution (tutor context or reveal flag).
func (s *ChallengeService) GetChallengeWithDetails(ctx context.Context, id string, userID string) (models.Challenge, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+challengeSelectColumnsWithSolution+`
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`,
		id, userID,
	)
	c, err := scanChallenge(row, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ErrChallengeNotFound
		}
		return models.Challenge{}, err
	}
	return c, nil
}