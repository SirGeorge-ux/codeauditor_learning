package dictionary

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Test infrastructure --------------------------------------------------

// fakeService is a hand-rolled stand-in for the production dictionary
// service. It implements the addTermInput/getTermInput/listTermsInput/
// searchTermsInput method set the MCP tools need so tests can run without a
// database. Production wires *services.DictionaryService in via a thin
// adapter (see WithService below); the fake mirrors the contract.
type fakeService struct {
	mu sync.Mutex

	// In-memory store keyed by userID|lower-term.
	store map[string]storedTerm

	// Behavior knobs (set per-test).
	addErr    error
	getErr    error
	listErr   error
	searchErr error

	// Recorded inputs (last call).
	lastAddInput     addTermInput
	lastGetID        string
	lastListCategory string
	lastListTag      string
	lastSearchQuery  string
}

type storedTerm struct {
	id          string
	userID      string
	term        string
	explanation string
	category    string
	example     string
	synonyms    []string
	tags        []string
}

func newFakeService() *fakeService {
	return &fakeService{store: make(map[string]storedTerm)}
}

func (s *fakeService) Add(ctx context.Context, in addTermInput) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastAddInput = in
	if s.addErr != nil {
		return nil, s.addErr
	}
	key := in.UserID + "|" + strings.ToLower(in.Term)
	if _, exists := s.store[key]; exists {
		return nil, errConflictStub()
	}
	t := storedTerm{
		id:          "fake-" + in.Term,
		userID:      in.UserID,
		term:        strings.ToLower(in.Term),
		explanation: in.Explanation,
		category:    in.Category,
		synonyms:    in.Synonyms,
		tags:        in.Tags,
	}
	if in.Example != nil {
		t.example = *in.Example
	}
	s.store[key] = t
	raw, _ := json.Marshal(map[string]any{
		"id":          t.id,
		"term":        t.term,
		"category":    t.category,
		"explanation": t.explanation,
	})
	return raw, nil
}

func (s *fakeService) Get(ctx context.Context, userID, termID string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetID = termID
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, t := range s.store {
		if t.userID == userID && t.id == termID {
			raw, _ := json.Marshal(map[string]any{
				"id":          t.id,
				"term":        t.term,
				"category":    t.category,
				"explanation": t.explanation,
			})
			return raw, nil
		}
	}
	return nil, errNotFoundStub()
}

func (s *fakeService) List(ctx context.Context, userID, category, tag string) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastListCategory = category
	s.lastListTag = tag
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := []json.RawMessage{}
	for _, t := range s.store {
		if t.userID != userID {
			continue
		}
		if category != "" && t.category != category {
			continue
		}
		if tag != "" && !containsStringLocal(t.tags, tag) {
			continue
		}
		raw, _ := json.Marshal(map[string]any{"id": t.id, "term": t.term})
		out = append(out, raw)
	}
	return out, nil
}

func (s *fakeService) Search(ctx context.Context, userID, query string) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSearchQuery = query
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	q := strings.ToLower(query)
	out := []json.RawMessage{}
	for _, t := range s.store {
		if t.userID != userID {
			continue
		}
		haystack := strings.ToLower(t.term) + " " + strings.ToLower(t.explanation)
		for _, x := range t.synonyms {
			haystack += " " + strings.ToLower(x)
		}
		for _, x := range t.tags {
			haystack += " " + strings.ToLower(x)
		}
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}
		raw, _ := json.Marshal(map[string]any{"id": t.id, "term": t.term})
		out = append(out, raw)
	}
	return out, nil
}

func containsStringLocal(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// errConflictStub and errNotFoundStub mirror the production sentinels
// (services.ErrDictionaryValidation, ports.ErrTermConflict,
// ports.ErrTermNotFound). Returning the production sentinels lets
// errors.Is comparisons exercise the production wire shape end-to-end;
// importing ports just for this is cheap and the fake stays minimal.
func errConflictStub() error { return ports.ErrTermConflict }
func errNotFoundStub() error { return ports.ErrTermNotFound }

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-dictionary.search_term ---

func TestSearchTermTool_HappyPath_ReturnsResults(t *testing.T) {
	svc := newFakeService()
	svc.store["u|closure"] = storedTerm{
		id: "fake-closure", userID: "u", term: "closure", explanation: "captures",
	}

	tool := NewSearchTermTool(svc)
	if tool.Name() != "mcp-dictionary.search_term" {
		t.Errorf("name = %q, want mcp-dictionary.search_term", tool.Name())
	}

	args := json.RawMessage(`{"query":"closure"}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastSearchQuery != "closure" {
		t.Errorf("search routed query=%q, want closure", svc.lastSearchQuery)
	}
	arr, ok := got.([]json.RawMessage)
	if !ok {
		t.Fatalf("expected []json.RawMessage, got %T", got)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 result, got %d", len(arr))
	}
}

func TestSearchTermTool_EmptyQuery_FallsBackToList(t *testing.T) {
	svc := newFakeService()
	tool := NewSearchTermTool(svc)
	args := json.RawMessage(`{"query":""}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute(empty query): %v", err)
	}
	arr, ok := got.([]json.RawMessage)
	if !ok {
		t.Fatalf("expected []json.RawMessage, got %T", got)
	}
	if arr == nil {
		t.Errorf("empty list should be [] not null")
	}
}

func TestSearchTermTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewSearchTermTool(svc)
	// No user_id in context — tool MUST refuse.
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"closure"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-dictionary.get_term ---

func TestGetTermTool_HappyPath_ReturnsTerm(t *testing.T) {
	svc := newFakeService()
	svc.store["u|closure"] = storedTerm{
		id: "fake-closure", userID: "u", term: "closure", explanation: "captures",
	}
	tool := NewGetTermTool(svc)
	args := json.RawMessage(`{"id":"fake-closure"}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetID != "fake-closure" {
		t.Errorf("get routed id=%q, want fake-closure", svc.lastGetID)
	}
	raw, ok := got.(json.RawMessage)
	if !ok {
		t.Fatalf("expected json.RawMessage result, got %T", got)
	}
	if string(raw) == "" || string(raw) == "null" {
		t.Errorf("expected non-null JSON object, got %q", string(raw))
	}
}

func TestGetTermTool_NotFound_ReturnsNullObject(t *testing.T) {
	// Spec scenario: "MCP get_term returns null for missing ID".
	svc := newFakeService()
	tool := NewGetTermTool(svc)
	args := json.RawMessage(`{"id":"missing"}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute(missing id): %v", err)
	}
	if string(got.(json.RawMessage)) != "null" {
		t.Errorf("missing-id payload = %q, want null", string(got.(json.RawMessage)))
	}
}

func TestGetTermTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetTermTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"id":"x"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-dictionary.add_term ---

func TestAddTermTool_HappyPath_ReturnsCreated(t *testing.T) {
	svc := newFakeService()
	tool := NewAddTermTool(svc)
	args := json.RawMessage(`{
		"term":"hardcode",
		"explanation":"Writing literal values extracted.",
		"category":"Concepto",
		"example":"if(x==0) y=x;",
		"synonyms":["hardcodear"],
		"tags":["general"]
	}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastAddInput.UserID != "u" || svc.lastAddInput.Term != "hardcode" {
		t.Errorf("add routed userID=%q term=%q, want u/hardcode",
			svc.lastAddInput.UserID, svc.lastAddInput.Term)
	}
	raw := got.(json.RawMessage)
	if string(raw) == "" || string(raw) == "null" {
		t.Errorf("expected non-empty JSON object, got %q", string(raw))
	}
}

func TestAddTermTool_ShortExplanation_ReturnsInvalidParams(t *testing.T) {
	// Spec scenario: "MCP add_term validates input — too short explanation".
	svc := newFakeService()
	tool := NewAddTermTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"),
		json.RawMessage(`{"term":"x","explanation":"too short","category":"Concepto"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(short explanation) = %v, want ErrInvalidParams", err)
	}
}

func TestAddTermTool_Duplicate_ReturnsConflict(t *testing.T) {
	svc := newFakeService()
	svc.store["u|hardcode"] = storedTerm{userID: "u", term: "hardcode"}
	tool := NewAddTermTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"term":"hardcode",
		"explanation":"Trying again with another explanation here.",
		"category":"Concepto"
	}`))
	if err == nil {
		t.Fatal("expected duplicate-error, got nil")
	}
	if errors.Is(err, ErrInvalidParams) {
		t.Fatalf("duplicate must NOT be reported as invalid params: %v", err)
	}
	if !regexp.MustCompile(`(?i)already|duplicate|conflict`).MatchString(err.Error()) {
		t.Errorf("error message should mention duplicate, got %q", err.Error())
	}
}

func TestAddTermTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewAddTermTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"term":"x","explanation":"valid explanation here.","category":"Concepto"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-dictionary.list_terms ---

func TestListTermsTool_NoFilter_ReturnsAllForUser(t *testing.T) {
	svc := newFakeService()
	svc.store["u|async"] = storedTerm{id: "1", userID: "u", term: "async"}
	svc.store["u|bloc"] = storedTerm{id: "2", userID: "u", term: "bloc"}

	tool := NewListTermsTool(svc)
	args := json.RawMessage(`{}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arr := got.([]json.RawMessage)
	if len(arr) != 2 {
		t.Errorf("expected 2 results, got %d", len(arr))
	}
}

func TestListTermsTool_FilterByCategory(t *testing.T) {
	// Spec scenario: "MCP list_terms filters by category".
	svc := newFakeService()
	svc.store["u|hardcode"] = storedTerm{
		id: "1", userID: "u", term: "hardcode",
		category:    "Anti-patrón",
		explanation: "Smells here.",
	}
	svc.store["u|closure"] = storedTerm{
		id: "2", userID: "u", term: "closure",
		category:    "Concepto",
		explanation: "Captures scope.",
	}
	tool := NewListTermsTool(svc)
	args := json.RawMessage(`{"category":"Anti-patrón"}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastListCategory != "Anti-patrón" {
		t.Errorf("list routed category=%q, want Anti-patrón", svc.lastListCategory)
	}
	arr := got.([]json.RawMessage)
	if len(arr) != 1 {
		t.Errorf("expected only Anti-patrón entries (1); got %d", len(arr))
	}
}

func TestListTermsTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewListTermsTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"category":"Patrón"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- Group-wide smoke ---

func TestNewTools_NeverNil(t *testing.T) {
	svc := newFakeService()
	if NewSearchTermTool(svc) == nil {
		t.Errorf("NewSearchTermTool returns nil")
	}
	if NewGetTermTool(svc) == nil {
		t.Errorf("NewGetTermTool returns nil")
	}
	if NewAddTermTool(svc) == nil {
		t.Errorf("NewAddTermTool returns nil")
	}
	if NewListTermsTool(svc) == nil {
		t.Errorf("NewListTermsTool returns nil")
	}
}
