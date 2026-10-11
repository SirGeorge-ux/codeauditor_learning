package supabase

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

// Compile-time guard: SupabaseAnalysisCache implements
// ports.AnalysisCache.
var _ ports.AnalysisCache = (*SupabaseAnalysisCache)(nil)

// SupabaseAnalysisCache is the Supabase/PostgreSQL adapter for
// the AnalysisCache port. It persists per-file analysis results
// in the analysis_cache table (key: user_id + file_sha) with a
// 7-day TTL enforced on read via the WHERE expires_at > NOW()
// filter (spec REQ-CODE-HEALTH-009). The Re-analyze CTA
// (force=true) calls Set with the same key; the adapter
// overwrites the row (ON CONFLICT) so the cache is never
// duplicated.
//
// The 7-day TTL default is set by the DB column's DEFAULT
// (NOW() + INTERVAL '7 days'); callers can override by passing a
// custom ttl to Set.
type SupabaseAnalysisCache struct {
	db *sql.DB
}

// NewSupabaseAnalysisCache constructs the adapter.
func NewSupabaseAnalysisCache(db *sql.DB) *SupabaseAnalysisCache {
	return &SupabaseAnalysisCache{db: db}
}

// Get returns the cached issues for (userID, fileSHA) when an
// unexpired row exists. sql.ErrNoRows (no row) and any row past
// expires_at are both treated as misses.
func (c *SupabaseAnalysisCache) Get(ctx context.Context, userID, fileSHA string) ([]models.CodeHealthIssue, bool, error) {
	const query = `SELECT result FROM public.analysis_cache
		WHERE user_id = $1 AND file_sha = $2 AND expires_at > NOW()
		LIMIT 1`
	var raw []byte
	err := c.db.QueryRowContext(ctx, query, userID, fileSHA).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("analysis cache get: %w", err)
	}
	var issues []models.CodeHealthIssue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, false, fmt.Errorf("analysis cache decode: %w", err)
	}
	return issues, true, nil
}

// Set writes the issues for (userID, fileSHA) with the supplied
// TTL. The row is overwritten when the key already exists
// (ON CONFLICT DO UPDATE) so the Re-analyze path stays a
// single round-trip. The TTL is applied per-call (no DB default);
// the analyzer always passes 7 * 24 * time.Hour per the spec.
func (c *SupabaseAnalysisCache) Set(ctx context.Context, userID, fileSHA string, issues []models.CodeHealthIssue, ttl time.Duration) error {
	if issues == nil {
		issues = []models.CodeHealthIssue{}
	}
	raw, err := json.Marshal(issues)
	if err != nil {
		return fmt.Errorf("analysis cache encode: %w", err)
	}
	expires := time.Now().Add(ttl)

	const query = `INSERT INTO public.analysis_cache
		(user_id, file_sha, result, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, file_sha) DO UPDATE
		SET result = EXCLUDED.result,
		    expires_at = EXCLUDED.expires_at`

	_, err = c.db.ExecContext(ctx, query, userID, fileSHA, raw, expires)
	if err != nil {
		return fmt.Errorf("analysis cache set: %w", err)
	}
	return nil
}
