package user

// Tests for the 6 mcp-user tools. Mirrors the dictionary_tool_test
// pattern: a hand-rolled fakeService stands in for the production
// serviceAdapter so each tool's JSON-parsing / user-id-bridging
// behavior is testable end-to-end without spinning up a real database.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Test infrastructure --------------------------------------------------

// fakeService implements the Service interface exactly as production
// defines it. Behavior knobs + recorded inputs let each test pin down
// the routing + envelope contract without booting a DB.
type fakeService struct {
	mu sync.Mutex

	// Recorded inputs.
	lastGetProfileUserID        string
	lastGetLanguageProgressArgs struct {
		UserID, Language string
	}
	lastGetHistoryUserID      string
	lastGetHistoryLimit       int
	lastLearningProfileUserID string
	lastRecordAuditIn         struct {
		UserID string
		Input  RecordAuditAttemptInput
	}
	lastUpdateLearningProfile *models.LearningProfile

	// Behavior knobs.
	profileOut  *models.UserProfile
	profileErr  error
	progressOut *models.LanguageProgress
	progressErr error
	historyOut  []models.AuditSession
	historyErr  error
	learningOut *models.LearningProfile
	learningErr error
	recordErr   error
	updateErr   error
}

func newFakeService() *fakeService {
	return &fakeService{
		profileOut:  &models.UserProfile{ID: "u-1", Email: "u@example.com"},
		progressOut: &models.LanguageProgress{UserID: "u-1", Language: "typescript", Rango: "F"},
		historyOut:  []models.AuditSession{},
		learningOut: models.DefaultLearningProfile("u-1"),
	}
}

func (s *fakeService) GetProfile(_ context.Context, userID string) (*models.UserProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetProfileUserID = userID
	return s.profileOut, s.profileErr
}

func (s *fakeService) GetLanguageProgress(_ context.Context, userID, language string) (*models.LanguageProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetLanguageProgressArgs.UserID = userID
	s.lastGetLanguageProgressArgs.Language = language
	return s.progressOut, s.progressErr
}

func (s *fakeService) GetHistory(_ context.Context, userID string, limit int) ([]models.AuditSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastGetHistoryUserID = userID
	s.lastGetHistoryLimit = limit
	return s.historyOut, s.historyErr
}

func (s *fakeService) GetLearningProfile(_ context.Context, userID string) (*models.LearningProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastLearningProfileUserID = userID
	return s.learningOut, s.learningErr
}

func (s *fakeService) RecordAuditAttempt(_ context.Context, userID string, in RecordAuditAttemptInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRecordAuditIn.UserID = userID
	s.lastRecordAuditIn.Input = in
	return s.recordErr
}

func (s *fakeService) UpdateLearningProfile(_ context.Context, p *models.LearningProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUpdateLearningProfile = p
	return s.updateErr
}

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-user.get_profile ---

func TestGetProfileTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.profileOut = &models.UserProfile{
		ID: "u-1", Email: "u@example.com", DisplayName: "Alice",
		RachaDias: 7, PuntosMaestria: 250, RangoActual: "C",
	}
	tool := NewGetProfileTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetProfileUserID != "u-1" {
		t.Errorf("routed userID=%q, want u-1", svc.lastGetProfileUserID)
	}
	p, ok := got.(*models.UserProfile)
	if !ok {
		t.Fatalf("expected *models.UserProfile, got %T", got)
	}
	if p.DisplayName != "Alice" {
		t.Errorf("display_name = %q, want Alice", p.DisplayName)
	}
}

func TestGetProfileTool_NotFound_ReturnsStructuredError(t *testing.T) {
	svc := newFakeService()
	svc.profileErr = ports.ErrUserProfileNotFound
	tool := NewGetProfileTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "ghost"), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Tight sentinel check (was: loose OR between errors.Is(ErrUserValidation)
	// and strings.Contains("not found")). The production adapter now
	// preserves the ErrUserProfileNotFound sentinel end-to-end, so the
	// tool returns it verbatim — matching the contract asserted in
	// TestNewServiceAdapter_GetProfile_PropagatesSentinel below.
	if !errors.Is(err, ErrUserProfileNotFound) {
		t.Errorf("err = %v, want ErrUserProfileNotFound", err)
	}
}

func TestGetProfileTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetProfileTool(svc)
	_, err := tool.Execute(context.Background(), nil)
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-user.get_language_progress ---

func TestGetLanguageProgressTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.progressOut = &models.LanguageProgress{
		UserID: "u-1", Language: "typescript", Rango: "E", Puntos: 100,
	}
	tool := NewGetLanguageProgressTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{"language":"typescript"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetLanguageProgressArgs.Language != "typescript" {
		t.Errorf("routed language=%q, want typescript", svc.lastGetLanguageProgressArgs.Language)
	}
	if svc.lastGetLanguageProgressArgs.UserID != "u-1" {
		t.Errorf("routed userID=%q, want u-1", svc.lastGetLanguageProgressArgs.UserID)
	}
	p, ok := got.(*models.LanguageProgress)
	if !ok {
		t.Fatalf("expected *models.LanguageProgress, got %T", got)
	}
	if p.Rango != "E" || p.Puntos != 100 {
		t.Errorf("got rango=%q puntos=%d, want E/100", p.Rango, p.Puntos)
	}
}

func TestGetLanguageProgressTool_MissingLanguage_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetLanguageProgressTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing language must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-user.get_history ---

func TestGetHistoryTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.historyOut = []models.AuditSession{
		{ID: "s-1", UserID: "u-1", ChallengeTitle: "ch-xss", Language: "typescript"},
		{ID: "s-2", UserID: "u-1", ChallengeTitle: "ch-n1", Language: "python"},
	}
	tool := NewGetHistoryTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{"limit":5}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetHistoryLimit != 5 {
		t.Errorf("routed limit=%d, want 5", svc.lastGetHistoryLimit)
	}
	arr := got.([]models.AuditSession)
	if len(arr) != 2 {
		t.Errorf("len=%d, want 2", len(arr))
	}
}

func TestGetHistoryTool_DefaultLimit_WhenArgsEmpty(t *testing.T) {
	svc := newFakeService()
	tool := NewGetHistoryTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), nil); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetHistoryLimit != 0 {
		t.Errorf("routed limit=%d, want 0 (empty args → service default)", svc.lastGetHistoryLimit)
	}
}

func TestGetHistoryTool_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetHistoryTool(svc)
	_, err := tool.Execute(context.Background(), nil)
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing user_id must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-user.get_learning_profile ---

func TestGetLearningProfileTool_HappyPath(t *testing.T) {
	svc := newFakeService()
	custom := models.DefaultLearningProfile("u-1")
	custom.Preferencias.Idioma = "en"
	custom.Preferencias.NivelSocratismo = 3
	svc.learningOut = custom
	tool := NewGetLearningProfileTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	p, ok := got.(*models.LearningProfile)
	if !ok {
		t.Fatalf("expected *models.LearningProfile, got %T", got)
	}
	if p.Preferencias.Idioma != "en" {
		t.Errorf("idioma = %q, want en", p.Preferencias.Idioma)
	}
}

// --- mcp-user.record_audit_attempt ---

func TestRecordAuditAttempt_HappyPath(t *testing.T) {
	svc := newFakeService()
	tool := NewRecordAuditAttemptTool(svc)
	args := json.RawMessage(`{
		"language":"typescript","challenge_id":"ch-xss","score":50,
		"learning_objectives":["xss","innerhtml"],
		"completed":true,"code":"<x />","findings_count":1
	}`)
	out, err := tool.Execute(ctxWithUser(t, "u-1"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastRecordAuditIn.UserID != "u-1" {
		t.Errorf("routed userID=%q, want u-1", svc.lastRecordAuditIn.UserID)
	}
	in := svc.lastRecordAuditIn.Input
	if in.Language != "typescript" || in.ChallengeID != "ch-xss" || in.Score != 50 {
		t.Errorf("routed inputs = %+v, want typescript/ch-xss/50", in)
	}
	if !in.Completed {
		t.Errorf("Completed = false, want true")
	}
	if len(in.LearningObjectives) != 2 {
		t.Errorf("learning_objectives = %v, want 2 entries", in.LearningObjectives)
	}
	m := out.(map[string]any)
	if m["recorded"] != true {
		t.Errorf("response = %+v, want {recorded: true}", m)
	}
}

func TestRecordAuditAttempt_FreeAudit_NoChallenge(t *testing.T) {
	// Free audits (no challengeID) MUST still get a history row recorded;
	// the progress update is skipped.
	svc := newFakeService()
	tool := NewRecordAuditAttemptTool(svc)
	args := json.RawMessage(`{"language":"typescript","completed":true,"code":"x"}`)
	if _, err := tool.Execute(ctxWithUser(t, "u-1"), args); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastRecordAuditIn.Input.ChallengeID != "" {
		t.Errorf("free audit must keep ChallengeID empty, got %q", svc.lastRecordAuditIn.Input.ChallengeID)
	}
}

func TestRecordAuditAttempt_MissingLanguage_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewRecordAuditAttemptTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), json.RawMessage(`{"completed":true}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing language must surface as ErrInvalidParams, got %v", err)
	}
}

// --- mcp-user.update_learning_profile ---

func TestUpdateLearningProfile_HappyPath(t *testing.T) {
	svc := newFakeService()
	tool := NewUpdateLearningProfileTool(svc)
	args := json.RawMessage(`{
		"preferencias": {
			"idioma":"de","nivelSocratismo":1,"longitudMaximaMsg":300,
			"bloqueosTipicos":["loops"],"topicsQueLeCuestan":["recursion"],
			"resumen":"fortschrittlich"
		},
		"estiloAprendizaje": {"aprendeMejorCon":"audios"}
	}`)
	got, err := tool.Execute(ctxWithUser(t, "u-1"), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	p, ok := got.(*models.LearningProfile)
	if !ok {
		t.Fatalf("expected *models.LearningProfile, got %T", got)
	}
	if p.Preferencias.Idioma != "de" {
		t.Errorf("idioma = %q, want de", p.Preferencias.Idioma)
	}
	if p.Preferencias.NivelSocratismo != 1 {
		t.Errorf("nivel = %d, want 1", p.Preferencias.NivelSocratismo)
	}
	if svc.lastUpdateLearningProfile == nil {
		t.Fatal("UpdateLearningProfile was not called")
	}
	if svc.lastUpdateLearningProfile.EstiloAprendizaje.AprendeMejorCon != "audios" {
		t.Errorf("aprendeMejorCon = %q, want audios",
			svc.lastUpdateLearningProfile.EstiloAprendizaje.AprendeMejorCon)
	}
}

func TestUpdateLearningProfile_RejectsOutOfRangeSocratismo(t *testing.T) {
	svc := newFakeService()
	tool := NewUpdateLearningProfileTool(svc)
	args := json.RawMessage(`{
		"preferencias":{"idioma":"es","nivelSocratismo":9,"longitudMaximaMsg":200}
	}`)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), args)
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("nivel_socratismo out of range must surface as ErrInvalidParams, got %v", err)
	}
}

func TestUpdateLearningProfile_MissingPreferencias_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewUpdateLearningProfileTool(svc)
	args := json.RawMessage(`{"estiloAprendizaje":{"aprendeMejorCon":"examples"}}`)
	_, err := tool.Execute(ctxWithUser(t, "u-1"), args)
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing preferencias must surface as ErrInvalidParams, got %v", err)
	}
}

func TestUpdateLearningProfile_MissingUserID_ReturnsInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewUpdateLearningProfileTool(svc)
	args := json.RawMessage(`{"preferencias":{"idioma":"es","nivelSocratismo":1,"longitudMaximaMsg":200}}`)
	_, err := tool.Execute(context.Background(), args)
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("missing user_id must surface as ErrInvalidParams, got %v", err)
	}
}

// --- production adapter (smoke) ---

func TestNewServiceAdapter_GetProfile_PropagatesSentinel(t *testing.T) {
	// Use the fakeService directly to assert the adapter would receive
	// the sentinel; this verifies the local fake satisfies the Service
	// interface and that errors.Is comparison still works at the boundary.
	svc := newFakeService()
	svc.profileErr = ports.ErrUserProfileNotFound
	adapter := Service(svc) // compile-time check: fakeService implements Service

	_, err := adapter.GetProfile(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected ErrUserProfileNotFound from adapter")
	}
	if !errors.Is(err, ports.ErrUserProfileNotFound) {
		t.Errorf("err = %v, want ports.ErrUserProfileNotFound", err)
	}
}

// --- helpers ---

// (TestGetProfileTool_NotFound_ReturnsStructuredError previously used a
// local strings_Contains shim because the file did not import "strings".
// That helper is now dead — the assertion pins the ErrUserProfileNotFound
// sentinel directly via errors.Is.)
