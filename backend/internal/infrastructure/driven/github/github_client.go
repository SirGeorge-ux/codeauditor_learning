package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// portsFileEntry is a local alias of ports.FileEntry. We use the
// alias so the adapter can return port-shaped values without
// importing ports into every line of the implementation (the
// import is at the top of the file; the alias is purely a
// readability aid in the GetFileContent / ListFiles signatures).
type portsFileEntry = ports.FileEntry

// portsFileContent is a local alias of ports.FileContent.
type portsFileContent = ports.FileContent

// Compile-time guard: GitHubClient implements ports.SourceClient.
var _ ports.SourceClient = (*GitHubClient)(nil)

// maxFileSize is the per-file cap (1 MB) shared with the gogs
// adapter (see gogs_client.go for the constant). Duplicating the
// value here keeps the two adapters independent (no shared
// `internal` package); they will stay in lock-step via the spec
// (REQ-CODE-HEALTH-001 / REQ-CODE-HEALTH-002).
const maxFileSize int64 = 1 << 20 // 1 MB

// ErrGitHubNotFound is returned by the adapter when GitHub
// responds with 404. The driving layer surfaces this to the user
// as "Repo not found / not accessible" (spec REQ-CODE-HEALTH-002
// "Private or missing repo" scenario).
var ErrGitHubNotFound = errors.New("github: repository or file not found")

// ErrGitHubRateLimited is returned when GitHub responds with 403
// AND x-ratelimit-remaining:0. The driving layer maps this to
// HTTP 429 with the reset timestamp.
var ErrGitHubRateLimited = errors.New("github: API rate limit exceeded")

// GitHubError wraps a structured GitHub API error. The Message
// field is the upstream "message" field; Status is the HTTP code.
// The token value is NEVER included in the formatted error.
type GitHubError struct {
	Message string
	Code    string
	Status  int
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("github %s (code: %s, status: %d)", e.Message, e.Code, e.Status)
}

// GitHubClient is the driven adapter for the GitHub REST API. It
// implements ports.SourceClient (ListFiles, GetFileContent) — the
// service selects the adapter by source string ("github" vs
// "gogs"). The auth model is the app-wide GITHUB_TOKEN env var
// (PAT); per-user OAuth is S6+ (spec REQ-CODE-HEALTH-002).
//
// Base URL is the GitHub API root (e.g. "https://api.github.com")
// or a GitHub Enterprise equivalent. The constructor trims any
// trailing slash so path concatenation is safe.
type GitHubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGitHubClient constructs the adapter with a 30s per-request
// timeout (matches the gogs adapter; spec REQ-CODE-HEALTH-002).
func NewGitHubClient(baseURL, token string) *GitHubClient {
	return &GitHubClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListFiles calls GET /repos/{owner}/{repo}/git/trees/{branch}?recursive=1
// and returns the blob entries as []ports.FileEntry. Tree entries
// (directories) are filtered out. The SHA is the git blob SHA so
// the analysis cache can key on it across renames (spec
// REQ-CODE-HEALTH-009).
func (c *GitHubClient) ListFiles(ctx context.Context, owner, repo, branch string) ([]portsFileEntry, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1",
		c.baseURL, owner, repo, branch)
	body, err := c.doRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &GitHubError{
			Message: fmt.Sprintf("failed to decode trees response: %v", err),
			Code:    "GITHUB_PARSE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}
	out := make([]portsFileEntry, 0, len(resp.Tree))
	for _, entry := range resp.Tree {
		if entry.Type != "blob" {
			continue
		}
		out = append(out, portsFileEntry{
			Path: entry.Path,
			SHA:  entry.SHA,
			Size: entry.Size,
		})
	}
	return out, nil
}

// GetFileContent calls GET /repos/{owner}/{repo}/contents/{path}?ref={branch}
// and returns the file's plaintext (base64-decoded). The 1 MB
// per-file cap is enforced — files larger than the cap return a
// GitHubError with code FILE_TOO_LARGE.
func (c *GitHubClient) GetFileContent(ctx context.Context, owner, repo, branch, path string) (*portsFileContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		c.baseURL, owner, repo, path, branch)
	body, err := c.doRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		SHA      string `json:"sha"`
		Size     int64  `json:"size"`
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &GitHubError{
			Message: fmt.Sprintf("failed to decode file content response: %v", err),
			Code:    "GITHUB_PARSE_ERROR",
			Status:  http.StatusBadGateway,
		}
	}
	if resp.Size > maxFileSize {
		return nil, &GitHubError{
			Message: fmt.Sprintf("file exceeds maximum size of 1 MB (%d bytes)", resp.Size),
			Code:    "FILE_TOO_LARGE",
			Status:  http.StatusRequestEntityTooLarge,
		}
	}
	// GitHub returns content as base64 by default; the spec
	// assumes this is the canonical encoding. A future PR can
	// detect the "none" encoding (raw, used for files GitHub
	// considers binary) and return a binary marker.
	var decoded string
	switch resp.Encoding {
	case "base64":
		// GitHub inserts newlines every 60 chars; strip them
		// before decoding.
		cleaned := strings.ReplaceAll(resp.Content, "\n", "")
		raw, decErr := base64.StdEncoding.DecodeString(cleaned)
		if decErr != nil {
			return nil, &GitHubError{
				Message: fmt.Sprintf("failed to decode base64 content: %v", decErr),
				Code:    "GITHUB_DECODE_ERROR",
				Status:  http.StatusBadGateway,
			}
		}
		decoded = string(raw)
	default:
		// "none" or unknown encoding: pass through as-is.
		decoded = resp.Content
	}
	return &portsFileContent{
		Path:    resp.Path,
		Content: decoded,
		SHA:     resp.SHA,
		Size:    resp.Size,
	}, nil
}

// doRequest executes an HTTP GET against the GitHub API and returns
// the body. It handles the canonical error cases:
//
//	404 -> ErrGitHubNotFound
//	403 + x-ratelimit-remaining:0 -> ErrGitHubRateLimited
//	other 4xx/5xx -> *GitHubError with code GITHUB_API_ERROR
//
// The token is sent as `Authorization: Bearer <token>` (NOT the
// `token <value>` format gogs uses). The Accept header pins
// `application/vnd.github+json` to opt into the API-versioned
// response shape.
func (c *GitHubClient) doRequest(ctx context.Context, method, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, &GitHubError{
			Message: fmt.Sprintf("failed to create request: %v", err),
			Code:    "GITHUB_REQUEST_ERROR",
			Status:  http.StatusInternalServerError,
		}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &GitHubError{
			Message: "GitHub service is unavailable",
			Code:    "GITHUB_UNREACHABLE",
			Status:  http.StatusBadGateway,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &GitHubError{
			Message: fmt.Sprintf("failed to read response: %v", err),
			Code:    "GITHUB_READ_ERROR",
			Status:  http.StatusBadGateway,
		}
	}

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrGitHubNotFound
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("x-ratelimit-remaining") == "0":
		return nil, ErrGitHubRateLimited
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &GitHubError{
			Message: "GitHub authentication failed",
			Code:    "GITHUB_AUTH_FAILED",
			Status:  http.StatusBadGateway,
		}
	case resp.StatusCode >= 400:
		return nil, &GitHubError{
			Message: fmt.Sprintf("GitHub API error: status %d", resp.StatusCode),
			Code:    "GITHUB_API_ERROR",
			Status:  resp.StatusCode,
		}
	}

	return body, nil
}
