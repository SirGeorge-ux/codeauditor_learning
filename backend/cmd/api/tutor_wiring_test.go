package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/llm_cascade"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// stubProfileRepo is the hand-rolled stub profile repo the wiring
// tests use so the mcp-monolith Supabase code path is not booted.
// Mirrors the ports.UserProgressRepository shape.
type stubProfileRepo struct{}

func (*stubProfileRepo) GetLanguageProgress(_ context.Context, _, _ string) (*models.LanguageProgress, error) {
	return &models.LanguageProgress{}, nil
}
func (*stubProfileRepo) UpdateLanguageProgress(_ context.Context, _ *models.LanguageProgress) error {
	return nil
}
func (*stubProfileRepo) GetAllLanguageProgress(_ context.Context, _ string) ([]*models.LanguageProgress, error) {
	return []*models.LanguageProgress{}, nil
}
func (*stubProfileRepo) GetLearningProfile(_ context.Context, _ string) (*models.LearningProfile, error) {
	return models.DefaultLearningProfile("u1"), nil
}
func (*stubProfileRepo) UpdateLearningProfile(_ context.Context, _ *models.LearningProfile) error {
	return nil
}

// TestTutorSystem_NilProfileRepoReturnsError — defensa dura: un
// wiring sin repo se rechaza.
func TestTutorSystem_NilProfileRepoReturnsError(t *testing.T) {
	_, err := TutorSystem(TutorSystemDeps{})
	if err == nil {
		t.Fatalf("expected error for nil ProfileRepository, got nil")
	}
	if !strings.Contains(err.Error(), "ProfileRepository") {
		t.Errorf("err = %q, want ProfileRepository mention", err.Error())
	}
}

// TestTutorSystem_MissingPromptsReturnsErrTutorPromptsMissing — el
// error sentinel del loader se propaga al wiring helper.
func TestTutorSystem_MissingPromptsReturnsErrTutorPromptsMissing(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "no-prompts")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	_, err := TutorSystem(TutorSystemDeps{
		ProfileRepository: &stubProfileRepo{},
		PromptsDir:        dir,
		Providers:         TutorProvidersConfig{},
	})
	if err == nil {
		t.Fatalf("expected ErrTutorPromptsMissing for empty directory")
	}
	if !strings.Contains(err.Error(), "tutor prompts missing") {
		t.Errorf("err = %q, want ErrTutorPromptsMissing suffix", err.Error())
	}
}

// TestTutorSystem_AllProviderKeysEmpty_DegradedModeSucceeds — un
// wiring con todas las API keys vacías (modo degradado) aún
// produce service + handler. La cascada skipea providers
// disabled y devuelve ErrCascadeAllFailed al primer stream.
func TestTutorSystem_AllProviderKeysEmpty_DegradedModeSucceeds(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{
		"tutor-system.md",
		"tutor-socratic-0.md",
		"tutor-socratic-1.md",
		"tutor-socratic-2.md",
		"tutor-socratic-3.md",
	} {
		body := []byte("# stub " + name + "\n")
		if err := os.WriteFile(filepath.Join(tmp, name), body, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	wiring, err := TutorSystem(TutorSystemDeps{
		ProfileRepository: &stubProfileRepo{},
		PromptsDir:        tmp,
		Providers:         TutorProvidersConfig{},
	})
	if err != nil {
		t.Fatalf("TutorSystem err: %v", err)
	}
	if wiring.Service == nil {
		t.Fatalf("Service nil")
	}
	if wiring.Handler == nil {
		t.Fatalf("Handler nil")
	}
	if wiring.Cascade == nil {
		t.Fatalf("Cascade nil")
	}
	if wiring.Cache == nil {
		t.Fatalf("Cache nil")
	}

	// Drive an HTTP request. Without a JWT we expect 400 (defense
	// in depth), but the wiring itself is intact.
	body := `{"message":"hi","challengeId":"c1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tutor/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	wiring.Handler.StreamChat(w, req)
	// 200 with `done` (empty stream closes with done) or 400 for
	// missing user_id; both are acceptable wiring outcomes.
	if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
		t.Errorf("unexpected status %d", w.Code)
	}
}

// TestTutorSystem_StreamChatPathLeadsToCascadeFailure — degraded
// mode + a non-empty message drives the cascade, which has every
// provider disabled, so we get the sentinel error wrapping.
func TestTutorSystem_StreamChatPathLeadsToCascadeFailure(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{
		"tutor-system.md",
		"tutor-socratic-0.md",
		"tutor-socratic-1.md",
		"tutor-socratic-2.md",
		"tutor-socratic-3.md",
	} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("body "+name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	wiring, err := TutorSystem(TutorSystemDeps{
		ProfileRepository: &stubProfileRepo{},
		PromptsDir:        tmp,
		Providers:         TutorProvidersConfig{},
	})
	if err != nil {
		t.Fatalf("TutorSystem: %v", err)
	}
	// Use the service directly to inspect the cascade outcome.
	evCh, err := wiring.Service.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID: "u1", Message: "ping", ChallengeID: "c1",
	})
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	var sawError bool
	for ev := range evCh {
		if ev.Type == "error" {
			sawError = true
			if !strings.Contains(ev.Content, "all providers") {
				t.Errorf("error content = %q, want 'all providers'", ev.Content)
			}
		}
	}
	if !sawError {
		t.Errorf("expected an error event in degraded mode")
	}
}

// TestTutorSystem_SessionCacheIsInMemoryAndIsolated — el cache del
// wiring helper es un SessionCache real (10-msg cap). El test
// verifica que Set + Snapshot respeta el cap.
func TestTutorSystem_SessionCacheIsInMemoryAndIsolated(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{
		"tutor-system.md",
		"tutor-socratic-0.md",
		"tutor-socratic-1.md",
		"tutor-socratic-2.md",
		"tutor-socratic-3.md",
	} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("x"+name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	wiring, err := TutorSystem(TutorSystemDeps{
		ProfileRepository: &stubProfileRepo{},
		PromptsDir:        tmp,
		Providers:         TutorProvidersConfig{},
	})
	if err != nil {
		t.Fatalf("TutorSystem: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < 15; i++ {
		wiring.Cache.Set(ctx, "sess-1", models.ChatMessage{
			Role:    models.ChatRoleUser,
			Content: "msg",
		})
	}
	if got := len(wiring.Cache.Snapshot(ctx, "sess-1")); got != 10 {
		t.Errorf("ring buffer cap = %d, want 10", got)
	}
	// Cache must satisfy the real CachePort implementation contract.
	var _ ports.CachePort = wiring.Cache
}

// Compile-time guard: a SessionCache (the production type) satisfies
// ports.CachePort. The wiring helper binds its `Cache` field to the
// real concrete type, but we double-check to be safe.

// suppressUnusedImport keeps the linter happy if a future refactor
// drops an import.
var _ = llmcascade.NewCascade
