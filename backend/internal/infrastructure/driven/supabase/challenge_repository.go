package supabase

// SupabaseChallengeRepository is the Supabase/PostgreSQL adapter for the
// ChallengeRepository port. It owns all SQL and JSON marshaling concerns
// for the public.challenges table the curated-mode Dojo reads and the
// mcp-challenges tools mutate. The service layer is free to assume "if
// it came back, it belongs to me" because every read here scopes the
// query with the supplied userID at the SQL layer, mirroring the
// existing DictionaryRepository contract.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time guard: SupabaseChallengeRepository implements
// ports.ChallengeRepository.
var _ ports.ChallengeRepository = (*SupabaseChallengeRepository)(nil)

// SupabaseChallengeRepository is the Supabase adapter for the
// ChallengeRepository port.
type SupabaseChallengeRepository struct {
	db *sql.DB
}

// NewSupabaseChallengeRepository constructs the adapter backed by the
// given database/sql connection.
func NewSupabaseChallengeRepository(db *sql.DB) *SupabaseChallengeRepository {
	return &SupabaseChallengeRepository{db: db}
}

// challengeSelectColumns lists every v1 + v2 column the SELECT paths
// project. Mirrors the historical challengeSelectColumns constant that
// used to live in core/services/challenge_service.go.
const challengeSelectColumns = `id, title, description, difficulty, category, language,
	repo_url, code, code_smell, status, created_at, user_id, source_repo,
	learning_objectives, hints, common_mistakes, estimated_time_minutes,
	expected_findings, test_cases, linter_rules,
	base_points, bonus_points, penalty_per_hint, time_bonus,
	origin, source_path, generated_by, created_by`

// challengeSelectColumnsWithSolution adds solution_code and solution_explanation.
const challengeSelectColumnsWithSolution = challengeSelectColumns + `,
	solution_code, solution_explanation`

// ChallengeSelectColumns exposes the column list for test matching
// from other packages (handler tests use it to build sqlmock
// expectations). Mirrors the historical public function that used
// to live in core/services/challenge_service.go.
func ChallengeSelectColumns() string {
	return challengeSelectColumns
}

// ChallengeSelectColumnsWithSolution exposes the column list (with
// solution) for test matching from other packages.
func ChallengeSelectColumnsWithSolution() string {
	return challengeSelectColumnsWithSolution
}

// rowScanner is implemented by *sql.Rows and *sql.Row. The historical
// helper lived in core/services; it now lives in the adapter because
// only the adapter knows the column list and the NULL handling.
type rowScanner interface {
	Scan(dest ...any) error
}

// unmarshalJSONB parses a JSONB column scanned as []byte into the
// target. Nil or empty bytes leave the target at its zero value.
func unmarshalJSONB(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

// marshalJSONB converts a Go value to JSON bytes suitable for JSONB
// insertion. Nil slices/interfaces are serialized as "[]" to match DB
// defaults. Mirrors the historical helper in core/services.
func marshalJSONB(v any) []byte {
	if v == nil {
		return []byte("[]")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("[]")
	}
	// json.Marshal produces "null" for nil slices — convert to "[]"
	// for JSONB.
	if string(data) == "null" {
		return []byte("[]")
	}
	return data
}

// scanChallenge reads every column from a sql row into a Challenge
// struct. When includeSolution is true, solution_code and
// solution_explanation are also scanned (they must be the last two
// columns in the SELECT). Mirrors the historical scanChallenge helper
// in core/services; the column projection is now an adapter-private
// contract.
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

// FindBySourceRepoAndUser is the dedup probe used by Create. Returns
// ports.ErrChallengeNotFound when no row matches the (lowercased
// source_repo, user_id, status='available') tuple.
func (r *SupabaseChallengeRepository) FindBySourceRepoAndUser(ctx context.Context, sourceRepo, userID string) (models.Challenge, error) {
	const query = `SELECT ` + challengeSelectColumns + `
		 FROM public.challenges
		 WHERE LOWER(source_repo) = $1 AND user_id = $2 AND status = 'available'`

	row := r.db.QueryRowContext(ctx, query, sourceRepo, userID)
	c, err := scanChallenge(row, false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ports.ErrChallengeNotFound
		}
		return models.Challenge{}, fmt.Errorf("find challenge by source: %w", err)
	}
	return c, nil
}

// Insert persists a new challenge. The caller pre-populates the
// Challenge model (id, all v1 + v2 fields, normalized source_repo).
func (r *SupabaseChallengeRepository) Insert(ctx context.Context, c *models.Challenge) (time.Time, error) {
	const query = `INSERT INTO public.challenges (
			id, title, description, difficulty, category, language, repo_url, source_repo, code, code_smell, status, user_id,
			learning_objectives, hints, common_mistakes, estimated_time_minutes,
			expected_findings, test_cases, linter_rules,
			solution_code, solution_explanation,
			base_points, bonus_points, penalty_per_hint, time_bonus,
			origin, source_path, generated_by, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		RETURNING created_at`

	var createdAt time.Time
	err := r.db.QueryRowContext(
		ctx, query,
		c.ID, c.Title, c.Description, c.Difficulty, c.Category, c.Language,
		c.RepoURL, c.SourceRepo, c.Code, c.CodeSmell, c.Status, derefString(c.UserID),
		marshalJSONB(c.LearningObjectives), marshalJSONB(c.Hints), marshalJSONB(c.CommonMistakes),
		c.EstimatedTimeMinutes, marshalJSONB(c.ExpectedFindings),
		marshalJSONB(c.TestCases), marshalJSONB(c.LinterRules),
		c.SolutionCode, c.SolutionExplanation,
		c.BasePoints, c.BonusPoints, c.PenaltyPerHint, c.TimeBonus,
		c.Origin, c.SourcePath, c.GeneratedBy, c.CreatedBy,
	).Scan(&createdAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("insert challenge: %w", err)
	}
	return createdAt, nil
}

// ListVisibleToUser returns every 'available' challenge visible to the
// supplied userID (public seeds where user_id IS NULL plus the
// caller's own rows), ordered by created_at DESC.
func (r *SupabaseChallengeRepository) ListVisibleToUser(ctx context.Context, userID string) ([]models.Challenge, error) {
	const query = `SELECT ` + challengeSelectColumns + `
		 FROM public.challenges
		 WHERE status = 'available' AND (user_id IS NULL OR user_id = $1)
		 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list challenges: %w", err)
	}
	defer rows.Close()

	var out []models.Challenge
	for rows.Next() {
		c, err := scanChallenge(rows, false)
		if err != nil {
			return nil, fmt.Errorf("scan challenge row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate challenge rows: %w", err)
	}
	return out, nil
}

// FindByIDVisibleToUser returns a single 'available' challenge by id,
// scoped to the supplied userID. Solution fields are NOT included.
func (r *SupabaseChallengeRepository) FindByIDVisibleToUser(ctx context.Context, id, userID string) (models.Challenge, error) {
	const query = `SELECT ` + challengeSelectColumns + `
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`

	row := r.db.QueryRowContext(ctx, query, id, userID)
	c, err := scanChallenge(row, false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ports.ErrChallengeNotFound
		}
		return models.Challenge{}, fmt.Errorf("get challenge by id: %w", err)
	}
	return c, nil
}

// FindByIDWithSolutionVisibleToUser is the same lookup as
// FindByIDVisibleToUser but also populates SolutionCode and
// SolutionExplanation.
func (r *SupabaseChallengeRepository) FindByIDWithSolutionVisibleToUser(ctx context.Context, id, userID string) (models.Challenge, error) {
	const query = `SELECT ` + challengeSelectColumnsWithSolution + `
		 FROM public.challenges
		 WHERE id = $1 AND status = 'available' AND (user_id IS NULL OR user_id = $2)`

	row := r.db.QueryRowContext(ctx, query, id, userID)
	c, err := scanChallenge(row, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Challenge{}, ports.ErrChallengeNotFound
		}
		return models.Challenge{}, fmt.Errorf("get challenge with details: %w", err)
	}
	return c, nil
}

// derefString returns the underlying string for a *string pointer, or
// the empty string if the pointer is nil. Used when binding
// Challenge.UserID (nullable) to a TEXT column.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
