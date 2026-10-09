package brave

// Tests for the Brave Search adapter. The PR 4 implementation uses
// net/http against the upstream, so each test stands up an
// httptest.NewServer and points the adapter at it. This keeps the JSON
// wiring + subscription-token-header + status-code branches honest
// without leaving the test process to talk to a real HTTP endpoint.

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
	c := NewClient("https://api.search.brave.com/", "k")
	if c.baseURL != "https://api.search.brave.com" {
		t.Errorf("trailing slash must be trimmed, got %q", c.baseURL)
	}
}

func TestSearchQuery_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"web": {
				"results": [
					{"title": "React docs", "url": "https://react.dev", "description": "the official site"},
					{"title": "React hooks", "url": "https://example/h"}
				]
			}
		}`)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "key", srv.Client())
	hits, err := c.SearchQuery(context.Background(), "react hooks")
	if err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(hits))
	}
	if hits[0].Title != "React docs" {
		t.Errorf("hits[0].Title = %q, want %q", hits[0].Title, "React docs")
	}
	if hits[0].Snippet != "the official site" {
		t.Errorf("hits[0].Snippet = %q, want %q", hits[0].Snippet, "the official site")
	}
}

func TestSearchQuery_EmptyQueryReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("upstream must not be hit for empty query")
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchQuery(context.Background(), "   ")
	if err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if hits == nil {
		t.Error("empty-result must be []WebHit{}, not nil")
	}
	if len(hits) != 0 {
		t.Errorf("len(hits) = %d, want 0", len(hits))
	}
}

func TestSearchQuery_NoResultsReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"web":{"results":[]}}`)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchQuery(context.Background(), "x")
	if err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if hits == nil || len(hits) != 0 {
		t.Errorf("want empty slice, got %v", hits)
	}
}

func TestSearchQuery_ClampsAtMaxSearchHits(t *testing.T) {
	// Build a payload with MaxSearchHits+5 result entries; the loop
	// inside the adapter must stop at MaxSearchHits.
	var b strings.Builder
	b.WriteString(`{"web":{"results":[`)
	for i := 0; i < MaxSearchHits+5; i++ {
		if i > 0 {
			b.WriteRune(',')
		}
		b.WriteString(`{"title":"t","url":"u"}`)
	}
	b.WriteString(`]}}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, b.String())
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	hits, err := c.SearchQuery(context.Background(), "x")
	if err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if len(hits) != MaxSearchHits {
		t.Errorf("len(hits) = %d, want %d (clamped)", len(hits), MaxSearchHits)
	}
}

func TestSearchQuery_Non2xxSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	_, err := c.SearchQuery(context.Background(), "react")
	if err == nil {
		t.Fatal("rate-limited response must surface as error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %v, want it to mention status 429", err)
	}
}

func TestSearchQuery_Non2xxStatusIsSanitized(t *testing.T) {
	// The error message must NOT leak the API key in any branch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("X-Subscription-Token: SECRET_KEY_VALUE"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "SECRET_KEY_VALUE", srv.Client())
	_, err := c.SearchQuery(context.Background(), "react")
	if err == nil {
		t.Fatal("upstream 500 must surface as error")
	}
	if strings.Contains(err.Error(), "SECRET_KEY_VALUE") {
		t.Errorf("error leaked API key: %v", err)
	}
}

func TestSearchQuery_SendsSubscriptionTokenHeader(t *testing.T) {
	var gotToken atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken.Store(r.Header.Get("X-Subscription-Token"))
		_, _ = io.WriteString(w, `{"web":{"results":[]}}`)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "my-key", srv.Client())
	if _, err := c.SearchQuery(context.Background(), "x"); err != nil {
		t.Fatalf("SearchQuery: %v", err)
	}
	if got := gotToken.Load(); got != "my-key" {
		t.Errorf("X-Subscription-Token = %v, want my-key", got)
	}
}

func TestSearchQuery_MissingAPIKeyFailsFast(t *testing.T) {
	// No httptest server is started: a missing-key call MUST NOT
	// even attempt HTTP.
	c := NewClient("https://example.invalid", "")
	_, err := c.SearchQuery(context.Background(), "react")
	if err == nil {
		t.Fatal("missing API key must surface as error")
	}
	if !strings.Contains(err.Error(), "api key is required") {
		t.Errorf("err = %v, want 'api key is required' message", err)
	}
}

func TestSearchQuery_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.URL, "k", srv.Client())
	_, err := c.SearchQuery(context.Background(), "react")
	if err == nil {
		t.Fatal("malformed upstream JSON must surface as error")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("err = %v, want decode-step mention", err)
	}
}
