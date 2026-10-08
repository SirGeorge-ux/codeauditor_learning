package challenges

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
)

// fakeService is the hand-rolled stand-in for *services.ChallengeService
// inside this package. It records the last inputs and lets each test pin
// down the response shape without touching the database.
type fakeService struct {
	mu sync.Mutex

	// Stored challenges keyed by id; per-user visibility is enforced by
	// GetChallenge and ListChallenges.
	store map[string]*models.Challenge

	// Knobs.
	listErr       error
	getErr        error
	createErr     error
	generateErr   error
	validateErr   error
	dedupHit      bool // CreateTempChallenge returns created=false
	missingID     string

	// Recorded.
	lastListUserID    string
	lastGetID         string
	lastGetReveal     bool
	lastCreateInput   challengeInputEnvelope
	lastCreateUserID  string
	lastGenerateUser  string
	lastGenerateHint  GenerateHint
	lastValidateIn    validateChallengeInput
}

func newFakeService() *fakeService {
	return &fakeService{store: map[string]*models.Challenge{}}
}

func (f *fakeService) ListChallenges(_ context.Context, userID string) ([]json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastListUserID = userID
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []json.RawMessage{}
	for _, c := range f.store {
		if c.UserID != nil && *c.UserID != "" && *c.UserID != userID {
			continue // cross-user isolation
		}
		raw, _ := json.Marshal(map[string]any{"id": c.ID, "title": c.Title, "difficulty": c.Difficulty})
		out = append(out, raw)
	}
	return out, nil
}

func (f *fakeService) GetChallenge(_ context.Context, userID, id string, reveal bool) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGetID = id
	f.lastGetReveal = reveal
	if f.getErr != nil {
		return nil, f.getErr
	}
	c, ok := f.store[id]
	if !ok || (f.missingID != "" && f.missingID == id) {
		return nil, ErrChallengeNotFound
	}
	if c.UserID != nil && *c.UserID != "" && *c.UserID != userID {
		return nil, ErrChallengeNotFound
	}
	payload := map[string]any{"id": c.ID, "title": c.Title, "difficulty": c.Difficulty, "language": c.Language}
	if reveal {
		payload["solutionCode"] = c.SolutionCode
	}
	raw, _ := json.Marshal(payload)
	return raw, nil
}

func (f *fakeService) CreateTempChallenge(_ context.Context, userID string, in challengeInputEnvelope) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastCreateUserID = userID
	f.lastCreateInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	id := in.SourceRepo
	if id == "" {
		id = in.Title
	}
	if _, exists := f.store[id]; exists && f.dedupHit {
		payload := map[string]any{"id": id, "title": in.Title, "deduped": true}
		raw, _ := json.Marshal(payload)
		return raw, nil
	}
	userIDCopy := userID
	c := &models.Challenge{
		ID:       id,
		Title:    in.Title,
		Difficulty: in.Difficulty,
		Language: in.Language,
		UserID:   &userIDCopy,
	}
	f.store[id] = c
	payload := map[string]any{"id": id, "title": in.Title, "deduped": false}
	raw, _ := json.Marshal(payload)
	return raw, nil
}

func (f *fakeService) GenerateChallenge(_ context.Context, userID string, hint GenerateHint) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGenerateUser = userID
	f.lastGenerateHint = hint
	if f.generateErr != nil {
		return nil, f.generateErr
	}
	return nil, ErrNotImplemented
}

func (f *fakeService) ValidateChallenge(_ context.Context, _ string, in validateChallengeInput) (ValidateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastValidateIn = in
	if f.validateErr != nil {
		return ValidateResult{}, f.validateErr
	}
	// Mirror the production v2 validator enough so a "happy" test case
	// triggers Valid=true and a short description triggers Valid=false.
	errorsList := []string{}
	if len(in.Description) < 10 {
		errorsList = append(errorsList, "description is too short")
	}
	if in.ExpectedFindings != nil && (len(in.ExpectedFindings) < 1 || len(in.ExpectedFindings) > 3) {
		errorsList = append(errorsList, "expected_findings must have 1-3 items")
	}
	return ValidateResult{Valid: len(errorsList) == 0, Errors: errorsList}, nil
}

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// --- mcp-challenges.list_challenges ---

func TestListChallengesTool_HappyPath_ReturnsForUser(t *testing.T) {
	svc := newFakeService()
	svc.store["ch-1"] = &models.Challenge{ID: "ch-1", Title: "A", Difficulty: "mid", Language: "go"}
	svc.store["ch-2"] = &models.Challenge{ID: "ch-2", Title: "B", Difficulty: "junior", Language: "ts"}
	tool := NewListChallengesTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	arr := got.([]json.RawMessage)
	if len(arr) != 2 {
		t.Errorf("expected 2 challenges, got %d", len(arr))
	}
	if svc.lastListUserID != "u" {
		t.Errorf("last list userID = %q, want u", svc.lastListUserID)
	}
}

func TestListChallengesTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewListChallengesTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-challenges.get_challenge ---

func TestGetChallengeTool_HappyPath_ReturnsChallenge(t *testing.T) {
	svc := newFakeService()
	svc.store["ch-1"] = &models.Challenge{ID: "ch-1", Title: "A", Difficulty: "mid", Language: "go"}
	tool := NewGetChallengeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"id":"ch-1"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastGetID != "ch-1" {
		t.Errorf("get routed id=%q, want ch-1", svc.lastGetID)
	}
	if svc.lastGetReveal {
		t.Errorf("reveal must be false by default; got true")
	}
	raw := got.(json.RawMessage)
	if string(raw) == "" || string(raw) == "null" {
		t.Errorf("expected a JSON object, got %q", string(raw))
	}
}

func TestGetChallengeTool_RevealTrue_ForwardsToService(t *testing.T) {
	svc := newFakeService()
	svc.store["ch-1"] = &models.Challenge{ID: "ch-1", Title: "A", SolutionCode: "fix"}
	tool := NewGetChallengeTool(svc)
	if _, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"id":"ch-1","reveal":true}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !svc.lastGetReveal {
		t.Errorf("reveal flag was not forwarded to service")
	}
}

func TestGetChallengeTool_NotFound_ReturnsErrNotFound(t *testing.T) {
	svc := newFakeService()
	svc.missingID = "missing"
	tool := NewGetChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"id":"missing"}`))
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	if !regexp.MustCompile(`(?i)challenge not found`).MatchString(err.Error()) {
		t.Errorf("err message should contain 'challenge not found', got %q", err.Error())
	}
}

func TestGetChallengeTool_MissingID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing id) = %v, want ErrInvalidParams", err)
	}
}

func TestGetChallengeTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetChallengeTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"id":"x"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-challenges.create_temp_challenge ---

func TestCreateTempChallengeTool_HappyPath_PersistsAndReturnsDedupFalse(t *testing.T) {
	svc := newFakeService()
	tool := NewCreateTempChallengeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "Hardcode hunt",
		"description": "Find literal values baked into the loop body.",
		"difficulty": "mid",
		"language": "typescript"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastCreateUserID != "u" {
		t.Errorf("routed userID=%q, want u", svc.lastCreateUserID)
	}
	raw := got.(json.RawMessage)
	if !strings.Contains(string(raw), `"deduped":false`) {
		t.Errorf("payload should include deduped=false on first create, got %s", string(raw))
	}
}

func TestCreateTempChallengeTool_Dedup_ReturnsExistingChallenge(t *testing.T) {
	svc := newFakeService()
	svc.dedupHit = true
	svc.store["github.com/foo/bar"] = &models.Challenge{ID: "github.com/foo/bar", Title: "Hardcode hunt"}
	tool := NewCreateTempChallengeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "Hardcode hunt",
		"description": "Find literal values baked into the loop body.",
		"difficulty": "mid",
		"language": "typescript",
		"sourceRepo": "github.com/foo/bar"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	raw := got.(json.RawMessage)
	if !strings.Contains(string(raw), `"deduped":true`) {
		t.Errorf("payload should include deduped=true on dedup hit, got %s", string(raw))
	}
}

func TestCreateTempChallengeTool_MissingRequiredFields_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewCreateTempChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "only-title",
		"language": "go"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing fields) = %v, want ErrInvalidParams", err)
	}
}

func TestCreateTempChallengeTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewCreateTempChallengeTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"title": "T",
		"description": "long enough description here",
		"difficulty": "mid",
		"language": "go"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-challenges.generate_challenge ---

func TestGenerateChallengeTool_StubReturnsErrNotImplemented(t *testing.T) {
	svc := newFakeService()
	tool := NewGenerateChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "typescript",
		"difficulty": "senior"
	}`))
	if err == nil {
		t.Fatalf("expected ErrNotImplemented, got nil")
	}
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("err = %v, want ErrNotImplemented", err)
	}
	if svc.lastGenerateUser != "u" {
		t.Errorf("generate routed userID=%q, want u", svc.lastGenerateUser)
	}
	if svc.lastGenerateHint.Language != "typescript" {
		t.Errorf("hint.language = %q, want typescript", svc.lastGenerateHint.Language)
	}
}

func TestGenerateChallengeTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGenerateChallengeTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-challenges.validate_challenge ---

func TestValidateChallengeTool_HappyPath_ReturnsValid(t *testing.T) {
	svc := newFakeService()
	tool := NewValidateChallengeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "T",
		"description": "long enough description here",
		"difficulty": "mid",
		"language": "go"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, ok := got.(ValidateResult)
	if !ok {
		t.Fatalf("expected ValidateResult, got %T", got)
	}
	if !result.Valid {
		t.Errorf("result.Valid = false; want true. errors=%v", result.Errors)
	}
}

func TestValidateChallengeTool_InvalidInput_ReportsViolation(t *testing.T) {
	svc := newFakeService()
	tool := NewValidateChallengeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "T",
		"description": "short",
		"difficulty": "mid",
		"language": "go"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	result, ok := got.(ValidateResult)
	if !ok {
		t.Fatalf("expected ValidateResult, got %T", got)
	}
	if result.Valid {
		t.Errorf("result.Valid = true; want false (description too short)")
	}
	if len(result.Errors) == 0 {
		t.Errorf("validation result should include the violation message")
	}
}

func TestValidateChallengeTool_MissingRequiredFields_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewValidateChallengeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"title": "T"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(missing fields) = %v, want ErrInvalidParams", err)
	}
}

// --- Group-wide smoke ---

func TestNewTools_NeverNil(t *testing.T) {
	svc := newFakeService()
	if NewListChallengesTool(svc) == nil {
		t.Errorf("NewListChallengesTool returned nil")
	}
	if NewGetChallengeTool(svc) == nil {
		t.Errorf("NewGetChallengeTool returned nil")
	}
	if NewCreateTempChallengeTool(svc) == nil {
		t.Errorf("NewCreateTempChallengeTool returned nil")
	}
	if NewGenerateChallengeTool(svc) == nil {
		t.Errorf("NewGenerateChallengeTool returned nil")
	}
	if NewValidateChallengeTool(svc) == nil {
		t.Errorf("NewValidateChallengeTool returned nil")
	}
}

// Reference compiles-time guard so a future service signature drift
// breaks the build at the boundary, not in production.
var _ = services.IsValidationError
