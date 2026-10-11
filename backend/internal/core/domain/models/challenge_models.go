package models

import "time"

// Hint represents a progressive hint for a challenge (v2 model).
// JSON tags use snake_case to match the DB JSONB column format.
type Hint struct {
	Level      int    `json:"level"`
	Content    string `json:"content"`
	CostPoints int    `json:"cost_points"`
}

// ExpectedFinding represents an objective finding the auditor should discover (v2 model).
type ExpectedFinding struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Message      string `json:"message"`
	Evidence     string `json:"evidence"`
	SuggestedFix string `json:"suggested_fix"`
}

// TestCase represents a functional test case for scoring (v2 model).
type TestCase struct {
	Name           string `json:"name"`
	Input          any    `json:"input"`
	ExpectedOutput any    `json:"expected_output"`
	Weight         int    `json:"weight"`
}

// LinterRule represents a linter configuration for objective evaluation (v2 model).
type LinterRule struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

// AuditInput represents the results of a code audit, used by ScoringService.
// Each boolean slice is indexed to match the corresponding Challenge slice.
type AuditInput struct {
	TestsPassed     []bool  // index-aligned with Challenge.TestCases
	LintClean       []bool  // index-aligned with Challenge.LinterRules
	FindingsMatched []bool  // index-aligned with Challenge.ExpectedFindings
	HintsUsed       []bool  // index-aligned with Challenge.Hints
	ElapsedMinutes  float64 // time the user took to complete the challenge
}

// ScoreBreakdown provides a transparent breakdown of the audit score.
type ScoreBreakdown struct {
	BasePoints      int `json:"base_points"`
	TestPoints      int `json:"test_points"`      // sum(test_passed * weight * 30)
	LintPoints      int `json:"lint_points"`      // sum(lint_clean * 20)
	FindingsMatched int `json:"findings_matched"` // sum(expected_finding_matched * 10)
	HintsPenalty    int `json:"hints_penalty"`    // sum(hint_used * cost)
	TimeBonus       int `json:"time_bonus"`       // 10% of base_points when within time
	Total           int `json:"total"`            // floored at 0
}

// Challenge represents a code-audit challenge.
//
// v1 fields (RepoURL, CodeSmell) are retained for backward compatibility with
// the existing ChallengeService SQL queries. They will be removed in PR 2 when
// the backend wiring is rewritten to use v2 columns exclusively.
type Challenge struct {
	// v1 fields (deprecated — will be removed in PR 2)
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Difficulty  string    `json:"difficulty"`
	Category    string    `json:"category"`
	Language    string    `json:"language"`
	RepoURL     string    `json:"repoUrl,omitempty"`
	Code        string    `json:"code"`
	CodeSmell   string    `json:"codeSmell,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UserID      *string   `json:"userId,omitempty"`
	SourceRepo  string    `json:"sourceRepo,omitempty"`

	// v2 fields (camelCase JSON tags for API serialization)
	LearningObjectives   []string          `json:"learningObjectives"`
	Hints                []Hint            `json:"hints"`
	CommonMistakes       []string          `json:"commonMistakes"`
	EstimatedTimeMinutes int               `json:"estimatedTimeMinutes"`
	ExpectedFindings     []ExpectedFinding `json:"expectedFindings"`
	TestCases            []TestCase        `json:"testCases"`
	LinterRules          []LinterRule      `json:"linterRules"`
	SolutionCode         string            `json:"solutionCode,omitempty"`
	SolutionExplanation  string            `json:"solutionExplanation,omitempty"`
	BasePoints           int               `json:"basePoints"`
	BonusPoints          int               `json:"bonusPoints"`
	PenaltyPerHint       int               `json:"penaltyPerHint"`
	TimeBonus            bool              `json:"timeBonus"`
	Origin               string            `json:"origin"`
	SourcePath           string            `json:"sourcePath,omitempty"`
	GeneratedBy          string            `json:"generatedBy,omitempty"`
	CreatedBy            string            `json:"createdBy,omitempty"`
	// SourceIssueID is the code-health issue UUID the S4
	// flow set when generating the challenge. Empty for
	// S3 free-practice rows. Persisted to the
	// challenges.source_issue_id column (added in
	// migration 010_create_code_health.sql).
	SourceIssueID string `json:"sourceIssueId,omitempty"`
}

// GenerateChallengeHint is the domain input for ChallengeService.GenerateChallenge.
// The MCP mcp-challenges.generate_challenge tool surfaces a JSON shape with
// these optional fields; the adapter translates the wire form into
// this struct before calling the service. Keeping the type in domain
// (instead of buried in the MCP layer) lets the S5 LLM cascade own the
// canonical input shape regardless of transport.
//
// S3 PR 2 extended the shape with TopicID + Level so the un-stubbed
// GenerateChallenge can drive curriculum + pedagogical lookups without
// changing the public transport contract for callers that omit them
// (existing v1 hints keep compiling + behaving as before).
type GenerateChallengeHint struct {
	// Language biases the produced challenge to a target language
	// (typescript, python, go, ...). Empty means "no preference".
	Language string `json:"language,omitempty"`
	// Difficulty biases the produced challenge to a range
	// (junior, mid, senior, architect). Empty means "no preference".
	Difficulty string `json:"difficulty,omitempty"`
	// Focus biases the produced challenge to a code-smell category
	// (n+1, callback hell, ...). Empty means "no preference".
	Focus string `json:"focus,omitempty"`
	// TopicID is the curriculum topic slug the user selected on
	// /practice/free (e.g. "inheritance"). The service reads
	// `.atl/curricula/<language>.md` to source the topic body. Empty
	// means "no curriculum-driven generation".
	TopicID string `json:"topicId,omitempty"`
	// Level is the F..S rank the user picked from the language
	// progress model. The service maps Level to the persisted
	// `Challenge.Difficulty` field (F,E→junior, D,C→mid,
	// B,A→senior, S→architect). Empty means "use Hint.Difficulty
	// as a fallback".
	Level string `json:"level,omitempty"`
	// SourceIssueID is the code-health issue id the S4 flow
	// passes when the user clicks "Generate challenge from
	// this issue" on a code-health dashboard. The service
	// branches on this field: when non-empty, the persisted
	// row carries Origin="code_health", GeneratedBy="code-
	// health-v1", and SourcePath=<issue.file_path>. Empty
	// means "S3 free-practice path" (no code-health metadata,
	// Origin="generated", GeneratedBy="free-practice-v1").
	//
	// S4 PR 2 (code-health-dashboard) added this field as
	// an additive change. Existing S3 callers that omit it
	// continue to compile + behave as before (omitempty).
	SourceIssueID string `json:"sourceIssueId,omitempty"`
}

// CreateChallengeInput is the request body for creating a challenge.
//
// v1 fields (RepoURL, CodeSmell) are retained for backward compatibility.
// v2 fields are optional with zero-value defaults.
type CreateChallengeInput struct {
	// v1 fields
	Title       string `json:"title"`
	Description string `json:"description"`
	Difficulty  string `json:"difficulty"`
	Category    string `json:"category"`
	Language    string `json:"language"`
	RepoURL     string `json:"repoUrl"`
	SourceRepo  string `json:"sourceRepo"`
	Code        string `json:"code"`
	CodeSmell   string `json:"codeSmell"`

	// v2 fields (optional — camelCase JSON for API consistency)
	LearningObjectives   []string          `json:"learningObjectives,omitempty"`
	Hints                []Hint            `json:"hints,omitempty"`
	CommonMistakes       []string          `json:"commonMistakes,omitempty"`
	EstimatedTimeMinutes int               `json:"estimatedTimeMinutes,omitempty"`
	ExpectedFindings     []ExpectedFinding `json:"expectedFindings,omitempty"`
	TestCases            []TestCase        `json:"testCases,omitempty"`
	LinterRules          []LinterRule      `json:"linterRules,omitempty"`
	SolutionCode         string            `json:"solutionCode,omitempty"`
	SolutionExplanation  string            `json:"solutionExplanation,omitempty"`
	BasePoints           int               `json:"basePoints,omitempty"`
	BonusPoints          int               `json:"bonusPoints,omitempty"`
	PenaltyPerHint       int               `json:"penaltyPerHint,omitempty"`
	TimeBonus            bool              `json:"timeBonus,omitempty"`
	Origin               string            `json:"origin,omitempty"`
	SourcePath           string            `json:"sourcePath,omitempty"`
	GeneratedBy          string            `json:"generatedBy,omitempty"`
	CreatedBy            string            `json:"createdBy,omitempty"`
}
