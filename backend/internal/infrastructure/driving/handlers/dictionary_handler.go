package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// DictionaryHandler is the HTTP driving adapter for the user's personal
// tech dictionary. It owns JSON parsing and write errors; the dark logic
// lives in services.DictionaryService so the same engine is reused by the
// MCP dictionary tools in Phase 4.
//
// Hexagonal contract: this type imports core/services, ports, and net/http
// adapters only. It does NOT touch database/sql or any Supabase SDK.
type DictionaryHandler struct {
	service *services.DictionaryService
}

// NewDictionaryHandler constructs a handler backed by the given service. The
// caller wires the chain so the same DictionaryService instance is reused
// for MCP tool calls — the handler does not own the service.
func NewDictionaryHandler(service *services.DictionaryService) *DictionaryHandler {
	return &DictionaryHandler{service: service}
}

// CreateTermRequest is the JSON shape consumed by POST /api/v1/dictionary.
// Field-level errors are mapped to 400 with the underlying service message.
type CreateTermRequest struct {
	Term        string   `json:"term"`
	Explanation string   `json:"explanation"`
	Category    string   `json:"category"`
	Example     *string  `json:"example,omitempty"`
	Synonyms    []string `json:"synonyms"`
	Tags        []string `json:"tags"`
}

// UpdateTermRequest is the JSON shape consumed by PUT /api/v1/dictionary/:id.
// All fields are required because the spec's PUT semantics replace the row
// (not PATCH); partial updates are not part of the dictionary MVP.
type UpdateTermRequest struct {
	Term        string   `json:"term"`
	Explanation string   `json:"explanation"`
	Category    string   `json:"category"`
	Example     *string  `json:"example,omitempty"`
	Synonyms    []string `json:"synonyms"`
	Tags        []string `json:"tags"`
}

// CreateTerm handles POST /api/v1/dictionary.
//
// Status codes:
//   - 201 Created — term persisted (matches spec "Create term via POST")
//   - 400 Bad Request — invalid payload (validation errors, invalid category,
//     short explanation), or unparseable JSON
//   - 401 Unauthorized — middleware gate (not handled here)
//   - 409 Conflict — duplicate (user_id, term) at the database level
//   - 500 Internal Server Error — unexpected failure (sanitized detail)
func (h *DictionaryHandler) CreateTerm(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())

	var req CreateTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	created, err := h.service.AddTerm(r.Context(), services.AddTermInput{
		UserID:      userID,
		Term:        req.Term,
		Explanation: req.Explanation,
		Category:    req.Category,
		Example:     req.Example,
		Synonyms:    req.Synonyms,
		Tags:        req.Tags,
	})
	if err != nil {
		writeDictionaryServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// GetTerm handles GET /api/v1/dictionary/:id.
//
// Spec mapping:
//   - 200 OK with the term — happy path
//   - 404 Not Found — missing id OR cross-user id (no information leak per
//     spec "Delete term owned by another user → 404")
func (h *DictionaryHandler) GetTerm(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing term id")
		return
	}

	term, err := h.service.GetTerm(r.Context(), userID, id)
	if err != nil {
		writeDictionaryServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, term)
}

// UpdateTerm handles PUT /api/v1/dictionary/:id.
//
// Status codes:
//   - 200 OK — term replaced (returns the new row from the service's
//     read-after-write fetch)
//   - 400 Bad Request — invalid category/short explanation during validation
//   - 404 Not Found — term belongs to another user or has been deleted
//   - 409 Conflict — updated term collides with another row owned by user
func (h *DictionaryHandler) UpdateTerm(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing term id")
		return
	}

	var req UpdateTermRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updated, err := h.service.UpdateTerm(r.Context(), userID, id, services.UpdateTermInput{
		Term:        req.Term,
		Explanation: req.Explanation,
		Category:    req.Category,
		Example:     req.Example,
		Synonyms:    req.Synonyms,
		Tags:        req.Tags,
	})
	if err != nil {
		writeDictionaryServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteTerm handles DELETE /api/v1/dictionary/:id.
//
// Spec mapping:
//   - 204 No Content — successful delete (matches spec "Delete term")
//   - 404 Not Found — id missing or owned by another user (no info leak)
func (h *DictionaryHandler) DeleteTerm(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing term id")
		return
	}

	if err := h.service.DeleteTerm(r.Context(), userID, id); err != nil {
		writeDictionaryServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListOrSearchTerms handles GET /api/v1/dictionary. It dispatches:
//
//   - ?q=<text>    → ranked full-text search (capped at 20)
//   - ?category=&tag= → filtered list
//   - no params    → all terms owned by user
//
// Per the spec, an empty `q` falls through to ListTerms (returns all
// terms). Search hits smaller than MaxResults are still returned with the
// same 200 + JSON array envelope.
func (h *DictionaryHandler) ListOrSearchTerms(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	tag := r.URL.Query().Get("tag")

	var (
		terms []*models.Term
		err   error
	)
	if query != "" {
		terms, err = h.service.SearchTerms(r.Context(), userID, query)
	} else {
		terms, err = h.service.ListTerms(r.Context(), userID, category, tag)
	}
	if err != nil {
		writeDictionaryServiceError(w, err)
		return
	}

	// Guarantee a stable `[]` not `null` JSON wire shape — spec scenario
	// "Empty query returns all terms" + frontend pagination.
	if terms == nil {
		terms = []*models.Term{}
	}
	writeJSON(w, http.StatusOK, terms)
}

// --- helpers --------------------------------------------------------------

// writeJSON encodes v as JSON with the given status code. It mirrors the
// shared helper used by the MCP server's responder.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONError writes a JSON error envelope with the given status. The
// wire shape `{ "error": "..." }` matches what the rest of the handlers
// package produces.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- error translation ---------------------------------------------------

// writeDictionaryServiceError maps dictionary-service errors to HTTP status
// codes per the S2 spec.
//
//	ErrDictionaryValidation   → 400 Bad Request with the underlying detail
//	ports.ErrTermNotFound     → 404 Not Found (no info leak)
//	ports.ErrTermConflict     → 409 Conflict with detail
//	anything else             → 500 Internal Server Error (sanitized message)
func writeDictionaryServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrDictionaryValidation):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ports.ErrTermNotFound):
		writeJSONError(w, http.StatusNotFound, "Term not found")
	case errors.Is(err, ports.ErrTermConflict):
		writeJSONError(w, http.StatusConflict, err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, "Internal error")
	}
}
