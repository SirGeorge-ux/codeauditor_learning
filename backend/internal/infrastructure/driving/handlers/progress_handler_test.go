package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- mock progress repository (implements ports.UserProgressRepository) ---

type mockProgressRepo struct {
	languages map[string]*models.LanguageProgress
	profiles  map[string]*models.LearningProfile
}

func newMockProgressRepo() *mockProgressRepo {
	return &mockProgressRepo{
		languages: make(map[string]*models.LanguageProgress),
		profiles:  make(map[string]*models.LearningProfile),
	}
}

func progressKey(userID, language string) string { return userID + "|" + language }

var _ ports.UserProgressRepository = (*mockProgressRepo)(nil)

func (m *mockProgressRepo) GetLanguageProgress(_ context.Context, userID, language string) (*models.LanguageProgress, error) {
	if p, ok := m.languages[progressKey(userID, language)]; ok {
		return cloneMockProgress(p), nil
	}
	def := &models.LanguageProgress{
		UserID:                userID,
		Language:              language,
		Rango:                 "F",
		Puntos:                0,
		ChallengesCompletados: 0,
		ChallengesIntentados:  0,
		TasaExito:             0,
		TopicsDominados:       []string{},
		CompletedChallengeIDs: []string{},
		RangoCodeHealth:       "Junior",
		PuntosCodeHealth:      0,
	}
	m.languages[progressKey(userID, language)] = def
	return cloneMockProgress(def), nil
}

func (m *mockProgressRepo) UpdateLanguageProgress(_ context.Context, p *models.LanguageProgress) error {
	m.languages[progressKey(p.UserID, p.Language)] = cloneMockProgress(p)
	return nil
}

func (m *mockProgressRepo) GetAllLanguageProgress(_ context.Context, userID string) ([]*models.LanguageProgress, error) {
	var out []*models.LanguageProgress
	for k, p := range m.languages {
		if len(k) >= len(userID)+1 && k[:len(userID)] == userID && k[len(userID)] == '|' {
			out = append(out, cloneMockProgress(p))
		}
	}
	return out, nil
}

func (m *mockProgressRepo) GetLearningProfile(_ context.Context, userID string) (*models.LearningProfile, error) {
	if p, ok := m.profiles[userID]; ok {
		return p, nil
	}
	def := models.DefaultLearningProfile(userID)
	m.profiles[userID] = def
	return def, nil
}

func (m *mockProgressRepo) UpdateLearningProfile(_ context.Context, p *models.LearningProfile) error {
	m.profiles[p.UserID] = p
	return nil
}

func cloneMockProgress(p *models.LanguageProgress) *models.LanguageProgress {
	if p == nil {
		return nil
	}
	cp := *p
	if p.TopicsDominados != nil {
		cp.TopicsDominados = append([]string(nil), p.TopicsDominados...)
	}
	if p.CompletedChallengeIDs != nil {
		cp.CompletedChallengeIDs = append([]string(nil), p.CompletedChallengeIDs...)
	}
	return &cp
}

// --- mock auth validator (returns configurable user ID) ---

type mockUserIDValidator struct {
	userID string
}

func (m *mockUserIDValidator) ValidateToken(_ context.Context, _ string) error { return nil }
func (m *mockUserIDValidator) UserIDFromToken(_ string) (string, error)          { return m.userID, nil }

// seedProgress adds a LanguageProgress row to the mock repo.
func (m *mockProgressRepo) seedProgress(p *models.LanguageProgress) {
	m.languages[progressKey(p.UserID, p.Language)] = p
}

// buildProgressRouter creates a Chi router with auth middleware (configurable
// user ID) and the progress handler routes, matching the production wiring.
func buildProgressRouter(userID string, handler *ProgressHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(&mockUserIDValidator{userID: userID}))
		r.Get("/users/{id}/progress", handler.GetAllProgress)
		r.Get("/users/{id}/progress/{lang}", handler.GetLanguageProgress)
		r.Put("/users/{id}/progress/{lang}", handler.UpdateLanguageProgress)
		r.Get("/users/{id}/learning-profile", handler.GetLearningProfile)
		r.Put("/users/{id}/learning-profile", handler.UpdateLearningProfile)
	})
	return r
}

// --- Auth gating tests ---

func TestProgressHandler_GetAllProgress_MismatchedUserID_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("jwt-user-123", handler)

	// Path :id is "different-user" but JWT says "jwt-user-123"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/different-user/progress", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for mismatched user_id, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestProgressHandler_GetLanguageProgress_MismatchedUserID_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("real-user", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/attacker/progress/typescript", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestProgressHandler_GetLearningProfile_MismatchedUserID_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("real-user", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/someone-else/learning-profile", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestProgressHandler_UpdateLearningProfile_MismatchedUserID_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("real-user", handler)

	body, _ := json.Marshal(map[string]any{"preferencias": map[string]string{"idioma": "en"}})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/wrong-user/learning-profile", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake-jwt")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestProgressHandler_NoAuthHeader_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("real-user", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/real-user/progress", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d", w.Code)
	}
}

// --- Success path tests ---

func TestProgressHandler_GetAllProgress_SuccessWithRangoGlobal(t *testing.T) {
	repo := newMockProgressRepo()
	repo.seedProgress(&models.LanguageProgress{
		UserID: "u1", Language: "typescript", Rango: "C", Puntos: 500,
	})
	repo.seedProgress(&models.LanguageProgress{
		UserID: "u1", Language: "go", Rango: "B", Puntos: 1000,
	})
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u1", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/progress", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp progressListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.RangoGlobal == "" {
		t.Errorf("expected non-empty rangoGlobal")
	}
	if len(resp.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(resp.Languages))
	}
}

func TestProgressHandler_GetAllProgress_EmptyUser_ReturnsJunior(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("new-user", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/new-user/progress", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp progressListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.RangoGlobal != "Junior" {
		t.Errorf("expected rangoGlobal 'Junior' for empty user, got %q", resp.RangoGlobal)
	}
	if len(resp.Languages) != 0 {
		t.Errorf("expected 0 languages for new user, got %d", len(resp.Languages))
	}
}

func TestProgressHandler_GetLanguageProgress_Success(t *testing.T) {
	repo := newMockProgressRepo()
	repo.seedProgress(&models.LanguageProgress{
		UserID: "u2", Language: "python", Rango: "A", Puntos: 2500,
	})
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u2", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u2/progress/python", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var progress models.LanguageProgress
	if err := json.Unmarshal(w.Body.Bytes(), &progress); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if progress.Language != "python" {
		t.Errorf("expected language 'python', got %q", progress.Language)
	}
	if progress.Rango != "A" {
		t.Errorf("expected rango 'A', got %q", progress.Rango)
	}
}

func TestProgressHandler_GetLanguageProgress_DefaultRowCreatedOnFirstRead(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u3", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u3/progress/rust", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var progress models.LanguageProgress
	if err := json.Unmarshal(w.Body.Bytes(), &progress); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if progress.Rango != "F" {
		t.Errorf("expected default rango 'F', got %q", progress.Rango)
	}
	if progress.RangoCodeHealth != "Junior" {
		t.Errorf("expected default rangoCodeHealth 'Junior', got %q", progress.RangoCodeHealth)
	}
}

func TestProgressHandler_GetLearningProfile_Success_ReturnsDefaults(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u4", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u4/learning-profile", nil)
	req.Header.Set("Authorization", "Bearer fake-jwt")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var profile models.LearningProfile
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if profile.Preferencias.Idioma != "es" {
		t.Errorf("expected default idioma 'es', got %q", profile.Preferencias.Idioma)
	}
	if profile.Preferencias.NivelSocratismo != 2 {
		t.Errorf("expected default nivel_socratismo 2, got %d", profile.Preferencias.NivelSocratismo)
	}
}

func TestProgressHandler_UpdateLearningProfile_Success_UpdatesPreferences(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u5", handler)

	body, _ := json.Marshal(map[string]any{
		"preferencias": map[string]any{
			"idioma":          "en",
			"nivelSocratismo": 0,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u5/learning-profile", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake-jwt")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var updated models.LearningProfile
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if updated.Preferencias.Idioma != "en" {
		t.Errorf("expected updated idioma 'en', got %q", updated.Preferencias.Idioma)
	}
	if updated.Preferencias.NivelSocratismo != 0 {
		t.Errorf("expected nivel_socratismo 0, got %d", updated.Preferencias.NivelSocratismo)
	}
}

func TestProgressHandler_UpdateLearningProfile_InvalidBody_Returns400(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u6", handler)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u6/learning-profile", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer fake-jwt")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestProgressHandler_UpdateLanguageProgress_Success_OverridesPuntos(t *testing.T) {
	repo := newMockProgressRepo()
	repo.seedProgress(&models.LanguageProgress{
		UserID: "u7", Language: "go", Rango: "F", Puntos: 0,
	})
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u7", handler)

	body, _ := json.Marshal(map[string]any{"puntos": 900})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/u7/progress/go", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake-jwt")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var updated models.LanguageProgress
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if updated.Puntos != 900 {
		t.Errorf("expected puntos 900, got %d", updated.Puntos)
	}
	// Rango should have been recalculated from puntos. 900 falls in B range (< 2000).
	if updated.Rango != "B" {
		t.Errorf("expected rango 'B' for 900 puntos, got %q", updated.Rango)
	}
}

func TestProgressHandler_UpdateLanguageProgress_MismatchedUserID_Returns401(t *testing.T) {
	repo := newMockProgressRepo()
	svc := services.NewUserProgressService(repo)
	handler := NewProgressHandler(svc)
	r := buildProgressRouter("u8", handler)

	body, _ := json.Marshal(map[string]any{"puntos": 100})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/attacker/progress/go", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer fake-jwt")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for mismatched user, got %d; body: %s", w.Code, w.Body.String())
	}
}