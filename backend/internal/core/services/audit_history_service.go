package services

import (
	"context"
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// AuditHistoryService saves and retrieves audit session history
// through the AuditHistoryRepository port. The service owns
// business logic (title derivation, snippet truncation); the port
// owns the SQL.
type AuditHistoryService struct {
	repo ports.AuditHistoryRepository
}

// NewAuditHistoryService creates a new AuditHistoryService backed by
// the given repository. The repository MUST be non-nil; nil-safe
// wiring is the wiring layer's job (e.g. cmd/api/wiring_test.go
// passes nil to verify registry count without invoking any tool).
func NewAuditHistoryService(repo ports.AuditHistoryRepository) *AuditHistoryService {
	return &AuditHistoryService{repo: repo}
}

// defaultHistoryLimit is the default row cap when GetHistory is
// called with a non-positive limit. Matches the historical
// "limit <= 0 ⇒ 20" behavior.
const defaultHistoryLimit = 20

// maxSnippetLen caps the stored code_snippet column at 2000 chars
// to keep the row size bounded. Mirrors the historical truncation in
// the *sql.DB version.
const maxSnippetLen = 2000

// SaveSession persists an audit session after completion.
func (s *AuditHistoryService) SaveSession(ctx context.Context, userID string, req models.AuditRequest, findingsCount int) error {
	title := req.ChallengeID
	if title == "" {
		title = "Custom Audit"
	}

	// Truncate code snippet for storage.
	snippet := req.Code
	if len(snippet) > maxSnippetLen {
		snippet = snippet[:maxSnippetLen]
	}

	return s.repo.Save(ctx, models.AuditSession{
		UserID:         userID,
		ChallengeTitle: title,
		Language:       req.Language,
		CodeSnippet:    snippet,
		FindingsCount:  findingsCount,
	})
}

// GetHistory returns the last N audit sessions for a user.
func (s *AuditHistoryService) GetHistory(ctx context.Context, userID string, limit int) ([]models.AuditSession, error) {
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	out, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit sessions: %w", err)
	}
	return out, nil
}

// GetStats returns aggregated stats for a user.
func (s *AuditHistoryService) GetStats(ctx context.Context, userID string) (totalAudits int, totalFindings int, err error) {
	return s.repo.GetStats(ctx, userID)
}
