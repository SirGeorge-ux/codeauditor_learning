// Package user implements the mcp-user tool group of the MCP monolith.
// It exposes six driving adapters — get_profile, get_language_progress,
// get_history, get_learning_profile, record_audit_attempt,
// update_learning_profile — that wrap the user's per-language progress,
// audit history, and learning profile for the tutor LLM.
//
// Hexagonal contract: this package imports ONLY the local Service
// interface (and shared mcpkit/mcpctx helpers) plus core/services for
// the domain shapes. It MUST NOT import database/sql, net/http, or any
// external service adapter. The wiring in cmd/api/main.go adapts the
// production UserService + UserProgressService + AuditHistoryService to
// the local Service so the tools stay swappable for tests.
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit/mcpctx"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ErrInvalidParams is the shared sentinel the MCP dispatcher maps to
// JSON-RPC -32602. Re-exported as a type alias so tool-local code can use
// the short name; the underlying value lives in mcpkit/mcpctx so the
// dispatcher's errors.Is check matches across the boundary.
var ErrInvalidParams = mcpctx.ErrInvalidParams

// ErrUserProfileNotFound mirrors ports.ErrUserProfileNotFound so the
// get_profile tool can branch on it without re-importing ports for the
// sentinel value.
var ErrUserProfileNotFound = ports.ErrUserProfileNotFound

// userIDKey is the local context key for the authenticated user_id.
type userIDKey struct{}

// ContextWithUserID returns a child context carrying userID.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// userIDFromContext extracts the authenticated user id stored in ctx. It
// first consults this package's own key, then falls back to the canonical
// MCP bridge key (mcpctx.AuthUserIDKey). Returns "" when neither was
// stamped — callers translate that into ErrInvalidParams.
func userIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDKey{}).(string); ok && v != "" {
		return v
	}
	return mcpctx.UserIDFromContext(ctx)
}

// propertyArg is the standardized JSON envelope produced for LLM tool
// routing.
type propertyArg struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// schema builds an OpenAI-compatible function schema with a single object
// parameter list. The shape matches the dictionary/tutor/sandbox tools so
// the tutor LLM sees a consistent contract across groups.
func schema(name, description string, props map[string]propertyArg, required []string) openai.Tool {
	return openai.Tool{
		Type: "function",
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
}

func requiredProps(names ...string) []string { return names }

// RecordAuditAttemptInput is the wire shape the record_audit_attempt tool
// accepts. Maps to the production service input (RecordAuditCompletion +
// SaveSession). We accept LearningObjectives even when Completed=false so
// the caller can submit them retroactively; the service ignores them in
// that case.
//
// Exported so the HTTP transport test (which lives in the mcp_test
// package outside the user package) can construct it for the fake
// Service's RecordAuditAttempt method.
type RecordAuditAttemptInput struct {
	Language           string   `json:"language"`
	ChallengeID        string   `json:"challenge_id"`
	Score              int      `json:"score,omitempty"`
	LearningObjectives []string `json:"learning_objectives,omitempty"`
	Completed          bool     `json:"completed"`
	Code               string   `json:"code,omitempty"`
	FindingsCount      int      `json:"findings_count,omitempty"`
}

// --- Service interface ----------------------------------------------------

// Service is the local interface every MCP user tool depends on. The
// production wiring supplies UserService + UserProgressService +
// AuditHistoryService through NewServiceAdapter (below); tests substitute
// a fake to verify each tool's behavior end-to-end.
type Service interface {
	// GetProfile returns the authenticated user's profile row.
	GetProfile(ctx context.Context, userID string) (*models.UserProfile, error)
	// GetLanguageProgress returns the per-language progress row, upserting
	// a default when the pair does not exist.
	GetLanguageProgress(ctx context.Context, userID, language string) (*models.LanguageProgress, error)
	// GetHistory returns the last N audit sessions (cap configurable; the
	// service clamps <=0 to 20).
	GetHistory(ctx context.Context, userID string, limit int) ([]models.AuditSession, error)
	// GetLearningProfile returns the learning profile, upserting a
	// default on first read.
	GetLearningProfile(ctx context.Context, userID string) (*models.LearningProfile, error)
	// RecordAuditAttempt records an audit attempt end-to-end: saves the
	// session to history + updates per-language progress (the progress
	// service handles the anti-gaming dedup). The service's internal
	// AuditHistoryService.SaveSession and UserProgressService
	// .RecordAuditCompletion decide which side-effects fire based on
	// `input.Completed` (full scoring vs. attempted-only).
	RecordAuditAttempt(ctx context.Context, userID string, input RecordAuditAttemptInput) error
	// UpdateLearningProfile persists the supplied learning profile.
	UpdateLearningProfile(ctx context.Context, profile *models.LearningProfile) error
}

// --- serviceAdapter: production services → Service ------------------------

// serviceAdapter bridges UserService + UserProgressService +
// AuditHistoryService to the local Service interface. It owns the
// cross-service record_audit_attempt workflow: SaveSession +
// RecordAuditCompletion happen in sequence so a session is recorded
// even when the progress update is a no-op (e.g. no challenge attached).
type serviceAdapter struct {
	userSvc     *services.UserService
	progressSvc *services.UserProgressService
	historySvc  *services.AuditHistoryService
}

// NewServiceAdapter returns the production wiring. The adapter lives in
// this package so the JSON envelope + cross-service choreography stay
// consistent with the tools' schemas.
func NewServiceAdapter(user *services.UserService, progress *services.UserProgressService, history *services.AuditHistoryService) Service {
	return &serviceAdapter{
		userSvc:     user,
		progressSvc: progress,
		historySvc:  history,
	}
}

func (a *serviceAdapter) GetProfile(ctx context.Context, userID string) (*models.UserProfile, error) {
	profile, err := a.userSvc.GetProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, services.ErrUserValidation) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
		return nil, err
	}
	return profile, nil
}

func (a *serviceAdapter) GetLanguageProgress(ctx context.Context, userID, language string) (*models.LanguageProgress, error) {
	p, err := a.progressSvc.GetLanguageProgress(ctx, userID, language)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (a *serviceAdapter) GetHistory(ctx context.Context, userID string, limit int) ([]models.AuditSession, error) {
	sessions, err := a.historySvc.GetHistory(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		return []models.AuditSession{}, nil
	}
	return sessions, nil
}

func (a *serviceAdapter) GetLearningProfile(ctx context.Context, userID string) (*models.LearningProfile, error) {
	p, err := a.progressSvc.GetLearningProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (a *serviceAdapter) RecordAuditAttempt(ctx context.Context, userID string, in RecordAuditAttemptInput) error {
	// Step 1: persist the audit session (history row). The service's
	// SaveSession internally constructs the AuditRequest; we use the
	// supplied `code` verbatim (or truncate) and FindingsCount as-is.
	if a.historySvc != nil {
		req := models.AuditRequest{
			ChallengeID: in.ChallengeID,
			Language:    in.Language,
			Code:        in.Code,
		}
		if in.Code == "" {
			req.Code = in.Code // keep empty
		}
		if err := a.historySvc.SaveSession(ctx, userID, req, in.FindingsCount); err != nil {
			return err
		}
	}
	// Step 2: update per-language progress. A blank challenge_id is a
	// no-op for the progress service (consistent with AuditService).
	if a.progressSvc != nil && in.ChallengeID != "" && in.Language != "" {
		if err := a.progressSvc.RecordAuditCompletion(ctx, userID, in.Language, in.ChallengeID, in.Score, in.LearningObjectives, in.Completed); err != nil {
			return err
		}
	}
	return nil
}

func (a *serviceAdapter) UpdateLearningProfile(ctx context.Context, profile *models.LearningProfile) error {
	if profile == nil {
		return fmt.Errorf("%w: profile is required", ErrInvalidParams)
	}
	return a.progressSvc.UpdateLearningProfile(ctx, profile)
}

// --- mcp-user.get_profile ------------------------------------------------

// getProfileTool returns the user's authenticated profile row.
type getProfileTool struct {
	svc Service
}

// NewGetProfileTool registers the get_profile action.
func NewGetProfileTool(svc Service) *getProfileTool { return &getProfileTool{svc: svc} }

func (t *getProfileTool) Name() string { return "mcp-user.get_profile" }

func (t *getProfileTool) Description() string {
	return "Fetch the authenticated user's profile row (id, email, display " +
		"name, racha_dias, puntos_maestria, rango_actual, ultimo_intento). " +
		"Returns ErrInvalidParams (-32602) for not-found or empty user_id. " +
		"Requires authenticated user_id in context."
}

func (t *getProfileTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{}, []string{})
}

func (t *getProfileTool) Execute(ctx context.Context, _ json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	profile, err := t.svc.GetProfile(ctx, userID)
	if err != nil {
		// Preserve the ErrUserProfileNotFound sentinel so callers can
		// errors.Is against it; the sentinel's own message is already
		// "user profile not found", so wrapping would just lose the
		// chain without adding information.
		return nil, err
	}
	return profile, nil
}

// --- mcp-user.get_language_progress ---------------------------------------

// getLanguageProgressTool returns the per-language progress row.
type getLanguageProgressTool struct {
	svc Service
}

// NewGetLanguageProgressTool registers the get_language_progress action.
func NewGetLanguageProgressTool(svc Service) *getLanguageProgressTool {
	return &getLanguageProgressTool{svc: svc}
}

func (t *getLanguageProgressTool) Name() string { return "mcp-user.get_language_progress" }

func (t *getLanguageProgressTool) Description() string {
	return "Fetch the authenticated user's progress row for a language " +
		"(rango F-S, puntos, challenges_completados/intentados, tasa_exito, " +
		"topics_dominados). Upserts a default row (rango=F, puntos=0) on " +
		"first read. Requires authenticated user_id in context."
}

func (t *getLanguageProgressTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language": {Type: "string", Description: "Lowercase language key (e.g. 'typescript', 'python')."},
		},
		requiredProps("language"),
	)
}

func (t *getLanguageProgressTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if a.Language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	p, err := t.svc.GetLanguageProgress(ctx, userID, a.Language)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// --- mcp-user.get_history -------------------------------------------------

// getHistoryTool returns the last N audit sessions.
type getHistoryTool struct {
	svc Service
}

// NewGetHistoryTool registers the get_history action.
func NewGetHistoryTool(svc Service) *getHistoryTool { return &getHistoryTool{svc: svc} }

func (t *getHistoryTool) Name() string { return "mcp-user.get_history" }

func (t *getHistoryTool) Description() string {
	return "Return the authenticated user's last N audit sessions " +
		"(id, challenge_title, language, code_snippet truncated 2000 chars, " +
		"findings_count, created_at). Default limit 20. " +
		"Requires authenticated user_id in context."
}

func (t *getHistoryTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"limit": {Type: "integer", Description: "Optional cap (1..200). Defaults to 20 when unset or <=0."},
		},
		[]string{},
	)
}

func (t *getHistoryTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var a struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
		}
	}
	sessions, err := t.svc.GetHistory(ctx, userID, a.Limit)
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		return []models.AuditSession{}, nil
	}
	return sessions, nil
}

// --- mcp-user.get_learning_profile ---------------------------------------

// getLearningProfileTool returns the user's learning profile.
type getLearningProfileTool struct {
	svc Service
}

// NewGetLearningProfileTool registers the get_learning_profile action.
func NewGetLearningProfileTool(svc Service) *getLearningProfileTool {
	return &getLearningProfileTool{svc: svc}
}

func (t *getLearningProfileTool) Name() string { return "mcp-user.get_learning_profile" }

func (t *getLearningProfileTool) Description() string {
	return "Fetch the authenticated user's learning profile (idioma, " +
		"nivel_socratismo, longitud_maxima_msg, bloqueos_tipicos, " +
		"topics_que_le_cuestan, resumen, estilo_aprendizaje, " +
		"ultima_actualizacion). Upserts default ('es', nivel=2) on first " +
		"read. Requires authenticated user_id in context."
}

func (t *getLearningProfileTool) Schema() openai.Tool {
	return schema(t.Name(), t.Description(), map[string]propertyArg{}, []string{})
}

func (t *getLearningProfileTool) Execute(ctx context.Context, _ json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	profile, err := t.svc.GetLearningProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

// --- mcp-user.record_audit_attempt --------------------------------------

// recordAuditAttemptTool records an audit attempt end-to-end.
type recordAuditAttemptTool struct {
	svc Service
}

// NewRecordAuditAttemptTool registers the record_audit_attempt action.
func NewRecordAuditAttemptTool(svc Service) *recordAuditAttemptTool {
	return &recordAuditAttemptTool{svc: svc}
}

func (t *recordAuditAttemptTool) Name() string { return "mcp-user.record_audit_attempt" }

func (t *recordAuditAttemptTool) Description() string {
	return "Record an audit attempt end-to-end: persists the session row AND " +
		"updates per-language progress. Pass completed=true for a successful " +
		"audit (full scoring path, anti-gaming dedup); pass completed=false to " +
		"record a failed attempt (challenges_intentados only, no double-count). " +
		"When challenge_id is empty, the history row is recorded but the progress " +
		"update is skipped (free audits). Requires authenticated user_id in context."
}

func (t *recordAuditAttemptTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"language":            {Type: "string", Description: "Target language."},
			"challenge_id":        {Type: "string", Description: "Challenge UUID when applicable; empty for free audits."},
			"score":               {Type: "integer", Description: "Awarded score (only counted when completed=true AND not a duplicate)."},
			"learning_objectives": {Type: "array", Description: "Optional topic ids claimed by this attempt."},
			"completed":           {Type: "boolean", Description: "True when the audit succeeded; false for failed attempts."},
			"code":                {Type: "string", Description: "Source code (truncated to 2000 chars when persisted)."},
			"findings_count":      {Type: "integer", Description: "Number of findings detected."},
		},
		requiredProps("language", "completed"),
	)
}

func (t *recordAuditAttemptTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	var in RecordAuditAttemptInput
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if in.Language == "" {
		return nil, fmt.Errorf("%w: language is required", ErrInvalidParams)
	}
	if err := t.svc.RecordAuditAttempt(ctx, userID, in); err != nil {
		return nil, err
	}
	return map[string]any{"recorded": true}, nil
}

// --- mcp-user.update_learning_profile ------------------------------------

// updateLearningProfileTool persists the user's learning profile.
type updateLearningProfileTool struct {
	svc Service
}

// NewUpdateLearningProfileTool registers the update_learning_profile action.
func NewUpdateLearningProfileTool(svc Service) *updateLearningProfileTool {
	return &updateLearningProfileTool{svc: svc}
}

func (t *updateLearningProfileTool) Name() string { return "mcp-user.update_learning_profile" }

func (t *updateLearningProfileTool) Description() string {
	return "Persist the authenticated user's learning profile. The body must " +
		"include preferencias (idioma, nivel_socratismo 0..3, " +
		"longitud_maxima_msg, bloqueos_tipicos, topics_que_le_cuestan, resumen) " +
		"and estilo_aprendizaje. Returns the persisted profile. " +
		"Requires authenticated user_id in context."
}

func (t *updateLearningProfileTool) Schema() openai.Tool {
	return schema(
		t.Name(), t.Description(),
		map[string]propertyArg{
			"preferencias":      {Type: "object", Description: "Preferencias object (idioma, nivelSocratismo, longitudMaximaMsg, bloqueosTipicos, topicsQueLeCuestan, resumen)."},
			"estiloAprendizaje": {Type: "object", Description: "EstiloAprendizaje object (aprendeMejorCon)."},
		},
		requiredProps("preferencias"),
	)
}

func (t *updateLearningProfileTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	userID := userIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("%w: missing user_id in context", ErrInvalidParams)
	}
	// Check the raw JSON for the required `preferencias` key so missing
	// vs present-but-empty are distinguishable (an empty object is a
	// legitimate input; a missing key is a structural mistake).
	var rawKeys map[string]json.RawMessage
	if err := json.Unmarshal(args, &rawKeys); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	if _, ok := rawKeys["preferencias"]; !ok {
		return nil, fmt.Errorf("%w: preferencias is required", ErrInvalidParams)
	}
	var body struct {
		Preferencias      models.Preferencias      `json:"preferencias"`
		EstiloAprendizaje models.EstiloAprendizaje `json:"estiloAprendizaje"`
	}
	if err := json.Unmarshal(args, &body); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, err.Error())
	}
	profile := models.DefaultLearningProfile(userID)
	profile.Preferencias = body.Preferencias
	profile.EstiloAprendizaje = body.EstiloAprendizaje
	// Validate nivelSocratismo range here so the LLM never gets a
	// stored domain value out of the configured 0..3 range.
	if profile.Preferencias.NivelSocratismo < 0 || profile.Preferencias.NivelSocratismo > 3 {
		return nil, fmt.Errorf("%w: nivel_socratismo must be 0..3", ErrInvalidParams)
	}
	if err := t.svc.UpdateLearningProfile(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}
