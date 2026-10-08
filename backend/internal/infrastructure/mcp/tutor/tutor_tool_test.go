package tutor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// --- Test infrastructure --------------------------------------------------

// fakeTutorService is a deterministic in-memory stand-in for
// *services.TutorLookupService that satisfies the local Service interface.
// It mirrors the production service's behavior closely enough that boundary
// tests for the MCP adapter (validation, JSON shape, socratic levels) cover
// the same surface without standing up Postgres or the real dictionary.
type fakeTutorService struct {
	mu sync.Mutex

	// In-memory dictionary keyed by userID|lower-term, value is the term.
	store map[string]*models.Term

	// Behavior knobs (per-test).
	tutorErr error // forced error returned from TutorLookup

	// Recorded last call (allows asserting that the adapter translated the
	// JSON args into the right domain input).
	lastIn services.TutorLookupInput

	// Pre-cooked result for the off-test scripted-call path; nil returns the
	// naturally computed result from store.
	scriptedOut *services.TutorLookupResult
}

func newFakeTutorService() *fakeTutorService {
	return &fakeTutorService{store: make(map[string]*models.Term)}
}

func (s *fakeTutorService) seed(userID, term, explanation string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := &models.Term{
		UserID:      userID,
		Term:        strings.ToLower(term),
		Explanation: explanation,
		Category:    "Concepto",
		Synonyms:    []string{},
		Tags:        []string{},
	}
	s.store[userID+"|"+clone.Term] = clone
}

func (s *fakeTutorService) TutorLookup(ctx context.Context, in services.TutorLookupInput) (*services.TutorLookupResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastIn = in
	if s.tutorErr != nil {
		return nil, s.tutorErr
	}
	if s.scriptedOut != nil {
		return s.scriptedOut, nil
	}
	// Mirror the production behavior: trim + lowercase + empty-query miss +
	// word-prefix AND on the term/explanation. The framing shape is part of
	// the production service's contract; tests of the adapter only need the
	// BOOLEANS (found, source, level) and the framing non-emptiness.
	if strings.TrimSpace(in.Query) == "" {
		return &services.TutorLookupResult{
			Found:         false,
			Framing:       "Based on what I know about this:",
			Source:        "general",
			SocraticLevel: clampLevelForTest(in.Level),
		}, nil
	}
	hits := []*models.Term{}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	for _, t := range s.store {
		if t.UserID != in.UserID {
			continue
		}
		haystack := strings.ToLower(t.Term) + " " + strings.ToLower(t.Explanation)
		if strings.Contains(haystack, q) {
			hits = append(hits, t)
		}
	}
	if len(hits) == 0 {
		return &services.TutorLookupResult{
			Found:         false,
			Framing:       "Based on what I know about " + q + ":",
			Source:        "general",
			SocraticLevel: clampLevelForTest(in.Level),
		}, nil
	}
	top := hits[0]
	return &services.TutorLookupResult{
		Found:         true,
		Term:          top,
		Framing:       "By your dictionary: " + top.Term + " — " + top.Explanation + ".",
		Source:        "dictionary",
		SocraticLevel: clampLevelForTest(in.Level),
	}, nil
}

// clampLevelForTest mirrors services.clampLevel by clamping the input to
// [0..3]. The fake does NOT call into the production service's helper to
// avoid a package-cycle.
func clampLevelForTest(level int) int {
	if level < 0 {
		return 0
	}
	if level > 3 {
		return 3
	}
	return level
}

func ctxWithUser(userID string) context.Context {
	if userID == "" {
		return context.Background()
	}
	return ContextWithUserID(context.Background(), userID)
}

// --- Adapter: production *services.TutorLookupService → Service ----------

func TestServiceAdapter_TutorLookup_FoundTerm_PassesInputThrough(t *testing.T) {
	// Wire a real TutorLookupService (no DB needed) through the adapter.
	repo := newRepoStub()
	dict := services.NewDictionaryService(repo)
	if _, err := dict.AddTerm(context.Background(), services.AddTermInput{
		UserID:      "u-1",
		Term:        "PR",
		Explanation: "Pull Request on a code review platform.",
		Category:    "Concepto",
	}); err != nil {
		t.Fatalf("AddTerm: %v", err)
	}
	tutorSvc := services.NewTutorLookupService(dict)
	adapter := NewServiceAdapter(tutorSvc)

	out, err := adapter.TutorLookup(context.Background(), services.TutorLookupInput{
		UserID: "u-1", Query: "PR", Level: 0,
	})
	if err != nil {
		t.Fatalf("adapter TutorLookup: %v", err)
	}
	if !out.Found {
		t.Errorf("Found = false, want true (term PR exists)")
	}
	if out.Source != "dictionary" {
		t.Errorf("Source = %q, want dictionary", out.Source)
	}
	if out.SocraticLevel != 0 {
		t.Errorf("SocraticLevel = %d, want 0", out.SocraticLevel)
	}
}

// --- HTTP transport / MCP wiring tests ----------------------------------

func TestTutorLookupTool_NameIsConsistentAcrossBoots(t *testing.T) {
	svc := newFakeTutorService()
	tool := NewTutorLookupTool(svc)
	if tool.Name() != "mcp-tutor.tutor_lookup" {
		t.Errorf("Name = %q, want mcp-tutor.tutor_lookup", tool.Name())
	}
}

func TestTutorLookupTool_Schema_HasQueryAndLevel(t *testing.T) {
	// The schema reaches the LLM as JSON, not as a Go map. We assert the
	// JSON wire shape (which is what the function-calling prompt consumes)
	// rather than the in-memory Go type (which is `map[string]propertyArg`,
	// not `map[string]any`, due to Go's retain-the-concrete-type rule on
	// `any` assignments).
	svc := newFakeTutorService()
	tool := NewTutorLookupTool(svc)
	raw, err := json.Marshal(tool.Schema().Function.Parameters)
	if err != nil {
		t.Fatalf("marshal Schema().Function.Parameters: %v", err)
	}
	var params struct {
		Type       string         `json:"type"`
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
		Extra      map[string]any `json:"-"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if params.Type != "object" {
		t.Errorf("schema.type = %q, want object", params.Type)
	}
	for _, want := range []string{"query", "level"} {
		if _, ok := params.Properties[want]; !ok {
			t.Errorf("schema missing %q property, got %v", want, params.Properties)
		}
	}
	if !containsString(params.Required, "query") || !containsString(params.Required, "level") {
		t.Errorf("required list = %v, want both query and level", params.Required)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestTutorLookupTool_FoundTerm_ReturnsTermAndFraming(t *testing.T) {
	svc := newFakeTutorService()
	svc.seed("u-1", "PR", "Pull Request on a code review platform.")
	tool := NewTutorLookupTool(svc)

	got, err := tool.Execute(ctxWithUser("u-1"), json.RawMessage(`{"query":"PR","level":0}`))
	if err != nil {
		t.Fatalf("Execute(found): %v", err)
	}
	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("Execute returned %T, want json.RawMessage", got)
	}
	var payload struct {
		Found         bool           `json:"found"`
		Term          *models.Term   `json:"term,omitempty"`
		Framing       string         `json:"framing"`
		Source        string         `json:"source"`
		SocraticLevel int            `json:"socratic_level"`
		Extra         map[string]any `json:"-"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if !payload.Found {
		t.Errorf("payload.found = false, want true")
	}
	if payload.Term == nil || payload.Term.Term != "pr" {
		t.Errorf("payload.term = %+v, want pr", payload.Term)
	}
	if payload.Source != "dictionary" {
		t.Errorf("payload.source = %q, want dictionary", payload.Source)
	}
	if payload.SocraticLevel != 0 {
		t.Errorf("payload.socratic_level = %d, want 0", payload.SocraticLevel)
	}
	if !strings.Contains(strings.ToLower(payload.Framing), "pull request") {
		t.Errorf("payload.framing should reference the dictionary explanation, got %q", payload.Framing)
	}
}

func TestTutorLookupTool_MissTerm_ReturnsGeneralSource_NoDictionaryLeak(t *testing.T) {
	// Spec scenario: "Tutor finds no matching term" — must silently fall back
	// and NEVER expose "dictionary" detail to the user-facing framing.
	svc := newFakeTutorService()
	tool := NewTutorLookupTool(svc)

	got, err := tool.Execute(ctxWithUser("u-1"), json.RawMessage(`{"query":"monad","level":0}`))
	if err != nil {
		t.Fatalf("Execute(miss): %v", err)
	}
	raw := got.(json.RawMessage)
	var payload struct {
		Found   bool   `json:"found"`
		Framing string `json:"framing"`
		Source  string `json:"source"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Found {
		t.Errorf("payload.found = true, want false")
	}
	if payload.Source != "general" {
		t.Errorf("payload.source = %q, want general", payload.Source)
	}
	if strings.Contains(strings.ToLower(payload.Framing), "dictionary") {
		t.Errorf("fallback framing leaked 'dictionary': %q", payload.Framing)
	}
}

func TestTutorLookupTool_SocraticLevelEchoesAcrossAllFourLevels(t *testing.T) {
	// Spec scenario "Tutor respects socratic level" — the production service
	// shapes the framing by level. Adapter must forward `level` verbatim.
	svc := newFakeTutorService()
	svc.seed("u-1", "PR", "Pull Request on a code review platform.")
	tool := NewTutorLookupTool(svc)

	for _, lvl := range []int{0, 1, 2, 3} {
		args := map[string]any{"query": "PR", "level": lvl}
		raw, _ := json.Marshal(args)
		got, err := tool.Execute(ctxWithUser("u-1"), raw)
		if err != nil {
			t.Fatalf("Execute(level=%d): %v", lvl, err)
		}
		var payload struct {
			SocraticLevel int `json:"socratic_level"`
		}
		if err := json.Unmarshal(got.(json.RawMessage), &payload); err != nil {
			t.Fatalf("unmarshal level=%d: %v", lvl, err)
		}
		if payload.SocraticLevel != lvl {
			t.Errorf("level=%d echoed back as %d", lvl, payload.SocraticLevel)
		}
	}
}

func TestTutorLookupTool_OutOfRangeLevelIsClampedByService(t *testing.T) {
	// Adapter forwards level as-provided; the production service clamps to
	// [0..3]. We assert that contract holds via the adapter boundary.
	repo := newRepoStub()
	dict := services.NewDictionaryService(repo)
	tutorSvc := services.NewTutorLookupService(dict)
	adapter := NewServiceAdapter(tutorSvc)

	for _, c := range []struct{ in, want int }{{-7, 0}, {99, 3}} {
		out, err := adapter.TutorLookup(context.Background(), services.TutorLookupInput{
			UserID: "u-1", Query: "anything", Level: c.in,
		})
		if err != nil {
			t.Fatalf("adapter TutorLookup(in=%d): %v", c.in, err)
		}
		if out.SocraticLevel != c.want {
			t.Errorf("in=%d -> level=%d, want %d", c.in, out.SocraticLevel, c.want)
		}
	}
}

func TestTutorLookupTool_EmptyQuery_ReturnsGeneralSourceWithoutError(t *testing.T) {
	svc := newFakeTutorService()
	svc.seed("u-1", "PR", "Pull Request on a code review platform.")
	tool := NewTutorLookupTool(svc)

	got, err := tool.Execute(ctxWithUser("u-1"), json.RawMessage(`{"query":"  ","level":2}`))
	if err != nil {
		t.Fatalf("Execute(empty): %v", err)
	}
	raw := got.(json.RawMessage)
	var payload struct {
		Found         bool `json:"found"`
		SocraticLevel int  `json:"socratic_level"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Found {
		t.Errorf("empty query should be a clean miss, got found=true")
	}
	if payload.SocraticLevel != 2 {
		t.Errorf("socratic_level echoed as %d, want 2", payload.SocraticLevel)
	}
}

func TestTutorLookupTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	// Without an authenticated user_id the tool MUST refuse — security gate
	// before any dictionary read.
	svc := newFakeTutorService()
	tool := NewTutorLookupTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"PR","level":0}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams", err)
	}
}

func TestTutorLookupTool_EchoesQueryAndLevelToService(t *testing.T) {
	// Adapter contract: args.user_id is NEVER trusted from JSON — the tool
	// derives user_id from the auth context only. Args.query/level are passed
	// through to the service after basic trimming.
	svc := newFakeTutorService()
	tool := NewTutorLookupTool(svc)

	if _, err := tool.Execute(ctxWithUser("u-from-context"), json.RawMessage(`{"query":"  hello  ","level":2,"user_id":"u-from-json"}`)); err != nil && !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastIn.UserID != "u-from-context" {
		t.Errorf("lastIn.UserID = %q, want u-from-context (json user_id MUST be ignored)", svc.lastIn.UserID)
	}
	if svc.lastIn.Level != 2 {
		t.Errorf("lastIn.Level = %d, want 2", svc.lastIn.Level)
	}
}

// newRepoStub returns an in-memory DictionaryRepository stub used by
// ServiceAdapter tests that wire the REAL TutorLookupService. It is kept
// local to the test file to keep the production package free of test stubs.
func newRepoStub() *inMemoryDictRepo {
	return &inMemoryDictRepo{byID: map[string]*models.Term{}}
}

type inMemoryDictRepo struct {
	byID map[string]*models.Term
	next int
}

func (r *inMemoryDictRepo) InsertTerm(_ context.Context, userID string, term *models.Term) (*models.Term, error) {
	r.next++
	clone := *term
	clone.UserID = userID
	clone.ID = "stub-" + itoa(r.next)
	r.byID[clone.ID] = &clone
	out := clone
	return &out, nil
}

func (r *inMemoryDictRepo) GetTerm(_ context.Context, _, id string) (*models.Term, error) {
	t, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	out := *t
	return &out, nil
}

func (r *inMemoryDictRepo) UpdateTerm(_ context.Context, _ string, _ *models.Term) error { return nil }

func (r *inMemoryDictRepo) DeleteTerm(_ context.Context, _, _ string) error { return nil }

func (r *inMemoryDictRepo) ListTerms(_ context.Context, _, _, _ string) ([]*models.Term, error) {
	return nil, nil
}

func (r *inMemoryDictRepo) SearchTerms(_ context.Context, _, q string) ([]*models.Term, error) {
	out := []*models.Term{}
	qLow := strings.ToLower(strings.TrimSpace(q))
	for _, t := range r.byID {
		if qLow != "" && !strings.Contains(strings.ToLower(t.Term+" "+t.Explanation), qLow) {
			continue
		}
		clone := *t
		out = append(out, &clone)
	}
	return out, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	return out
}
