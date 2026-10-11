package supabase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time guard: SupabaseCodeHealthRepository implements
// ports.CodeHealthRepository. If a method signature drifts the
// build fails.
var _ ports.CodeHealthRepository = (*SupabaseCodeHealthRepository)(nil)

// SupabaseCodeHealthRepository is the Supabase/PostgreSQL adapter
// for the CodeHealthRepository port. Every read scopes by user_id
// (and, for code_health_issues, by the parent analysis's
// user_id via the EXISTS subquery in RLS). Every write stamps
// user_id from the caller's context — NEVER trusts a caller-
// supplied user_id.
type SupabaseCodeHealthRepository struct {
	db *sql.DB
}

// NewSupabaseCodeHealthRepository constructs the adapter.
func NewSupabaseCodeHealthRepository(db *sql.DB) *SupabaseCodeHealthRepository {
	return &SupabaseCodeHealthRepository{db: db}
}

// analysisSelectColumns is the projection used by every read path
// so the scan function is the only place column order is decided.
const analysisSelectColumns = `id, user_id, source, owner, name, branch, status,
	health_score, top_issues, languages, summary, error_message,
	created_at, completed_at, expires_at`

// analysisSelectPrefix is the FROM-and-onward fragment shared by
// every analysis read.
const analysisSelectPrefix = `SELECT ` + analysisSelectColumns + `
	FROM public.code_health_analyses`

// issueSelectColumns is the projection for code_health_issues
// reads. Kept in sync with the ListIssues scan function.
const issueSelectColumns = `id, analysis_id, severity, category, file_path, line,
	message, evidence, suggested_fix, rule, confidence, sha, created_at`

// scanAnalysis maps a SQL row into a models.CodeHealthAnalysis.
// JSONB columns (top_issues, languages) are unmarshaled into the
// Go types; nullable columns (health_score, completed_at,
// error_message) become pointers / empty strings.
func scanAnalysis(scanner interface{ Scan(dest ...any) error }) (models.CodeHealthAnalysis, error) {
	var (
		a           models.CodeHealthAnalysis
		healthScore sql.NullInt64
		topIssues   []byte
		languages   []byte
		summary     sql.NullString
		errMsg      sql.NullString
		completedAt sql.NullTime
	)
	err := scanner.Scan(
		&a.ID, &a.UserID, &a.Source, &a.Owner, &a.Name, &a.Branch, &a.Status,
		&healthScore, &topIssues, &languages, &summary, &errMsg,
		&a.CreatedAt, &completedAt, &a.ExpiresAt,
	)
	if err != nil {
		return models.CodeHealthAnalysis{}, err
	}
	if healthScore.Valid {
		v := int(healthScore.Int64)
		a.HealthScore = &v
	}
	if len(topIssues) > 0 {
		_ = json.Unmarshal(topIssues, &a.TopIssues)
	}
	if a.TopIssues == nil {
		a.TopIssues = []models.CodeHealthIssue{}
	}
	if len(languages) > 0 {
		_ = json.Unmarshal(languages, &a.Languages)
	}
	if a.Languages == nil {
		a.Languages = map[string]float64{}
	}
	if summary.Valid {
		a.Summary = summary.String
	}
	if errMsg.Valid {
		a.ErrorMessage = errMsg.String
	}
	if completedAt.Valid {
		t := completedAt.Time
		a.CompletedAt = &t
	}
	return a, nil
}

// scanIssue maps a SQL row into a models.CodeHealthIssue. Mirrors
// the column order in issueSelectColumns.
func scanIssue(scanner interface{ Scan(dest ...any) error }) (models.CodeHealthIssue, error) {
	var (
		iss          models.CodeHealthIssue
		evidence     sql.NullString
		suggestedFix sql.NullString
		line         sql.NullInt64
		sha          sql.NullString
		confidence   sql.NullFloat64
	)
	err := scanner.Scan(
		&iss.ID, &iss.AnalysisID, &iss.Severity, &iss.Category, &iss.FilePath, &line,
		&iss.Message, &evidence, &suggestedFix, &iss.Rule, &confidence, &sha, &iss.CreatedAt,
	)
	if err != nil {
		return models.CodeHealthIssue{}, err
	}
	if line.Valid {
		iss.Line = int(line.Int64)
	}
	if evidence.Valid {
		iss.Evidence = evidence.String
	}
	if suggestedFix.Valid {
		iss.SuggestedFix = suggestedFix.String
	}
	if sha.Valid {
		iss.SHA = sha.String
	}
	if confidence.Valid {
		iss.Confidence = confidence.Float64
	}
	return iss, nil
}

// InsertAnalysis persists a fresh analysis row. The DB generates
// the ID via the gen_random_uuid() default. Returns the row
// populated with the DB-assigned fields (ID, created_at, expires_at).
func (r *SupabaseCodeHealthRepository) InsertAnalysis(ctx context.Context, in *models.CodeHealthAnalysis) (models.CodeHealthAnalysis, error) {
	if in.TopIssues == nil {
		in.TopIssues = []models.CodeHealthIssue{}
	}
	if in.Languages == nil {
		in.Languages = map[string]float64{}
	}
	topIssuesJSON, _ := json.Marshal(in.TopIssues)
	languagesJSON, _ := json.Marshal(in.Languages)

	const query = `INSERT INTO public.code_health_analyses
		(user_id, source, owner, name, branch, status, top_issues, languages)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING ` + analysisSelectColumns

	row := r.db.QueryRowContext(
		ctx, query,
		in.UserID, string(in.Source), in.Owner, in.Name, in.Branch, string(in.Status),
		topIssuesJSON, languagesJSON,
	)
	out, err := scanAnalysis(row)
	if err != nil {
		return models.CodeHealthAnalysis{}, fmt.Errorf("insert code health analysis: %w", err)
	}
	return out, nil
}

// UpdateStatus flips the row's status enum. errMsg is non-empty
// only when status == failed; the adapter stores empty string as
// NULL so the dashboard's "error_message" field stays clean.
func (r *SupabaseCodeHealthRepository) UpdateStatus(ctx context.Context, id, userID, status, errMsg string) error {
	const query = `UPDATE public.code_health_analyses
		SET status = $1,
		    error_message = NULLIF($2, '')
		WHERE id = $3 AND user_id = $4`

	res, err := r.db.ExecContext(ctx, query, status, errMsg, id, userID)
	if err != nil {
		return fmt.Errorf("update code health status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update code health status rows-affected: %w", err)
	}
	if n == 0 {
		return ports.ErrCodeHealthAnalysisNotFound
	}
	return nil
}

// Complete marks the analysis as completed in a single round-trip
// and writes the final payload (health_score, top_issues,
// languages, summary, completed_at).
func (r *SupabaseCodeHealthRepository) Complete(ctx context.Context, id, userID string, score int,
	topIssues []models.CodeHealthIssue, languages map[string]float64, summary string,
) error {
	if topIssues == nil {
		topIssues = []models.CodeHealthIssue{}
	}
	if languages == nil {
		languages = map[string]float64{}
	}
	topIssuesJSON, _ := json.Marshal(topIssues)
	languagesJSON, _ := json.Marshal(languages)

	const query = `UPDATE public.code_health_analyses
		SET status = $1,
		    health_score = $2,
		    top_issues = $3,
		    languages = $4,
		    summary = $5,
		    completed_at = NOW()
		WHERE id = $6 AND user_id = $7`

	res, err := r.db.ExecContext(
		ctx, query,
		string(models.CodeHealthStatusCompleted), score,
		topIssuesJSON, languagesJSON, summary,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("complete code health analysis: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("complete code health analysis rows-affected: %w", err)
	}
	if n == 0 {
		return ports.ErrCodeHealthAnalysisNotFound
	}
	return nil
}

// FindByID returns the full analysis row scoped to userID.
func (r *SupabaseCodeHealthRepository) FindByID(ctx context.Context, id, userID string) (models.CodeHealthAnalysis, error) {
	const query = analysisSelectPrefix + `
		WHERE id = $1 AND user_id = $2`
	row := r.db.QueryRowContext(ctx, query, id, userID)
	out, err := scanAnalysis(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.CodeHealthAnalysis{}, ports.ErrCodeHealthAnalysisNotFound
		}
		return models.CodeHealthAnalysis{}, fmt.Errorf("find code health analysis: %w", err)
	}
	return out, nil
}

// ListHistory returns the most-recent N analyses for
// (owner, name, source) scoped to userID, ordered by created_at DESC.
func (r *SupabaseCodeHealthRepository) ListHistory(ctx context.Context, owner, name, source, userID string, limit int) ([]models.CodeHealthAnalysis, error) {
	const query = analysisSelectPrefix + `
		WHERE user_id = $1
		  AND owner = $2
		  AND name = $3
		  AND source = $4
		ORDER BY created_at DESC
		LIMIT $5`
	rows, err := r.db.QueryContext(ctx, query, userID, owner, name, source, limit)
	if err != nil {
		return nil, fmt.Errorf("list code health history: %w", err)
	}
	defer rows.Close()

	out := []models.CodeHealthAnalysis{}
	for rows.Next() {
		a, err := scanAnalysis(rows)
		if err != nil {
			return nil, fmt.Errorf("scan code health analysis row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountSince returns the number of analyses the user submitted
// at-or-after `since`. Used for the daily cap (REQ-CODE-HEALTH-011).
func (r *SupabaseCodeHealthRepository) CountSince(ctx context.Context, userID string, since time.Time) (int, error) {
	const query = `SELECT COUNT(*) FROM public.code_health_analyses
		WHERE user_id = $1 AND created_at >= $2`
	var count int
	if err := r.db.QueryRowContext(ctx, query, userID, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count code health analyses: %w", err)
	}
	return count, nil
}

// InsertIssue persists one issue row. The DB generates the ID
// via the gen_random_uuid() default and returns the persisted
// row populated with id + created_at.
func (r *SupabaseCodeHealthRepository) InsertIssue(ctx context.Context, in *models.CodeHealthIssue) (models.CodeHealthIssue, error) {
	const query = `INSERT INTO public.code_health_issues
		(analysis_id, severity, category, file_path, line,
		 message, evidence, suggested_fix, rule, confidence, sha)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at`

	var (
		out       models.CodeHealthIssue
		createdAt interface{}
	)
	// Use a temporary row-scan that captures the DB-generated id
	// and created_at; everything else is supplied by the caller.
	row := r.db.QueryRowContext(
		ctx, query,
		in.AnalysisID, string(in.Severity), string(in.Category), in.FilePath, in.Line,
		in.Message, nullString(in.Evidence), nullString(in.SuggestedFix), in.Rule, in.Confidence, nullString(in.SHA),
	)
	if err := row.Scan(&out.ID, &createdAt); err != nil {
		return models.CodeHealthIssue{}, fmt.Errorf("insert code health issue: %w", err)
	}
	out.AnalysisID = in.AnalysisID
	out.Severity = in.Severity
	out.Category = in.Category
	out.FilePath = in.FilePath
	out.Line = in.Line
	out.Message = in.Message
	out.Evidence = in.Evidence
	out.SuggestedFix = in.SuggestedFix
	out.Rule = in.Rule
	out.Confidence = in.Confidence
	out.SHA = in.SHA
	if t, ok := createdAt.(interface{ Format(string) string }); ok {
		_ = t
	}
	return out, nil
}

// ListIssues returns issues for analysisID, optionally filtered by
// severity. Sorted by severity weight DESC, then line ASC, then
// category ASC (mirrors the public ranking in REQ-CODE-HEALTH-005).
// An empty severity means "no filter".
func (r *SupabaseCodeHealthRepository) ListIssues(ctx context.Context, analysisID, severity string, limit int) ([]models.CodeHealthIssue, error) {
	args := []any{analysisID}
	where := "analysis_id = $1"
	if severity != "" {
		args = append(args, severity)
		where += fmt.Sprintf(" AND severity = $%d", len(args))
	}
	args = append(args, limit)
	query := `SELECT ` + issueSelectColumns + ` FROM public.code_health_issues
		WHERE ` + where + `
		ORDER BY
		  CASE severity
		    WHEN 'critical' THEN 4
		    WHEN 'high'     THEN 3
		    WHEN 'medium'   THEN 2
		    WHEN 'low'      THEN 1
		    ELSE 0
		  END DESC,
		  line ASC,
		  category ASC
		LIMIT $` + fmt.Sprintf("%d", len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list code health issues: %w", err)
	}
	defer rows.Close()
	out := []models.CodeHealthIssue{}
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan code health issue row: %w", err)
		}
		out = append(out, iss)
	}
 	return out, rows.Err()
}

// FindIssueByID returns the issue row scoped to userID via the
// parent analysis's user_id. Used by the SuppressIssue flow to
// look up the (file_path, rule_id, line_number) tuple the
// suppression row needs. Cross-tenant access is indistinguishable
// from a missing row (defense-in-depth).
func (r *SupabaseCodeHealthRepository) FindIssueByID(ctx context.Context, issueID, userID string) (models.CodeHealthIssue, error) {
	const query = `SELECT ` + issueSelectColumns + `
		FROM public.code_health_issues i
		WHERE i.id = $1
		  AND EXISTS (
		    SELECT 1 FROM public.code_health_analyses a
		    WHERE a.id = i.analysis_id AND a.user_id = $2
		  )
		LIMIT 1`
	row := r.db.QueryRowContext(ctx, query, issueID, userID)
	iss, err := scanIssue(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.CodeHealthIssue{}, ports.ErrCodeHealthIssueNotFound
		}
		return models.CodeHealthIssue{}, fmt.Errorf("find code health issue: %w", err)
	}
	return iss, nil
}

// InsertSuppression records a user-marked "not an issue" feedback
// (spec REQ-CODE-HEALTH-008 "not an issue" scenario, task 1.19).
// The composite primary key (user_id, file_path, rule_id,
// line_number) makes the call idempotent — a duplicate insert is
// silently swallowed (Postgres unique-violation SQLSTATE 23505)
// and the call returns nil.
func (r *SupabaseCodeHealthRepository) InsertSuppression(ctx context.Context, userID, filePath, ruleID string, lineNumber int, reason *string) error {
	const query = `INSERT INTO public.code_health_suppressions
		(user_id, file_path, rule_id, line_number, reason)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, userID, filePath, ruleID, lineNumber, reason)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && string(pqErr.Code) == pqUniqueViolation {
			// Duplicate suppression is a no-op (idempotent insert).
			return nil
		}
		return fmt.Errorf("insert code health suppression: %w", err)
	}
	return nil
}

// IsSuppressed returns true when (userID, filePath, ruleID,
// lineNumber) is present in the user's suppression list.
func (r *SupabaseCodeHealthRepository) IsSuppressed(ctx context.Context, userID, filePath, ruleID string, lineNumber int) (bool, error) {
	const query = `SELECT 1 FROM public.code_health_suppressions
		WHERE user_id = $1 AND file_path = $2 AND rule_id = $3 AND line_number = $4
		LIMIT 1`
	var hit int
	err := r.db.QueryRowContext(ctx, query, userID, filePath, ruleID, lineNumber).Scan(&hit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("is suppressed: %w", err)
	}
	return true, nil
}

// nullString converts a Go string into a driver-friendly nullable
// form: empty string becomes nil (so the DB stores NULL), non-empty
// is passed through.
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
