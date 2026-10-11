package rules

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// File is the value type every Rule.Detect receives. It carries
// everything a rule needs to scan a single file: the path (for
// reporting and for the suppression PK), the language hint (some
// rules gate on language), and the full text content. SHA is the
// git blob SHA so the analyzer can use the same value as the cache
// key (analysis:<sha>) and the same value the issue row persists.
type File struct {
	Path     string
	Language string
	Content  string
	SHA      string
}

// Rule is the contract every code-health rule implements. The
// registry calls Detect once per file; the analyzer gathers the
// returned issues and routes them through the suppression filter
// before persisting. The interface is intentionally narrow —
// registry + category coverage + detection only. The per-rule
// regexes and detection logic live in concrete files (secret.go,
// injection.go, ...).
type Rule interface {
	// ID is the stable canonical identifier following the spec
	// RULE-<CAT>-<NAME>-<NNN> format (REQ-CODE-HEALTH-007). It is
	// persisted on every issue row and is the suppression PK
	// component (task 1.19).
	ID() string
	// Category is one of the 5 canonical IssueCategory values
	// (REQ-CODE-HEALTH-006). The registry rejects any rule whose
	// category is not in models.AllowedIssueCategories.
	Category() models.IssueCategory
	// DefaultSeverity is the baseline severity band the rule
	// emits. LLM-verified findings can lower the confidence but
	// the severity band stays anchored to this default.
	DefaultSeverity() models.Severity
	// Weight is the impact weight (mirrors models.Weight on
	// severity). Surfaced on the rule for the registry's
	// post-condition assertion; the health-score formula always
	// looks up the weight via models.Weight(sev).
	Weight() int
	// Detect scans file and returns zero or more issues. A
	// non-nil error aborts the per-file analysis (the analyzer
	// surfaces it as a failed file scan, NOT as a general
	// analysis failure).
	Detect(ctx context.Context, file File) ([]models.CodeHealthIssue, error)
}

// ErrCodeHealthInvalidCategory is returned by Registry.Register when
// a rule declares a category outside the 5 canonical values. The
// rule is NOT added; the caller (typically the boot-time wiring
// in cmd/api/main.go) MUST treat this as a fatal startup error.
var ErrCodeHealthInvalidCategory = errors.New("code-health rule category is not in the allowed list")

// ErrCodeHealthEmptyID is returned by Registry.Register when a rule
// has an empty ID. The rule is NOT added.
var ErrCodeHealthEmptyID = errors.New("code-health rule ID must be non-empty")

// AllowedIssueCategories is the canonical list every rule's
// Category() return value MUST belong to. Mirrors the
// code_health_issues.category CHECK constraint.
var AllowedIssueCategories = []models.IssueCategory{
	models.CategorySecurity,
	models.CategoryPerformance,
	models.CategoryRefactor,
	models.CategoryStyle,
	models.CategoryArchitecture,
}

// isAllowedCategory returns true when cat is in
// AllowedIssueCategories. Equality is exact — case must match.
func isAllowedCategory(cat models.IssueCategory) bool {
	for _, c := range AllowedIssueCategories {
		if c == cat {
			return true
		}
	}
	return false
}

// Registry is the in-memory catalog of all rules the analyzer can
// invoke. The constructor returns an empty registry; the boot-time
// wiring calls NewDefaultRegistry to get the seeded six. Register
// is safe for sequential calls at boot (no concurrent Register
// is expected) but the slice reads via Rules() are protected by
// a mutex so the analyzer's parallel per-file goroutines cannot
// observe a torn slice.
type Registry struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewRegistry returns an empty registry. Production code should
// use NewDefaultRegistry to get the six seeded rules; tests use
// NewRegistry to drive contract assertions.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds rule to the registry. Returns
// ErrCodeHealthInvalidCategory if the rule's category is not in
// the 5 allowed values, ErrCodeHealthEmptyID if the ID is empty.
// On error the registry is unchanged (no partial state).
func (r *Registry) Register(rule Rule) error {
	if rule.ID() == "" {
		return ErrCodeHealthEmptyID
	}
	if !isAllowedCategory(rule.Category()) {
		return fmt.Errorf("%w: %q", ErrCodeHealthInvalidCategory, rule.Category())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, rule)
	return nil
}

// Rules returns a COPY of the registered rules slice so callers
// cannot mutate internal state by appending. The returned slice is
// in registration order (no sorting); the analyzer iterates in
// that order, so the first matching rule wins on duplicate IDs
// (a guard the test suite enforces).
func (r *Registry) Rules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Rule, len(r.rules))
	copy(out, r.rules)
	return out
}

// Categories returns the unique set of categories currently
// represented in the registry, sorted alphabetically. The dashboard
// uses this to render the per-category breakdown even when no
// rules have fired in some buckets.
func (r *Registry) Categories() []models.IssueCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[models.IssueCategory]struct{}, len(AllowedIssueCategories))
	for _, rule := range r.rules {
		seen[rule.Category()] = struct{}{}
	}
	out := make([]models.IssueCategory, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
