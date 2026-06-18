package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
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
	challenges, err := h.service.GetAll(r.Context())
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

	challenge, err := h.service.GetByID(r.Context(), id)
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