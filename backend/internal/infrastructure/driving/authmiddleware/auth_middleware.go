package authmiddleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// UserIDContextKey is the context key for user ID.
type UserIDContextKey struct{}

// AuthMiddleware creates a Chi middleware that validates JWT tokens.
func AuthMiddleware(validator ports.AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			// Extract token from "Bearer <token>"
			token := authHeader
			if len(authHeader) > 7 && strings.ToLower(authHeader[:7]) == "bearer " {
				token = authHeader[7:]
			}
			token = strings.TrimSpace(token)

			if token == "" {
				http.Error(w, "Missing token", http.StatusUnauthorized)
				return
			}

			// Validate token
			if err := validator.ValidateToken(r.Context(), token); err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Extract user ID from token
			userID, err := validator.UserIDFromToken(token)
			if err != nil {
				http.Error(w, "Failed to extract user ID: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// Store user ID in context
			ctx := context.WithValue(r.Context(), UserIDContextKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDContextKey{}).(string); ok {
		return userID
	}
	return ""
}

// UserIDParam middleware extracts user_id from chi URL params and validates JWT.
func UserIDParam(validator ports.AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			token := authHeader
			if len(authHeader) > 7 && strings.ToLower(authHeader[:7]) == "bearer " {
				token = authHeader[7:]
			}
			token = strings.TrimSpace(token)

			if token == "" {
				http.Error(w, "Missing token", http.StatusUnauthorized)
				return
			}

			if err := validator.ValidateToken(r.Context(), token); err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			userID, err := validator.UserIDFromToken(token)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey{}, userID)
			_ = chi.URLParam(r, "user_id") // unused but keeps pattern consistent
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
