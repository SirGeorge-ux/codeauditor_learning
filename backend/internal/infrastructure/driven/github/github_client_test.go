package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- ListFiles tests ---

// TestListFiles_HappyPath covers the spec REQ-CODE-HEALTH-002
// "Happy-path GitHub import" scenario. The tree endpoint returns a
// blob-heavy payload; the adapter MUST filter to type=blob and
// return []FileEntry with Path, Size, SHA populated.
func TestListFiles_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/octocat/hello-world/git/trees/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization Bearer test-token, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("expected Accept application/vnd.github+json, got %q", r.Header.Get("Accept"))
		}
		// Recursive tree with both blobs and trees.
		resp := map[string]any{
			"sha":       "tree-sha",
			"url":       "https://api.github.com/repos/octocat/hello-world/git/trees/tree-sha",
			"truncated": false,
			"tree": []map[string]any{
				{"path": "src", "type": "tree", "sha": "tree-src", "size": 0},
				{"path": "src/main.go", "type": "blob", "sha": "blob-main", "size": 1024},
				{"path": "README.md", "type": "blob", "sha": "blob-readme", "size": 256},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	files, err := client.ListFiles(context.Background(), "octocat", "hello-world", "main")
	if err != nil {
		t.Fatalf("ListFiles unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListFiles len = %d, want 2 (blobs only)", len(files))
	}
	if files[0].Path != "src/main.go" {
		t.Errorf("ListFiles[0].Path = %q, want src/main.go", files[0].Path)
	}
	if files[0].SHA != "blob-main" {
		t.Errorf("ListFiles[0].SHA = %q, want blob-main", files[0].SHA)
	}
	if files[0].Size != 1024 {
		t.Errorf("ListFiles[0].Size = %d, want 1024", files[0].Size)
	}
	if files[1].Path != "README.md" {
		t.Errorf("ListFiles[1].Path = %q, want README.md", files[1].Path)
	}
}

// TestListFiles_NotFound covers REQ-CODE-HEALTH-002 "Private or
// missing repo (404)" — the adapter MUST translate the upstream
// 404 into ErrGitHubNotFound (not a generic error). The driving
// layer surfaces this to the user as "Repo not found / not
// accessible".
func TestListFiles_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	_, err := client.ListFiles(context.Background(), "octocat", "missing", "main")
	if !errors.Is(err, ErrGitHubNotFound) {
		t.Fatalf("ListFiles 404 error = %v, want ErrGitHubNotFound", err)
	}
}

// TestListFiles_RateLimited covers REQ-CODE-HEALTH-002 "GitHub rate
// limit" — when GitHub responds 403 with x-ratelimit-remaining:0,
// the adapter MUST return ErrGitHubRateLimited. A 403 without that
// header is a different error (auth).
func TestListFiles_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "0")
		w.Header().Set("x-ratelimit-reset", "1700000000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	_, err := client.ListFiles(context.Background(), "octocat", "hello-world", "main")
	if !errors.Is(err, ErrGitHubRateLimited) {
		t.Fatalf("ListFiles rate-limit error = %v, want ErrGitHubRateLimited", err)
	}
}

// TestListFiles_Forbidden_WithoutRateLimit covers the negative
// case: a 403 without the rate-limit header is NOT a rate-limit
// error (it's likely an auth/permission issue). The adapter
// returns the generic 4xx/5xx error in that case.
func TestListFiles_Forbidden_WithoutRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Deliberately omit x-ratelimit-remaining:0.
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "bad-token")
	_, err := client.ListFiles(context.Background(), "octocat", "private", "main")
	if errors.Is(err, ErrGitHubRateLimited) {
		t.Errorf("403 without rate-limit header MUST NOT be ErrGitHubRateLimited, got: %v", err)
	}
	if err == nil {
		t.Errorf("expected an error for 403 response")
	}
}

// TestListFiles_EmptyRepo covers the "empty repo" scenario
// (truncated=true OR an empty tree slice). The adapter returns an
// empty slice without error.
func TestListFiles_EmptyRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"sha":       "empty",
			"truncated": false,
			"tree":      []map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	files, err := client.ListFiles(context.Background(), "octocat", "empty", "main")
	if err != nil {
		t.Fatalf("ListFiles empty repo: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("ListFiles empty repo len = %d, want 0", len(files))
	}
}

// --- GetFileContent tests ---

// TestGetFileContent_Base64Decode covers the spec: GitHub returns
// file content as base64 (unlike Gogs which also uses base64 — the
// adapter mirrors the canonical pattern). The adapter MUST decode
// to a plaintext string before returning.
func TestGetFileContent_Base64Decode(t *testing.T) {
	plaintext := "package main\n\nfunc main() {}\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"name":     "main.go",
			"path":     "main.go",
			"sha":      "blob-main",
			"size":     len(plaintext),
			"type":     "file",
			"encoding": "base64",
			"content":  encoded,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	fc, err := client.GetFileContent(context.Background(), "octocat", "hello-world", "main", "main.go")
	if err != nil {
		t.Fatalf("GetFileContent: %v", err)
	}
	if fc.Content != plaintext {
		t.Errorf("GetFileContent decoded = %q, want %q", fc.Content, plaintext)
	}
	if fc.SHA != "blob-main" {
		t.Errorf("GetFileContent SHA = %q, want blob-main", fc.SHA)
	}
	if fc.Path != "main.go" {
		t.Errorf("GetFileContent Path = %q, want main.go", fc.Path)
	}
}

// TestGetFileContent_NotFound — same 404 translation as ListFiles.
func TestGetFileContent_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	_, err := client.GetFileContent(context.Background(), "octocat", "hello-world", "main", "missing.go")
	if !errors.Is(err, ErrGitHubNotFound) {
		t.Fatalf("GetFileContent 404 error = %v, want ErrGitHubNotFound", err)
	}
}

// TestGetFileContent_Timeout covers the design contract that the
// client uses a 30s timeout (matching the gogs adapter). The
// adapter MUST surface a timeout error when the upstream exceeds
// the budget.
func TestGetFileContent_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "test-token")
	client.httpClient = &http.Client{Timeout: 100 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.GetFileContent(ctx, "octocat", "hello-world", "main", "main.go")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// TestNewGitHubClient_TrailingSlashTrimmed mirrors the gogs
// adapter's contract — the base URL trailing slash is trimmed so
// `client.baseURL + "/path"` doesn't produce `//path`.
func TestNewGitHubClient_TrailingSlashTrimmed(t *testing.T) {
	c := NewGitHubClient("https://api.github.com/", "tok")
	if c.baseURL != "https://api.github.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://api.github.com")
	}
}

// TestTokenNotInErrorMessage is a security check: the token value
// MUST NOT appear in any error message (matches the gogs adapter's
// contract — see gogs_client_test.go for the same assertion).
func TestTokenNotInErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"oops"}`))
	}))
	defer server.Close()

	client := NewGitHubClient(server.URL, "secret-token-12345")
	_, err := client.ListFiles(context.Background(), "o", "r", "main")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret-token-12345") {
		t.Errorf("token leaked in error message: %s", err.Error())
	}
}
