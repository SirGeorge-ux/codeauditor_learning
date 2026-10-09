package ports

import "context"

// DocsClient is the contract for a third-party library documentation
// provider used by mcp-external_docs.get_library_docs and .search_documentation.
//
// Implementations live in internal/infrastructure/driven (Context7 in
// PR 4; future providers — e.g. DeepWiki — can drop in without
// touching the port). core/services/ExternalDocsService depends on this
// interface so the service stays swappable and testable without an
// HTTP roundtrip.
//
// Hexagonal contract: implementations MAY parse upstream JSON into the
// DocsHit struct this package owns; that struct is the canonical wire
// shape carried across the port boundary.
type DocsClient interface {
	// GetLibraryDocs returns the documentation excerpt for a library
	// (e.g. "react", "typescript") and optional topic. The empty
	// `topic` returns the library overview. Errors are surfaced as-is;
	// the MCP layer maps them to the appropriate JSON-RPC envelope.
	GetLibraryDocs(ctx context.Context, library, topic string) (string, error)
	// SearchDocumentation performs a documentation-centric search and
	// returns the top N hits (provider-specific cap). An empty result
	// slice (not nil) signals "no matches".
	SearchDocumentation(ctx context.Context, query string) ([]DocsHit, error)
}

// DocsHit is the canonical search-hit shape produced by every DocsClient
// implementation. Keeping the type in the port package prevents the
// service from importing an adapter-internal struct.
type DocsHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// WebSearchClient is the contract for a general-purpose web search
// provider used by mcp-external_docs.search_documentation and
// .get_real_example. The default S2 adapter is Brave Search
// (internal/infrastructure/driven/brave); future providers can drop in
// without service changes.
//
// Hexagonal contract: implementations parse upstream JSON into the
// WebHit struct this package owns.
type WebSearchClient interface {
	// SearchQuery runs a web search and returns the top N hits
	// (provider-specific cap; Brave defaults to 20). An empty result
	// slice (not nil) signals "no matches".
	SearchQuery(ctx context.Context, query string) ([]WebHit, error)
}

// WebHit is the canonical web-search-hit shape produced by every
// WebSearchClient implementation.
type WebHit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}
