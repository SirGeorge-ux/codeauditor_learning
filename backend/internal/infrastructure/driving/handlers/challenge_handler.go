package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// ChallengeHandler handles challenge HTTP endpoints.
type ChallengeHandler struct {
	service *services.ChallengeService
}

// NewChallengeHandler creates a new ChallengeHandler.
func NewChallengeHandler(service *services.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{service: service}
}

// ListChallenges handles GET /api/v1/challenges.
func (h *ChallengeHandler) ListChallenges(w http.ResponseWriter, r *http.Request) {
	userID := authmiddleware.GetUserID(r.Context())

	challenges, err := h.service.GetAll(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to load challenges", http.StatusInternalServerError)
		return
	}

	if challenges == nil {
		challenges = []models.Challenge{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(challenges)
}

// GetChallenge handles GET /api/v1/challenges/{id}.
func (h *ChallengeHandler) GetChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Missing challenge ID", http.StatusBadRequest)
		return
	}

	userID := authmiddleware.GetUserID(r.Context())

	challenge, err := h.service.GetByID(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, services.ErrChallengeNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Challenge not found"})
			return
		}
		http.Error(w, "Failed to load challenge", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(challenge)
}

// CreateChallenge handles POST /api/v1/challenges.
func (h *ChallengeHandler) CreateChallenge(w http.ResponseWriter, r *http.Request) {
	var input models.CreateChallengeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Basic field validation
	if input.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Title is required"})
		return
	}

	userID := authmiddleware.GetUserID(r.Context())

	challenge, created, err := h.service.Create(r.Context(), input, userID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidDifficulty) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Error(w, "Failed to create challenge", http.StatusInternalServerError)
		return
	}

	// Return 200 for dedup (existing challenge), 201 for new insert
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(challenge)
}