package ports

import (
	"context"
	"errors"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// FileEntry is the lightweight descriptor for one file in a source repo.
// Returned by SourceClient.ListFiles and persisted (offline) by the
// analyzer before per-file fetches begin. SHA is the git blob SHA so a
// rename does not invalidate the analysis cache (spec REQ-CODE-HEALTH-009).
type FileEntry struct {
	Path string
	Size int64
	SHA  string
}

// FileContent is the full text of a source file returned by
// SourceClient.GetFileContent. Content is already base64-decoded by the
// adapter (GitHub returns base64; Gogs as well, mirrored here). Size is
// the raw byte count the adapter received from the host.
type FileContent struct {
	Path    string
	Content string
	SHA     string
	Size    int64
}

// SourceClient abstracts a git host's tree + blob API. Both GogsClient
// and GitHubClient satisfy it; the code-health service selects by source
// string. The interface is intentionally narrow — every method maps 1:1
// to a host endpoint so adapters can keep their translation logic
// isolated (spec REQ-CODE-HEALTH-001 / REQ-CODE-HEALTH-002).
type SourceClient interface {
	// ListFiles returns the repo tree, filtered to type=blob (files).
	// Sub-directories (type=tree) are dropped by the adapter. The
	// service applies the 300-file / 1 MB caps on top of the
	// adapter's filter pass.
	ListFiles(ctx context.Context, owner, repo, branch string) ([]FileEntry, error)
	// GetFileContent returns the file's text (base64-decoded by the
	// adapter). The service applies the 1 MB cap before calling.
	GetFileContent(ctx context.Context, owner, repo, branch, path string) (*FileContent, error)
}

// ErrCodeHealthAnalysisNotFound is returned by CodeHealthRepository
// methods when the supplied analysis_id does not exist (or belongs to
// a different user, indistinguishable from "missing" to prevent cross-
// tenant information leaks).
var ErrCodeHealthAnalysisNotFound = errors.New("code-health analysis not found")

// CodeHealthRepository is the driven (secondary) port that persists
// `code_health_analyses` and `code_health_issues` rows. Every read
// scopes by userID at the SQL layer; every write stamps userID from
// the authenticated context (never the caller). Implementations live
// in infrastructure/driven (Supabase).
//
// Surface mirrors the canonical code-health spec (REQ-CODE-HEALTH-003
// async lifecycle, REQ-CODE-HEALTH-012 history, REQ-CODE-HEALTH-011
// daily cap, REQ-CODE-HEALTH-013 auth-on-everything). Suppression
// feedback lives behind a separate pair of methods (task 1.19).
type CodeHealthRepository interface {
	// InsertAnalysis persists a fresh analysis row. The adapter
	// generates the ID via the SQL `gen_random_uuid()` default.
	InsertAnalysis(ctx context.Context, a *models.CodeHealthAnalysis) error

	// UpdateStatus flips the row's status enum. errMsg is non-empty
	// when status = failed; the adapter writes the column as NULL
	// otherwise. userID is mandatory — the WHERE clause scopes to
	// the authenticated tenant.
	UpdateStatus(ctx context.Context, id, userID, status, errMsg string) error

	// Complete marks an analysis as completed in a single round-trip
	// and writes the final payload (health_score, top_issues,
	// languages, summary, completed_at). It MUST be called from the
	// analysis goroutine after a successful analyze pass.
	Complete(ctx context.Context, id, userID string, score int,
		topIssues []models.CodeHealthIssue, languages map[string]float64, summary string) error

	// FindByID returns the full analysis row scoped to userID. Returns
	// ErrCodeHealthAnalysisNotFound when no row matches.
	FindByID(ctx context.Context, id, userID string) (models.CodeHealthAnalysis, error)

	// ListHistory returns the most-recent N analyses for
	// (owner, name, source) scoped to userID, ordered by created_at
	// DESC. The service caps `limit` at 10 per spec.
	ListHistory(ctx context.Context, owner, name, source, userID string, limit int) ([]models.CodeHealthAnalysis, error)

	// CountSince returns the number of analyses the user submitted
	// at-or-after `since`. The service uses this for the daily cap
	// (>= 5 since UTC midnight → reject).
	CountSince(ctx context.Context, userID string, since time.Time) (int, error)

	// InsertIssue persists one issue row. The service calls this in
	// bulk after the per-file analysis loop; the adapter batches in a
	// single transaction.
	InsertIssue(ctx context.Context, i *models.CodeHealthIssue) error

	// ListIssues returns issues for analysisID, optionally filtered by
	// severity and capped at limit. Sorted by severity weight DESC
	// (matches the public ranking in REQ-CODE-HEALTH-005).
	ListIssues(ctx context.Context, analysisID, severity string, limit int) ([]models.CodeHealthIssue, error)

	// InsertSuppression records a user-marked "not an issue" feedback
	// (spec REQ-CODE-HEALTH-008 "not an issue" scenario, task 1.19).
	// The composite primary key (user_id, file_path, rule_id,
	// line_number) makes the call idempotent — a duplicate insert is
	// a no-op.
	InsertSuppression(ctx context.Context, userID, filePath, ruleID string, lineNumber int, reason *string) error

	// IsSuppressed returns true when the (userID, filePath, ruleID,
	// lineNumber) tuple is present in the user's suppression list.
	// The analyzer calls this for every rule hit before emitting an
	// issue.
	IsSuppressed(ctx context.Context, userID, filePath, ruleID string, lineNumber int) (bool, error)
}

// AnalysisCache is the per-file cache, keyed by `analysis:<file_sha>`,
// namespaced by userID. The adapter is DB-backed (Supabase
// `analysis_cache` table) with a 7-day TTL. A cache hit skips the
// per-file analysis loop entirely (regex + LLM).
type AnalysisCache interface {
	// Get returns the cached issues and `true` when an unexpired
	// entry exists for (userID, fileSHA); otherwise `false, nil`.
	// Expired rows are treated as misses.
	Get(ctx context.Context, userID, fileSHA string) ([]models.CodeHealthIssue, bool, error)
	// Set writes the issues for (userID, fileSHA) with the supplied
	// TTL. The adapter is responsible for setting `expires_at = now()
	// + ttl` in the DB so the read filter (`expires_at > NOW()`) does
	// the eviction.
	Set(ctx context.Context, userID, fileSHA string, issues []models.CodeHealthIssue, ttl time.Duration) error
}

// OllamaGenerator is the narrow non-streaming JSON-mode contract for
// the <=50 KB file-level LLM lane. The audit-path OllamaClient port is
// streaming-only, so the code-health service declares its own narrow
// port instead of bloating that contract. The driven
// `infrastructure/driven/ollama.Client` already satisfies it
// (non-streaming Generate + JSON mode are the same call).
//
// The contract is intentionally minimal: take a system + prompt, return
// raw text. The caller is responsible for parsing the JSON envelope and
// mapping it to CodeHealthIssue (with confidence) — keeping JSON
// parsing outside the port lets the service unit-test it independently
// from the Ollama HTTP call.
type OllamaGenerator interface {
	Generate(ctx context.Context, system, prompt string) (string, error)
}
