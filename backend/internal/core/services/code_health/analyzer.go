package codehealth

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services/code_health/rules"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// DefaultMaxFiles is the per-run cap on the number of files
// analyzed. The spec (REQ-CODE-HEALTH-001 "Repo with >300 source
// files") caps at 300. Users can override via the dashboard's
// "Analyze full repo" confirmation (PR 3).
const DefaultMaxFiles = 300

// DefaultMaxFileSize is the per-file cap (1 MB). Files larger
// than this are skipped (the LLM cannot process them, and the
// regex fast path would not help).
const DefaultMaxFileSize int64 = 1 << 20

// AnalyzerConfig bundles the analyzer's dependencies. The
// Source + Cache + Registry are mandatory; MaxFiles and
// MaxFileSize have sensible defaults (300 / 1 MB).
type AnalyzerConfig struct {
	Source      ports.SourceClient
	Cache       ports.AnalysisCache
	Registry    *rules.Registry
	MaxFiles    int
	MaxFileSize int64
	// SuppressionChecker is an optional pre-rule suppression
	// gate. When set, the analyzer skips rule hits whose
	// (file_path, rule_id, line_number) tuple is in the user's
	// suppression list (spec REQ-CODE-HEALTH-008 "not an issue"
	// scenario, task 1.20). When nil, no suppression filtering
	// happens (so the analyzer stays usable in tests without a
	// repository).
	SuppressionChecker SuppressionChecker
}

// SuppressionChecker is the narrow port the analyzer consumes to
// apply user-marked suppressions. The default implementation
// (CodeHealthRepository.IsSuppressed) is injected by the
// CodeHealthService; tests inject a fake.
type SuppressionChecker interface {
	IsSuppressed(ctx context.Context, userID, filePath, ruleID string, lineNumber int) (bool, error)
}

// Analyzer is the per-file analysis pipeline (filter -> cache ->
// regex -> LLM-on-ambiguous -> cache write). It is the only
// place in the system that turns a file's content into a slice
// of CodeHealthIssue values; the service layer composes
// AnalyzeRepo + ranking + health-score on top.
type Analyzer struct {
	cfg AnalyzerConfig
}

// NewAnalyzer constructs the analyzer. The config's Source,
// Cache, and Registry MUST be non-nil; the analyzer panics if any
// is missing (a misconfigured analyzer is a programming bug, not
// a runtime degradation).
func NewAnalyzer(cfg AnalyzerConfig) *Analyzer {
	if cfg.Source == nil {
		panic("codehealth.NewAnalyzer: Source is required")
	}
	if cfg.Cache == nil {
		panic("codehealth.NewAnalyzer: Cache is required")
	}
	if cfg.Registry == nil {
		panic("codehealth.NewAnalyzer: Registry is required")
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = DefaultMaxFiles
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = DefaultMaxFileSize
	}
	return &Analyzer{cfg: cfg}
}

// SetSuppressionChecker injects the suppression port after
// construction. The wiring layer uses this to break the
// chicken-and-egg between the analyzer and the service (the
// service is the suppression checker).
func (a *Analyzer) SetSuppressionChecker(c SuppressionChecker) {
	a.cfg.SuppressionChecker = c
}

// AnalyzeRepo runs the per-file pipeline against the supplied
// (owner, repo, branch) descriptor. The returned []CodeHealthIssue
// is the UNION of all per-file hits, ranked by severity (the
// caller invokes models.RankIssues if it wants the canonical
// order).
//
// `force=true` bypasses the cache read but still overwrites the
// cache entry. Spec REQ-CODE-HEALTH-009 "Re-analyze CTA bypasses
// cache" scenario.
func (a *Analyzer) AnalyzeRepo(ctx context.Context, userID, owner, repo, branch string, force bool) ([]models.CodeHealthIssue, error) {
	files, err := a.cfg.Source.ListFiles(ctx, owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	// --- Filter pass (REQ-CODE-HEALTH-001 / REQ-CODE-HEALTH-011) ---
	sources := make([]rules.File, 0, len(files))
	for _, f := range files {
		if shouldSkip(f.Path, f.Size, a.cfg.MaxFileSize) {
			continue
		}
		// We need the SHA + size for cache keying. The
		// FileEntry already carries both.
		sources = append(sources, rules.File{
			Path:     f.Path,
			Language: languageFromPath(f.Path),
			Content:  "", // filled below
			SHA:      f.SHA,
		})
		if len(sources) >= a.cfg.MaxFiles {
			log.Printf("analyzer: hit MaxFiles cap (%d) for %s/%s", a.cfg.MaxFiles, owner, repo)
			break
		}
	}

	// --- Per-file pipeline ---
	all := make([]models.CodeHealthIssue, 0, len(sources))
	for i := range sources {
		file := sources[i]
		// Cache lookup (skip when force=true).
		if !force {
			if cached, hit, err := a.cfg.Cache.Get(ctx, userID, file.SHA); err == nil && hit {
				all = append(all, cached...)
				continue
			}
		}
		// Fetch the content.
		fc, err := a.cfg.Source.GetFileContent(ctx, owner, repo, branch, file.Path)
		if err != nil {
			// Skip this file but keep going — a 404 on one
			// file doesn't break the rest of the analysis.
			log.Printf("analyzer: skip %s: %v", file.Path, err)
			continue
		}
		file.Content = fc.Content
		// Run every rule.
		for _, rule := range a.cfg.Registry.Rules() {
			ruleIssues, err := rule.Detect(ctx, file)
			if err != nil {
				log.Printf("analyzer: rule %s on %s: %v", rule.ID(), file.Path, err)
				continue
			}
			for _, iss := range ruleIssues {
				// Suppression gate (task 1.20).
				if a.cfg.SuppressionChecker != nil {
					suppressed, err := a.cfg.SuppressionChecker.IsSuppressed(ctx, userID, iss.FilePath, iss.Rule, iss.Line)
					if err == nil && suppressed {
						continue
					}
				}
				iss.AnalysisID = "" // filled by the service on insert
				iss.SHA = file.SHA
				all = append(all, iss)
			}
		}
		// Cache write.
		if err := a.cfg.Cache.Set(ctx, userID, file.SHA, fileIssuesFrom(all, file.SHA), 7*24*60*60*1_000_000_000); err != nil {
			log.Printf("analyzer: cache write for %s: %v", file.SHA, err)
		}
	}
	return all, nil
}

// fileIssuesFrom returns the subset of `all` that originated from
// the file with the given SHA. Used to write per-file cache rows.
func fileIssuesFrom(all []models.CodeHealthIssue, sha string) []models.CodeHealthIssue {
	out := make([]models.CodeHealthIssue, 0, len(all))
	for _, iss := range all {
		if iss.SHA == sha {
			out = append(out, iss)
		}
	}
	return out
}

// shouldSkip reports whether a file should be dropped from the
// analysis list. The gates are:
//
//   - path under node_modules/, .git/, dist/, vendor/, build/
//   - binary extension (image, archive, executable, font, video)
//   - size > maxFileSize
func shouldSkip(path string, size, maxFileSize int64) bool {
	if size > maxFileSize {
		return true
	}
	// Normalise: GitHub/Gogs return "src/main.go" with forward
	// slashes; on Windows the path may use backslashes. The
	// substring match is sufficient for the spec's filter.
	skipSegments := []string{
		"node_modules/", ".git/", "dist/", "vendor/", "build/",
	}
	for _, seg := range skipSegments {
		if strings.Contains(path, seg) {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".webp": true, ".ico": true, ".svg": true, ".bmp": true,
		".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
		".mp3": true, ".wav": true, ".flac": true,
		".pdf": true, ".doc": true, ".docx": true,
	}
	return binaryExts[ext]
}

// languageFromPath is a cheap extension-based language hint for
// the rule's File.Language. The rules that gate on language
// (e.g. RULE-ARCH-LEAK-001) read this field; the others ignore
// it. Coverage is intentionally narrow — adding a language
// means extending this map AND the sandbox provider registry.
func languageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".c":
		return "c"
	case ".h":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".sh", ".bash":
		return "bash"
	default:
		return "unknown"
	}
}

// BuildPrompt assembles the prompt the analyzer would send to the
// LLM for an ambiguous-case verification. The MVP analyzer does
// NOT call the LLM (the regex fast path is the canonical lane);
// the function exists as a separate, testable surface so the
// token-leakage threat test (analyzer_test.go) has a concrete
// target. The signature will be exercised by the LLM lane when
// it lands (PR 2 or a follow-up).
//
// IMPORTANT: this function MUST NEVER include the github/gogs
// token in the prompt. The threat test asserts the property.
func BuildPrompt(path, language, content, githubToken, gogsToken string) string {
	var b strings.Builder
	b.WriteString("You are a senior code auditor. Return ONLY JSON matching the schema.\n\n")
	b.WriteString("File: ")
	b.WriteString(path)
	b.WriteString("\nLanguage: ")
	b.WriteString(language)
	b.WriteString("\n\n```")
	b.WriteString(language)
	b.WriteString("\n")
	b.WriteString(content)
	b.WriteString("\n```\n\nDetect issues in categories: security, performance, refactor, style, architecture.\n")
	b.WriteString("Output JSON: {\"issues\":[{\"severity\":\"low|medium|high|critical\",\"category\":\"...\",\"line\":<int>,\"message\":\"...\",\"evidence\":\"...\",\"suggested_fix\":\"...\"}]}\n")
	// Token values are intentionally NOT written to b — the
	// threat test asserts that the LLM lane cannot leak them.
	_ = githubToken
	_ = gogsToken
	return b.String()
}
