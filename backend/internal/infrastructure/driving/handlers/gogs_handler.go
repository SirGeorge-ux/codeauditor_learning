package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/gogs"
)

// GogsHandler handles HTTP requests for the Gogs proxy endpoints.
type GogsHandler struct {
	gogsClient *gogs.GogsClient
}

// NewGogsHandler creates a new GogsHandler.
func NewGogsHandler(client *gogs.GogsClient) *GogsHandler {
	return &GogsHandler{gogsClient: client}
}

// GetFileRequest represents the JSON body for POST /api/v1/gogs/file.
type GetFileRequest struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// GetFileResponse represents the JSON response for POST /api/v1/gogs/file.
type GetFileResponse struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
}

// errorResponse represents a structured JSON error.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// ListRepos handles GET /api/v1/gogs/repos.
func (h *GogsHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.gogsClient.ListRepos(r.Context())
	if err != nil {
		writeGogsError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(repos)
}

// GetFile handles POST /api/v1/gogs/file.
func (h *GogsHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	var req GetFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse{
			Error: "invalid request body",
			Code:  "BAD_REQUEST",
		})
		return
	}

	// Validate required fields
	if missing := validateGetFileRequest(&req); missing != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse{
			Error: "missing required field: " + missing,
			Code:  "BAD_REQUEST",
		})
		return
	}

	fc, err := h.gogsClient.GetFileContents(r.Context(), req.Owner, req.Repo, req.Branch, req.Path)
	if err != nil {
		writeGogsError(w, err)
		return
	}

	resp := GetFileResponse{
		Owner:    req.Owner,
		Repo:     req.Repo,
		Branch:   req.Branch,
		Path:     fc.Path,
		Content:  fc.Content,
		Encoding: fc.Encoding,
		Language: inferLanguage(fc.Path),
		Size:     fc.Size,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// writeGogsError maps a GogsError to an appropriate HTTP response.
func writeGogsError(w http.ResponseWriter, err error) {
	gogsErr, ok := err.(*gogs.GogsError)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(errorResponse{
			Error: "internal server error",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(gogsErr.Status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: gogsErr.Message,
		Code:  gogsErr.Code,
	})
}

// validateGetFileRequest checks all required fields and returns the first missing one.
func validateGetFileRequest(req *GetFileRequest) string {
	if req.Owner == "" {
		return "owner"
	}
	if req.Repo == "" {
		return "repo"
	}
	if req.Branch == "" {
		return "branch"
	}
	if req.Path == "" {
		return "path"
	}
	return ""
}

// inferLanguage maps file extensions to language identifiers.
func inferLanguage(path string) string {
	// Get the file extension
	idx := strings.LastIndex(path, ".")
	if idx < 0 || idx == len(path)-1 {
		return "unknown"
	}
	ext := strings.ToLower(path[idx+1:])

	switch ext {
	case "go":
		return "go"
	case "ts":
		return "typescript"
	case "tsx":
		return "typescript"
	case "js":
		return "javascript"
	case "jsx":
		return "javascript"
	case "py":
		return "python"
	case "rs":
		return "rust"
	case "java":
		return "java"
	case "rb":
		return "ruby"
	case "c":
		return "c"
	case "cpp", "cc", "cxx":
		return "cpp"
	case "cs":
		return "csharp"
	case "php":
		return "php"
	case "swift":
		return "swift"
	case "kt":
		return "kotlin"
	case "scala":
		return "scala"
	case "groovy":
		return "groovy"
	case "sh":
		return "bash"
	case "zig":
		return "zig"
	case "yml", "yaml":
		return "yaml"
	case "json":
		return "json"
	case "xml":
		return "xml"
	case "html":
		return "html"
	case "css":
		return "css"
	case "sql":
		return "sql"
	case "r":
		return "r"
	case "hs":
		return "haskell"
	case "ex", "exs":
		return "elixir"
	case "clj":
		return "clojure"
	case "sol":
		return "solidity"
	case "erl":
		return "erlang"
	case "dart":
		return "dart"
	case "jl":
		return "julia"
	case "nim":
		return "nim"
	case "ps1":
		return "powershell"
	case "m":
		return "objective-c"
	case "fs", "fsx":
		return "fsharp"
	case "cbl", "cob":
		return "cobol"
	case "rkt":
		return "racket"
	case "md":
		return "markdown"
	default:
		return "unknown"
	}
}
