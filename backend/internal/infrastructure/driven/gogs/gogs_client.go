package gogs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// Compile-time guard: GogsClient implements ports.SourceClient.
var _ ports.SourceClient = (*GogsClient)(nil)

const maxFileSize int64 = 1 << 20 // 1 MB

// GogsClient is a driven adapter that proxies requests to a Gogs instance.
// It follows the same pattern as SupabaseClient — concrete adapter, no port.
type GogsClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Repo represents a Gogs repository returned by the API.
type Repo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
}

// FileContent represents a file's contents returned by the Gogs contents API.
type FileContent struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Content  string `json:"content"`  // base64-encoded
	Encoding string `json:"encoding"` // "base64"
	Size     int64  `json:"size"`
}

// GogsError represents a structured error from the Gogs proxy.
type GogsError struct {
	Message string
	Code    string
	Status  int
}

func (e *GogsError) Error() string {
	return fmt.Sprintf("%s (code: %s, status: %d)", e.Message, e.Code, e.Status)
}

// NewGogsClient creates a new GogsClient with the given base URL and token.
func NewGogsClient(baseURL, token string) *GogsClient {
	return &GogsClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListRepos calls GET /api/v1/user/repos to list repositories accessible to the token owner.
func (c *GogsClient) ListRepos(ctx context.Context) ([]Repo, error) {
	url := c.baseURL + "/api/v1/user/repos"

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var repos []Repo
	if err := json.Unmarshal(respBody, &repos); err != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to decode repos response: %v", err),
			Code:    "GOGS_PARSE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}

	return repos, nil
}

// ListFiles calls GET /api/v1/repos/:owner/:repo/git/trees/:ref?recursive=1
// to fetch the repo tree, then returns the blob entries as
// []ports.FileEntry (Path, Size, SHA). Tree entries (directories)
// are filtered out — the analyzer's per-file loop operates on
// blobs only. SHA is the git blob SHA so the analysis cache can
// key on it across renames (spec REQ-CODE-HEALTH-009).
func (c *GogsClient) ListFiles(ctx context.Context, owner, repo, ref string) ([]ports.FileEntry, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/git/trees/%s?recursive=1",
		c.baseURL, owner, repo, ref)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SHA  string `json:"sha"`
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to decode tree response: %v", err),
			Code:    "GOGS_PARSE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}

	out := make([]ports.FileEntry, 0, len(resp.Tree))
	for _, entry := range resp.Tree {
		if entry.Type != "blob" {
			continue
		}
		out = append(out, ports.FileEntry{
			Path: entry.Path,
			SHA:  entry.SHA,
			Size: entry.Size,
		})
	}
	return out, nil
}

// GetFileContents calls GET /api/v1/repos/:owner/:repo/contents/:path?ref=:ref
// to fetch file contents. It enforces a 1 MB size limit.
func (c *GogsClient) GetFileContents(ctx context.Context, owner, repo, ref, path string) (*FileContent, error) {
	url := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=%s",
		c.baseURL, owner, repo, path, ref)

	respBody, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var fc FileContent
	if err := json.Unmarshal(respBody, &fc); err != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to decode file content response: %v", err),
			Code:    "GOGS_PARSE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}

	// Enforce max file size
	if fc.Size > maxFileSize {
		return nil, &GogsError{
			Message: fmt.Sprintf("file exceeds maximum size of 1 MB (%d bytes)", fc.Size),
			Code:    "FILE_TOO_LARGE",
			Status:  http.StatusRequestEntityTooLarge,
		}
	}

	return &fc, nil
}

// GetFileContent is the port-shaped wrapper around GetFileContents.
// The code-health service consumes this through ports.SourceClient
// (the interface defined in internal/ports/code_health.go); the
// legacy gogs HTTP handler keeps the original GetFileContents
// method for backward compatibility.
func (c *GogsClient) GetFileContent(ctx context.Context, owner, repo, branch, path string) (*ports.FileContent, error) {
	fc, err := c.GetFileContents(ctx, owner, repo, branch, path)
	if err != nil {
		return nil, err
	}
	// Gogs returns base64-encoded content; decode to plaintext.
	decoded, decErr := decodeBase64Content(fc.Content, fc.Encoding)
	if decErr != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to decode base64 content: %v", decErr),
			Code:    "GOGS_DECODE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}
	return &ports.FileContent{
		Path:    fc.Path,
		Content: decoded,
		SHA:     "",
		Size:    fc.Size,
	}, nil
}

// decodeBase64Content is a tiny helper that mirrors the GitHub
// adapter's content decoding. Gogs returns base64 by default;
// the spec assumes the canonical encoding.
func decodeBase64Content(content, encoding string) (string, error) {
	switch encoding {
	case "base64":
		// Gogs may insert newlines every 60 chars; strip them
		// before decoding.
		cleaned := strings.ReplaceAll(content, "\n", "")
		raw, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	default:
		// "none" or unknown: pass through.
		return content, nil
	}
}

// doRequest executes an HTTP request against the Gogs API and returns the body.
// It handles common error cases: unreachable, auth failure, not found, and rate limits.
func (c *GogsClient) doRequest(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to create request: %v", err),
			Code:    "GOGS_REQUEST_ERROR",
			Status:  http.StatusInternalServerError,
		}
	}

	// Gogs uses "token <value>" header format (not "Bearer")
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &GogsError{
			Message: "Gogs service is unavailable",
			Code:    "GOGS_UNREACHABLE",
			Status:  http.StatusBadGateway,
		}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &GogsError{
			Message: fmt.Sprintf("failed to read response: %v", err),
			Code:    "GOGS_READ_ERROR",
			Status:  http.StatusBadGateway,
		}
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &GogsError{
			Message: "Gogs authentication failed",
			Code:    "GOGS_AUTH_FAILED",
			Status:  http.StatusBadGateway,
		}
	case resp.StatusCode == http.StatusForbidden:
		return nil, &GogsError{
			Message: "Gogs rate limit exceeded",
			Code:    "GOGS_RATE_LIMITED",
			Status:  http.StatusTooManyRequests,
		}
	case resp.StatusCode == http.StatusNotFound:
		return nil, &GogsError{
			Message: "file not found",
			Code:    "FILE_NOT_FOUND",
			Status:  http.StatusNotFound,
		}
	case resp.StatusCode >= 400:
		return nil, &GogsError{
			Message: fmt.Sprintf("Gogs API error: status %d", resp.StatusCode),
			Code:    "GOGS_API_ERROR",
			Status:  resp.StatusCode,
		}
	}

	return respBody, nil
}
