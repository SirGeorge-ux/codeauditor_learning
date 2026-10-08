package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// fakeService is the hand-rolled stand-in for the production sandbox service.
// It records the security profile and the last RunCode input so tests can
// pin down both the wiring shape and the read-only enforcement without a
// real docker daemon.
type fakeService struct {
	mu sync.Mutex

	// Behavior knobs.
	profile         SecurityProfile
	runResult       RunResult
	runErr          error
	health          HealthStatus
	healthErr       error
	listOut         []LanguageInfo
	listErr         error
	installHintOut  map[string]string
	installHintErr  error
	unknownLanguage string

	// Recorded inputs.
	lastRunUserID    string
	lastRunLanguage  string
	lastRunCode      string
	lastRunTimeout   int
	lastInstallQuery string
}

func newFakeService() *fakeService {
	return &fakeService{
		profile:        SecurityProfile{Backend: "docker", RequiredFlags: append([]string(nil), requiredFlags...)},
		installHintOut: map[string]string{},
	}
}

func (f *fakeService) SecurityProfile() SecurityProfile {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.profile
}

func (f *fakeService) RunCode(_ context.Context, userID, language, code string, timeoutSeconds int) (RunResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastRunUserID = userID
	f.lastRunLanguage = language
	f.lastRunCode = code
	f.lastRunTimeout = timeoutSeconds
	if f.runErr != nil {
		return RunResult{}, f.runErr
	}
	if f.unknownLanguage != "" && language == f.unknownLanguage {
		return RunResult{}, ErrUnknownLanguage
	}
	return f.runResult, nil
}

func (f *fakeService) ListLanguages(_ context.Context) ([]LanguageInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listOut, nil
}

func (f *fakeService) Healthcheck(_ context.Context) (HealthStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.healthErr != nil {
		// Return the populated status alongside the error so the tool can
		// surface Issues even when the runtime is degraded.
		return f.health, f.healthErr
	}
	return f.health, nil
}

func (f *fakeService) GetInstallHint(_ context.Context, language string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastInstallQuery = language
	if f.installHintErr != nil {
		return "", f.installHintErr
	}
	hint, ok := f.installHintOut[language]
	if !ok {
		return "", ErrUnknownLanguage
	}
	return hint, nil
}

func ctxWithUser(t *testing.T, userID string) context.Context {
	t.Helper()
	return ContextWithUserID(context.Background(), userID)
}

// TestRunCodeTool_LocalBackend_RejectsWithInvalidParams verifies the RED
// scenario from the MCP spec: "mcp-sandbox.run_code enforces
// --cap-drop=ALL --network=none". A backend with Backend="local" and no
// required flags MUST be rejected; the tool must never fall back to the
// local runtime even if the production code path provides one.
func TestRunCodeTool_LocalBackend_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.profile = SecurityProfile{Backend: "local"} // no RequiredFlags
	tool := NewRunCodeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "python",
		"code": "print('hi')"
	}`))
	if err == nil {
		t.Fatalf("local backend must be rejected; got result %+v", got)
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams (-32602)", err)
	}
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("err message should mention the offending backend, got %q", err.Error())
	}
}

// TestRunCodeTool_MissingRequiredFlag_RejectsWithInvalidParams verifies the
// second RED scenario: a docker backend that lost one of the spec-required
// flags (--cap-drop=ALL, --network=none, --read-only) MUST also be rejected.
// The test denies --network=none and asserts the message names the missing
// flag so the LLM gets actionable feedback.
func TestRunCodeTool_MissingRequiredFlag_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.profile = SecurityProfile{
		Backend:       "docker",
		RequiredFlags: []string{"--cap-drop=ALL", "--read-only"}, // missing --network=none
	}
	tool := NewRunCodeTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "go",
		"code": "package main"
	}`))
	if err == nil {
		t.Fatalf("docker backend missing --network=none must be rejected; got %+v", got)
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("err = %v, want ErrInvalidParams (-32602)", err)
	}
	if !strings.Contains(err.Error(), "--network=none") {
		t.Errorf("err message should name the missing flag, got %q", err.Error())
	}
	// Verify RunCode was NOT called — the security check must reject before
	// delegating to the executor.
	if svc.lastRunLanguage != "" {
		t.Errorf("RunCode should not be called when security check fails; got language=%q", svc.lastRunLanguage)
	}
}

// TestRunCodeTool_HardenedDocker_HappyPath verifies the GREEN scenario: a
// docker backend with the full flag set delegates to the service with the
// authenticated user_id, defaults the timeout to 30, clamps oversize
// timeouts to 30, and returns the JSON-shaped RunResult.
func TestRunCodeTool_HardenedDocker_HappyPath(t *testing.T) {
	svc := newFakeService()
	svc.runResult = RunResult{
		Language:   "python",
		Output:     "hi\n",
		DurationMs: 42,
	}
	tool := NewRunCodeTool(svc)

	got, err := tool.Execute(ctxWithUser(t, "u-123"), json.RawMessage(`{
		"language": "python",
		"code": "print('hi')"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastRunUserID != "u-123" {
		t.Errorf("routed userID = %q, want u-123", svc.lastRunUserID)
	}
	if svc.lastRunLanguage != "python" {
		t.Errorf("routed language = %q, want python", svc.lastRunLanguage)
	}
	if svc.lastRunTimeout != 30 {
		t.Errorf("defaulted timeout = %d, want 30", svc.lastRunTimeout)
	}
	result, ok := got.(RunResult)
	if !ok {
		t.Fatalf("expected RunResult, got %T", got)
	}
	if result.Output != "hi\n" {
		t.Errorf("output = %q, want hi\n", result.Output)
	}
}

func TestRunCodeTool_Timeout_ClampedToMax(t *testing.T) {
	svc := newFakeService()
	svc.runResult = RunResult{Language: "go", Output: "ok\n"}
	tool := NewRunCodeTool(svc)

	if _, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "go",
		"code": "package main",
		"timeoutSeconds": 9999
	}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastRunTimeout != 30 {
		t.Errorf("oversize timeout was not clamped; routed=%d, want 30", svc.lastRunTimeout)
	}
}

func TestRunCodeTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewRunCodeTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{
		"language": "python",
		"code": "print('hi')"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

func TestRunCodeTool_EmptyCode_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewRunCodeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "python",
		"code": "    "
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(empty code) = %v, want ErrInvalidParams", err)
	}
}

func TestRunCodeTool_UnknownLanguage_SurfacesAsInvalidParams(t *testing.T) {
	svc := newFakeService()
	svc.unknownLanguage = "made-up-lang"
	tool := NewRunCodeTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{
		"language": "made-up-lang",
		"code": "x"
	}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(unknown lang) = %v, want ErrInvalidParams", err)
	}
}
