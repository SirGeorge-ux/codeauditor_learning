package codehealth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// HealthReport is the rendered artifact returned by
// CodeHealthService.GenerateHealthReport. The MCP tool
// `mcp-code-health.generate_health_report` exposes this struct on
// the wire (REQ-MCP-CODE-HEALTH-001).
type HealthReport struct {
	ReportID string
	Format   string // "md" today; "pdf" is S5 (per design.md)
	Content  string
}

// ErrCodeHealthPDFNotImplemented is returned by
// GenerateHealthReport when format == "pdf". The MCP tool maps
// this to JSON-RPC -32603 (per design.md tradeoffs).
var ErrCodeHealthPDFNotImplemented = errors.New("code-health: pdf report format is not implemented yet (S5)")

// ReportFormat is the canonical format enum for the report
// renderer. Mirrors the MCP wire shape (snake_case).
const (
	ReportFormatMarkdown = "md"
	ReportFormatPDF      = "pdf"
)

// ServiceOptions bundles the optional configuration the
// CodeHealthService accepts. The Service struct itself is the
// constructor; option funcs shape the post-construction
// behaviour.
type ServiceOptions struct {
	// Now is the clock injection point; tests pass a fixed
	// time. Defaults to time.Now if nil.
	Now func() time.Time
	// AsyncStart gates whether AnalyzeRepo spawns a goroutine
	// (production) or runs synchronously (test mode). When
	// false, AnalyzeRepo blocks until the analysis is complete
	// — convenient for unit tests that need deterministic
	// status transitions.
	AsyncStart bool
	// MaxFiles / MaxFileSize forward to the analyzer.
	MaxFiles    int
	MaxFileSize int64
}

// ServiceOption is the functional-options setter.
type ServiceOption func(*ServiceOptions)

// WithSyncStart forces AnalyzeRepo to run synchronously. The
// default (production) is async; tests opt into the sync path
// so they can assert post-conditions immediately.
func WithSyncStart() ServiceOption {
	return func(o *ServiceOptions) { o.AsyncStart = false }
}

// WithAsyncStart explicitly enables the production async
// behaviour (the default in NewService, but the test helper
// can toggle).
func WithAsyncStart() ServiceOption {
	return func(o *ServiceOptions) { o.AsyncStart = true }
}

// WithClock injects a fixed clock; useful for the expires_at
// expiry test.
func WithClock(now func() time.Time) ServiceOption {
	return func(o *ServiceOptions) {
		if now != nil {
			o.Now = now
		}
	}
}

// WithMaxFiles overrides the per-run file cap. The default
// comes from the analyzer constant.
func WithMaxFiles(n int) ServiceOption {
	return func(o *ServiceOptions) { o.MaxFiles = n }
}

// WithMaxFileSize overrides the per-file size cap (bytes).
func WithMaxFileSize(n int64) ServiceOption {
	return func(o *ServiceOptions) { o.MaxFileSize = n }
}

// CodeHealthService is the orchestrator. It owns:
//
//   - the async lifecycle (insert pending -> spawn goroutine ->
//     pending -> running -> completed|failed)
//   - the per-file analyzer pipeline
//   - the GetAnalysis read path with expiry semantics
//   - the GetRepoIssues read path (severity filter + limit)
//   - the GenerateHealthReport renderer (md only, pdf deferred)
//
// Hexagonal contract: this type imports only stdlib + ports +
// the local models/rules. No database/sql, no HTTP, no LLM
// SDKs.
type CodeHealthService struct {
	repo     ports.CodeHealthRepository
	analyzer *Analyzer
	opts     ServiceOptions
}

// NewCodeHealthService constructs the service. `repo` is the
// driven port (Supabase adapter in production, fake in tests);
// `analyzer` is the per-file pipeline; `opts` are optional
// config knobs.
func NewCodeHealthService(repo ports.CodeHealthRepository, analyzer *Analyzer, opts ...ServiceOption) *CodeHealthService {
	o := ServiceOptions{
		Now:         time.Now,
		AsyncStart:  true, // production default
		MaxFiles:    DefaultMaxFiles,
		MaxFileSize: DefaultMaxFileSize,
	}
	for _, fn := range opts {
		fn(&o)
	}
	// Apply MaxFiles / MaxFileSize to the analyzer's config so
	// the per-file pipeline honors the service-level knobs.
	if o.MaxFiles > 0 {
		analyzer.cfg.MaxFiles = o.MaxFiles
	}
	if o.MaxFileSize > 0 {
		analyzer.cfg.MaxFileSize = o.MaxFileSize
	}
	return &CodeHealthService{repo: repo, analyzer: analyzer, opts: o}
}

// AnalyzeRepo is the async entry point (REQ-CODE-HEALTH-003).
// It inserts a pending row, returns the analysis_id immediately
// (<500ms in production), and spawns a goroutine that runs the
// per-file analyzer and transitions the row to running ->
// completed | failed. With WithSyncStart the goroutine is
// inlined; tests use that mode.
//
// `source` selects which underlying SourceClient the analyzer
// will use (the multiSourceClient in main.go dispatches by
// string). Valid values are "gogs" or "github" — callers (the
// MCP tool layer) MUST validate before invoking this method.
func (s *CodeHealthService) AnalyzeRepo(ctx context.Context, userID, owner, repo, source, branch string, force bool) (string, error) {
	// Default source to "gogs" when empty (back-compat for
	// tests that pre-date the multi-source refactor).
	if source == "" {
		source = string(models.SourceGogs)
	}
	// Insert the pending row first so we have a row to update.
	pending := &models.CodeHealthAnalysis{
		UserID:    userID,
		Source:    models.CodeHealthSource(source),
		Owner:     owner,
		Name:      repo,
		Branch:    branch,
		Status:    models.CodeHealthStatusPending,
		Languages: map[string]float64{},
		TopIssues: []models.CodeHealthIssue{},
	}
	row, err := s.repo.InsertAnalysis(ctx, pending)
	if err != nil {
		return "", fmt.Errorf("insert analysis: %w", err)
	}

	if !s.opts.AsyncStart {
		// Test mode: run synchronously, blocking until done.
		s.runAnalysis(context.Background(), userID, row.ID, source, owner, repo, branch, force)
		return row.ID, nil
	}

	// Production: spawn a goroutine. We use context.Background
	// (not the request ctx) because the request may be cancelled
	// by the time the analysis completes.
	go s.runAnalysis(context.Background(), userID, row.ID, source, owner, repo, branch, force)
	return row.ID, nil
}

// runAnalysis is the goroutine body. It does:
//
//  1. pending -> running (best-effort; a race with the user's
//     poll happens if the row vanished)
//  2. invoke the analyzer.AnalyzeRepo
//  3. rank + compute the health score
//  4. running -> completed (via Complete) or failed (via
//     UpdateStatus)
//
// All errors are logged + persisted as `failed` with
// error_message. The goroutine is fire-and-forget; the
// orchestrator / MCP tool will surface the failure via
// get_analysis.
func (s *CodeHealthService) runAnalysis(ctx context.Context, userID, analysisID, source, owner, repo, branch string, force bool) {
	// Best-effort status flip. A 0-rows response is logged and
	// ignored — the row may have been deleted by the user.
	if err := s.repo.UpdateStatus(ctx, analysisID, userID, string(models.CodeHealthStatusRunning), ""); err != nil {
		log.Printf("code-health: pending->running %s: %v", analysisID, err)
	}

	// Route the analyzer's SourceClient to the requested
	// source via a runtime type assertion (the analyzer's
	// Source is the multiSourceClient in main.go, which
	// implements the optional SourceSelector interface). This
	// keeps the SourceClient port narrow (single-source
	// clients like GogsClient / GitHubClient do NOT need to
	// implement SetSource).
	if sel, ok := s.analyzer.cfg.Source.(SourceSelector); ok {
		sel.SetSource(source)
	}

	issues, err := s.analyzer.AnalyzeRepo(ctx, userID, owner, repo, branch, force)
	if err != nil {
		if upErr := s.repo.UpdateStatus(ctx, analysisID, userID, string(models.CodeHealthStatusFailed), err.Error()); upErr != nil {
			log.Printf("code-health: failed status update %s: %v", analysisID, upErr)
		}
		return
	}

	ranked := models.RankIssues(issues)
	score := models.HealthScore(ranked)
	top := topN(ranked, 10)
	languages := computeLanguages(issues) // approximate; refined in PR 3 by analyzer
	summary := summarize(ranked, score)

	if err := s.repo.Complete(ctx, analysisID, userID, score, top, languages, summary); err != nil {
		log.Printf("code-health: complete %s: %v", analysisID, err)
	}
}

// topN returns the first n issues; if the slice is shorter the
// whole slice is returned.
func topN(issues []models.CodeHealthIssue, n int) []models.CodeHealthIssue {
	if len(issues) <= n {
		return issues
	}
	return issues[:n]
}

// computeLanguages returns a per-language byte-share map derived
// from the issues' file_path extensions. This is a coarse
// approximation — the per-repo language breakdown lives in the
// analyzer (PR 2 / PR 3 widen the surface); the MVP uses a
// simple file-extension roll-up.
func computeLanguages(issues []models.CodeHealthIssue) map[string]float64 {
	if len(issues) == 0 {
		return map[string]float64{}
	}
	counts := map[string]int{}
	for _, iss := range issues {
		ext := strings.ToLower(extOf(iss.FilePath))
		lang := langForExt(ext)
		counts[lang]++
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(counts))
	for k, c := range counts {
		out[k] = float64(c) / float64(total)
	}
	return out
}

// extOf returns the file extension including the dot, or "".
func extOf(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx < 0 {
		return ""
	}
	return path[idx:]
}

// langForExt maps the canonical extensions to the language
// names the dashboard renders. Falls back to the raw extension
// (minus the dot) when the extension is unknown.
func langForExt(ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	switch ext {
	case "go":
		return "go"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "py":
		return "python"
	case "rb":
		return "ruby"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "kt":
		return "kotlin"
	case "swift":
		return "swift"
	case "c", "h":
		return "c"
	case "cpp", "cc", "cxx":
		return "cpp"
	case "cs":
		return "csharp"
	case "php":
		return "php"
	case "sh", "bash":
		return "bash"
	default:
		return ext
	}
}

// summarize returns a one-line human summary of the analysis.
func summarize(issues []models.CodeHealthIssue, score int) string {
	counts := map[models.Severity]int{}
	for _, iss := range issues {
		counts[iss.Severity]++
	}
	parts := []string{}
	for _, sev := range []models.Severity{models.SeverityCritical, models.SeverityHigh, models.SeverityMedium, models.SeverityLow} {
		if counts[sev] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sev], sev))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Health score %d. No issues found.", score)
	}
	sort.Strings(parts) // deterministic order
	return fmt.Sprintf("Health score %d. Issues: %s.", score, strings.Join(parts, ", "))
}

// GetAnalysis returns the analysis row scoped to userID. A row
// past expires_at with no completed_at is reported as
// `failed` ("job expired") per REQ-CODE-HEALTH-003.
func (s *CodeHealthService) GetAnalysis(ctx context.Context, userID, analysisID string) (models.CodeHealthAnalysis, error) {
	a, err := s.repo.FindByID(ctx, analysisID, userID)
	if err != nil {
		return models.CodeHealthAnalysis{}, err
	}
	// Expiry check: pending/running past expires_at -> failed.
	if a.CompletedAt == nil && s.opts.Now().After(a.ExpiresAt) {
		if a.Status == models.CodeHealthStatusPending || a.Status == models.CodeHealthStatusRunning {
			a.Status = models.CodeHealthStatusFailed
			a.ErrorMessage = "job expired (24h TTL exceeded)"
		}
	}
	return a, nil
}

// IsSuppressed is the SuppressionChecker implementation the
// analyzer consumes. It forwards to the repository so the
// analyzer stays decoupled from the storage layer. Returns
// (false, nil) on transient errors so the analyzer never
// drops a real issue just because the suppression lookup
// failed.
func (s *CodeHealthService) IsSuppressed(ctx context.Context, userID, filePath, ruleID string, lineNumber int) (bool, error) {
	return s.repo.IsSuppressed(ctx, userID, filePath, ruleID, lineNumber)
}

// GetRepoIssues returns ranked issues for the most-recent
// analysis of (owner, name, source) scoped to userID, filtered
// by severity (optional) and capped at limit. The ranking
// follows REQ-CODE-HEALTH-005 (weight DESC, line ASC, category
// ASC); the SQL ORDER BY in the repository mirrors this.
func (s *CodeHealthService) GetRepoIssues(ctx context.Context, userID, owner, name, source, severity string, limit int) ([]models.CodeHealthIssue, error) {
	history, err := s.repo.ListHistory(ctx, owner, name, source, userID, 1)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	if len(history) == 0 {
		return []models.CodeHealthIssue{}, nil
	}
	analysisID := history[0].ID
	if limit <= 0 {
		limit = 50
	}
	issues, err := s.repo.ListIssues(ctx, analysisID, severity, limit)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	return issues, nil
}

// GenerateHealthReport renders the analysis as a markdown (or
// pdf — S5) report. The MVP supports markdown only; pdf returns
// ErrCodeHealthPDFNotImplemented so the MCP tool can map it to
// -32603.
func (s *CodeHealthService) GenerateHealthReport(ctx context.Context, userID, analysisID, format string) (HealthReport, error) {
	switch format {
	case ReportFormatMarkdown, "markdown":
		// fall through to the renderer below
	case ReportFormatPDF: // == "pdf"
		return HealthReport{}, ErrCodeHealthPDFNotImplemented
	default:
		return HealthReport{}, fmt.Errorf("unsupported report format %q (want md or pdf)", format)
	}

	a, err := s.GetAnalysis(ctx, userID, analysisID)
	if err != nil {
		return HealthReport{}, err
	}
	if a.HealthScore == nil {
		return HealthReport{}, fmt.Errorf("analysis %s not yet completed", analysisID)
	}
	content := renderMarkdownReport(a)
	// ReportID uses a stable hash of the analysis_id so a
	// regeneration produces a deterministic id; we don't expose
	// the raw analysis_id in the wire shape (defense in depth).
	reportID := "rpt-" + shortHash(analysisID)
	return HealthReport{
		ReportID: reportID,
		Format:   ReportFormatMarkdown,
		Content:  content,
	}, nil
}

// shortHash returns a 6-char base36 slice of the input string's
// FNV-1a hash. The output is stable (deterministic per input)
// and short enough for the report_id wire field. A 6-char base36
// value is sufficient for a UI identifier; the analysis_id is
// the canonical server-side key.
func shortHash(s string) string {
	const offset uint64 = 14695981039346656037
	const prime uint64 = 1099511628211
	h := offset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	out := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		out[i] = digits[h%36]
		h /= 36
	}
	return string(out)
}

// renderMarkdownReport assembles the markdown report. Mirrors
// the spec's "generate_health_report" wire shape: header with
// the health score, the top issues table, a language breakdown,
// and a "generate challenges" hint.
func renderMarkdownReport(a models.CodeHealthAnalysis) string {
	var b strings.Builder
	b.WriteString("# Code Health Report\n\n")
	b.WriteString("**Repository:** ")
	b.WriteString(string(a.Source))
	b.WriteString(":")
	b.WriteString(a.Owner)
	b.WriteString("/")
	b.WriteString(a.Name)
	b.WriteString(" @ ")
	b.WriteString(a.Branch)
	b.WriteString("\n")
	b.WriteString("**Health score:** ")
	if a.HealthScore != nil {
		b.WriteString(fmt.Sprintf("%d", *a.HealthScore))
	}
	b.WriteString(" / 100\n\n")
	if a.Summary != "" {
		b.WriteString("**Summary:** ")
		b.WriteString(a.Summary)
		b.WriteString("\n\n")
	}
	b.WriteString("## Top issues\n\n")
	if len(a.TopIssues) == 0 {
		b.WriteString("_No issues._\n")
	} else {
		b.WriteString("| # | Severity | Category | File | Line | Rule | Message |\n")
		b.WriteString("|---|----------|----------|------|------|------|---------|\n")
		for i, iss := range a.TopIssues {
			b.WriteString(fmt.Sprintf("| %d | %s | %s | `%s` | %d | `%s` | %s |\n",
				i+1, iss.Severity, iss.Category, iss.FilePath, iss.Line, iss.Rule, iss.Message))
		}
	}
	b.WriteString("\n## Languages\n\n")
	if len(a.Languages) == 0 {
		b.WriteString("_No source files matched._\n")
	} else {
		// Deterministic order: alphabetical key.
		keys := make([]string, 0, len(a.Languages))
		for k := range a.Languages {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(fmt.Sprintf("- %s: %.0f%%\n", k, a.Languages[k]*100))
		}
	}
	b.WriteString("\n## Generate challenges\n\n")
	b.WriteString("Click \"Generate challenges from these\" on each issue to turn it into a dojo challenge.\n")
	return b.String()
}
