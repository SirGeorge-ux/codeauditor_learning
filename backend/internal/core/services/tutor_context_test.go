package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// fakeUserProgressRepo is a hand-rolled stub implementing
// ports.UserProgressRepository sufficient for the ChatContext tests.
// It only stores LearningProfile rows; per-language progress methods
// are stubs returning zero values, since the chat-context path never
// touches them.
type fakeUserProgressRepo struct {
	profile *models.LearningProfile
	updated *models.LearningProfile
}

func (f *fakeUserProgressRepo) GetLanguageProgress(_ context.Context, _, _ string) (*models.LanguageProgress, error) {
	return &models.LanguageProgress{}, nil
}

func (f *fakeUserProgressRepo) UpdateLanguageProgress(_ context.Context, p *models.LanguageProgress) error {
	return nil
}

func (f *fakeUserProgressRepo) GetAllLanguageProgress(_ context.Context, _ string) ([]*models.LanguageProgress, error) {
	return []*models.LanguageProgress{}, nil
}

func (f *fakeUserProgressRepo) GetLearningProfile(_ context.Context, _ string) (*models.LearningProfile, error) {
	if f.profile == nil {
		return models.DefaultLearningProfile("u1"), nil
	}
	return f.profile, nil
}

func (f *fakeUserProgressRepo) UpdateLearningProfile(_ context.Context, p *models.LearningProfile) error {
	clone := *p
	f.updated = &clone
	return nil
}

// TestAssembleChatContext_ReturnsFiveSystemMessages cubre la forma
// mínima: 5 system messages (identity, socratic, user summary,
// location, history+prefs) + 1 user message al final.
func TestAssembleChatContext_ReturnsFiveSystemMessages(t *testing.T) {
	repo := &fakeUserProgressRepo{profile: models.DefaultLearningProfile("u1")}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	msgs, err := assembleChatContext(
		context.Background(), repo, nil, prompts,
		ports.StreamChatRequest{
			UserID:      "u1",
			Message:     "donde está el bug?",
			Route:       "/dojo/ch-sqli",
			ChallengeID: "ch-sqli",
			Code:        "const x = 1;",
		},
		models.SocraticPure,
	)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	// 5 system + 1 user = 6 total.
	if len(msgs) != 6 {
		t.Fatalf("got %d messages, want 6", len(msgs))
	}
	for i := 0; i < 5; i++ {
		if msgs[i].Role != models.ChatRoleSystem {
			t.Errorf("msgs[%d].Role = %v, want system", i, msgs[i].Role)
		}
	}
	if msgs[5].Role != models.ChatRoleUser {
		t.Errorf("msgs[5].Role = %v, want user", msgs[5].Role)
	}
	if msgs[5].Content != "donde está el bug?" {
		t.Errorf("user message lost: %q", msgs[5].Content)
	}
}

// TestAssembleChatContext_TruncatesLongCode valida el cap de 2KB
// sobre el campo Code. Sin truncado, un blob gigante rompería la
// ventana de contexto del LLM.
func TestAssembleChatContext_TruncatesLongCode(t *testing.T) {
	repo := &fakeUserProgressRepo{profile: models.DefaultLearningProfile("u1")}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	longCode := strings.Repeat("x", maxChatContextCodeBytes+1000)
	msgs, err := assembleChatContext(
		context.Background(), repo, nil, prompts,
		ports.StreamChatRequest{
			UserID:  "u1",
			Message: "check this",
			Route:   "/dojo/x",
			Code:    longCode,
		},
		models.SocraticDirect,
	)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	// The location system message is index 3.
	if !strings.Contains(msgs[3].Content, "(truncated)") {
		t.Errorf("expected truncation marker, got %d-char message", len(msgs[3].Content))
	}
}

// TestAssembleChatContext_IncludesSocraticLevel verifica que el
// segundo system message contiene el cuerpo del prompt del nivel
// activo (no del default).
func TestAssembleChatContext_IncludesSocraticLevel(t *testing.T) {
	repo := &fakeUserProgressRepo{profile: models.DefaultLearningProfile("u1")}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	for level := models.SocraticDirect; level <= models.SocraticDiscovery; level++ {
		msgs, err := assembleChatContext(
			context.Background(), repo, nil, prompts,
			ports.StreamChatRequest{UserID: "u1", Message: "hi"},
			level,
		)
		if err != nil {
			t.Fatalf("assemble: %v", err)
		}
		if msgs[1].Content != tpsSocratic[level] {
			t.Errorf("level %d: socratic body = %q, want %q", level, msgs[1].Content, tpsSocratic[level])
		}
	}
}

// TestAssembleChatContext_IncludesUserSummaryAndPreferences
// asegura que history (cache snapshot) y preferencias (idioma +
// maxLength) están presentes en el último system message.
func TestAssembleChatContext_IncludesUserSummaryAndPreferences(t *testing.T) {
	profile := models.DefaultLearningProfile("u1")
	profile.Preferencias.Idioma = "en"
	profile.Preferencias.LongitudMaximaMsg = 120
	profile.Preferencias.Resumen = "Aprendió SQL basics."
	repo := &fakeUserProgressRepo{profile: profile}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	msgs, err := assembleChatContext(
		context.Background(), repo, nil, prompts,
		ports.StreamChatRequest{UserID: "u1", Message: "ping"},
		models.SocraticPure,
	)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	combined := msgs[2].Content + "|" + msgs[4].Content
	if !strings.Contains(combined, "Aprendió SQL basics.") {
		t.Errorf("missing summary in messages: msgs[2]=%q msgs[4]=%q", msgs[2].Content, msgs[4].Content)
	}
	if !strings.Contains(combined, "idioma=en") || !strings.Contains(combined, "longitud_maxima_msg=120") {
		t.Errorf("missing preferences: msgs[2]=%q msgs[4]=%q", msgs[2].Content, msgs[4].Content)
	}
}

// TestAssembleChatContext_FetchProfileErrorPropagates cubre el
// camino de error: si GetLearningProfile falla, el error envuelve
// la falla original sin tragarla.
func TestAssembleChatContext_FetchProfileErrorPropagates(t *testing.T) {
	repo := &failingProfileRepo{}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err = assembleChatContext(
		context.Background(), repo, nil, prompts,
		ports.StreamChatRequest{UserID: "u1", Message: "x"},
		models.SocraticPure,
	)
	if err == nil {
		t.Fatalf("expected error propagation, got nil")
	}
}

// failingProfileRepo returns a forced error from GetLearningProfile
// so the assemble error path can be exercised without spinning up a
// mock.
type failingProfileRepo struct{}

func (*failingProfileRepo) GetLanguageProgress(_ context.Context, _, _ string) (*models.LanguageProgress, error) {
	return nil, errBoom
}

func (*failingProfileRepo) UpdateLanguageProgress(_ context.Context, _ *models.LanguageProgress) error {
	return nil
}

func (*failingProfileRepo) GetAllLanguageProgress(_ context.Context, _ string) ([]*models.LanguageProgress, error) {
	return nil, nil
}

func (*failingProfileRepo) GetLearningProfile(_ context.Context, _ string) (*models.LearningProfile, error) {
	return nil, errBoom
}

func (*failingProfileRepo) UpdateLearningProfile(_ context.Context, _ *models.LearningProfile) error {
	return nil
}

// errBoom is the sentinel error raised by failingProfileRepo.
var errBoom = &stringErr{"boom"}

// stringErr is a tiny error type so we don't import errors just for
// the boom constant.
type stringErr struct{ s string }

func (e *stringErr) Error() string { return e.s }

// Compile-time guard for the trivialPrompts placeholders — let the
// compile error surface if the helper shape drifts.
var _ = time.Now // suppress unused if a future commit drops time usage
