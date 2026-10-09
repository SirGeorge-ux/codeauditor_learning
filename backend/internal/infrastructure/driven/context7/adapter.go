// Package context7 is a driven adapter for the Context7 documentation
// API.
//
// Hexagonal contract: it implements the docs port (see
// internal/ports/external_docs.go) and is the ONLY place that imports
// Context7 HTTP concerns. core/services/ExternalDocsService depends on
// the port, never on this package directly.
//
// Wire shape
// -----------
// Context7 exposes:
//
//	GET {base}/search?query=<q>             → { "results": [ { title, url, snippet } ] }
//	GET {base}/{library}                    → text (library overview)
//	GET {base}/{library}/topic/{topic}      → text (scoped excerpt)
//
// The base URL defaults to Endpoint (https://context7.com/api) and is
// overridable per environment. The API key is sent via the
// `Authorization: Bearer <key>` header. The key NEVER leaves the
// backend; PR 5's MCPClientAdapter only sees the rendered text.
//
// PR 1 shipped a compilable stub. PR 4 fleshes the adapter out with a
// pure net/http client (no third-party SDK) and a unit test against
// httptest.NewServer so the JSON wiring is exercised without leaving
// the host's network namespace.
package context7

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Endpoint is the Context7 base URL (overridable per environment).
const Endpoint = "https://context7.com/api"

// Default timeout for any Context7 HTTP call. The server-side Context7
// API is documented to respond in <2s for cached libraries; 10s gives
// the upstream room to recover from cache misses.
const defaultRequestTimeout = 10 * time.Second

// MaxSearchHits caps a single SearchDocumentation response. Brave
// defaults to 20; Context7 has no documented cap, so we clamp at 20 to
// keep the tool's tool-result envelope under the 50KB dispatcher cap.
const MaxSearchHits = 20

// Client is the driven adapter for Context7.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a real Context7 HTTP client. The API key lives only
// in the backend .env / Coolify secret and is never exposed to the
// frontend.
//
// The client owns its *http.Client so tests can swap it for one whose
// transport points at an httptest.NewServer endpoint.
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = Endpoint
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultRequestTimeout},
	}
}

// NewClientWithHTTP is the test-only constructor that lets the caller
// inject a preconfigured *http.Client (typically wired to an
// httptest.NewServer transport). Production callers must use
// NewClient.
func NewClientWithHTTP(baseURL, apiKey string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: defaultRequestTimeout}
	}
	c := NewClient(baseURL, apiKey)
	c.httpClient = hc
	return c
}

// Compile-time guard: the adapter MUST satisfy the DocsClient port
// core/services depends on. Any drift between the driven adapter and
// the port surfaces at compile time, not at runtime, so a future port
// shape change is caught at PR review.
var _ ports.DocsClient = (*Client)(nil)

// docsSearchResponse mirrors the documented Context7 search payload
// (subset; the API also returns scores + library metadata we don't
// surface).
type docsSearchResponse struct {
	Results []ports.DocsHit `json:"results"`
}

// GetLibraryDocs fetches the documentation excerpt for `library` and
// optional `topic`. An empty `topic` returns the library overview.
//
// Path templates:
//
//	{base}/{library}                  (empty topic)
//	{base}/{library}/topic/{topic}    (non-empty topic)
//
// The Context7 API returns text/plain for these endpoints; we treat
// the body verbatim. Errors are wrapped with the upstream status code
// so the call site can branch (rate-limit vs. not-found).
func (c *Client) GetLibraryDocs(ctx context.Context, library, topic string) (string, error) {
	library = strings.TrimSpace(library)
	if library == "" {
		return "", errMissingLibrary
	}
	path := "/" + url.PathEscape(library)
	if t := strings.TrimSpace(topic); t != "" {
		path += "/topic/" + url.PathEscape(t)
	}
	body, err := c.doGET(ctx, path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SearchDocumentation performs a documentation-centric search. Returns
// up to MaxSearchHits hits (enforced client-side; the upstream may
// return more, we clamp).
func (c *Client) SearchDocumentation(ctx context.Context, query string) ([]ports.DocsHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []ports.DocsHit{}, nil
	}
	q := url.Values{"query": {query}}
	body, err := c.doGET(ctx, "/search?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var resp docsSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("context7: decode search: %w", err)
	}
	hits := resp.Results
	if len(hits) > MaxSearchHits {
		hits = hits[:MaxSearchHits]
	}
	if hits == nil {
		hits = []ports.DocsHit{}
	}
	return hits, nil
}

// doGET is the single network path: builds the request with the auth
// header, runs it through c.httpClient, and surfaces the body + a
// status-aware error. The error message intentionally omits the API
// key; the dispatcher.SanitizeErrorMessage sweep would strip it
// anyway, but defense-in-depth is cheap.
func (c *Client) doGET(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("context7: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("context7: call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("context7: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// NOTE: the upstream body is intentionally NOT included in the
		// error message. A misbehaving upstream could echo credentials
		// or other tokens in its 5xx body; the dispatcher's
		// SanitizeErrorMessage redactor would catch most of those
		// patterns, but adapter-level defense is cheaper than relying
		// on a downstream sweep. Operators debug via the structured
		// log + the request ID echoed by the MCP server, not via this
		// error string.
		return nil, fmt.Errorf("context7: upstream returned %d", resp.StatusCode)
	}
	return body, nil
}
