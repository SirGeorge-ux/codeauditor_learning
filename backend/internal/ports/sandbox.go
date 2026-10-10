package ports

import (
	"context"
	"io"
)

// TestCase is the data shape for one challenge test (input → expectedOutput).
// Input/ExpectedOutput are JSON-encoded bytes — the test driver parses them,
// invokes the solution with Input (as stdin or a generated main() wrapper
// depending on the language runtime), captures stdout, and diffs the result
// against ExpectedOutput. Weight is persisted for future weighted scoring
// (Phase 5 just tracks a passed/not-passed binary).
type TestCase struct {
	Name           string `json:"name"`
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
	Weight         int    `json:"weight"`
}

// TestOutcome is the per-test result of an ExecuteWithTests run. The
// orchestrator reads Passed to drive the retry gate; ActualOutput and
// ErrorMessage are surfaced back into the LLM prompt on the next attempt
// so the model has concrete context for the failure.
type TestOutcome struct {
	Name         string `json:"name"`
	Passed       bool   `json:"passed"`
	ActualOutput string `json:"actualOutput,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// TestRunResult is the aggregate result of ExecuteWithTests.
//
// TestsRun / TestsPass drive the gate decision ("all passed → persist").
// PerTest exposes each test's outcome for the retry-augmentation prompt.
// ExitCode carries the test-driver exit status (0 = driver succeeded;
// non-zero means the driver itself crashed, distinct from "all tests
// failed"). Output is the raw driver stdout for downstream logging;
// when not surfaced (e.g. degraded mode for languages without test
// support), it is nil.
type TestRunResult struct {
	Output    io.Reader     `json:"-"`
	TestsRun  int           `json:"testsRun"`
	TestsPass int           `json:"testsPass"`
	PerTest   []TestOutcome `json:"perTest"`
	ExitCode  int           `json:"exitCode"`
}

// SandboxExecutor defines the contract for executing code in an isolated sandbox.
// It is a driven (secondary) port — the application use cases call it.
//
// Phase 5 (free-practice-mode) extends the contract with ExecuteWithTests
// so ChallengeService.GenerateChallenge can satisfy the spec scenario
// "Solution-Validates Sandbox Gate" (solutionCode + testCases executed
// together). AuditService continues to call Execute (the legacy
// lint-style streaming path) for the per-language linter output it
// forwards to the SSE client.
type SandboxExecutor interface {
	// Execute runs the given code snippet in an isolated environment.
	// Language must be one of: "typescript", "javascript", "go",
	// "python", "ruby", "php", "lua", "bash", "perl". stdout and
	// stderr are streamed back via the returned ReadCloser. Used by
	// AuditService (legacy lint path).
	Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error)

	// ExecuteWithTests runs the given solution code together with the
	// supplied test cases in the sandbox and returns a per-test
	// outcome. The test driver is language-specific (a Node.js driver
	// is the canonical implementation for typescript/javascript; other
	// languages return a degraded result until their drivers land).
	// The orchestrator retries the LLM call when TestsPass < TestsRun
	// or the test driver exits non-zero.
	ExecuteWithTests(ctx context.Context, language, code string, tests []TestCase, timeoutSeconds int) (TestRunResult, error)

	// Healthcheck verifies the sandbox runtime is responsive.
	Healthcheck(ctx context.Context) error
}
