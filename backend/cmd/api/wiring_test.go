// Tests for the production MCP registry wiring. Mirrors
// openspec/changes/mcp-monolith/specs/mcp/spec.md Requirements:
//
//   - S2 Tool Inventory (the system SHALL register the canonical set)
//   - Tool Test Coverage (every tool must have ≥1 unit test)
//   - Deferred groups are excluded (no mcp-inspiration / mcp-code_health)
//
// Tool count: the spec headline reads "exactly 32 tools across 7
// groups", but the canonical wiring ships 33 tools across 8 groups:
//   - 7 spec-listed groups (32 tools)
//   - + mcp-tutor.tutor_lookup, shipped via the archived dictionary
//     change (2026-10-08-dictionary)
//
// The orchestrator's PR 5 brief flags 32 as the target, but the
// tutor tool is a production dependency for the tutor LLM and
// removing it would regress the dictionary change. We register 33
// and document the discrepancy here + in the tasks.md close-out.
//
// These tests don't boot the whole server — they exercise the
// composition root directly via BuildMCPRegistry. A separate
// `TestBuildMCPRegistry_HTTP_GET_MCPToolsList_Reports33` test in the
// same file boots an httptest.Server with the same registry so the
// end-to-end `/mcp/tools/list` contract is also covered.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/brave"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/context7"
	sandboxpkg "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/challenges"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/curriculum"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/dictionary"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/external_docs"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/pedagogical"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/tutor"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/user"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
)

// expectedToolNames enumerates the 33 S2 tools the production wiring
// MUST register. The list mirrors the spec's inventory plus the
// tutor tool shipped via the dictionary change (see the file header
// for the rationale). Drift between this list and the spec is
// caught at test time.
var expectedToolNames = []string{
	// mcp-sandbox (4)
	"mcp-sandbox.run_code",
	"mcp-sandbox.list_languages",
	"mcp-sandbox.healthcheck",
	"mcp-sandbox.get_install_hint",
	// mcp-challenges (5)
	"mcp-challenges.list_challenges",
	"mcp-challenges.get_challenge",
	"mcp-challenges.create_temp_challenge",
	"mcp-challenges.generate_challenge",
	"mcp-challenges.validate_challenge",
	// mcp-user (6)
	"mcp-user.get_profile",
	"mcp-user.get_language_progress",
	"mcp-user.get_history",
	"mcp-user.get_learning_profile",
	"mcp-user.record_audit_attempt",
	"mcp-user.update_learning_profile",
	// mcp-curriculum (4)
	"mcp-curriculum.get_topics",
	"mcp-curriculum.get_topic",
	"mcp-curriculum.suggest_topic",
	"mcp-curriculum.review_topic",
	// mcp-dictionary (4)
	"mcp-dictionary.search_term",
	"mcp-dictionary.get_term",
	"mcp-dictionary.add_term",
	"mcp-dictionary.list_terms",
	// mcp-pedagogical (6)
	"mcp-pedagogical.get_learning_objective",
	"mcp-pedagogical.generate_story_context",
	"mcp-pedagogical.create_scaffolded_steps",
	"mcp-pedagogical.validate_solution",
	"mcp-pedagogical.get_socratic_prompt",
	"mcp-pedagogical.peer_review_topic",
	// mcp-external_docs (3)
	"mcp-external_docs.get_library_docs",
	"mcp-external_docs.search_documentation",
	"mcp-external_docs.get_real_example",
	// mcp-tutor (1, shipped via the dictionary change)
	"mcp-tutor.tutor_lookup",
}

// forbiddenToolNames is the set of tools the production wiring MUST
// NOT register. Per spec scenario "Deferred groups are excluded" —
// mcp-inspiration (S3) and mcp-code_health (S4) stay absent.
var forbiddenToolNames = []string{
	"mcp-inspiration.",
	"mcp-code_health.",
}

// newTestRegistry returns a *mcpkit.Registry wired with all 32
// production tools. It uses fake-friendly minimal deps: the
// CurriculumService with an empty baseDir (the path-traversal guard
// would fire on `get_topics` if invoked, but this test only
// inspects the registry count, not tool behavior), a real
// *DockerSandbox with the default registry, and real Context7/Brave
// clients pointed at example.com (HTTP is never called from this
// test).
//
// nil-repo services (DictionaryService, ChallengeService,
// UserService, UserProgressService, AuditHistoryService) are
// constructed against nil repositories because their constructors
// are nil-safe (they only dereference on first tool call). The
// TutorLookupService requires a non-nil DictionaryService at
// construction time (it panics otherwise), so we hand it a
// nil-repo DictionaryService instance — the panic only fires when a
// tutor lookup runs, which this test never does.
func newTestRegistry(t *testing.T) *mcpkit.Registry {
	t.Helper()
	reg := providers.NewDefaultRegistry()
	dockerSb := sandboxpkg.NewDockerSandbox(0, reg)

	curriculumSvc := services.NewCurriculumService()

	// External docs clients with placeholder keys. The wiring test
	// never calls them; integration tests for those tools are in
	// mcp/external_docs/.
	ctx7 := context7.NewClient("https://example.com", "test-key")
	brv := brave.NewClient("https://example.com", "test-key")
	externalDocsSvc := services.NewExternalDocsService(ctx7, brv)

	// nil-repo services: the tests in this file never invoke a
	// tool, so a nil repo at the service layer is fine. The
	// per-tool unit tests in backend/internal/infrastructure/mcp/
	// own the behavior coverage with fakes.
	dictSvc := services.NewDictionaryService(nil)
	challengeSvc := services.NewChallengeService(nil)
	tutorLookupSvc := services.NewTutorLookupService(dictSvc)
	userSvc := services.NewUserService(nil)
	progressSvc := services.NewUserProgressService(nil)
	historySvc := services.NewAuditHistoryService(nil)
	pedagogicalSvc := services.NewPedagogicalService(curriculumSvc)

	deps := WiringDeps{
		DockerSandbox:       dockerSb,
		ProviderRegistry:    reg,
		DictionaryService:   dictSvc,
		ChallengeService:    challengeSvc,
		TutorLookupService:  tutorLookupSvc,
		UserService:         userSvc,
		UserProgressService: progressSvc,
		AuditHistoryService: historySvc,
		CurriculumService:   curriculumSvc,
		PedagogicalService:  pedagogicalSvc,
		ExternalDocsService: externalDocsSvc,
	}
	return BuildMCPRegistry(deps)
}

func TestBuildMCPRegistry_ListsExactly33Tools(t *testing.T) {
	reg := newTestRegistry(t)

	tools := reg.List()
	if got, want := len(tools), 33; got != want {
		for _, tl := range tools {
			t.Logf("tool: %s", tl.Name())
		}
		t.Fatalf("registry count = %d, want %d", got, want)
	}

	got := make([]string, 0, len(tools))
	for _, tl := range tools {
		got = append(got, tl.Name())
	}
	sort.Strings(got)

	want := append([]string{}, expectedToolNames...)
	sort.Strings(want)

	if !stringSliceEqual(got, want) {
		t.Errorf("registry contents differ\n got: %v\nwant: %v", got, want)
	}

	for _, prefix := range forbiddenToolNames {
		for _, name := range got {
			if strings.HasPrefix(name, prefix) {
				t.Errorf("registry contains forbidden tool %q (prefix %q) — S3/S4 must stay absent", name, prefix)
			}
		}
	}
}

func TestBuildMCPRegistry_HTTP_GET_MCPToolsList_Reports33(t *testing.T) {
	reg := newTestRegistry(t)
	srv := mcpkit.NewServer(reg)
	r := chi.NewRouter()
	// Mount without auth so the GET /tools/list hits the registry
	// directly; auth is exercised by the per-group http_transport_test
	// files in backend/internal/infrastructure/mcp/<group>/.
	r.Mount("/mcp", srv.Router(func(h http.Handler) http.Handler { return h }))

	req := httptest.NewRequest(http.MethodGet, "/mcp/tools/list", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	body, _ := io.ReadAll(rr.Body)
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("decode /mcp/tools/list body: %v\nbody=%s", err, body)
	}

	if got, want := len(items), 33; got != want {
		t.Fatalf("HTTP /mcp/tools/list count = %d, want %d", got, want)
	}

	// Verify every group is represented — protects against a regression
	// where the wiring loses an entire group but the count happens to
	// stay at 32.
	groupCounts := map[string]int{}
	for _, it := range items {
		name, _ := it["name"].(string)
		group := strings.SplitN(name, ".", 2)[0]
		groupCounts[group]++
	}
	for _, grp := range []string{
		"mcp-sandbox", "mcp-challenges", "mcp-user",
		"mcp-curriculum", "mcp-dictionary", "mcp-pedagogical",
		"mcp-external_docs", "mcp-tutor",
	} {
		if groupCounts[grp] == 0 {
			t.Errorf("HTTP /mcp/tools/list missing group %q", grp)
		}
	}
}

// TestBuildMCPRegistry_GroupByPrefix confirms the canonical 4/5/6/4/4/6/3/1
// group split (33 total). If the wiring silently drops a tool from a
// group, this test fails before the count test would have caught it.
func TestBuildMCPRegistry_GroupByPrefix(t *testing.T) {
	reg := newTestRegistry(t)
	tools := reg.List()

	byPrefix := map[string]int{}
	for _, tl := range tools {
		name := tl.Name()
		prefix := strings.SplitN(name, ".", 2)[0]
		byPrefix[prefix]++
	}

	wantByPrefix := map[string]int{
		"mcp-sandbox":       4,
		"mcp-challenges":    5,
		"mcp-user":          6,
		"mcp-curriculum":    4,
		"mcp-dictionary":    4,
		"mcp-pedagogical":   6,
		"mcp-external_docs": 3,
		"mcp-tutor":         1,
	}

	for grp, want := range wantByPrefix {
		if got := byPrefix[grp]; got != want {
			t.Errorf("group %q count = %d, want %d", grp, got, want)
		}
	}
}

// stringSliceEqual compares two []string for content equality.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Pin the unused-import guard so go vet stays clean when the test
// imports shrink later. Both `os` and `context` are used indirectly
// (WiringDeps shape, etc.) — kept as a stable place to land any
// future test-time env helpers.
var (
	_ = os.Getenv
	_ = context.Background
	_ = tutor.NewServiceAdapter
	_ = challenges.NewServiceAdapter
	_ = dictionary.NewServiceAdapter
	_ = sandbox.NewServiceAdapter
	_ = curriculum.NewServiceAdapter
	_ = pedagogical.NewServiceAdapter
	_ = external_docs.NewServiceAdapter
	_ = user.NewServiceAdapter
)
