package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// ProgressHandler handles per-user language progress and learning-profile
// HTTP endpoints. All endpoints require JWT auth, and the path parameter :id
// MUST match the authenticated user's JWT subject.
type ProgressHandler struct {
	progressService *services.UserProgressService
}

// NewProgressHandler creates a new ProgressHandler.
func NewProgressHandler(progressService *services.UserProgressService) *ProgressHandler {
	return &ProgressHandler{progressService: progressService}
}

// progressListResponse is the JSON body for GET /api/v1/users/{id}/progress.
type progressListResponse struct {
	RangoGlobal string                   `json:"rangoGlobal"`
	Languages   []*models.LanguageProgress `json:"languages"`
}

// learningProfileUpdateBody is the JSON body for PUT /api/v1/users/{id}/learning-profile.
// Both fields are optional; only provided fields are applied as a partial update.
type learningProfileUpdateBody struct {
	Preferencias      *models.Preferencias      `json:"preferencias,omitempty"`
	EstiloAprendizaje *models.EstiloAprendizaje `json:"estiloAprendizaje,omitempty"`
}

// languageProgressUpdateBody is the JSON body for PUT /api/v1/users/{id}/progress/{lang}.
// Allows manual override of score-related fields (admin-like).
type languageProgressUpdateBody struct {
	Puntos          *int      `json:"puntos,omitempty"`
	Rango           *string   `json:"rango,omitempty"`
	TopicsDominados *[]string `json:"topicsDominados,omitempty"`
}

// GetAllProgress handles GET /api/v1/users/{id}/progress — returns all language
// progress rows for the user plus the computed rangoGlobal.
func (h *ProgressHandler) GetAllProgress(w http.ResponseWriter, r *http.Request) {
	if !checkPathUserMatchesJWT(w, r) {
		return
	}

	userID := authmiddleware.GetUserID(r.Context())

	languages, err := h.progressService.GetAllLanguageProgress(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load progress", http.StatusInternalServerError)
		return
	}
	if languages == nil {
		languages = []*models.LanguageProgress{}
	}

	rangoGlobal, err := h.progressService.GetGlobalRank(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to compute global rank", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progressListResponse{
		RangoGlobal: rangoGlobal,
		Languages:   languages,
	})
}

// GetLanguageProgress handles GET /api/v1/users/{id}/progress/{lang} — returns
// a single language progress row.
func (h *ProgressHandler) GetLanguageProgress(w http.ResponseWriter, r *http.Request) {
	if !checkPathUserMatchesJWT(w, r) {
		return
	}

	userID := authmiddleware.GetUserID(r.Context())
	language := chi.URLParam(r, "lang")
	if language == "" {
		http.Error(w, "Missing language parameter", http.StatusBadRequest)
		return
	}

	progress, err := h.progressService.GetLanguageProgress(r.Context(), userID, language)
	if err != nil {
		http.Error(w, "Failed to load language progress", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress)
}

// GetLearningProfile handles GET /api/v1/users/{id}/learning-profile — returns
// the learning profile (upserting a default if it doesn't exist yet).
func (h *ProgressHandler) GetLearningProfile(w http.ResponseWriter, r *http.Request) {
	if !checkPathUserMatchesJWT(w, r) {
		return
	}

	userID := authmiddleware.GetUserID(r.Context())

	profile, err := h.progressService.GetLearningProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load learning profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// UpdateLearningProfile handles PUT /api/v1/users/{id}/learning-profile —
// applies a partial update to the user's learning preferences and learning style.
func (h *ProgressHandler) UpdateLearningProfile(w http.ResponseWriter, r *http.Request) {
	if !checkPathUserMatchesJWT(w, r) {
		return
	}

	userID := authmiddleware.GetUserID(r.Context())

	// Fetch existing profile (creates default on first access).
	profile, err := h.progressService.GetLearningProfile(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load learning profile", http.StatusInternalServerError)
		return
	}

	// Decode partial update.
	var body learningProfileUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.Preferencias != nil {
		profile.Preferencias = *body.Preferencias
	}
	if body.EstiloAprendizaje != nil {
		profile.EstiloAprendizaje = *body.EstiloAprendizaje
	}

	if err := h.progressService.UpdateLearningProfile(r.Context(), profile); err != nil {
		http.Error(w, "Failed to update learning profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// UpdateLanguageProgress handles PUT /api/v1/users/{id}/progress/{lang} —
// allows a manual override of a language progress row (admin-like).
func (h *ProgressHandler) UpdateLanguageProgress(w http.ResponseWriter, r *http.Request) {
	if !checkPathUserMatchesJWT(w, r) {
		return
	}

	userID := authmiddleware.GetUserID(r.Context())
	language := chi.URLParam(r, "lang")
	if language == "" {
		http.Error(w, "Missing language parameter", http.StatusBadRequest)
		return
	}

	// Fetch existing row (creates default on first access).
	progress, err := h.progressService.GetLanguageProgress(r.Context(), userID, language)
	if err != nil {
		http.Error(w, "Failed to load language progress", http.StatusInternalServerError)
		return
	}

	var body languageProgressUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if body.Puntos != nil {
		progress.Puntos = *body.Puntos
		progress.Rango = models.RangoFromPuntosFree(progress.Puntos)
	}
	if body.Rango != nil {
		progress.Rango = *body.Rango
	}
	if body.TopicsDominados != nil {
		progress.TopicsDominados = *body.TopicsDominados
	}

	if err := h.progressService.UpdateLanguageProgress(r.Context(), progress); err != nil {
		http.Error(w, "Failed to update language progress", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress)
}

// checkPathUserMatchesJWT verifies that the path :id matches the JWT user_id
// from the auth context. Writes a 401 response on mismatch and returns false.
func checkPathUserMatchesJWT(w http.ResponseWriter, r *http.Request) bool {
	pathUserID := chi.URLParam(r, "id")
	jwtUserID := authmiddleware.GetUserID(r.Context())
	if pathUserID == "" || jwtUserID == "" || pathUserID != jwtUserID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized: user ID mismatch"})
		return false
	}
	return true
}