package services

import (
	"context"
	"database/sql"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// AuditSession represents a saved audit session for vault display.
type AuditSession struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ChallengeTitle string    `json:"challenge_title"`
	Language       string    `json:"language"`
	CodeSnippet    string    `json:"code_snippet"`
	FindingsCount  int       `json:"findings_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// AuditHistoryService saves and retrieves audit session history.
type AuditHistoryService struct {
	db *sql.DB
}

// NewAuditHistoryService creates a new AuditHistoryService.
func NewAuditHistoryService(db *sql.DB) *AuditHistoryService {
	return &AuditHistoryService{db: db}
}

// SaveSession persists an audit session after completion.
func (s *AuditHistoryService) SaveSession(ctx context.Context, userID string, req models.AuditRequest, findingsCount int) error {
	title := req.ChallengeID
	if title == "" {
		title = "Custom Audit"
	}

	// Truncate code snippet for storage (first 2000 chars)
	snippet := req.Code
	if len(snippet) > 2000 {
		snippet = snippet[:2000]
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO public.audit_sessions (user_id, challenge_title, language, code_snippet, findings_count)
		 VALUES ($1, $2, $3, $4, $5)`,
		userID, title, req.Language, snippet, findingsCount,
	)
	return err
}

// GetHistory returns the last N audit sessions for a user.
func (s *AuditHistoryService) GetHistory(ctx context.Context, userID string, limit int) ([]AuditSession, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, challenge_title, language, code_snippet, findings_count, created_at
		 FROM public.audit_sessions
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []AuditSession
	for rows.Next() {
		var s AuditSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.ChallengeTitle, &s.Language, &s.CodeSnippet, &s.FindingsCount, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// GetStats returns aggregated stats for a user.
func (s *AuditHistoryService) GetStats(ctx context.Context, userID string) (totalAudits int, totalFindings int, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(findings_count), 0)
		 FROM public.audit_sessions WHERE user_id = $1`, userID,
	).Scan(&totalAudits, &totalFindings)
	return
}
