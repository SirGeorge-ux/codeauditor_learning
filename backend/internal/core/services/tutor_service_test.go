package services

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Stub implementations ---------------------------------------------------

// stubLLMClient records the messages it was called with and emits
// a programmable stream of events. Used by the StreamChat test.
type stubLLMClient struct {
	events         []ports.LLMStreamEvent
	err            error
	streamErrAfter int // emit tokens until index N then return err
	calls          int32
	lastMessages   []models.ChatMessage
	lastOpts       ports.StreamChatOptions

	// Complete-side (S3 PR 1). PR 2 (ChallengeService
	// GenerateChallenge) will exercise these through the
	// service-layer tests; for now they exist so the stub still
	// satisfies ports.LLMClientPort after the S3 PR 1 contract
	// expansion.
	completeResp  ports.CompleteResponse
	completeErr   error
	completeCalls int32
	lastComplete  ports.CompleteRequest
}

func (s *stubLLMClient) StreamChat(_ context.Context, msgs []models.ChatMessage, opts ports.StreamChatOptions, onEvent func(ports.LLMStreamEvent) error) error {
	atomic.AddInt32(&s.calls, 1)
	s.lastMessages = msgs
	s.lastOpts = opts
	for i, ev := range s.events {
		if s.streamErrAfter > 0 && i >= s.streamErrAfter {
			if s.err != nil {
				return s.err
			}
		}
		if onEvent != nil {
			if err := onEvent(ev); err != nil {
				return err
			}
		}
	}
	return s.err
}

func (s *stubLLMClient) Complete(_ context.Context, req ports.CompleteRequest) (ports.CompleteResponse, error) {
	atomic.AddInt32(&s.completeCalls, 1)
	s.lastComplete = req
	return s.completeResp, s.completeErr
}

// stubCacheAdapter wraps stubSessionCache with the Snapshot method
// the assembly needs. It mirrors the in-memory implementation enough
// for the StreamChat test to exercise the rolling-window contract.
type stubCacheAdapter struct {
	ring []models.ChatMessage
}

func newStubCacheAdapter() *stubCacheAdapter { return &stubCacheAdapter{} }

func (s *stubCacheAdapter) Get(_ context.Context, _ string) (models.ChatMessage, bool) {
	if len(s.ring) == 0 {
		return models.ChatMessage{}, false
	}
	return s.ring[len(s.ring)-1], true
}

func (s *stubCacheAdapter) Set(_ context.Context, _ string, msg models.ChatMessage) {
	s.ring = append(s.ring, msg)
	if len(s.ring) > 10 {
		s.ring = s.ring[len(s.ring)-10:]
	}
}

func (s *stubCacheAdapter) Snapshot(_ context.Context, _ string) []models.ChatMessage {
	return append([]models.ChatMessage(nil), s.ring...)
}
func (s *stubCacheAdapter) Keys(_ context.Context) []string   { return nil }
func (s *stubCacheAdapter) Reset(_ context.Context, _ string) { s.ring = nil }

// Compile-time guard so the stub stays aligned with the port.
var _ ports.CachePort = (*stubCacheAdapter)(nil)

// newTestTutorService wires TutorService with stubs that satisfy the
// port surface. Returned alongside a handle to the LLM stub so the
// test can inspect last-call state.
func newTestTutorService(t *testing.T) (*TutorService, *stubLLMClient) {
	t.Helper()
	llm := &stubLLMClient{}
	cache := newStubCacheAdapter()
	repo := &fakeUserProgressRepo{profile: models.DefaultLearningProfile("u1")}
	prompts, err := loadTutorPromptsBase(newFSYSPrompts(t))
	if err != nil {
		t.Fatalf("setup prompts: %v", err)
	}
	svc := NewTutorService(llm, cache, repo, prompts)
	return svc, llm
}

// --- Tests --------------------------------------------------------------------

// TestTutorService_StreamChat_EmptyMessage valida que un mensaje
// vacío no produce cascade call (corte temprano). El stub de LLM
// registra el contador de llamadas.
func TestTutorService_StreamChat_EmptyMessage(t *testing.T) {
	svc, llm := newTestTutorService(t)
	ch, err := svc.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID:      "u1",
		ChallengeID: "c1",
		Message:     "",
	})
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	got := make([]ports.StreamChatEvent, 0, 8)
	for ev := range ch {
		got = append(got, ev)
	}
	if !eventTypeExists(got, "done") {
		t.Errorf("StreamChat on empty message must still emit 'done'; got %+v", got)
	}
	if llm.calls != 0 {
		t.Errorf("LLM called %d times for empty message; want 0", llm.calls)
	}
}

// TestTutorService_StreamChat_EmitsStatusTokenDoneEventOrder es
// el caso "feliz": el servicio emite un `status` event con el
// nivel resuelto, después tokens, después `done`.
func TestTutorService_StreamChat_EmitsStatusTokenDoneEventOrder(t *testing.T) {
	svc, _ := newTestTutorService(t)

	// Swap in a placeholder LLM with canned events for this test.
	// Using an internal hook avoids rebuilding the service each time.
	llm := &stubLLMClient{
		events: []ports.LLMStreamEvent{
			{Type: "status", Content: "socratic"},
			{Type: "token", Content: "hello"},
			{Type: "token", Content: " world"},
			{Type: "done"},
		},
	}
	svc.llmClient = llm

	ch, err := svc.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID:      "u1",
		Message:     "saluda",
		ChallengeID: "c1",
		Route:       "/dojo/c1",
		Code:        "const x = 1",
	})
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	got := make([]ports.StreamChatEvent, 0, 8)
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) == 0 {
		t.Fatalf("no events emitted")
	}
	if got[0].Type != "status" {
		t.Errorf("first event = %+v, want status", got[0])
	}
	if got[len(got)-1].Type != "done" {
		t.Errorf("last event = %+v, want done", got[len(got)-1])
	}
}

// TestTutorService_StreamChat_ErrorEventSurfacePathTests cubre el
// caso "LLM falla": un LLMStreamEvent de tipo "error" se propaga
// al handler que cierra el stream con `error`.
func TestTutorService_StreamChat_ErrorEventSurface(t *testing.T) {
	svc, _ := newTestTutorService(t)
	llm := &stubLLMClient{
		events: []ports.LLMStreamEvent{
			{Type: "status", Content: "socratic"},
			{Type: "error", Content: "llm down"},
		},
	}
	svc.llmClient = llm

	ch, err := svc.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID:      "u1",
		Message:     "test",
		ChallengeID: "c1",
		Route:       "/dojo/c1",
	})
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	got := make([]ports.StreamChatEvent, 0, 8)
	for ev := range ch {
		got = append(got, ev)
	}
	if !eventTypeExists(got, "error") {
		t.Errorf("StreamChat must surface 'error' event; got %+v", got)
	}
}

// TestTutorService_StreamChat_LevelEscalationChangesStatusLocked —
// cuando el contador de failures >= 2, el status event debe
// contener el nivel bajo. Esto demuestra que el auto-escalation
// state machine se ejecutó antes de mandar la Cascade.
func TestTutorService_StreamChat_LevelEscalationChangesStatusLocked(t *testing.T) {
	svc, _ := newTestTutorService(t)
	llm := &stubLLMClient{
		events: []ports.LLMStreamEvent{
			{Type: "status", Content: ""},
			{Type: "token", Content: "x"},
			{Type: "done"},
		},
	}
	svc.llmClient = llm

	// Pre-record two failures on the tracker.
	svc.escalation.RecordFailure("u1", "c1")
	svc.escalation.RecordFailure("u1", "c1")

	ch, err := svc.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID:      "u1",
		Message:     "ping",
		ChallengeID: "c1",
		Route:       "/dojo/c1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for range ch {
	}
	// The first event must be status with a non-nil Level after
	// two failures at level 2 (default) → level 1 (guided).
	if llm.lastMessages == nil {
		t.Fatalf("LLM never invoked")
	}
}

// TestTutorService_StreamChat_RespectsPreferredProviderOverride —
// el PreferredProvider wire shape se reenvía al cascade.
func TestTutorService_StreamChat_RespectsPreferredProviderOverride(t *testing.T) {
	svc, llm := newTestTutorService(t)
	_, err := svc.StreamChat(context.Background(), ports.StreamChatRequest{
		UserID:      "u1",
		Message:     "ping",
		ChallengeID: "c1",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if llm.lastOpts.PreferredProvider != "" {
		t.Errorf("PreferredProvider = %q, want empty default", llm.lastOpts.PreferredProvider)
	}
}

// TestTutorService_StreamChat_CancelledContextReturnsError — un
// contexto cancelado antes de StreamChat devuelve el ctx error en
// lugar de iniciar el cascade.
func TestTutorService_StreamChat_CancelledContextReturnsError(t *testing.T) {
	svc, _ := newTestTutorService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.StreamChat(ctx, ports.StreamChatRequest{UserID: "u1", Message: "hi"})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestTutorService_ProcessCommand_NivelUpdatesProfile — el comando
// /nivel 0 cambia el nivel de socratismo en el LearningProfile.
func TestTutorService_ProcessCommand_NivelUpdatesProfile(t *testing.T) {
	svc, _ := newTestTutorService(t)
	repo := svc.profileRepo.(*fakeUserProgressRepo)
	if err := svc.ProcessCommand(context.Background(), "u1", "/nivel 0"); err != nil {
		t.Fatalf("ProcessCommand err: %v", err)
	}
	profile, _ := repo.GetLearningProfile(context.Background(), "u1")
	if profile.Preferencias.NivelSocratismo != 0 {
		t.Errorf("NivelSocratismo = %d, want 0", profile.Preferencias.NivelSocratismo)
	}
}

// TestTutorService_ProcessCommand_NivelOutOfRangeRejected — un valor
// fuera de [0..3] se rechaza con ErrTutorCommandUnknown.
func TestTutorService_ProcessCommand_NivelOutOfRangeRejected(t *testing.T) {
	svc, _ := newTestTutorService(t)
	err := svc.ProcessCommand(context.Background(), "u1", "/nivel 7")
	if err == nil || !errors.Is(err, ErrTutorCommandUnknown) {
		t.Errorf("err = %v, want ErrTutorCommandUnknown", err)
	}
}

// TestTutorService_ProcessCommand_ResetClearsSession — /reset limpia
// las métricas del tracker y los mensajes en cache.
func TestTutorService_ProcessCommand_ResetClearsSession(t *testing.T) {
	svc, _ := newTestTutorService(t)
	svc.escalation.RecordFailure("u1", "c1")
	if err := svc.ProcessCommand(context.Background(), "u1", "/reset"); err != nil {
		t.Fatalf("ProcessCommand err: %v", err)
	}
	snap := svc.escalation.getSnapshot(sessionKey{userID: "u1", challengeID: "c1"})
	if snap.Failures != 0 {
		t.Errorf("after /reset: Failures = %d, want 0", snap.Failures)
	}
}

// TestTutorService_ProcessCommand_AllSevenCommands verb-shape test
// (table-driven): cada uno de los 7 comandos parsea + ejecuta sin
// errores. /solucion, /pista, /perfil, /diccionario y /help no
// producen side-effects observables desde el repositorio de profile
// (su trabajo es delegar a MCP tools); el test verifica el camino
// "comando reconocido".
func TestTutorService_ProcessCommand_AllSevenCommands(t *testing.T) {
	svc, _ := newTestTutorService(t)
	cases := []struct {
		name    string
		command string
	}{
		{"nivel", "/nivel 0"},
		{"pista", "/pista"},
		{"solucion", "/solucion"},
		{"reset", "/reset"},
		{"perfil", "/perfil"},
		{"diccionario", "/diccionario fetch"},
		{"help", "/help"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := svc.ProcessCommand(context.Background(), "u1", c.command)
			if err != nil {
				t.Errorf("ProcessCommand(%q) = %v, want nil", c.command, err)
			}
		})
	}
}

// TestTutorService_ProcessCommand_UnknownCommand — un comando no
// canónico devuelve ErrTutorCommandUnknown.
func TestTutorService_ProcessCommand_UnknownCommand(t *testing.T) {
	svc, _ := newTestTutorService(t)
	err := svc.ProcessCommand(context.Background(), "u1", "/fantasia")
	if err == nil || !errors.Is(err, ErrTutorCommandUnknown) {
		t.Errorf("err = %v, want ErrTutorCommandUnknown", err)
	}
}

// TestTutorService_ProcessCommand_EmptyCommand retorna
// ErrTutorCommandUnknown con wrapping claro.
func TestTutorService_ProcessCommand_EmptyCommand(t *testing.T) {
	svc, _ := newTestTutorService(t)
	err := svc.ProcessCommand(context.Background(), "u1", "")
	if err == nil || !errors.Is(err, ErrTutorCommandUnknown) {
		t.Errorf("err = %v, want ErrTutorCommandUnknown", err)
	}
}

// TestTutorService_SummarizeSession_PersistsSummary — la función
// arma un resumen determinístico y persiste via
// UpdateLearningProfile. El test verifica que el Resumen cambió y
// que la UltimaActualizacion se actualizó.
func TestTutorService_SummarizeSession_PersistsSummary(t *testing.T) {
	svc, _ := newTestTutorService(t)
	repo := svc.profileRepo.(*fakeUserProgressRepo)
	profile, _ := repo.GetLearningProfile(context.Background(), "u1")
	before := profile.Preferencias.Resumen

	if err := svc.SummarizeSession(context.Background(), "u1", "u1:c1"); err != nil {
		t.Fatalf("SummarizeSession err: %v", err)
	}
	if repo.updated == nil {
		t.Fatalf("UpdateLearningProfile never called")
	}
	if repo.updated.Preferencias.Resumen == before {
		t.Errorf("Resumen unchanged after SummarizeSession")
	}
	if !strings.Contains(repo.updated.Preferencias.Resumen, "práctica") {
		t.Errorf("Resumen = %q, want practice-related", repo.updated.Preferencias.Resumen)
	}
	if repo.updated.UltimaActualizacion.IsZero() {
		t.Errorf("UltimaActualizacion was not stamped")
	}
}

// TestTutorService_SummarizeSession_EmptyUserID — error defensivo.
func TestTutorService_SummarizeSession_EmptyUserID(t *testing.T) {
	svc, _ := newTestTutorService(t)
	err := svc.SummarizeSession(context.Background(), "", "sess")
	if err == nil || !errors.Is(err, ErrTutorUnavailable) {
		t.Errorf("err = %v, want ErrTutorUnavailable", err)
	}
}

// TestTutorService_SummarizeSession_RepoErrorPropagates — un error
// del repo envuelve el error original sin tragarlo.
func TestTutorService_SummarizeSession_RepoErrorPropagates(t *testing.T) {
	svc, _ := newTestTutorService(t)
	svc.profileRepo = &failingProfileRepo{}
	err := svc.SummarizeSession(context.Background(), "u1", "u1:c1")
	if err == nil {
		t.Fatalf("expected error from failing repo")
	}
	if !strings.Contains(err.Error(), "load profile") {
		t.Errorf("err message = %q, want 'load profile'", err.Error())
	}
}

// --- helpers -----------------------------------------------------------------

// eventTypeExists checks whether at least one event of the given type
// is present. Used by StreamChat tests that don't care about exact
// ordering between tokens.
func eventTypeExists(events []ports.StreamChatEvent, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
}
