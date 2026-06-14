package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/supabase"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
)

// AuthHandler handles authentication HTTP endpoints.
type AuthHandler struct {
	supabaseClient *supabase.SupabaseClient
	authAdapter    *supabase.SupabaseAuthAdapter
	db             *sql.DB
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(supabaseClient *supabase.SupabaseClient, authAdapter *supabase.SupabaseAuthAdapter, db *sql.DB) *AuthHandler {
	return &AuthHandler{
		supabaseClient: supabaseClient,
		authAdapter:    authAdapter,
		db:             db,
	}
}

// Register handles user registration.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	resp, err := h.supabaseClient.SignUp(req.Email, req.Password)
	if err != nil {
		log.Printf("Registration failed: %v", err)
		http.Error(w, "Registration failed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Login handles user authentication.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	resp, err := h.supabaseClient.SignIn(req.Email, req.Password)
	if err != nil {
		log.Printf("Login failed: %v", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Logout handles user sign out.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing authorization header", http.StatusUnauthorized)
		return
	}

	// Extract token from "Bearer <token>"
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	if err := h.supabaseClient.SignOut(token); err != nil {
		log.Printf("Logout failed: %v", err)
		// Still return success to client - local session is cleared
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user's profile.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userIDStr := authmiddleware.GetUserID(r.Context())
	if userIDStr == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Query public.usuarios for user profile
	query := `
		SELECT id, email, COALESCE(display_name, ''), racha_dias, puntos_maestria, 
		       rango_actual, ultimo_intento_valido, created_at, updated_at
		FROM public.usuarios
		WHERE id = $1
	`

	var profile models.UserProfile
	var ultimoIntento sql.NullTime

	err := h.db.QueryRowContext(r.Context(), query, userIDStr).Scan(
		&profile.ID,
		&profile.Email,
		&profile.DisplayName,
		&profile.RachaDias,
		&profile.PuntosMaestria,
		&profile.RangoActual,
		&ultimoIntento,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Failed to query user profile: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if ultimoIntento.Valid {
		profile.UltimoIntento = &ultimoIntento.Time
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

// UserIDFromRequest extracts user ID from Chi context set by middleware.
func UserIDFromRequest(r *http.Request) string {
	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		return authmiddleware.GetUserID(r.Context())
	}
	return userID
}
