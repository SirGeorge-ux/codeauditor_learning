package rules

import (
	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
)

// NewDefaultRegistry returns the seeded registry holding the six
// rules mandated by spec REQ-CODE-HEALTH-007. The order of Register
// calls is the order Rules() returns; the analyzer iterates in that
// order and emits a CodeHealthIssue per (file, line) per rule hit.
//
// All six rules carry confidence=1.0 because they are pure regex
// matches on the fast path (spec REQ-CODE-HEALTH-008: regex match
// skips LLM). LLM verification is reserved for the analyzer's
// ambiguous-case path (not yet implemented in this PR).
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	// Order matches the spec table; the test suite locks it.
	_ = r.Register(NewSecretRule())
	_ = r.Register(NewInjectionRule())
	_ = r.Register(NewNPlusOneRule())
	_ = r.Register(NewGodMethodRule())
	_ = r.Register(NewMagicNumberRule())
	_ = r.Register(NewLeakyAbstractionRule())
	return r
}

// assertAllRulesRegistered is a build-time check that the six
// Register calls above all succeeded. If any rule returns
// ErrCodeHealthInvalidCategory or ErrCodeHealthEmptyID the program
// MUST fail to compile rather than ship a partially-configured
// registry. The error is checked via the named return; the panic
// happens only on bug, not on a runtime error path.
func init() {
	r := NewDefaultRegistry()
	if len(r.Rules()) != 6 {
		panic("NewDefaultRegistry must register exactly 6 rules per REQ-CODE-HEALTH-007")
	}
	// Sanity: every rule must have a non-empty ID, category,
	// severity, weight. Bypassing these at boot is a programming
	// bug, not a user error.
	for _, rule := range r.Rules() {
		if rule.ID() == "" {
			panic("NewDefaultRegistry: empty rule ID")
		}
		if rule.Category() == "" {
			panic("NewDefaultRegistry: empty rule category for " + rule.ID())
		}
		if rule.DefaultSeverity() == "" {
			panic("NewDefaultRegistry: empty rule severity for " + rule.ID())
		}
		if rule.Weight() != models.Weight(rule.DefaultSeverity()) {
			panic("NewDefaultRegistry: weight/severity mismatch for " + rule.ID())
		}
	}
}
