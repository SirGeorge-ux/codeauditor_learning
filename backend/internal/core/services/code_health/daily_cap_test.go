// Tests for the daily-cap defense-in-depth in
// CodeHealthService.AnalyzeRepo (PR 2, task 2.11).
//
// Scope: validates the spec scenario "At-cap user is rejected"
// (REQ-CODE-HEALTH-011) — a user with >= 5 analyses since UTC
// midnight is rejected with a 'Daily limit reached' error so
// the MCP tool can surface JSON-RPC -32609.
package codehealth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// errCodeHealthDailyCapReached is the sentinel the service
// returns when the user has hit the 5/day cap. The MCP
// analyze_repo tool maps this to mcpkit.NewRateLimitedError
// (JSON-RPC -32609) by checking the error message; the
// production mcp-code-health service returns a
// mcpkit.NewRateLimitedError so the dispatcher's
// IsRateLimitedError branch fires. Tests assert the error
// mentions the cap so the LLM consumer can present the
// "Daily limit reached, resets at 00:00 UTC" notice.
var errCodeHealthDailyCapReached = errors.New("code-health: daily limit reached, resets at 00:00 UTC")

// recordingRepoWithCap extends the existing recordingRepo
// to support a configurable CountSince response. The base
// recordingRepo returns 0; the daily-cap tests override via
// the capCount knob.
type recordingRepoWithCap struct {
	*recordingRepo
	capCount int
}

func (r *recordingRepoWithCap) CountSince(_ context.Context, _ string, _ time.Time) (int, error) {
	return r.capCount, nil
}

// newServiceForTestWithCap is a small helper that mirrors
// newServiceForTest but takes a CodeHealthRepository so the
// daily-cap tests can inject a CountSince stub.
func newServiceForTestWithCap(t *testing.T, src *fakeSource, cache *fakeCache, reg *rulesReg, repo ports.CodeHealthRepository) *CodeHealthService {
	t.Helper()
	a := NewAnalyzer(AnalyzerConfig{
		Source:   src,
		Cache:    cache,
		Registry: reg,
	})
	return NewCodeHealthService(repo, a, WithSyncStart())
}

// TestService_AnalyzeRepo_DailyCap_AtLimitRejected is the spec
// scenario "At-cap user is rejected". The 6th analysis of
// the day is rejected; the service returns the daily-cap
// sentinel so the MCP tool surfaces it as JSON-RPC -32609.
func TestService_AnalyzeRepo_DailyCap_AtLimitRejected(t *testing.T) {
	src := &fakeSource{
		files:    map[string][]ports.FileEntry{"o/r": {}},
		contents: map[string]string{},
	}
	cache := newFakeCache()
	registry := defaultRegistry(t)
	repo := &recordingRepoWithCap{recordingRepo: newRecordingRepo(), capCount: 5}
	svc := newServiceForTestWithCap(t, src, cache, registry, repo)

	// The analyze_repo flow inserts a pending row FIRST
	// (so we have a row to update), then runs the daily-cap
	// check. With WithSyncStart + a cap count of 5 (>= 5),
	// the service must return the daily-cap error. The
	// pending row remains in the recording repo; the
	// goroutine body never runs.
	_, err := svc.AnalyzeRepo(context.Background(), "u-1", "o", "r", "gogs", "main", false)
	if err == nil {
		t.Fatalf("expected daily-cap error, got nil")
	}
	if !errors.Is(err, errCodeHealthDailyCapReached) && !contains(err.Error(), "daily limit reached") {
		t.Errorf("err = %v, want error mentioning 'daily limit reached'", err)
	}
}

// TestService_AnalyzeRepo_DailyCap_UnderCapProceeds is the
// spec scenario "Under-cap user can analyze". A user with 4
// analyses today can submit a 5th; the analyzer runs (sync
// mode here) and the row's status is completed.
func TestService_AnalyzeRepo_DailyCap_UnderCapProceeds(t *testing.T) {
	src := &fakeSource{
		files: map[string][]ports.FileEntry{
			"o/r": {{Path: "src/main.go", SHA: "sha-main", Size: 100}},
		},
		contents: map[string]string{
			"o/r/src/main.go": "package main\nfunc main() {}\n",
		},
	}
	cache := newFakeCache()
	registry := defaultRegistry(t)
	repo := &recordingRepoWithCap{recordingRepo: newRecordingRepo(), capCount: 4}
	svc := newServiceForTestWithCap(t, src, cache, registry, repo)

	id, err := svc.AnalyzeRepo(context.Background(), "u-1", "o", "r", "gogs", "main", false)
	if err != nil {
		t.Fatalf("under-cap AnalyzeRepo: %v", err)
	}
	if id == "" {
		t.Fatalf("AnalyzeRepo returned empty id; under-cap should proceed")
	}
	// The sync-mode run should have completed; verify the
	// row's status.
	got, err := repo.FindByID(context.Background(), id, "u-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Status != models.CodeHealthStatusCompleted {
		t.Errorf("status = %q, want completed (under-cap)", got.Status)
	}
}

// TestService_AnalyzeRepo_DailyCap_BoundaryCount5 is the
// off-by-one triangulation case: a count of exactly 5
// (the cap) must reject; a count of 4 must proceed. The
// 4-vs-5 split is the spec's "5/day" wording.
func TestService_AnalyzeRepo_DailyCap_BoundaryCount5(t *testing.T) {
	for _, tc := range []struct {
		name     string
		capCount int
		wantErr  bool
	}{
		{"count=4_proceeds", 4, false},
		{"count=5_rejected", 5, true},
		{"count=6_rejected", 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := &fakeSource{
				files:    map[string][]ports.FileEntry{"o/r": {}},
				contents: map[string]string{},
			}
			cache := newFakeCache()
			registry := defaultRegistry(t)
			repo := &recordingRepoWithCap{recordingRepo: newRecordingRepo(), capCount: tc.capCount}
			svc := newServiceForTestWithCap(t, src, cache, registry, repo)
			_, err := svc.AnalyzeRepo(context.Background(), "u-1", "o", "r", "gogs", "main", false)
			if tc.wantErr && err == nil {
				t.Errorf("count=%d: expected daily-cap error, got nil", tc.capCount)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("count=%d: expected proceed, got %v", tc.capCount, err)
			}
		})
	}
}

// contains is a tiny strings.Contains alias so the test
// file does not import "strings" (the recorder doesn't need
// it; this is the only place the package would use it).
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
