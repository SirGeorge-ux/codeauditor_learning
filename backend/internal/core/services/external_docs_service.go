package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ExternalDocsService fronts third-party documentation providers
// (Context7, Brave Search) for the mcp-external_docs tool group.
//
// The service depends exclusively on ports.DocsClient +
// ports.WebSearchClient; the HTTP-bearing adapters live in
// internal/infrastructure/driven. The interface split lets the
// service combine "library docs" (Context7) + "real-world examples"
// (Context7 + Brave fusion) without ever importing net/http.
//
// Hexagonal contract: this file imports ONLY the ports package and
// stdlib. The concrete adapters (context7.Client, brave.Client) live
// below the service-layer boundary and are injected via main.go.
//
// PR 1 shipped a compilable stub; PR 4 fleshes it with deterministic
// fusion logic (Context7 first, Brave fallback) so mcp-external_docs'
// three tools have a real end-to-end path. The S5 cascade refines
// candidate ranking via LLMClient when that port lands.
type ExternalDocsService struct {
	docs ports.DocsClient
	web  ports.WebSearchClient
}

// NewExternalDocsService wires the external-docs engine to its two
// driven ports. A nil docs or web is acceptable; the corresponding
// method returns a clear error so the dispatcher maps it to JSON-RPC
// -32603 (internal error) and the operator sees a misconfiguration
// immediately.
func NewExternalDocsService(docs ports.DocsClient, web ports.WebSearchClient) *ExternalDocsService {
	return &ExternalDocsService{docs: docs, web: web}
}

// --- Input/output shapes ---------------------------------------------------

// LibraryDocsInput scopes a Context7 documentation lookup.
type LibraryDocsInput struct {
	Library string `json:"library"`
	Topic   string `json:"topic,omitempty"`
}

// LibraryDocs is the documentation excerpt returned for a library topic.
type LibraryDocs struct {
	Library string `json:"library"`
	Topic   string `json:"topic,omitempty"`
	Content string `json:"content"`
	Source  string `json:"source,omitempty"`
}

// SearchDocsInput scopes a Brave Search documentation query.
type SearchDocsInput struct {
	Query string `json:"query"`
}

// SearchDocsResult is a single Brave Search hit.
type SearchDocsResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

// RealExampleInput scopes a real-world code example lookup.
type RealExampleInput struct {
	Library  string `json:"library"`
	Topic    string `json:"topic,omitempty"`
	Language string `json:"language,omitempty"`
}

// RealExample is a curated code example for a library topic.
type RealExample struct {
	Library  string `json:"library"`
	Topic    string `json:"topic,omitempty"`
	Language string `json:"language,omitempty"`
	Code     string `json:"code"`
	Source   string `json:"source,omitempty"`
}

// --- GetLibraryDocs --------------------------------------------------------

// GetLibraryDocs fetches documentation for a library + topic via the
// DocsClient port. An empty library fails fast (defense in depth —
// the MCP tool layer validates the same input).
func (s *ExternalDocsService) GetLibraryDocs(ctx context.Context, in LibraryDocsInput) (LibraryDocs, error) {
	library := strings.TrimSpace(in.Library)
	topic := strings.TrimSpace(in.Topic)
	if library == "" {
		return LibraryDocs{}, fmt.Errorf("%w: library is required", ErrCurriculumInvalidLanguage)
	}
	if s.docs == nil {
		return LibraryDocs{}, errors.New("external-docs: docs client not configured")
	}
	content, err := s.docs.GetLibraryDocs(ctx, library, topic)
	if err != nil {
		return LibraryDocs{}, fmt.Errorf("external-docs: library docs fetch: %w", err)
	}
	return LibraryDocs{
		Library: library,
		Topic:   topic,
		Content: content,
		Source:  "context7",
	}, nil
}

// --- SearchDocumentation ---------------------------------------------------

// SearchDocumentation performs a web documentation search via the
// WebSearchClient port. The default provider is Brave; the result
// is returned as []SearchDocsResult (the tool's wire shape).
func (s *ExternalDocsService) SearchDocumentation(ctx context.Context, in SearchDocsInput) ([]SearchDocsResult, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return []SearchDocsResult{}, nil
	}
	if s.web == nil {
		return nil, errors.New("external-docs: web client not configured")
	}
	hits, err := s.web.SearchQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("external-docs: search query: %w", err)
	}
	out := make([]SearchDocsResult, 0, len(hits))
	for _, h := range hits {
		out = append(out, SearchDocsResult{
			Title:   h.Title,
			URL:     h.URL,
			Snippet: h.Snippet,
		})
	}
	return out, nil
}

// --- GetRealExample --------------------------------------------------------

// GetRealExample fuses Context7 + Brave: it asks Context7 first for
// the library overview, extracts the first fenced code block, and
// falls back to a Brave snippet when Context7 returns nothing
// usable. The returned Source field identifies which provider the
// payload came from so the LLM caller / tutor can attribute.
func (s *ExternalDocsService) GetRealExample(ctx context.Context, in RealExampleInput) (RealExample, error) {
	library := strings.TrimSpace(in.Library)
	topic := strings.TrimSpace(in.Topic)
	language := strings.TrimSpace(in.Language)
	if library == "" {
		return RealExample{}, fmt.Errorf("%w: library is required", ErrCurriculumInvalidLanguage)
	}
	// Context7 first.
	if s.docs != nil {
		body, err := s.docs.GetLibraryDocs(ctx, library, topic)
		if err == nil {
			code := firstCodeFence(body)
			if code != "" {
				return RealExample{
					Library:  library,
					Topic:    topic,
					Language: language,
					Code:     code,
					Source:   "context7",
				}, nil
			}
		}
	}
	// Brave fallback: scan top hits for a fenced-code snippet.
	if s.web != nil {
		query := library + " example"
		if topic != "" {
			query = library + " " + topic + " example"
		}
		if language != "" {
			query = query + " " + language
		}
		hits, err := s.web.SearchQuery(ctx, query)
		if err == nil && len(hits) > 0 {
			return RealExample{
				Library:  library,
				Topic:    topic,
				Language: language,
				Code:     hits[0].Snippet,
				Source:   "brave:" + hits[0].URL,
			}, nil
		}
	}
	return RealExample{}, errors.New("external-docs: no example available")
}

// firstCodeFence extracts the first ```-fenced block from a markdown
// body. When the body has no fence (the common Context7 overview case)
// the whole body is returned so the LLM caller still gets content.
func firstCodeFence(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	start := strings.Index(body, "```")
	if start < 0 {
		return body
	}
	rest := body[start+3:]
	end := strings.Index(rest, "```")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	if nl := strings.Index(rest[:end], "\n"); nl >= 0 {
		return strings.TrimSpace(rest[nl+1 : end])
	}
	return strings.TrimSpace(rest[:end])
}
