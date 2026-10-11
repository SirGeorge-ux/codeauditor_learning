package codehealth

import (
	"context"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/services/code_health/rules"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// errSource is a tiny in-test SourceClient that always returns
// the supplied error. Used to drive the "failed" status
// transition test.
type errSource struct{ err error }

func (e *errSource) ListFiles(_ context.Context, _, _, _ string) ([]ports.FileEntry, error) {
	return nil, e.err
}

func (e *errSource) GetFileContent(_ context.Context, _, _, _, _ string) (*ports.FileContent, error) {
	return nil, e.err
}

// newServiceForTest builds a CodeHealthService with the test
// fakes. AsyncStart is OFF so the analysis runs synchronously
// inside the test goroutine; the test can assert post-conditions
// without Eventually.
func newServiceForTest(t *testing.T, src ports.SourceClient, cache ports.AnalysisCache, reg *rules.Registry, repo ports.CodeHealthRepository) *CodeHealthService {
	t.Helper()
	analyzer := NewAnalyzer(AnalyzerConfig{
		Source:      src,
		Cache:       cache,
		Registry:    reg,
		MaxFiles:    DefaultMaxFiles,
		MaxFileSize: DefaultMaxFileSize,
	})
	return NewCodeHealthService(
		repo, analyzer,
		WithSyncStart(),
		WithClock(func() time.Time { return time.Now() }),
	)
}

// newRulesDefault returns the seeded registry. Wraps
// rules.NewDefaultRegistry so the test's helper name reads
// "newRulesDefault" (the alias keeps the test file clean of
// the upstream package name).
func newRulesDefault() *rules.Registry {
	return rules.NewDefaultRegistry()
}

// rulesReg is a local alias of *rules.Registry used by
// defaultRegistry in service_test.go. The alias keeps the
// test's signature short.
type rulesReg = rules.Registry
