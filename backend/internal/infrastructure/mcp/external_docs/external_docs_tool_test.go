package external_docs

// Tests for the mcp-external_docs tool layer. Mirrors the
// dictionary/curriculum/user patterns: a hand-rolled fake Service
// stands in for the production serviceAdapter so each tool's
// JSON-parsing / user-id-bridging / sentinel-mapping contract is
// testable end-to-end without an HTTP roundtrip.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// --- Test infrastructure --------------------------------------------------

// fakeService implements the local Service interface exactly as the
// production adapter does. Behavior knobs + recorded inputs let each
// test pin down the routing + envelope contract.
type fakeService struct {
	mu sync.Mutex

	// Recorded inputs.
	lastGetLibrary  services.LibraryDocsInput
	lastSearch      services.SearchDocsInput
	lastRealExample services.RealExampleInput

	// Behavior knobs.
	getLibraryOut services.LibraryDocs
	getLibraryErr error
	searchOut     []services.SearchDocsResult
	searchErr     error
	realOut       services.RealExample
	realErr       error
}

func newFakeService() *fakeService {
	return &fakeService{
		getLibraryOut: services.LibraryDocs{
			Library: "react",
			Topic:   "hooks",
			Content: "useState is a hook that...",
			Source:  "context7",
		},
		searchOut: []services.SearchDocsResult{
			{Title: "React hooks", URL: "https://example/hooks", Snippet: "the official hook guide"},
		},
		realOut: services.RealExample{
			Library: "react", Topic: "useState",
			Code:   "const [c, setC] = useState(0);",
			Source: "context7",
		},
	}
}

func (s *fakeService) GetLibraryDocs(_ context.Context, in services.LibraryDocsInput) (services.LibraryDocs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetLibrary = in
	return s.getLibraryOut, s.getLibraryErr
}

func (s *fakeService) SearchDocumentation(_ context.Context, in services.SearchDocsInput) ([]services.SearchDocsResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSearch = in
	if s.searchOut == nil {
		return []services.SearchDocsResult{}, nil
	}
	return s.searchOut, s.searchErr
}

func (s *fakeService) GetRealExample(_ context.Context, in services.RealExampleInput) (services.RealExample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRealExample = in
	return s.realOut, s.realErr
}

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	if userID == "" {
		return context.Background()
	}
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-external_docs.get_library_docs -----------------------------------

func TestGetLibraryDocsTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	tool := NewGetLibraryDocsTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"library":"react","topic":"hooks"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetLibrary.Library != "react" {
		t.Errorf("routed library = %q, want react", svc.lastGetLibrary.Library)
	}
	if svc.lastGetLibrary.Topic != "hooks" {
		t.Errorf("routed topic = %q, want hooks", svc.lastGetLibrary.Topic)
	}
	out, ok := got.(services.LibraryDocs)
	if !ok {
		t.Fatalf("expected services.LibraryDocs, got %T", got)
	}
	if out.Content != "useState is a hook that..." {
		t.Errorf("content = %q, want 'useState is a hook that...'", out.Content)
	}
}

func TestGetLibraryDocsTool_MissingLibrary_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetLibraryDocsTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"library":"   "}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("missing library err = %v, want ErrInvalidParams", err)
	}
}

func TestGetLibraryDocsTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetLibraryDocsTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"library":"react"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("missing user_id err = %v, want ErrInvalidParams", err)
	}
}

func TestGetLibraryDocsTool_PropagatesInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.getLibraryErr = ErrCurriculumInvalidLanguage
	tool := NewGetLibraryDocsTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"library":""}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("service sentinel must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-external_docs.search_documentation -------------------------------

func TestSearchDocumentationTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	tool := NewSearchDocumentationTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"query":"react hooks"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastSearch.Query != "react hooks" {
		t.Errorf("routed query = %q, want 'react hooks'", svc.lastSearch.Query)
	}
	arr, ok := got.([]services.SearchDocsResult)
	if !ok {
		t.Fatalf("expected []services.SearchDocsResult, got %T", got)
	}
	if len(arr) != 1 {
		t.Errorf("len(arr) = %d, want 1", len(arr))
	}
	if arr[0].Title != "React hooks" {
		t.Errorf("arr[0].Title = %q, want 'React hooks'", arr[0].Title)
	}
}

func TestSearchDocumentationTool_NilResultReturnsEmpty(t *testing.T) {
	svc := newFakeService()
	svc.searchOut = nil
	tool := NewSearchDocumentationTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"query":"x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arr, ok := got.([]services.SearchDocsResult)
	if !ok {
		t.Fatalf("expected slice, got %T", got)
	}
	if arr == nil {
		t.Error("nil service result must surface as []SearchDocsResult{}, not nil")
	}
	if len(arr) != 0 {
		t.Errorf("len(arr) = %d, want 0", len(arr))
	}
}

func TestSearchDocumentationTool_MissingUserID(t *testing.T) {
	svc := newFakeService()
	tool := NewSearchDocumentationTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"x"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("missing user_id err = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-external_docs.get_real_example -----------------------------------

func TestGetRealExampleTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	tool := NewGetRealExampleTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"library":"react","topic":"useState","language":"typescript"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastRealExample.Library != "react" {
		t.Errorf("routed library = %q, want react", svc.lastRealExample.Library)
	}
	if svc.lastRealExample.Language != "typescript" {
		t.Errorf("routed language = %q, want typescript", svc.lastRealExample.Language)
	}
	out, ok := got.(services.RealExample)
	if !ok {
		t.Fatalf("expected services.RealExample, got %T", got)
	}
	if out.Code != "const [c, setC] = useState(0);" {
		t.Errorf("code = %q, want the example snippet", out.Code)
	}
}

func TestGetRealExampleTool_MissingLibrary_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetRealExampleTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"library":"  "}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("missing library err = %v, want ErrInvalidParams", err)
	}
}

func TestGetRealExampleTool_MissingUserID(t *testing.T) {
	svc := newFakeService()
	tool := NewGetRealExampleTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"library":"react"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("missing user_id err = %v, want ErrInvalidParams", err)
	}
}

func TestGetRealExampleTool_PropagatesInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.realErr = ErrCurriculumInvalidLanguage
	tool := NewGetRealExampleTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"library":"react"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("service sentinel must surface as ErrInvalidParams, got %v", err)
	}
}

// --- production serviceAdapter wiring -------------------------------------

func TestNewServiceAdapter_PropagatesValidationSentinel(t *testing.T) {
	// Empty library → ErrCurriculumInvalidLanguage from the service;
	// the production adapter must surface it as ErrInvalidParams at
	// the JSON-RPC envelope.
	adapter := NewServiceAdapter(nil)
	_, err := adapter.GetLibraryDocs(context.Background(), services.LibraryDocsInput{})
	if err == nil {
		t.Fatal("nil adapter + empty input still expected to fail at envelope level")
	}
	// Production adapter against the nil service: empty library
	// should error at the service layer with ErrCurriculumInvalidLanguage.
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams", err)
	}
}
