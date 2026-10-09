package context7

// Tests for the Context7 documentation adapter. The PR 4 implementation
// uses net/http against the upstream, so each test stands up an
// httptest.NewServer and points the adapter at it. This keeps the
// JSON wiring + auth-header + status-code branches honest without
// leaving the test process to talk to a real HTTP endpoint.
//
// Hexagonal purity: the tests live here (next to the adapter) because
// they exercise adapter-internal details (wire shape, header behavior).
// Ports-facing tests belong in core/services/external_docs_service_test.go
// (PR 4 commit 6).

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewClient_DefaultEndpoint(t *testing.T) {
	c := NewClient("", "key")
	if c.baseURL != Endpoint {
		t.Errorf("baseURL = %q, want %q", c.baseURL, Endpoint)
	}
	if c.apiKey != "key" {
		t.Errorf("apiKey = %q, want key", c.apiKey)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient must be initialized")
	}
	if c.httpClient.Timeout == 0 {
		t.Error("httpClient.Timeout should be non-zero")
	}
}

func TestNewClient_TrailingSlashTrimmed(t *testing.T) {
	c := NewClient("https://context7.com/api/", "k")
	if c.baseURL != "https://context7.com/api" {
		t.Errorf("trailing slash must be trimmed, got %q", c.baseURL)
	}
}

func TestGetLibraryDocs_HappyPath(t *testing.T) {
	var capturedPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath.Store(r.URL.Path)
		_, _ = w.Write([]byte("react documentation body"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "test-key", srv.Client())

	body, err := c.GetLibraryDocs(context.Background(), "react", "")
	if err != nil {
		t.Fatalf("GetLibraryDocs: %v", err)
	}
	if body != "react documentation body" {
		t.Errorf("body = %q, want %q", body, "react documentation body")
	}
	if got := capturedPath.Load(); got != "/react" {
		t.Errorf("path = %v, want /react", got)
	}
}

func TestGetLibraryDocs_WithTopic(t *testing.T) {
	var captured atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Store(r.URL.Path)
		_, _ = w.Write([]byte("hooks body"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	body, err := c.GetLibraryDocs(context.Background(), "react", "useState")
	if err != nil {
		t.Fatalf("GetLibraryDocs: %v", err)
	}
	if body != "hooks body" {
		t.Errorf("body = %q, want %q", body, "hooks body")
	}
	if got := captured.Load(); got != "/react/topic/useState" {
		t.Errorf("path = %v, want /react/topic/useState", got)
	}
}

func TestGetLibraryDocs_EmptyLibrary_FailsFast(t *testing.T) {
	c := NewClient("https://example.invalid", "k")
	if _, err := c.GetLibraryDocs(context.Background(), "  ", ""); err == nil {
		t.Fatal("empty library must fail")
	}
}

func TestGetLibraryDocs_Non2xxSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("library not found"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	_, err := c.GetLibraryDocs(context.Background(), "nope", "")
	if err == nil {
		t.Fatal("upstream 404 must surface as error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v, want it to mention status 404", err)
	}
}

func TestGetLibraryDocs_Non2xxStatusIsSanitized(t *testing.T) {
	// The error message must NOT leak the API key in any branch. The
	// dispatcher also redacts, but adapter-side defense matters for
	// callers that bypass the dispatcher.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Authorization: Bearer SECRET_KEY_VALUE"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "SECRET_KEY_VALUE", srv.Client())
	_, err := c.GetLibraryDocs(context.Background(), "react", "")
	if err == nil {
		t.Fatal("upstream 500 must surface as error")
	}
	if strings.Contains(err.Error(), "SECRET_KEY_VALUE") {
		t.Errorf("error leaked API key: %v", err)
	}
}

func TestGetLibraryDocs_SendsAuthHeader(t *testing.T) {
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "my-key", srv.Client())
	if _, err := c.GetLibraryDocs(context.Background(), "react", ""); err != nil {
		t.Fatalf("GetLibraryDocs: %v", err)
	}
	if got := gotAuth.Load(); got != "Bearer my-key" {
		t.Errorf("Authorization = %v, want 'Bearer my-key'", got)
	}
}

func TestGetLibraryDocs_OmitsAuthWhenNoKey(t *testing.T) {
	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "", srv.Client())
	if _, err := c.GetLibraryDocs(context.Background(), "react", ""); err != nil {
		t.Fatalf("GetLibraryDocs: %v", err)
	}
	if got := gotAuth.Load(); got != "" {
		t.Errorf("Authorization must be empty when key unset, got %v", got)
	}
}

func TestSearchDocumentation_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"results": [
				{"title": "React hooks intro", "url": "https://example/hooks", "snippet": "useState is..."},
				{"title": "useEffect pattern", "url": "https://example/effect"}
			]
		}`)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchDocumentation(context.Background(), "react hooks")
	if err != nil {
		t.Fatalf("SearchDocumentation: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(hits))
	}
	if hits[0].Title != "React hooks intro" {
		t.Errorf("hits[0].Title = %q, want %q", hits[0].Title, "React hooks intro")
	}
	if hits[0].Snippet != "useState is..." {
		t.Errorf("hits[0].Snippet = %q, want useState is...", hits[0].Snippet)
	}
}

func TestSearchDocumentation_EmptyQueryReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("upstream must not be hit for empty query")
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchDocumentation(context.Background(), "   ")
	if err != nil {
		t.Fatalf("SearchDocumentation: %v", err)
	}
	if hits == nil {
		t.Error("empty-result must be []DocsHit{}, not nil")
	}
	if len(hits) != 0 {
		t.Errorf("len(hits) = %d, want 0", len(hits))
	}
}

func TestSearchDocumentation_ClampsAtMaxSearchHits(t *testing.T) {
	many := make([]map[string]any, 0, MaxSearchHits+5)
	for i := 0; i < MaxSearchHits+5; i++ {
		many = append(many, map[string]any{
			"title": "t",
			"url":   "u",
		})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results": [`)
		for i, m := range many {
			if i > 0 {
				_, _ = io.WriteString(w, ",")
			}
			_, _ = io.WriteString(w, `{"title":"`+m["title"].(string)+`","url":"`+m["url"].(string)+`"}`)
		}
		_, _ = io.WriteString(w, `]}`)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchDocumentation(context.Background(), "x")
	if err != nil {
		t.Fatalf("SearchDocumentation: %v", err)
	}
	if len(hits) != MaxSearchHits {
		t.Errorf("len(hits) = %d, want %d (clamped)", len(hits), MaxSearchHits)
	}
}

func TestSearchDocumentation_Non2xxSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	_, err := c.SearchDocumentation(context.Background(), "react")
	if err == nil {
		t.Fatal("rate-limited response must surface as error")
	}
}

// The package-level compile-time guard
//
//	var _ ports.DocsClient = (*Client)(nil)
//
// in adapter.go is the canonical port-adapter invariant. It is
// enforced by `go build`, not by this test file; any drift between the
// adapter and the port surfaces at build time. We do not duplicate the
// check here because `go test` already runs `go build` first.
