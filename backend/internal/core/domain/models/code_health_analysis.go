package models

import "time"

// CodeHealthStatus enumerates the lifecycle states of an analysis
// job. Mirrors the `code_health_analyses.status` CHECK constraint
// and the REQ-CODE-HEALTH-003 spec scenarios.
type CodeHealthStatus string

// CodeHealthStatus values. Persisted in lowercase.
const (
	CodeHealthStatusPending   CodeHealthStatus = "pending"
	CodeHealthStatusRunning   CodeHealthStatus = "running"
	CodeHealthStatusCompleted CodeHealthStatus = "completed"
	CodeHealthStatusFailed    CodeHealthStatus = "failed"
)

// CodeHealthSource enumerates the supported git hosts. Persisted as
// TEXT with a CHECK constraint pinning the canonical lowercase form.
type CodeHealthSource string

// CodeHealthSource values.
const (
	SourceGogs   CodeHealthSource = "gogs"
	SourceGitHub CodeHealthSource = "github"
)

// CodeHealthAnalysis is the top-level row the analyzer produces. It
// carries the repo descriptor, the async-job status, the final
// health-score + ranking payload, and bookkeeping timestamps
// (created, completed, expires).
//
// The `TopIssues` slice is the 10-issue slice surfaced on the
// dashboard; the full list is queryable via
// `mcp-code-health.get_repo_issues` (REQ-CODE-HEALTH-012). Languages
// is the per-language byte-share breakdown (e.g. {"go": 0.6, "ts":
// 0.3}). The Score is `int` (0-100) and is `null` in the DB until
// the goroutine reaches `Complete`.
type CodeHealthAnalysis struct {
	ID           string             `json:"id"`
	UserID       string             `json:"user_id"`
	Source       CodeHealthSource   `json:"source"`
	Owner        string             `json:"owner"`
	Name         string             `json:"name"`
	Branch       string             `json:"branch"`
	Status       CodeHealthStatus   `json:"status"`
	HealthScore  *int               `json:"health_score"`
	TopIssues    []CodeHealthIssue  `json:"top_issues"`
	AllIssues    []CodeHealthIssue  `json:"all_issues,omitempty"`
	Languages    map[string]float64 `json:"languages"`
	Summary      string             `json:"summary,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	CompletedAt  *time.Time         `json:"completed_at,omitempty"`
	ExpiresAt    time.Time          `json:"expires_at"`
}
