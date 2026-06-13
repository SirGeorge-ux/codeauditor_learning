package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/gogs"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// mockAuthValidator is a minimal mock that always validates tokens successfully.
type mockAuthValidator struct{}

func (m *mockAuthValidator) ValidateToken(_ context.Context, _ string) error {
	return nil
}

func (m *mockAuthValidator) UserIDFromToken(_ string) (string, error) {
	return "test-user-id", nil
}

// --- ListRepos handler tests ---

func TestGogsHandler_ListRepos_Success(t *testing.T) {
	repos := []gogs.Repo{
		{
			ID:            1,
			Name:          "my-repo",
			FullName:      "user/my-repo",
			Description:   "A test repo",
			Private:       false,
			CloneURL:      "https://gogs.example.com/user/my-repo.git",
			DefaultBranch: "main",
		},
	}
	repoBody, _ := json.Marshal(repos)

	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(repoBody)
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	req.Header.Set("Authorization", "Bearer test-jwt")
	w := httptest.NewRecorder()

	handler.ListRepos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result []gogs.Repo
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0].Name != "my-repo" {
		t.Errorf("expected repo name 'my-repo', got %q", result[0].Name)
	}
}

func TestGogsHandler_ListRepos_EmptyList(t *testing.T) {
	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	w := httptest.NewRecorder()

	handler.ListRepos(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var result []gogs.Repo
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d repos", len(result))
	}
}

func TestGogsHandler_ListRepos_GogsUnreachable(t *testing.T) {
	// Client points to unreachable server
	client := gogs.NewGogsClient("http://127.0.0.1:0", "test-token")
	handler := NewGogsHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	w := httptest.NewRecorder()

	handler.ListRepos(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "GOGS_UNREACHABLE" {
		t.Errorf("expected code GOGS_UNREACHABLE, got %q", errResp.Code)
	}
}

// --- GetFile handler tests ---

func TestGogsHandler_GetFile_Success(t *testing.T) {
	fc := gogs.FileContent{
		Name:     "main.go",
		Path:     "cmd/app/main.go",
		Content:  "cGFja2FnZSBtYWlu",
		Encoding: "base64",
		Size:     12,
	}
	fcBody, _ := json.Marshal(fc)

	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fcBody)
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	reqBody := GetFileRequest{
		Owner:  "user",
		Repo:   "my-repo",
		Branch: "main",
		Path:   "cmd/app/main.go",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gogs/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp GetFileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Language != "go" {
		t.Errorf("expected language 'go', got %q", resp.Language)
	}
	if resp.Owner != "user" {
		t.Errorf("expected owner 'user', got %q", resp.Owner)
	}
	if resp.Encoding != "base64" {
		t.Errorf("expected encoding 'base64', got %q", resp.Encoding)
	}
}

func TestGogsHandler_GetFile_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		request GetFileRequest
		want    string
	}{
		{
			name:    "missing owner",
			request: GetFileRequest{Repo: "my-repo", Branch: "main", Path: "main.go"},
			want:    "owner",
		},
		{
			name:    "missing repo",
			request: GetFileRequest{Owner: "user", Branch: "main", Path: "main.go"},
			want:    "repo",
		},
		{
			name:    "missing branch",
			request: GetFileRequest{Owner: "user", Repo: "my-repo", Path: "main.go"},
			want:    "branch",
		},
		{
			name:    "missing path",
			request: GetFileRequest{Owner: "user", Repo: "my-repo", Branch: "main"},
			want:    "path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := gogs.NewGogsClient("http://unused.example.com", "test-token")
			handler := NewGogsHandler(client)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/gogs/file", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.GetFile(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d; body: %s", w.Code, w.Body.String())
			}

			var errResp errorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errResp.Code != "BAD_REQUEST" {
				t.Errorf("expected code BAD_REQUEST, got %q", errResp.Code)
			}
			if errResp.Error != "missing required field: "+tt.want {
				t.Errorf("expected error 'missing required field: %s', got %q", tt.want, errResp.Error)
			}
		})
	}
}

func TestGogsHandler_GetFile_InvalidBody(t *testing.T) {
	client := gogs.NewGogsClient("http://unused.example.com", "test-token")
	handler := NewGogsHandler(client)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gogs/file", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetFile(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var errResp errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "BAD_REQUEST" {
		t.Errorf("expected code BAD_REQUEST, got %q", errResp.Code)
	}
}

func TestGogsHandler_GetFile_NotFound(t *testing.T) {
	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "file not found"}`))
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	reqBody := GetFileRequest{
		Owner:  "user",
		Repo:   "my-repo",
		Branch: "main",
		Path:   "nonexistent.go",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gogs/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetFile(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "FILE_NOT_FOUND" {
		t.Errorf("expected code FILE_NOT_FOUND, got %q", errResp.Code)
	}
}

func TestGogsHandler_GetFile_TooLarge(t *testing.T) {
	fc := gogs.FileContent{
		Name:     "huge.bin",
		Path:     "huge.bin",
		Content:  "big",
		Encoding: "base64",
		Size:     2 << 20, // 2 MB
	}
	fcBody, _ := json.Marshal(fc)

	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(fcBody)
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	reqBody := GetFileRequest{
		Owner:  "user",
		Repo:   "my-repo",
		Branch: "main",
		Path:   "huge.bin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gogs/file", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.GetFile(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d; body: %s", w.Code, w.Body.String())
	}

	var errResp errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "FILE_TOO_LARGE" {
		t.Errorf("expected code FILE_TOO_LARGE, got %q", errResp.Code)
	}
}

// --- Auth middleware integration tests ---

func TestGogsHandler_AuthMiddleware_RejectsWithoutToken(t *testing.T) {
	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be called — auth middleware rejects first
		t.Error("Gogs server should not be called when auth fails")
		w.Write([]byte("[]"))
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(&mockAuthValidator{}))
		r.Get("/gogs/repos", handler.ListRepos)
		r.Post("/gogs/file", handler.GetFile)
	})

	// Request without Authorization header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGogsHandler_AuthMiddleware_AllowsWithToken(t *testing.T) {
	repos := []gogs.Repo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", DefaultBranch: "main"},
	}
	repoBody, _ := json.Marshal(repos)

	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(repoBody)
	}))
	defer gogsServer.Close()

	client := gogs.NewGogsClient(gogsServer.URL, "test-token")
	handler := NewGogsHandler(client)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(&mockAuthValidator{}))
		r.Get("/gogs/repos", handler.ListRepos)
		r.Post("/gogs/file", handler.GetFile)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	req.Header.Set("Authorization", "Bearer valid-jwt-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// --- Language inference tests ---

func TestInferLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"app.js", "javascript"},
		{"app.jsx", "javascript"},
		{"script.py", "python"},
		{"main.rs", "rust"},
		{"Main.java", "java"},
		{"Gemfile", "unknown"},
		{"config.yml", "yaml"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"style.css", "css"},
		{"page.html", "html"},
		{"query.sql", "sql"},
		{"README.md", "markdown"},
		{"app.rb", "ruby"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"Program.cs", "csharp"},
		{"page.php", "php"},
		{"app.swift", "swift"},
		{"app.kt", "kotlin"},
		{"App.scala", "scala"},
		{"run.sh", "shell"},
		{"data.xml", "xml"},
		{"Makefile", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := inferLanguage(tt.path)
			if result != tt.expected {
				t.Errorf("inferLanguage(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

// Verify token is never exposed in handler responses
func TestGogsHandler_TokenNotInResponse(t *testing.T) {
	repos := []gogs.Repo{
		{ID: 1, Name: "repo1", FullName: "user/repo1", DefaultBranch: "main"},
	}
	repoBody, _ := json.Marshal(repos)

	gogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(repoBody)
	}))
	defer gogsServer.Close()

	secretToken := "super-secret-gogs-token-xyz"
	client := gogs.NewGogsClient(gogsServer.URL, secretToken)
	handler := NewGogsHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gogs/repos", nil)
	w := httptest.NewRecorder()

	handler.ListRepos(w, req)

	body := w.Body.String()
	if containsString(body, secretToken) {
		t.Errorf("response should not contain the Gogs token, got: %s", body)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}