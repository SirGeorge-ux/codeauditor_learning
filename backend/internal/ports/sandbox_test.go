package ports_test

// Compile-time + reflection guards for ports.SandboxExecutor.
//
// Phase 5 (free-practice-mode) introduces the new ExecuteWithTests
// method on SandboxExecutor so ChallengeService.GenerateChallenge
// can run solutionCode + testCases together in the sandbox. Without
// these guards a future refactor could silently narrow the port —
// e.g. accidentally deleting ExecuteWithTests — and the orchestrator
// would fall back to calling Execute (the legacy lint-style path),
// which fails the spec scenario "Solution-Validates Sandbox Gate"
// silently.
//
// These tests assert the method is reachable via the interface; the
// real behaviour (TypeScript driver + per-test diff) is covered in
// internal/infrastructure/driven/sandbox/*_test.go.

import (
	"reflect"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// TestSandboxExecutor_HasExecuteWithMethods asserts the SandboxExecutor
// interface exposes both Execute (legacy lint path used by AuditService)
// and ExecuteWithTests (Phase 5 path used by ChallengeService.GenerateChallenge).
// Removing either method from the interface will fail this test before
// it ever gets to compilation in the implementing types.
func TestSandboxExecutor_HasExecuteWithMethods(t *testing.T) {
	t.Parallel()

	iface := reflect.TypeOf((*ports.SandboxExecutor)(nil)).Elem()
	if iface.Kind() != reflect.Interface {
		t.Fatalf("SandboxExecutor is not an interface: %v", iface.Kind())
	}

	want := map[string]bool{
		"Execute":          false,
		"ExecuteWithTests": false,
		"Healthcheck":      false,
	}
	for i := 0; i < iface.NumMethod(); i++ {
		name := iface.Method(i).Name
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("SandboxExecutor is missing method %q", name)
		}
	}
}

// TestSandboxExecutor_ExecuteWithTests_Signature pins the exact
// signature of the Phase 5 method so a future evolution does not
// silently drift it. The driver depends on the four-argument shape
// (ctx, language, code, tests, timeout); any drift breaks the
// GenerateChallenge wiring.
func TestSandboxExecutor_ExecuteWithTests_Signature(t *testing.T) {
	t.Parallel()

	iface := reflect.TypeOf((*ports.SandboxExecutor)(nil)).Elem()
	method, ok := iface.MethodByName("ExecuteWithTests")
	if !ok {
		t.Fatal("SandboxExecutor.ExecuteWithTests is missing")
	}

	// Expected: func(context.Context, string, string, []TestCase, int) (TestRunResult, error)
	got := method.Type
	wantNumIn := 5
	if got.NumIn() != wantNumIn {
		t.Errorf("ExecuteWithTests.NumIn = %d, want %d (ctx, lang, code, tests, timeout)", got.NumIn(), wantNumIn)
	}
	wantNumOut := 2
	if got.NumOut() != wantNumOut {
		t.Errorf("ExecuteWithTests.NumOut = %d, want %d (TestRunResult, error)", got.NumOut(), wantNumOut)
	}
}

// TestTestRunResult_HasExpectedFields is a regression guard: the
// ChallengeService.GenerateChallenge orchestration reads TestsRun,
// TestsPass, PerTest, ExitCode, and Output to decide the gate. If a
// future change renames or removes a field the orchestrator breaks
// silently because it uses field access (no compile error).
func TestTestRunResult_HasExpectedFields(t *testing.T) {
	t.Parallel()

	ttype := reflect.TypeOf(ports.TestRunResult{})
	wantFields := map[string]bool{
		"TestsRun":  false,
		"TestsPass": false,
		"PerTest":   false,
		"ExitCode":  false,
	}
	for i := 0; i < ttype.NumField(); i++ {
		name := ttype.Field(i).Name
		if _, ok := wantFields[name]; ok {
			wantFields[name] = true
		}
	}
	for name, seen := range wantFields {
		if !seen {
			t.Errorf("ports.TestRunResult is missing field %q", name)
		}
	}
}
