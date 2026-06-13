package gogs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- ListRepos tests ---

func TestListRepos_Success(t *testing.T) {
	repos := []Repo{
		{
			ID:            1,
			Name:          "my-repo",
			FullName:      "user/my-repo",
			Description:   "A test repo",
			Private:       false,
			CloneURL:      "https://gogs.example.com/user/my-repo.git",
			DefaultBranch: "main",
		},
		{
			ID:            2,
			Name:          "private-repo",
			FullName:      "user/private-repo",
			Description:   "",
			Private:       true,
			CloneURL:      "https://gogs.example.com/user/private-repo.git",
			DefaultBranch: "develop",
		},
	}

	body, _ := json.Marshal(repos)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the correct endpoint and auth header
		if r.URL.Path != "/api/v1/user/repos" {
			t.Errorf("expected path /api/v1/user/repos, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("expected Authorization 'token test-token', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	result, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(result))
	}
	if result[0].Name != "my-repo" {
		t.Errorf("expected repo name 'my-repo', got %q", result[0].Name)
	}
	if result[1].Private != true {
		t.Errorf("expected repo private=true, got false")
	}
}

func TestListRepos_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	result, err := client.ListRepos(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d repos", len(result))
	}
}

func TestListRepos_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "Unauthorized"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "bad-token")
	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "GOGS_AUTH_FAILED" {
		t.Errorf("expected code GOGS_AUTH_FAILED, got %q", gogsErr.Code)
	}
	if gogsErr.Status != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", gogsErr.Status)
	}
}

func TestListRepos_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "rate limit exceeded"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "GOGS_RATE_LIMITED" {
		t.Errorf("expected code GOGS_RATE_LIMITED, got %q", gogsErr.Code)
	}
	if gogsErr.Status != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", gogsErr.Status)
	}
}

func TestListRepos_Unreachable(t *testing.T) {
	// Use a URL that will fail to connect
	client := NewGogsClient("http://127.0.0.1:0", "test-token")
	// Use a short timeout client to make this test faster
	client.httpClient = &http.Client{Timeout: 1 * time.Second}

	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "GOGS_UNREACHABLE" {
		t.Errorf("expected code GOGS_UNREACHABLE, got %q", gogsErr.Code)
	}
}

// --- GetFileContents tests ---

func TestGetFileContents_Success(t *testing.T) {
	fc := FileContent{
		Name:     "main.go",
		Path:     "main.go",
		Content:  "cGFja2FnZSBtYWlu", // base64 for "package main"
		Encoding: "base64",
		Size:     12,
	}

	body, _ := json.Marshal(fc)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/repos/user/my-repo/contents/main.go"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		if r.URL.Query().Get("ref") != "main" {
			t.Errorf("expected ref=main, got ref=%s", r.URL.Query().Get("ref"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	result, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "main.go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Name != "main.go" {
		t.Errorf("expected name 'main.go', got %q", result.Name)
	}
	if result.Encoding != "base64" {
		t.Errorf("expected encoding 'base64', got %q", result.Encoding)
	}
}

func TestGetFileContents_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "file not found"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "nonexistent.go")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "FILE_NOT_FOUND" {
		t.Errorf("expected code FILE_NOT_FOUND, got %q", gogsErr.Code)
	}
	if gogsErr.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", gogsErr.Status)
	}
}

func TestGetFileContents_Exceeds1MB(t *testing.T) {
	fc := FileContent{
		Name:     "large.bin",
		Path:     "large.bin",
		Content:  "big",
		Encoding: "base64",
		Size:     2 << 20, // 2 MB
	}

	body, _ := json.Marshal(fc)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "large.bin")
	if err == nil {
		t.Fatal("expected error for file exceeding 1 MB")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "FILE_TOO_LARGE" {
		t.Errorf("expected code FILE_TOO_LARGE, got %q", gogsErr.Code)
	}
	if gogsErr.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", gogsErr.Status)
	}
}

func TestGetFileContents_ExactlyAt1MB(t *testing.T) {
	fc := FileContent{
		Name:     "exact.bin",
		Path:     "exact.bin",
		Content:  "data",
		Encoding: "base64",
		Size:     1 << 20, // exactly 1,048,576 bytes = 1 MB
	}

	body, _ := json.Marshal(fc)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	result, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "exact.bin")
	if err != nil {
		t.Fatalf("expected no error for file at exactly 1 MB, got: %v", err)
	}
	if result.Name != "exact.bin" {
		t.Errorf("expected name 'exact.bin', got %q", result.Name)
	}
}

func TestGetFileContents_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	client.httpClient = &http.Client{Timeout: 100 * time.Millisecond}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := client.GetFileContents(ctx, "user", "my-repo", "main", "main.go")
	if err == nil {
		t.Fatal("expected error for timeout")
	}
}

func TestGetFileContents_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message": "token is invalid"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "bad-token")
	_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "main.go")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	// Auth failure should NOT expose the token
	if strings.Contains(gogsErr.Message, "bad-token") {
		t.Error("error message should not contain the token value")
	}
}

func TestGetFileContents_EmptyPath(t *testing.T) {
	// This tests that Gogs returns 404 for empty paths
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "not found"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestNewGogsClient_TrailingSlash(t *testing.T) {
	client := NewGogsClient("https://gogs.example.com/", "token")
	if client.baseURL != "https://gogs.example.com" {
		t.Errorf("expected trailing slash trimmed, got %q", client.baseURL)
	}
}

// Verify that token never appears in error messages
func TestGogsError_TokenNotInMessage(t *testing.T) {
	client := NewGogsClient("http://127.0.0.1:0", "secret-token-12345")
	client.httpClient = &http.Client{Timeout: 1 * time.Second}

	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}

	errStr := err.Error()
	if strings.Contains(errStr, "secret-token-12345") {
		t.Errorf("token value should not appear in error message, got: %s", errStr)
	}
}

// Verify GogsError.Error() format
func TestGogsError_Format(t *testing.T) {
	e := &GogsError{Message: "test", Code: "TEST_CODE", Status: 500}
	errStr := e.Error()
	if !strings.Contains(errStr, "TEST_CODE") {
		t.Errorf("expected error to contain code, got: %s", errStr)
	}
	if !strings.Contains(errStr, "500") {
		t.Errorf("expected error to contain status, got: %s", errStr)
	}
}

// Verify doRequest handles 5xx errors
func TestListRepos_5xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "internal server error"}`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "GOGS_API_ERROR" {
		t.Errorf("expected code GOGS_API_ERROR, got %q", gogsErr.Code)
	}
}

// Verify doRequest handles malformed JSON
func TestListRepos_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.ListRepos(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	gogsErr, ok := err.(*GogsError)
	if !ok {
		t.Fatalf("expected *GogsError, got %T", err)
	}
	if gogsErr.Code != "GOGS_PARSE_ERROR" {
		t.Errorf("expected code GOGS_PARSE_ERROR, got %q", gogsErr.Code)
	}
}

// Verify doRequest path encoding
func TestGetFileContents_PathWithSlashes(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path

		fc := FileContent{
			Name:     "nested.go",
			Path:     "cmd/app/main.go",
			Content:  "cGFja2FnZSBtYWlu",
			Encoding: "base64",
			Size:     12,
		}
		body, _ := json.Marshal(fc)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "cmd/app/main.go")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	expected := "/api/v1/repos/user/my-repo/contents/cmd/app/main.go"
	if requestPath != expected {
		t.Errorf("expected path %q, got %q", expected, requestPath)
	}
}

// Test URL construction for base URL with trailing slash is trimmed
func TestNewGogsClient_EnsureNoDoubleSlash(t *testing.T) {
	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	_, _ = client.ListRepos(context.Background())

	// Should not have double slashes after trimming
	if strings.Contains(capturedURL, "//api") {
		// Note: httptest normalizes paths, but we verify baseURL is trimmed
		url := client.baseURL
		if strings.HasSuffix(url, "/") {
			t.Errorf("baseURL should not have trailing slash: %q", url)
		}
	}
}

// Test that a context cancellation is properly handled
func TestListRepos_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewGogsClient(server.URL, "test-token")
	client.httpClient = &http.Client{Timeout: 10 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.ListRepos(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// Test response with various status codes
func TestGetFileContents_VarStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantCode   string
		wantStatus int
	}{
		{"429 rate limit", 429, "GOGS_API_ERROR", 429},
		{"500 internal", 500, "GOGS_API_ERROR", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(fmt.Sprintf(`{"message": "error %d"}`, tt.statusCode)))
			}))
			defer server.Close()

			client := NewGogsClient(server.URL, "test-token")
			_, err := client.GetFileContents(context.Background(), "user", "my-repo", "main", "file.go")
			if err == nil {
				t.Fatal("expected error")
			}

			gogsErr, ok := err.(*GogsError)
			if !ok {
				t.Fatalf("expected *GogsError, got %T", err)
			}
			if gogsErr.Code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, gogsErr.Code)
			}
			if gogsErr.Status != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, gogsErr.Status)
			}
		})
	}
}