package models

import "sort"

// severityWeights is the canonical weight table for the health score
// (spec REQ-CODE-HEALTH-004) and the issue ranking
// (REQ-CODE-HEALTH-005). Weights are:
//
//	critical = 15
//	high     =  8
//	medium   =  3
//	low      =  1
//
// Changing any of these is a SPEC break — both the score formula and
// the ranking comparator would shift. Keep this map in sync with the
// orchestrator's worked example (2c+4h+6m+10l = 100-90 = 10).
var severityWeights = map[Severity]int{
	SeverityCritical: 15,
	SeverityHigh:     8,
	SeverityMedium:   3,
	SeverityLow:      1,
}

// Weight returns the impact weight of sev. Unknown severities return
// 0 so a typo'd severity cannot crash the score (the registry's
// category check is the real defense; this is belt-and-suspenders).
func Weight(sev Severity) int {
	return severityWeights[sev]
}

// HealthScore computes the 0-100 health score per spec
// REQ-CODE-HEALTH-004:
//
//	health_score = max(0, min(100, 100 − Σ(weight_i × count_i)))
//
// Where `weight_i` is the per-severity value above and `count_i` is
// the number of issues in that severity bucket. The clamp guards
// against repositories that emit more weight than the score can
// absorb (e.g. 20 critical = 300 weight → -200 → 0). The function is
// nil-safe: a nil or empty input returns 100 (clean repo).
func HealthScore(issues []CodeHealthIssue) int {
	sum := 0
	for i := range issues {
		sum += severityWeights[issues[i].Severity]
	}
	score := 100 - sum
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// RankIssues sorts the input issues in descending priority using the
// canonical comparator from spec REQ-CODE-HEALTH-005:
//
//	1. severity weight DESC   (critical > high > medium > low)
//	2. line ASC                (lower line first when severity ties)
//	3. category ASC             (alphabetical category as the
//	                            final tie-break, mirroring the spec
//	                            scenario "Same severity, same line,
//	                            ordered by category")
//
// The function uses sort.SliceStable so input order is preserved
// when every comparator key is equal. This is a deliberate choice:
// callers that emit two identical issues must see them in the same
// order in the ranked output (matters for the dashboard's "Drill
// into issue #N" path).
//
// RankIssues NEVER mutates the input. A fresh slice is returned; the
// caller may keep its own ordering for other purposes (e.g. cache
// lookups by file_path).
func RankIssues(issues []CodeHealthIssue) []CodeHealthIssue {
	if len(issues) == 0 {
		return []CodeHealthIssue{}
	}
	out := make([]CodeHealthIssue, len(issues))
	copy(out, issues)
	sort.SliceStable(out, func(i, j int) bool {
		wi := severityWeights[out[i].Severity]
		wj := severityWeights[out[j].Severity]
		if wi != wj {
			return wi > wj
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Category < out[j].Category
	})
	return out
}
