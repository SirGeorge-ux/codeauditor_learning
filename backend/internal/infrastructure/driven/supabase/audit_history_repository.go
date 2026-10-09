package supabase

// SupabaseAuditHistoryRepository is the Supabase/PostgreSQL adapter for
// the AuditHistoryRepository port. It owns all SQL for the
// public.audit_sessions table the vault HTTP handler reads and the
// mcp-user tools (get_history, record_audit_attempt, get_stats) mutate.
//
// Connection sharing: same db handle as the other Supabase adapters;
// main.go supplies it once at startup.

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time guard: SupabaseAuditHistoryRepository implements
// ports.AuditHistoryRepository.
var _ ports.AuditHistoryRepository = (*SupabaseAuditHistoryRepository)(nil)

// SupabaseAuditHistoryRepository is the Supabase adapter for the
// AuditHistoryRepository port.
type SupabaseAuditHistoryRepository struct {
	db *sql.DB
}

// NewSupabaseAuditHistoryRepository constructs the adapter backed by
// the given database/sql connection.
func NewSupabaseAuditHistoryRepository(db *sql.DB) *SupabaseAuditHistoryRepository {
	return &SupabaseAuditHistoryRepository{db: db}
}

// Save persists a new audit session row. The caller (service) is
// responsible for deriving the title (challenge_id or "Custom Audit"),
// truncating the code snippet, and computing all non-DB-default
// fields; the adapter just executes the INSERT.
func (r *SupabaseAuditHistoryRepository) Save(ctx context.Context, session models.AuditSession) error {
	const query = `INSERT INTO public.audit_sessions (user_id, challenge_title, language, code_snippet, findings_count)
		 VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.ExecContext(
		ctx, query,
		session.UserID, session.ChallengeTitle, session.Language, session.CodeSnippet, session.FindingsCount,
	)
	if err != nil {
		return fmt.Errorf("insert audit session: %w", err)
	}
	return nil
}

// ListByUser returns up to `limit` most-recent audit sessions for the
// supplied userID, ordered by created_at DESC.
func (r *SupabaseAuditHistoryRepository) ListByUser(ctx context.Context, userID string, limit int) ([]models.AuditSession, error) {
	const query = `SELECT id, user_id, challenge_title, language, code_snippet, findings_count, created_at
		 FROM public.audit_sessions
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit sessions: %w", err)
	}
	defer rows.Close()

	var out []models.AuditSession
	for rows.Next() {
		var s models.AuditSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.ChallengeTitle, &s.Language, &s.CodeSnippet, &s.FindingsCount, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit session row: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit session rows: %w", err)
	}
	return out, nil
}

// GetStats returns the aggregated (totalAudits, totalFindings) pair
// for the supplied userID. An empty history returns (0, 0, nil).
func (r *SupabaseAuditHistoryRepository) GetStats(ctx context.Context, userID string) (int, int, error) {
	const query = `SELECT COUNT(*), COALESCE(SUM(findings_count), 0)
		 FROM public.audit_sessions WHERE user_id = $1`

	var totalAudits, totalFindings int
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&totalAudits, &totalFindings); err != nil {
		return 0, 0, fmt.Errorf("aggregate audit stats: %w", err)
	}
	return totalAudits, totalFindings, nil
}
