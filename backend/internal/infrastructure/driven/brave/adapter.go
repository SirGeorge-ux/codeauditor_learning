// Package brave is a driven adapter for the Brave Search API used by
// the mcp-external_docs.search_documentation and .get_real_example
// tools.
//
// Hexagonal contract: it implements the web-search port (see
// internal/ports/external_docs.go) and is the ONLY place that imports
// Brave Search HTTP concerns. core/services/ExternalDocsService depends
// on the port, never on this package directly.
//
// Wire shape
// -----------
// Brave Search exposes:
//
//	GET {base}/res/v1/web/search?q=<q>&count=<n>  →  JSON { web: { results: [ { title, url, description } ] } }
//
// The base URL defaults to Endpoint (https://api.search.brave.com) and
// is overridable per environment. The API key is sent via the
// `X-Subscription-Token: <key>` header (Brave-specific naming). The key
// NEVER leaves the backend; PR 5's MCPClientAdapter only sees the
// rendered hits.
//
// PR 1 shipped a compilable stub. PR 4 fleshes the adapter out with a
// pure net/http client (no third-party SDK) and a unit test against
// httptest.NewServer so the JSON wiring is exercised without leaving
// the host's network namespace.
package brave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Endpoint is the Brave Search base URL (overridable per environment).
const Endpoint = "https://api.search.brave.com"

// Default request timeout. Brave's documented SLA is <2s for cached
// queries; 10s gives room for cache misses + network jitter.
const defaultRequestTimeout = 10 * time.Second

// MaxSearchHits caps a single SearchQuery response; Brave allows up
// to 20 by default in their free tier.
const MaxSearchHits = 20

// Client is the driven adapter for Brave Search.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a real Brave Search HTTP client. The API key lives
// only in the backend .env / Coolify secret and is never exposed to
// the frontend.
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

// Compile-time guard: the adapter MUST satisfy the WebSearchClient port
// the service depends on. Drift between the adapter and the port
// surfaces at compile time, not at runtime.
var _ ports.WebSearchClient = (*Client)(nil)

// braveSearchResponse mirrors the documented Brave Search v1 payload
// (subset; the API also returns faq / news / videos we don't surface).
type braveSearchResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

// SearchQuery runs a web documentation search via Brave Search. The
// query is passed verbatim; callers are expected to have URL-encoded
// any special characters via url.Values.Encode().
//
// On non-2xx responses the returned error intentionally OMITS the
// upstream body (see Context7 adapter for the same defense rationale).
func (c *Client) SearchQuery(ctx context.Context, query string) ([]ports.WebHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []ports.WebHit{}, nil
	}
	q := url.Values{
		"q":     {query},
		"count": {"20"}, // capped to MaxSearchHits below
	}
	path := "/res/v1/web/search?" + q.Encode()

	body, err := c.doGET(ctx, path)
	if err != nil {
		return nil, err
	}
	var resp braveSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("brave: decode search: %w", err)
	}
	hits := make([]ports.WebHit, 0, len(resp.Web.Results))
	for _, r := range resp.Web.Results {
		hits = append(hits, ports.WebHit{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Description,
		})
		if len(hits) >= MaxSearchHits {
			break
		}
	}
	return hits, nil
}

// doGET is the single network path for the Brave adapter: builds the
// request with the subscription-token header, runs it through
// c.httpClient, and surfaces non-2xx responses with a body-less error.
func (c *Client) doGET(ctx context.Context, path string) ([]byte, error) {
	if c.apiKey == "" {
		// Brave Search requires authentication for every call; fail
		// fast at the adapter boundary rather than handing the
		// request off to a 401 round-trip.
		return nil, errMissingAPIKey
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("brave: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Body intentionally omitted; see the matching rationale on
		// the Context7 adapter. Brave occasionally leaks server
		// diagnostics into its 5xx payload; the redactor downstream
		// is the second line, not the first.
		return nil, fmt.Errorf("brave: upstream returned %d", resp.StatusCode)
	}
	return body, nil
}

// errMissingAPIKey is returned when SearchQuery is invoked without an
// API key configured. Brave Search 401s on missing auth, but failing
// fast at the adapter boundary keeps tests deterministic and surfaces
// a clearer misconfiguration error to the operator.
var errMissingAPIKey = errors.New("brave: api key is required")
