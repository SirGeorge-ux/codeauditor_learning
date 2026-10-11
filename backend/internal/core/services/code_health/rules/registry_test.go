package rules

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// TestRegistry_Register_RejectsUnknownCategory covers the spec
// scenario "Unknown category rejected" (REQ-CODE-HEALTH-006). A rule
// with a typo'd category MUST NOT be loaded — silent acceptance would
// let the dashboard render an unlabeled bucket and the health-score
// weights would skip the entry.
func TestRegistry_Register_RejectsUnknownCategory(t *testing.T) {
	r := NewRegistry()
	err := r.Register(stubRule{
		id:       "RULE-X-FOO-001",
		category: models.IssueCategory("concurrency"), // not in the 5
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCodeHealthInvalidCategory)
	assert.Empty(t, r.Rules(), "rejected rule must not be in the registry")
}

// TestRegistry_Register_AcceptsAllFiveCategories covers the spec
// scenario "All categories covered by seeded rules"
// (REQ-CODE-HEALTH-006 + REQ-CODE-HEALTH-007). The five canonical
// categories MUST be loadable.
func TestRegistry_Register_AcceptsAllFiveCategories(t *testing.T) {
	r := NewRegistry()
	for _, cat := range []models.IssueCategory{
		models.CategorySecurity,
		models.CategoryPerformance,
		models.CategoryRefactor,
		models.CategoryStyle,
		models.CategoryArchitecture,
	} {
		err := r.Register(stubRule{id: "RULE-X-" + string(cat) + "-001", category: cat})
		require.NoError(t, err, "category %s must be acceptable", cat)
	}
	assert.Len(t, r.Rules(), 5)
}

// TestRegistry_Register_RejectsEmptyID is the second invariant — a
// rule without an ID is untrackable. The cache layer keys on SHA +
// rule ID; an empty ID would make every issue hit every cached
// rule.
func TestRegistry_Register_RejectsEmptyID(t *testing.T) {
	r := NewRegistry()
	err := r.Register(stubRule{category: models.CategorySecurity})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCodeHealthEmptyID)
	assert.Empty(t, r.Rules())
}

// TestRegistry_Categories_UniqueSorted covers the spec's
// category-coverage helper. The 5 categories must be present exactly
// once (no duplicates), in sorted order (deterministic UI rendering).
func TestRegistry_Categories_UniqueSorted(t *testing.T) {
	r := NewRegistry()
	// Register two rules in the same category to prove uniqueness
	// (Categories returns the set, not the count).
	require.NoError(t, r.Register(stubRule{id: "RULE-SEC-A-001", category: models.CategorySecurity}))
	require.NoError(t, r.Register(stubRule{id: "RULE-SEC-B-001", category: models.CategorySecurity}))
	require.NoError(t, r.Register(stubRule{id: "RULE-PERF-A-001", category: models.CategoryPerformance}))

	cats := r.Categories()
	assert.Equal(t, []models.IssueCategory{models.CategoryPerformance, models.CategorySecurity}, cats)
}

// TestRegistry_Rules_ReturnsCopy locks the contract that the slice
// returned by Rules() is decoupled from internal state — a caller
// must not be able to corrupt the registry by appending to the
// returned slice.
func TestRegistry_Rules_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(stubRule{id: "RULE-SEC-A-001", category: models.CategorySecurity}))

	first := r.Rules()
	assert.Len(t, first, 1)

	// Mutate the returned slice.
	first[0] = stubRule{id: "tampered", category: models.CategoryStyle}

	// Fetch again — the registry must still see the original.
	second := r.Rules()
	require.Len(t, second, 1)
	assert.Equal(t, "RULE-SEC-A-001", second[0].ID())
}

// TestDefaultRegistry_RegistersSixRules covers the spec scenario
// "Registry returns 6 rules" (REQ-CODE-HEALTH-007). The seeded
// registry MUST yield exactly six rules with non-empty ID,
// category, severity, and weight. This is the post-build invariant
// every new rule addition must preserve.
func TestDefaultRegistry_RegistersSixRules(t *testing.T) {
	r := NewDefaultRegistry()
	rules := r.Rules()
	require.Len(t, rules, 6, "exactly 6 seeded rules per REQ-CODE-HEALTH-007")

	for _, rule := range rules {
		assert.NotEmpty(t, rule.ID(), "rule %q has empty ID", rule.ID())
		assert.NotEmpty(t, rule.Category(), "rule %q has empty category", rule.ID())
		assert.NotEmpty(t, rule.DefaultSeverity(), "rule %q has empty severity", rule.ID())
		assert.Greater(t, rule.Weight(), 0, "rule %q has zero weight", rule.ID())
	}

	cats := r.Categories()
	assert.Len(t, cats, 5, "all 5 categories covered by 6 seeded rules")
	assert.ElementsMatch(t, []models.IssueCategory{
		models.CategorySecurity,
		models.CategoryPerformance,
		models.CategoryRefactor,
		models.CategoryStyle,
		models.CategoryArchitecture,
	}, cats)
}

// stubRule is a tiny in-test Rule implementation used to drive the
// registry's contract tests. The real rules live in their own files
// and are exercised by their per-rule Detect tests.
type stubRule struct {
	id       string
	category models.IssueCategory
	severity models.Severity
	weight   int
}

func (s stubRule) ID() string                       { return s.id }
func (s stubRule) Category() models.IssueCategory   { return s.category }
func (s stubRule) DefaultSeverity() models.Severity { return s.severity }
func (s stubRule) Weight() int                      { return s.weight }
func (s stubRule) Detect(_ context.Context, _ File) ([]models.CodeHealthIssue, error) {
	return nil, nil
}
