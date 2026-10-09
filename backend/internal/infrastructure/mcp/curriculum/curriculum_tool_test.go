package curriculum

// Tests for the four mcp-curriculum tools. Mirrors the dictionary_tool_test
// pattern: a hand-rolled fakeService stands in for
// *services.CurriculumService so the tool's JSON-parsing /
// user-id-bridging / sentinel-mapping contract is testable end-to-end
// without booting the filesystem reader.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// --- Test infrastructure --------------------------------------------------

// fakeService is the hand-rolled stand-in for *services.CurriculumService.
// It implements the Service interface exactly as production defines it.
type fakeService struct {
	mu sync.Mutex

	// Recorded inputs (last call).
	lastGetTopicsInput    services.GetTopicsInput
	lastGetTopicLang      string
	lastGetTopicID        string
	lastSuggestTopicInput services.SuggestTopicInput
	lastReviewTopicInput  services.ReviewTopicInput

	// Behavior knobs.
	getTopicsOut []services.Topic
	getTopicsErr error
	getTopicOut  services.TopicDetail
	getTopicErr  error
	suggestOut   services.Topic
	suggestErr   error
	reviewOut    services.ReviewTopicResult
	reviewErr    error
}

func newFakeService() *fakeService { return &fakeService{} }

func (s *fakeService) GetTopics(_ context.Context, in services.GetTopicsInput) ([]services.Topic, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetTopicsInput = in
	return s.getTopicsOut, s.getTopicsErr
}

func (s *fakeService) GetTopic(_ context.Context, language, topicID string) (services.TopicDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetTopicLang = language
	s.lastGetTopicID = topicID
	return s.getTopicOut, s.getTopicErr
}

func (s *fakeService) SuggestTopic(_ context.Context, in services.SuggestTopicInput) (services.Topic, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSuggestTopicInput = in
	return s.suggestOut, s.suggestErr
}

func (s *fakeService) ReviewTopic(_ context.Context, in services.ReviewTopicInput) (services.ReviewTopicResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastReviewTopicInput = in
	return s.reviewOut, s.reviewErr
}

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-curriculum.get_topics ---

func TestGetTopicsTool_HappyPath_ReturnsArray(t *testing.T) {
	svc := newFakeService()
	svc.getTopicsOut = []services.Topic{
		{ID: "variables", Title: "Variables", Difficulty: "F"},
		{ID: "types", Title: "Types", Difficulty: "E"},
	}
	tool := NewGetTopicsTool(svc)

	args := json.RawMessage(`{"language":"typescript"}`)
	got, err := tool.Execute(ctxWithUser(t, "u"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetTopicsInput.Language != "typescript" {
		t.Errorf("routed language=%q, want typescript", svc.lastGetTopicsInput.Language)
	}
	arr, ok := got.([]services.Topic)
	if !ok {
		t.Fatalf("expected []services.Topic, got %T", got)
	}
	if len(arr) != 2 {
		t.Errorf("len = %d, want 2", len(arr))
	}
}

func TestGetTopicsTool_PathTraversal_ReturnsInvalidParams(t *testing.T) {
	// Spec scenario: "Path Traversal in Curricula". The fake returns the
	// services-level sentinel (the production adapter translates it to
	// ErrInvalidParams at the boundary; the tool itself doesn't translate
	// because the production adapter does).
	svc := newFakeService()
	svc.getTopicsErr = services.ErrCurriculumInvalidLanguage
	tool := NewGetTopicsTool(svc)

	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"../etc"}`))
	if err == nil {
		t.Fatal("path traversal must be rejected")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams", err)
	}
}

func TestGetTopicsTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetTopicsTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"language":"typescript"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

func TestGetTopicsTool_EmptyLanguage_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetTopicsTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"   "}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("Execute(empty language) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-curriculum.get_topic ---

func TestGetTopicTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.getTopicOut = services.TopicDetail{
		ID:         "variables",
		Title:      "Variables",
		Difficulty: "F",
	}
	tool := NewGetTopicTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"typescript","topic_id":"variables"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetTopicLang != "typescript" {
		t.Errorf("routed language=%q, want typescript", svc.lastGetTopicLang)
	}
	if svc.lastGetTopicID != "variables" {
		t.Errorf("routed topicID=%q, want variables", svc.lastGetTopicID)
	}
	detail, ok := got.(services.TopicDetail)
	if !ok {
		t.Fatalf("expected services.TopicDetail, got %T", got)
	}
	if detail.ID != "variables" {
		t.Errorf("id = %q, want variables", detail.ID)
	}
}

func TestGetTopicTool_PathTraversal_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.getTopicErr = services.ErrCurriculumInvalidLanguage
	tool := NewGetTopicTool(svc)

	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"typescript","topic_id":"../etc"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("path traversal topic_id must surface as ErrInvalidParams, got %v", err)
	}
}

func TestGetTopicTool_MissingFields_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetTopicTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"typescript"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing topic_id must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-curriculum.suggest_topic ---

func TestSuggestTopicTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.suggestOut = services.Topic{
		ID: "types", Title: "Types", Difficulty: "E",
	}
	tool := NewSuggestTopicTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language":"typescript","level":"E","goal":"variables"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastSuggestTopicInput.Level != "E" {
		t.Errorf("routed level=%q, want E", svc.lastSuggestTopicInput.Level)
	}
	if svc.lastSuggestTopicInput.Goal != "variables" {
		t.Errorf("routed goal=%q, want variables", svc.lastSuggestTopicInput.Goal)
	}
	topic, ok := got.(services.Topic)
	if !ok {
		t.Fatalf("expected services.Topic, got %T", got)
	}
	if topic.ID != "types" {
		t.Errorf("topic.ID = %q, want types", topic.ID)
	}
}

func TestSuggestTopicTool_EmptyMatch_ReturnsEmptyObject(t *testing.T) {
	// Service returns the zero Topic (no match); the tool must surface
	// {} (not the zero-domain value) so the LLM can branch cleanly.
	svc := newFakeService()
	svc.suggestOut = services.Topic{} // ID == "" → "no match"
	tool := NewSuggestTopicTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"typescript","level":"E"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any for empty match, got %T", got)
	}
	if len(m) != 0 {
		t.Errorf("empty match should be {}, got %+v", m)
	}
}

func TestSuggestTopicTool_PathTraversal_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.suggestErr = services.ErrCurriculumInvalidLanguage
	tool := NewSuggestTopicTool(svc)

	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"../etc","level":"F"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("traversal must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-curriculum.review_topic ---

func TestReviewTopicTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.reviewOut = services.ReviewTopicResult{Reviewed: true}
	tool := NewReviewTopicTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language":"typescript","topic_id":"variables","rating":4,"notes":"felt solid"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastReviewTopicInput.Rating != 4 {
		t.Errorf("routed rating=%d, want 4", svc.lastReviewTopicInput.Rating)
	}
	if svc.lastReviewTopicInput.Notes != "felt solid" {
		t.Errorf("routed notes=%q, want 'felt solid'", svc.lastReviewTopicInput.Notes)
	}
	out, ok := got.(services.ReviewTopicResult)
	if !ok {
		t.Fatalf("expected services.ReviewTopicResult, got %T", got)
	}
	if !out.Reviewed {
		t.Errorf("Reviewed = false, want true")
	}
}

func TestReviewTopicTool_PathTraversal_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.reviewErr = services.ErrCurriculumInvalidLanguage
	tool := NewReviewTopicTool(svc)

	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language":"typescript","topic_id":"../variables","rating":3
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("traversal must surface as ErrInvalidParams, got %v", err)
	}
}

func TestReviewTopicTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewReviewTopicTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"language":"typescript","topic_id":"variables","rating":3
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing user_id must surface as ErrInvalidParams, got %v", err)
	}
}

// --- Service adapter wiring (production end) ---

func TestNewServiceAdapter_PropagatesValidationSentinel(t *testing.T) {
	// The production adapter is the wiring path main.go uses. Build a
	// real *services.CurriculumService rooted at an empty dir so its
	// internal guard rejects. Assert the adapter surfaces the
	// path-traversal as ErrInvalidParams — that's the MCP boundary the
	// dispatcher maps to -32602.
	dir := t.TempDir()
	svc := services.NewCurriculumService(services.WithBaseDir(dir))
	adapter := NewServiceAdapter(svc)

	_, err := adapter.GetTopics(context.Background(), services.GetTopicsInput{Language: "../etc"})
	if err == nil {
		t.Fatal("production adapter accepted ../etc; want ErrInvalidParams")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams", err)
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("err message should mention 'invalid'; got %q", err.Error())
	}
}
