package models

import "time"

// Severity enumerates the impact bands the code-health dashboard
// surfaces. The weight attached to each band drives both the health
// score (spec REQ-CODE-HEALTH-004) and the issue ranking
// (REQ-CODE-HEALTH-005). Persisted as TEXT with a CHECK constraint
// pinning the canonical lowercase form.
type Severity string

// Severity values — the canonical lowercase forms persisted in the DB
// and exposed on the wire. Keep this list and the
// `code_health_issues.severity` CHECK constraint in sync.
const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// IssueCategory enumerates the five categories every detected issue
// must fall into (spec REQ-CODE-HEALTH-006). Persisted as TEXT with a
// CHECK constraint pinning the canonical lowercase form.
type IssueCategory string

// IssueCategory values — keep in sync with the
// `code_health_issues.category` CHECK constraint and the
// `services/code_health/rules.Registry.AllowedCategories` list.
const (
	CategorySecurity     IssueCategory = "security"
	CategoryPerformance  IssueCategory = "performance"
	CategoryRefactor     IssueCategory = "refactor"
	CategoryStyle        IssueCategory = "style"
	CategoryArchitecture IssueCategory = "architecture"
)

// CodeHealthIssue is a single finding emitted by a code-health rule
// (regex or LLM). It mirrors the S1 ExpectedFinding shape (1:1 on
// Severity, Category, Message, Evidence, SuggestedFix) and adds three
// code-health-specific fields:
//
//   - Rule: the stable ID of the rule that detected the issue
//     (RULE-<CAT>-<NAME>-<NNN>).
//   - Confidence: [0,1] confidence score — regex matches carry 1.0
//     (per spec REQ-CODE-HEALTH-008); LLM-verified findings carry
//     the model-reported score.
//   - SHA: the file's git blob SHA — used by the cache layer to skip
//     re-analysis when the content has not changed.
//
// The Line field is `int` (zero when the rule cannot pinpoint a
// specific line, e.g. an architecture rule that flags an import
// block). JSON tags use snake_case to match the DB JSONB projection
// (mirrors the S2 dictionary convention).
type CodeHealthIssue struct {
	ID           string        `json:"id"`
	AnalysisID   string        `json:"analysis_id"`
	Severity     Severity      `json:"severity"`
	Category     IssueCategory `json:"category"`
	FilePath     string        `json:"file_path"`
	Line         int           `json:"line"`
	Message      string        `json:"message"`
	Evidence     string        `json:"evidence,omitempty"`
	SuggestedFix string        `json:"suggested_fix,omitempty"`
	Rule         string        `json:"rule"`
	Confidence   float64       `json:"confidence"`
	SHA          string        `json:"sha,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
}
