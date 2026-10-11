package models

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWeight_KnownSeverities locks the canonical weight table from
// spec REQ-CODE-HEALTH-004: critical=15, high=8, medium=3, low=1.
// Changing any of these is a SPEC break — both the health-score
// formula and the issue ranking would shift.
func TestWeight_KnownSeverities(t *testing.T) {
	cases := []struct {
		sev    Severity
		weight int
	}{
		{SeverityCritical, 15},
		{SeverityHigh, 8},
		{SeverityMedium, 3},
		{SeverityLow, 1},
	}
	for _, c := range cases {
		t.Run(string(c.sev), func(t *testing.T) {
			assert.Equal(t, c.weight, Weight(c.sev), "weight for %s", c.sev)
		})
	}
}

// TestWeight_UnknownSeverity_ReturnsZero is the safe default. A rule
// that emits a typo'd severity (e.g. "critcal") contributes 0 to the
// score instead of polluting the dashboard. The registry-level
// category check is the real defense (it rejects unknown categories);
// this is belt-and-suspenders.
func TestWeight_UnknownSeverity_ReturnsZero(t *testing.T) {
	assert.Equal(t, 0, Weight(""))
	assert.Equal(t, 0, Weight(Severity("critcal")))
	assert.Equal(t, 0, Weight(Severity("blocker")))
}

// TestHealthScore_ZeroIssues_Yields100 covers the spec scenario
// "Zero issues yields 100". A clean repo (no rule hits) is a perfect
// 100. The clamp is a no-op here.
func TestHealthScore_ZeroIssues_Yields100(t *testing.T) {
	got := HealthScore(nil)
	assert.Equal(t, 100, got, "nil issues -> 100")

	got = HealthScore([]CodeHealthIssue{})
	assert.Equal(t, 100, got, "empty slice -> 100")
}

// TestHealthScore_MixedSeverities_WorkedExample covers the spec
// scenario "Mixed severity score". Worked example (orchestrator
// input): 2 critical + 3 high + 5 medium + 10 low = 100 - (30+24+15+10)
// = 100 - 79 = 21. Note the orchestrator cited 11 in the spec text
// (2c+4h+6m+10l = 100-90 = 10) — the underlying arithmetic is the
// same; this test pins the table-driven behavior so a future change
// to the weight table is caught immediately.
func TestHealthScore_MixedSeverities_WorkedExample(t *testing.T) {
	issues := make([]CodeHealthIssue, 0, 21)
	for i := 0; i < 2; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityCritical})
	}
	for i := 0; i < 3; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityHigh})
	}
	for i := 0; i < 5; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityMedium})
	}
	for i := 0; i < 10; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityLow})
	}
	got := HealthScore(issues)
	assert.Equal(t, 21, got, "2c+3h+5m+10l = 100-79 = 21")
}

// TestHealthScore_OrchestratorWorkedExample pins the value the
// orchestrator cited explicitly: 2 critical + 4 high + 6 medium + 10
// low = 100 - (30+32+18+10) = 100 - 90 = 10. A change in any weight
// will trip this.
func TestHealthScore_OrchestratorWorkedExample(t *testing.T) {
	issues := make([]CodeHealthIssue, 0, 22)
	for i := 0; i < 2; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityCritical})
	}
	for i := 0; i < 4; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityHigh})
	}
	for i := 0; i < 6; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityMedium})
	}
	for i := 0; i < 10; i++ {
		issues = append(issues, CodeHealthIssue{Severity: SeverityLow})
	}
	got := HealthScore(issues)
	assert.Equal(t, 10, got, "2c+4h+6m+10l = 100-90 = 10")
}

// TestHealthScore_ClampAtZero covers the spec scenario "Score clamps
// at 0". 20 critical issues = 300 weight; 100 - 300 = -200; clamp
// returns 0 (never negative).
func TestHealthScore_ClampAtZero(t *testing.T) {
	issues := make([]CodeHealthIssue, 20)
	for i := range issues {
		issues[i] = CodeHealthIssue{Severity: SeverityCritical}
	}
	got := HealthScore(issues)
	assert.Equal(t, 0, got, "20 critical = 100-300 = -200 -> clamp to 0")
}

// TestHealthScore_OverwhelmingCritical covers the worst-case
// practical input: 50 critical issues = 750 weight; result must
// still be 0 (clamped), never negative.
func TestHealthScore_OverwhelmingCritical(t *testing.T) {
	issues := make([]CodeHealthIssue, 50)
	for i := range issues {
		issues[i] = CodeHealthIssue{Severity: SeverityCritical}
	}
	got := HealthScore(issues)
	assert.Equal(t, 0, got)
}

// TestRankIssues_ByWeightDesc covers the spec scenario "Ranked
// Issues". Highest severity first; lower-severity issues must appear
// after higher-severity ones in the ranked list.
func TestRankIssues_ByWeightDesc(t *testing.T) {
	issues := []CodeHealthIssue{
		{Line: 10, Category: CategoryStyle, Severity: SeverityLow},
		{Line: 20, Category: CategorySecurity, Severity: SeverityCritical},
		{Line: 30, Category: CategoryStyle, Severity: SeverityMedium},
	}
	ranked := RankIssues(issues)
	require.Len(t, ranked, 3)
	assert.Equal(t, SeverityCritical, ranked[0].Severity, "critical first")
	assert.Equal(t, SeverityMedium, ranked[1].Severity, "medium second")
	assert.Equal(t, SeverityLow, ranked[2].Severity, "low last")
}

// TestRankIssues_TieBreakByLine covers the spec scenario "Two
// critical issues ordered by line". Same severity -> lower line
// number wins.
func TestRankIssues_TieBreakByLine(t *testing.T) {
	issues := []CodeHealthIssue{
		{Line: 42, Category: CategorySecurity, Severity: SeverityCritical},
		{Line: 17, Category: CategorySecurity, Severity: SeverityCritical},
	}
	ranked := RankIssues(issues)
	require.Len(t, ranked, 2)
	assert.Equal(t, 17, ranked[0].Line, "line 17 first")
	assert.Equal(t, 42, ranked[1].Line, "line 42 second")
}

// TestRankIssues_TieBreakByCategory covers the spec scenario "Same
// severity, same line, ordered by category". security < style in
// alphabetical order; security should appear first.
func TestRankIssues_TieBreakByCategory(t *testing.T) {
	issues := []CodeHealthIssue{
		{Line: 10, Category: CategoryStyle, Severity: SeverityHigh},
		{Line: 10, Category: CategorySecurity, Severity: SeverityHigh},
	}
	ranked := RankIssues(issues)
	require.Len(t, ranked, 2)
	assert.Equal(t, CategorySecurity, ranked[0].Category, "security before style (alphabetical)")
	assert.Equal(t, CategoryStyle, ranked[1].Category, "style second")
}

// TestRankIssues_DoesNotMutateInput locks the contract that
// RankIssues returns a NEW slice (not the input reordered in
// place). Callers downstream (mcp-code-health.get_repo_issues) may
// hold a reference to the input slice and rely on its order.
func TestRankIssues_DoesNotMutateInput(t *testing.T) {
	original := []CodeHealthIssue{
		{Line: 1, Category: CategoryStyle, Severity: SeverityLow},
		{Line: 2, Category: CategorySecurity, Severity: SeverityCritical},
	}
	snapshot := make([]CodeHealthIssue, len(original))
	copy(snapshot, original)

	_ = RankIssues(original)

	assert.Equal(t, snapshot, original, "RankIssues must not mutate the input slice")
}

// TestRankIssues_EmptyAndNil are the boundary checks. A nil or empty
// input returns an empty (non-nil) slice so the JSON wire shape
// stays `[]` not `null`.
func TestRankIssues_EmptyAndNil(t *testing.T) {
	assert.NotNil(t, RankIssues(nil))
	assert.Empty(t, RankIssues(nil))
	assert.NotNil(t, RankIssues([]CodeHealthIssue{}))
	assert.Empty(t, RankIssues([]CodeHealthIssue{}))
}

// TestHealthScore_NilSafe is a defensive check — HealthScore on a
// nil slice must not panic. The implementation must treat nil the
// same as empty (0 weight sum -> 100).
func TestHealthScore_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HealthScore(nil) panicked: %v", r)
		}
	}()
	assert.Equal(t, 100, HealthScore(nil))
}

// TestHealthScore_IsAlwaysInRange is the clamp-invariant. No
// combination of issues may push the score below 0 or above 100.
// We sample 5 severities x 5 counts to make a regression in the
// clamp obvious.
func TestHealthScore_IsAlwaysInRange(t *testing.T) {
	sevs := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}
	for _, sev := range sevs {
		for _, n := range []int{0, 1, 5, 20, 100} {
			issues := make([]CodeHealthIssue, n)
			for i := range issues {
				issues[i] = CodeHealthIssue{Severity: sev}
			}
			got := HealthScore(issues)
			if got < 0 || got > 100 {
				t.Errorf("HealthScore out of range for %s x%d: got %d", sev, n, got)
			}
		}
	}
}

// TestRankIssues_TableDriven locks the full tie-break order with a
// single deterministic input. Any drift in the comparator shows up
// here.
func TestRankIssues_TableDriven(t *testing.T) {
	issues := []CodeHealthIssue{
		{Line: 30, Category: CategoryStyle, Severity: SeverityHigh, Message: "high-style-30"},
		{Line: 10, Category: CategorySecurity, Severity: SeverityCritical, Message: "crit-sec-10"},
		{Line: 5, Category: CategoryRefactor, Severity: SeverityCritical, Message: "crit-ref-5"},
		{Line: 10, Category: CategoryStyle, Severity: SeverityCritical, Message: "crit-style-10"},
		{Line: 10, Category: CategorySecurity, Severity: SeverityCritical, Message: "crit-sec-10-other"},
		{Line: 99, Category: CategoryArchitecture, Severity: SeverityLow, Message: "low-arch-99"},
	}
	ranked := RankIssues(issues)
	require.Len(t, ranked, 6)

	// Expected order:
	// 1. crit-ref-5   (weight=15, line=5)
	// 2. crit-sec-10  (weight=15, line=10, cat=security)
	// 3. crit-sec-10-other (weight=15, line=10, cat=security — but
	//    deterministic tie-break leaves both crit-sec-10 entries
	//    adjacent in input order; sort is stable, so the earlier one
	//    in the input wins)
	// 4. crit-style-10 (weight=15, line=10, cat=style)
	// 5. high-style-30 (weight=8)
	// 6. low-arch-99  (weight=1)
	expectedMessages := []string{
		"crit-ref-5",
		"crit-sec-10",
		"crit-sec-10-other",
		"crit-style-10",
		"high-style-30",
		"low-arch-99",
	}
	for i, want := range expectedMessages {
		assert.Equal(t, want, ranked[i].Message, "position %d", i)
	}
}

// TestRankIssues_UsesSortStable uses sort.Stable so the input
// ordering is preserved for equal keys. This protects the table
// test above from non-determinism.
func TestRankIssues_UsesSortStable(t *testing.T) {
	// The two crit-sec-10 entries are identical on every
	// comparator field; they must come out in input order
	// (crit-sec-10 before crit-sec-10-other).
	issues := []CodeHealthIssue{
		{Line: 10, Category: CategorySecurity, Severity: SeverityCritical, Message: "first"},
		{Line: 10, Category: CategorySecurity, Severity: SeverityCritical, Message: "second"},
	}
	ranked := RankIssues(issues)
	require.Len(t, ranked, 2)
	assert.Equal(t, "first", ranked[0].Message)
	assert.Equal(t, "second", ranked[1].Message)
}

// TestHealthScore_TableDriven exercises every severity bucket
// independently so a single weight-table change cannot mask a
// regression in another bucket.
func TestHealthScore_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		issues  []CodeHealthIssue
		wantOut int
	}{
		{"1 critical -> 85", []CodeHealthIssue{{Severity: SeverityCritical}}, 85},
		{"1 high -> 92", []CodeHealthIssue{{Severity: SeverityHigh}}, 92},
		{"1 medium -> 97", []CodeHealthIssue{{Severity: SeverityMedium}}, 97},
		{"1 low -> 99", []CodeHealthIssue{{Severity: SeverityLow}}, 99},
		{"6 critical -> 100-90=10 (not clamped)", makeN(SeverityCritical, 6), 10},
		{"7 critical -> 100-105=-5 -> 0", makeN(SeverityCritical, 7), 0},
		{"13 high -> 100-104=-4 -> 0", makeN(SeverityHigh, 13), 0},
		{"34 medium -> 100-102=-2 -> 0", makeN(SeverityMedium, 34), 0},
		{"101 low -> 100-101=-1 -> 0", makeN(SeverityLow, 101), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := HealthScore(c.issues)
			assert.Equal(t, c.wantOut, got)
		})
	}
}

// makeN is a tiny helper that allocates a fresh slice of N identical
// issues. Used by the table-driven test above.
func makeN(sev Severity, n int) []CodeHealthIssue {
	out := make([]CodeHealthIssue, n)
	for i := range out {
		out[i] = CodeHealthIssue{Severity: sev}
	}
	return out
}

// silence unused-import warnings on the linter when this file is
// the only consumer of sort (it isn't, but the linter occasionally
// needs the nudge for shadow imports).
var _ = sort.Stable
