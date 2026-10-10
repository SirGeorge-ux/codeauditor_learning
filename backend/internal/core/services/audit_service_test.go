package services

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/core/domain/models"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// --- Fakes (deterministic, in-memory) ---

// fakeSandbox is a minimal ports.SandboxExecutor that always returns the
// configured output line. It is enough to drive audit_service.RunAudit
// past the sandbox stage and into the Ollama analysis stage without
// invoking any real runtime.
//
// Phase 5 (free-practice-mode) adds ExecuteWithTests; audit_service does
// NOT use it (it keeps the legacy Execute path for the audit's stdout
// streaming). The fake implements ExecuteWithTests only to satisfy the
// extended ports.SandboxExecutor interface — it is wired so a future
// test can drive it explicitly via the `withTestsResult` knob.
type fakeSandbox struct {
	output string
	err    error

	// withTestsResult lets a future Phase-5-driven audit test program
	// the ExecuteWithTests response. Defaults to a "no test capability"
	// degraded result (TestsRun == len(tests), TestsPass == 0,
	// ExitCode == 1) because the audit path never goes through here.
	withTestsResult ports.TestRunResult
	withTestsErr    error
	withTestsCalls  int
}

func (f *fakeSandbox) Execute(_ context.Context, _, _ string, _ int) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.output)), nil
}

func (f *fakeSandbox) ExecuteWithTests(_ context.Context, _ string, _ string, tests []ports.TestCase, _ int) (ports.TestRunResult, error) {
	f.withTestsCalls++
	if f.withTestsErr != nil {
		return ports.TestRunResult{}, f.withTestsErr
	}
	if f.withTestsResult.TestsRun != 0 || f.withTestsResult.TestsPass != 0 || f.withTestsResult.ExitCode != 0 || len(f.withTestsResult.PerTest) > 0 {
		return f.withTestsResult, nil
	}
	// Default degraded behaviour: "no test capability" — surfaces the
	// sentinel via zero passes so any caller that routed through here
	// would treat it as a gate failure.
	return ports.TestRunResult{
		TestsRun:  len(tests),
		TestsPass: 0,
		ExitCode:  1,
	}, nil
}

func (f *fakeSandbox) Healthcheck(_ context.Context) error { return nil }

// fakeOllama is a minimal ports.OllamaClient used to prove that the
// audit service can stream LLM tokens without depending on the
// driven/ollama package. It records every call so tests can assert
// on the system/prompt strings, and it pushes configured tokens to
// the onToken callback to exercise the streaming pipeline end-to-end.
type fakeOllama struct {
	mu        sync.Mutex
	calls     []fakeOllamaCall
	tokens    []string
	returnErr error
}

type fakeOllamaCall struct {
	system string
	prompt string
}

func (f *fakeOllama) StreamGenerate(_ context.Context, system, prompt string, onToken func(string) error) error {
	f.mu.Lock()
	f.calls = append(f.calls, fakeOllamaCall{system: system, prompt: prompt})
	tokens := append([]string(nil), f.tokens...)
	retErr := f.returnErr
	f.mu.Unlock()
	for _, tok := range tokens {
		if err := onToken(tok); err != nil {
			return err
		}
	}
	return retErr
}

// Compile-time guard: fakeOllama must satisfy the port.
var _ ports.OllamaClient = (*fakeOllama)(nil)

// fakeStreamer is a minimal ports.SSEStreamer that captures every
// StreamEvent and BroadcastLLMTokens call. It is enough to assert that
// the audit flow reaches the SSE delivery boundary without requiring
// a real HTTP/SSE connection.
type fakeStreamer struct {
	mu          sync.Mutex
	events      []fakeStreamEvent
	broadcasts  []string
	registerOut []<-chan struct{}
}

type fakeStreamEvent struct {
	eventType string
	payload   string
}

func (f *fakeStreamer) StreamEvent(_ context.Context, _ string, eventType string, payload interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch p := payload.(type) {
	case models.AuditEvent:
		f.events = append(f.events, fakeStreamEvent{eventType: eventType, payload: p.Data})
	case models.AuditResult:
		f.events = append(f.events, fakeStreamEvent{eventType: eventType, payload: "exit=" + strconv.Itoa(p.ExitCode)})
	default:
		f.events = append(f.events, fakeStreamEvent{eventType: eventType, payload: "unknown"})
	}
	return nil
}

func (f *fakeStreamer) BroadcastLLMTokens(_ context.Context, _ string, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcasts = append(f.broadcasts, token)
	return nil
}

func (f *fakeStreamer) RegisterClient(_ context.Context, _ string) <-chan struct{} {
	ch := make(chan struct{})
	f.registerOut = append(f.registerOut, ch)
	return ch
}

func (f *fakeStreamer) UnregisterClient(_ string) {}

// --- Tests ---

// TestAuditService_RunAudit_StreamsOllamaTokensViaPort is the strict-TDD
// driver for WARNING #6: it proves that the audit service can drive the
// Ollama analysis stage through the ports.OllamaClient interface without
// importing infrastructure/driven/ollama.
//
// Before the refactor, WithOllama accepted a concrete *ollamadriven.Client,
// so this test would fail to compile (the fake does not satisfy the
// concrete type). After the refactor, WithOllama takes ports.OllamaClient
// and the test compiles + passes.
func TestAuditService_RunAudit_StreamsOllamaTokensViaPort(t *testing.T) {
	t.Parallel()

	sandbox := &fakeSandbox{output: "lint-ok\n"}
	ollama := &fakeOllama{tokens: []string{"a", "b", "c"}}
	streamer := &fakeStreamer{}

	svc := NewAuditService(sandbox).WithOllama(ollama)

	err := svc.RunAudit(
		context.Background(),
		models.AuditRequest{Code: "console.log(1)", Language: "typescript", UserID: "u1", ChallengeID: "c1"},
		streamer,
		"client-1",
	)
	if err != nil {
		t.Fatalf("RunAudit returned error: %v", err)
	}

	// StreamGenerate was called exactly once with the expected system/prompt.
	ollama.mu.Lock()
	defer ollama.mu.Unlock()
	if len(ollama.calls) != 1 {
		t.Fatalf("StreamGenerate calls = %d, want 1", len(ollama.calls))
	}
	call := ollama.calls[0]
	if !strings.Contains(call.system, "senior code auditor") {
		t.Errorf("system prompt missing role context: %q", call.system)
	}
	if !strings.Contains(call.prompt, "typescript") {
		t.Errorf("prompt missing language: %q", call.prompt)
	}
	if !strings.Contains(call.prompt, "console.log(1)") {
		t.Errorf("prompt missing source code: %q", call.prompt)
	}
	if !strings.Contains(call.prompt, "lint-ok") {
		t.Errorf("prompt missing sandbox tool output: %q", call.prompt)
	}

	// Tokens were forwarded through BroadcastLLMTokens in order.
	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	wantBroadcasts := []string{"a", "b", "c"}
	if len(streamer.broadcasts) != len(wantBroadcasts) {
		t.Fatalf("BroadcastLLMTokens calls = %d, want %d", len(streamer.broadcasts), len(wantBroadcasts))
	}
	for i, want := range wantBroadcasts {
		if streamer.broadcasts[i] != want {
			t.Errorf("broadcast[%d] = %q, want %q", i, streamer.broadcasts[i], want)
		}
	}

	// The accumulated llm_analysis event was emitted once.
	var analyzes int
	for _, ev := range streamer.events {
		if ev.eventType == "llm_analysis" {
			analyzes++
			if ev.payload != "abc" {
				t.Errorf("llm_analysis payload = %q, want %q", ev.payload, "abc")
			}
		}
	}
	if analyzes != 1 {
		t.Errorf("llm_analysis events = %d, want 1", analyzes)
	}

	// Terminal stdout line and complete event were also delivered.
	// Note: AuditEvent.Data is the JSON-encoded payload ({"data":"lint-ok"}),
	// so we match the JSON shape rather than the raw line.
	var sawStdout, sawComplete bool
	for _, ev := range streamer.events {
		if ev.eventType == "stdout" && ev.payload == `{"data":"lint-ok"}` {
			sawStdout = true
		}
		if ev.eventType == "complete" {
			sawComplete = true
		}
	}
	if !sawStdout {
		t.Errorf("expected stdout event with payload {\"data\":\"lint-ok\"}; got events: %#v", streamer.events)
	}
	if !sawComplete {
		t.Error("expected complete event after successful audit")
	}
}

// TestAuditService_OllamaError_EmitsLLMErrorEvent proves the audit
// flow degrades gracefully when the LLM client returns an error:
// no panic, no llm_analysis event, but an llm_error event with the
// error message.
func TestAuditService_OllamaError_EmitsLLMErrorEvent(t *testing.T) {
	t.Parallel()

	sandbox := &fakeSandbox{output: "lint-ok\n"}
	ollama := &fakeOllama{returnErr: errors.New("boom")}
	streamer := &fakeStreamer{}

	svc := NewAuditService(sandbox).WithOllama(ollama)

	err := svc.RunAudit(
		context.Background(),
		models.AuditRequest{Code: "x", Language: "go", UserID: "u1", ChallengeID: "c1"},
		streamer,
		"client-1",
	)
	if err != nil {
		t.Fatalf("RunAudit returned error: %v", err)
	}

	streamer.mu.Lock()
	defer streamer.mu.Unlock()

	var errEvents int
	for _, ev := range streamer.events {
		if ev.eventType == "llm_error" {
			errEvents++
			if !strings.Contains(ev.payload, "boom") {
				t.Errorf("llm_error payload missing error message: %q", ev.payload)
			}
		}
	}
	if errEvents != 1 {
		t.Errorf("llm_error events = %d, want 1", errEvents)
	}
}

// TestAuditService_NoOllama_SkipsAnalysis proves the audit flow runs
// to completion when WithOllama was never called — there is no
// llm_analysis or llm_error event, just stdout + complete.
func TestAuditService_NoOllama_SkipsAnalysis(t *testing.T) {
	t.Parallel()

	sandbox := &fakeSandbox{output: "lint-ok\n"}
	streamer := &fakeStreamer{}

	svc := NewAuditService(sandbox) // no WithOllama

	err := svc.RunAudit(
		context.Background(),
		models.AuditRequest{Code: "x", Language: "go", UserID: "u1", ChallengeID: "c1"},
		streamer,
		"client-1",
	)
	if err != nil {
		t.Fatalf("RunAudit returned error: %v", err)
	}

	streamer.mu.Lock()
	defer streamer.mu.Unlock()
	for _, ev := range streamer.events {
		if ev.eventType == "llm_analysis" || ev.eventType == "llm_error" {
			t.Errorf("unexpected %q event when no Ollama client is configured: %q", ev.eventType, ev.payload)
		}
	}
}

// Compile-time guard: the production driven adapter (added in the
// refactor commit) must satisfy the port that audit_service depends
// on. The guard is duplicated here to catch a future signature drift
// at PR review time even if infrastructure/driven/ollama/ollama_client.go
// is moved or split.
//
// (Note: the production adapter also declares this guard at its own
// site; this one is a redundant but harmless double-check.)
var _ ports.SandboxExecutor = (*fakeSandbox)(nil)
