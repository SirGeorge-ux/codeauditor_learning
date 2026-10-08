package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// --- mcp-sandbox.list_languages ---

func TestListLanguagesTool_HappyPath_ReturnsCatalog(t *testing.T) {
	svc := newFakeService()
	svc.listOut = []LanguageInfo{
		{Language: "go", FileExtension: ".go", DockerImage: "golang:1.23-alpine", LocalCommand: "go", InstallHint: "install Go via asdf"},
		{Language: "python", FileExtension: ".py", DockerImage: "python:3.12-alpine", LocalCommand: "python3", InstallHint: "apt install python3"},
	}
	tool := NewListLanguagesTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out, ok := got.([]LanguageInfo)
	if !ok {
		t.Fatalf("expected []LanguageInfo, got %T", got)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 languages, got %d", len(out))
	}
	if out[0].Language != "go" || out[1].Language != "python" {
		t.Errorf("catalog order = [%s,%s], want [go,python]", out[0].Language, out[1].Language)
	}
}

func TestListLanguagesTool_NilResult_ReturnsEmpty(t *testing.T) {
	svc := newFakeService()
	svc.listOut = nil
	tool := NewListLanguagesTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := got.([]LanguageInfo)
	if out == nil {
		t.Errorf("nil catalog should be [], not null")
	}
}

func TestListLanguagesTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewListLanguagesTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-sandbox.healthcheck ---

func TestHealthcheckTool_HappyPath_ReturnsOK(t *testing.T) {
	svc := newFakeService()
	svc.health = HealthStatus{OK: true, Backend: "docker"}
	tool := NewHealthcheckTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	status, ok := got.(HealthStatus)
	if !ok {
		t.Fatalf("expected HealthStatus, got %T", got)
	}
	if !status.OK {
		t.Errorf("status.OK = false, want true")
	}
	if status.Backend != "docker" {
		t.Errorf("status.Backend = %q, want docker", status.Backend)
	}
}

func TestHealthcheckTool_RuntimeUnavailable_ReturnsStatusAndError(t *testing.T) {
	svc := newFakeService()
	svc.healthErr = ErrSandboxUnavailable
	svc.health = HealthStatus{
		OK:      false,
		Backend: "docker",
		Issues:  []string{"docker daemon not reachable"},
	}
	tool := NewHealthcheckTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected ErrSandboxUnavailable, got nil")
	}
	if !errors.Is(err, ErrSandboxUnavailable) {
		t.Errorf("err = %v, want ErrSandboxUnavailable", err)
	}
	status, ok := got.(HealthStatus)
	if !ok {
		t.Fatalf("expected HealthStatus even on error, got %T", got)
	}
	if status.OK {
		t.Errorf("status.OK = true on degraded runtime")
	}
	if !reflect.DeepEqual(status.Issues, []string{"docker daemon not reachable"}) {
		t.Errorf("Issues = %v, want [docker daemon not reachable]", status.Issues)
	}
}

func TestHealthcheckTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewHealthcheckTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- mcp-sandbox.get_install_hint ---

func TestGetInstallHintTool_HappyPath_ReturnsHint(t *testing.T) {
	svc := newFakeService()
	svc.installHintOut = map[string]string{
		"go":     "apt install golang (or asdf install golang)",
		"python": "apt install python3",
	}
	tool := NewGetInstallHintTool(svc)
	got, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if svc.lastInstallQuery != "go" {
		t.Errorf("routed language = %q, want go", svc.lastInstallQuery)
	}
	payload, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}
	if payload["language"] != "go" {
		t.Errorf("payload.language = %v, want go", payload["language"])
	}
	hint, _ := payload["installHint"].(string)
	if hint == "" {
		t.Errorf("installHint must be populated")
	}
}

func TestGetInstallHintTool_UnknownLanguage_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	// installHintOut is empty — any language is unknown.
	tool := NewGetInstallHintTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":"made-up"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(unknown lang) = %v, want ErrInvalidParams", err)
	}
}

func TestGetInstallHintTool_EmptyLanguage_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetInstallHintTool(svc)
	_, err := tool.Execute(ctxWithUser(t, "u"), json.RawMessage(`{"language":""}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(empty language) = %v, want ErrInvalidParams", err)
	}
}

func TestGetInstallHintTool_MissingUserID_RejectsWithInvalidParams(t *testing.T) {
	svc := newFakeService()
	tool := NewGetInstallHintTool(svc)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"language":"go"}`))
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Execute(no ctx user_id) = %v, want ErrInvalidParams", err)
	}
}

// --- Group-wide smoke ---

func TestNewTools_NeverNil(t *testing.T) {
	svc := newFakeService()
	if NewRunCodeTool(svc) == nil {
		t.Errorf("NewRunCodeTool returned nil")
	}
	if NewListLanguagesTool(svc) == nil {
		t.Errorf("NewListLanguagesTool returned nil")
	}
	if NewHealthcheckTool(svc) == nil {
		t.Errorf("NewHealthcheckTool returned nil")
	}
	if NewGetInstallHintTool(svc) == nil {
		t.Errorf("NewGetInstallHintTool returned nil")
	}
}
