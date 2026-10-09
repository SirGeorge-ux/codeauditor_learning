package services

// Tests for ExternalDocsService. The service depends on
// ports.DocsClient + ports.WebSearchClient; the tests use hand-rolled
// fakes so the fusion logic (Context7-first, Brave-fallback) is
// exercised end-to-end without an HTTP roundtrip.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- fakes -----------------------------------------------------------------

type fakeDocs struct {
	mu sync.Mutex
	// behavior knobs
	getLibraryOut string
	getLibraryErr error
	searchOut     []ports.DocsHit
	searchErr     error
	// recorded inputs
	lastGetLibraryLib   string
	lastGetLibraryTopic string
	lastSearchQuery     string
}

func (f *fakeDocs) GetLibraryDocs(_ context.Context, library, topic string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGetLibraryLib = library
	f.lastGetLibraryTopic = topic
	return f.getLibraryOut, f.getLibraryErr
}

func (f *fakeDocs) SearchDocumentation(_ context.Context, query string) ([]ports.DocsHit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSearchQuery = query
	if f.searchOut == nil {
		return []ports.DocsHit{}, nil
	}
	return f.searchOut, f.searchErr
}

type fakeWeb struct {
	mu        sync.Mutex
	searchOut []ports.WebHit
	searchErr error
	lastQuery string
}

func (f *fakeWeb) SearchQuery(_ context.Context, query string) ([]ports.WebHit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastQuery = query
	out := f.searchOut
	if out == nil {
		out = []ports.WebHit{}
	}
	return out, f.searchErr
}

// --- GetLibraryDocs --------------------------------------------------------

func TestExternalDocs_GetLibraryDocs_RequiresLibrary(t *testing.T) {
	svc := NewExternalDocsService(&fakeDocs{getLibraryOut: "x"}, &fakeWeb{})
	_, err := svc.GetLibraryDocs(context.Background(), LibraryDocsInput{Library: "  ", Topic: "hooks"})
	if err == nil {
		t.Fatal("missing library must surface an error")
	}
	if !errors.Is(err, ErrCurriculumInvalidLanguage) {
		t.Errorf("err = %v, want ErrCurriculumInvalidLanguage", err)
	}
}

func TestExternalDocs_GetLibraryDocs_NilDocsClient(t *testing.T) {
	svc := NewExternalDocsService(nil, nil)
	_, err := svc.GetLibraryDocs(context.Background(), LibraryDocsInput{Library: "react"})
	if err == nil {
		t.Fatal("nil docs client must surface an error")
	}
}

func TestExternalDocs_GetLibraryDocs_HappyPath(t *testing.T) {
	docs := &fakeDocs{getLibraryOut: "react documentation body"}
	svc := NewExternalDocsService(docs, &fakeWeb{})
	out, err := svc.GetLibraryDocs(context.Background(), LibraryDocsInput{Library: "react", Topic: "hooks"})
	if err != nil {
		t.Fatalf("GetLibraryDocs: %v", err)
	}
	if out.Library != "react" {
		t.Errorf("Library = %q, want react", out.Library)
	}
	if out.Topic != "hooks" {
		t.Errorf("Topic = %q, want hooks", out.Topic)
	}
	if out.Content != "react documentation body" {
		t.Errorf("Content = %q, want %q", out.Content, "react documentation body")
	}
	if out.Source != "context7" {
		t.Errorf("Source = %q, want context7", out.Source)
	}
	if docs.lastGetLibraryLib != "react" {
		t.Errorf("lastGetLibraryLib = %q, want react", docs.lastGetLibraryLib)
	}
}

func TestExternalDocs_GetLibraryDocs_PropagatesUpstreamError(t *testing.T) {
	docs := &fakeDocs{getLibraryErr: errors.New("upstream 503")}
	svc := NewExternalDocsService(docs, &fakeWeb{})
	_, err := svc.GetLibraryDocs(context.Background(), LibraryDocsInput{Library: "react"})
	if err == nil {
		t.Fatal("upstream error must surface")
	}
	if !strings.Contains(err.Error(), "upstream 503") {
		t.Errorf("err = %v, want 'upstream 503' in the chain", err)
	}
}

// --- SearchDocumentation ---------------------------------------------------

func TestExternalDocs_SearchDocumentation_EmptyQueryReturnsEmpty(t *testing.T) {
	web := &fakeWeb{}
	svc := NewExternalDocsService(&fakeDocs{}, web)
	hits, err := svc.SearchDocumentation(context.Background(), SearchDocsInput{Query: "   "})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if hits == nil {
		t.Error("hits must be a non-nil empty slice")
	}
	if len(hits) != 0 {
		t.Errorf("len(hits) = %d, want 0", len(hits))
	}
}

func TestExternalDocs_SearchDocumentation_NilWebClient(t *testing.T) {
	svc := NewExternalDocsService(&fakeDocs{}, nil)
	_, err := svc.SearchDocumentation(context.Background(), SearchDocsInput{Query: "react"})
	if err == nil {
		t.Fatal("nil web client must surface an error")
	}
}

func TestExternalDocs_SearchDocumentation_HappyPath(t *testing.T) {
	web := &fakeWeb{
		searchOut: []ports.WebHit{
			{Title: "React docs", URL: "https://react.dev", Snippet: "the official site"},
			{Title: "Hooks guide", URL: "https://example/h"},
		},
	}
	svc := NewExternalDocsService(&fakeDocs{}, web)
	hits, err := svc.SearchDocumentation(context.Background(), SearchDocsInput{Query: "react hooks"})
	if err != nil {
		t.Fatalf("SearchDocumentation: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(hits))
	}
	if hits[0].Title != "React docs" {
		t.Errorf("hits[0].Title = %q, want React docs", hits[0].Title)
	}
	if hits[0].URL != "https://react.dev" {
		t.Errorf("hits[0].URL = %q, want https://react.dev", hits[0].URL)
	}
	if hits[0].Snippet != "the official site" {
		t.Errorf("hits[0].Snippet = %q, want 'the official site'", hits[0].Snippet)
	}
	if web.lastQuery != "react hooks" {
		t.Errorf("web.lastQuery = %q, want 'react hooks'", web.lastQuery)
	}
}

// --- GetRealExample --------------------------------------------------------

func TestExternalDocs_GetRealExample_RequiresLibrary(t *testing.T) {
	svc := NewExternalDocsService(&fakeDocs{}, &fakeWeb{})
	_, err := svc.GetRealExample(context.Background(), RealExampleInput{Library: "  ", Topic: "x"})
	if err == nil {
		t.Fatal("missing library must surface an error")
	}
	if !errors.Is(err, ErrCurriculumInvalidLanguage) {
		t.Errorf("err = %v, want ErrCurriculumInvalidLanguage", err)
	}
}

func TestExternalDocs_GetRealExample_PrefersContext7(t *testing.T) {
	docs := &fakeDocs{
		getLibraryOut: "Here's a snippet:\n\n```ts\nconst x: number = 1;\n```\n\nMore text.",
	}
	web := &fakeWeb{
		searchOut: []ports.WebHit{
			{Title: "fallback", URL: "https://example", Snippet: "fallback snippet"},
		},
	}
	svc := NewExternalDocsService(docs, web)
	out, err := svc.GetRealExample(context.Background(), RealExampleInput{
		Library: "react", Topic: "hooks", Language: "typescript",
	})
	if err != nil {
		t.Fatalf("GetRealExample: %v", err)
	}
	if out.Source != "context7" {
		t.Errorf("Source = %q, want context7 (preferred)", out.Source)
	}
	if !strings.Contains(out.Code, "const x: number") {
		t.Errorf("Code = %q, want it to include the fenced snippet", out.Code)
	}
}

func TestExternalDocs_GetRealExample_FallsBackToBraveWhenContext7Empty(t *testing.T) {
	docs := &fakeDocs{getLibraryOut: ""} // empty body → fusion must drop to Brave
	web := &fakeWeb{
		searchOut: []ports.WebHit{
			{Title: "example", URL: "https://example/react-hook", Snippet: "function useFoo() {...}"},
		},
	}
	svc := NewExternalDocsService(docs, web)
	out, err := svc.GetRealExample(context.Background(), RealExampleInput{
		Library: "react", Topic: "useFoo", Language: "ts",
	})
	if err != nil {
		t.Fatalf("GetRealExample: %v", err)
	}
	if !strings.HasPrefix(out.Source, "brave:") {
		t.Errorf("Source = %q, want brave: prefix (fallback)", out.Source)
	}
	if out.Code != "function useFoo() {...}" {
		t.Errorf("Code = %q, want Brave snippet", out.Code)
	}
	if web.lastQuery != "react useFoo example ts" {
		t.Errorf("web.lastQuery = %q, want 'react useFoo example ts'", web.lastQuery)
	}
}

func TestExternalDocs_GetRealExample_NoProviderResults(t *testing.T) {
	docs := &fakeDocs{getLibraryOut: ""}
	web := &fakeWeb{} // no hits
	svc := NewExternalDocsService(docs, web)
	_, err := svc.GetRealExample(context.Background(), RealExampleInput{Library: "react"})
	if err == nil {
		t.Fatal("no provider results must surface an error")
	}
}

func TestExternalDocs_GetRealExample_Context7ErrorFallsBackToBrave(t *testing.T) {
	docs := &fakeDocs{getLibraryErr: errors.New("upstream 500")}
	web := &fakeWeb{
		searchOut: []ports.WebHit{
			{Title: "fallback", URL: "https://example/x", Snippet: "snip"},
		},
	}
	svc := NewExternalDocsService(docs, web)
	out, err := svc.GetRealExample(context.Background(), RealExampleInput{Library: "react"})
	if err != nil {
		t.Fatalf("GetRealExample: %v (Context7 errors should fall through to Brave)", err)
	}
	if !strings.HasPrefix(out.Source, "brave:") {
		t.Errorf("Source = %q, want brave: (fallback after Context7 error)", out.Source)
	}
}

func TestExternalDocs_GetRealExample_OnlyDocsConfigured_NoFallback(t *testing.T) {
	// web=nil: a working Context7 must succeed.
	docs := &fakeDocs{getLibraryOut: "```go\npackage main\n```"}
	svc := NewExternalDocsService(docs, nil)
	out, err := svc.GetRealExample(context.Background(), RealExampleInput{Library: "go-stdlib"})
	if err != nil {
		t.Fatalf("GetRealExample: %v", err)
	}
	if out.Source != "context7" {
		t.Errorf("Source = %q, want context7", out.Source)
	}
	if !strings.Contains(out.Code, "package main") {
		t.Errorf("Code = %q, want fenced Go snippet", out.Code)
	}
}

// --- firstCodeFence helper -------------------------------------------------

func TestFirstCodeFence_EmptyReturnsEmpty(t *testing.T) {
	if got := firstCodeFence(""); got != "" {
		t.Errorf("firstCodeFence('') = %q, want ''", got)
	}
}

func TestFirstCodeFence_NoFenceReturnsBody(t *testing.T) {
	in := "no fence here"
	if got := firstCodeFence(in); got != in {
		t.Errorf("firstCodeFence(%q) = %q, want %q", in, got, in)
	}
}

func TestFirstCodeFence_StripsLanguageTag(t *testing.T) {
	in := "before\n```ts\nconst x = 1;\n```\nafter"
	got := firstCodeFence(in)
	if !strings.Contains(got, "const x = 1;") {
		t.Errorf("firstCodeFence = %q, want it to include the body", got)
	}
	if strings.Contains(got, "```") {
		t.Errorf("firstCodeFence = %q, fences must be stripped", got)
	}
}

func TestFirstCodeFence_UnclosedFenceReturnsRemaining(t *testing.T) {
	in := "before\n```ts\nconst x = 1;\n(no close)"
	got := firstCodeFence(in)
	if !strings.Contains(got, "const x = 1;") {
		t.Errorf("firstCodeFence = %q, want it to include the body", got)
	}
}
