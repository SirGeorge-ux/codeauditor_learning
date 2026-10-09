package ports

import (
	"context"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// AuditHistoryRepository is the driven (secondary) port that persists the
// public.audit_sessions rows the vault HTTP handler reads and the
// mcp-user tools (get_history, record_audit_attempt, get_stats) mutate.
// Implementations live in infrastructure/driven (e.g. Supabase).
//
// The port surface mirrors the three operations the current
// AuditHistoryService exposes — one write, two reads. A future "delete
// audit session" feature lands here as a fourth method, with its own
// scenario test, rather than being smuggled into an existing method.
type AuditHistoryRepository interface {
	// Save persists a new audit session. The caller (service) is
	// responsible for deriving the title (challenge_id or "Custom
	// Audit"), truncating the code snippet, and computing all
	// non-DB-default fields; the adapter just executes the INSERT.
	Save(ctx context.Context, session models.AuditSession) error

	// ListByUser returns up to `limit` most-recent audit sessions for
	// the supplied userID, ordered by created_at DESC. A non-positive
	// limit is the caller's responsibility to normalize before invoking
	// the port (the service applies the default 20).
	ListByUser(ctx context.Context, userID string, limit int) ([]models.AuditSession, error)

	// GetStats returns the aggregated (totalAudits, totalFindings)
	// pair for the supplied userID. The semantics match the historical
	// query: COUNT(*) and COALESCE(SUM(findings_count), 0) over the
	// user's rows. An empty history returns (0, 0, nil) — not an error.
	GetStats(ctx context.Context, userID string) (totalAudits int, totalFindings int, err error)
}
