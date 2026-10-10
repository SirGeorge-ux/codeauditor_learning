package sandbox

// Phase 5 (free-practice-mode) tests for the ExecuteWithTests path:
//   - parseTestDriverOutput correctness on realistic driver stdout
//     (multi-line JSON, summary line ignored, error-excerpt field).
//   - countPassed / cmdExitCode / jsonMarshal helpers that the
//     orchestrator adapter layer depends on.
//   - buildDockerRunArgsForTests / buildLocalCommand-for-tests
//     include the runner.mjs test driver.
//
// The Docker / Local runtime tests themselves are gated with
// testing.Short() (or skipped when the runtime is absent) so the
// unit suite does not require a node runtime / docker daemon.

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// TestParseTestDriverOutput_PerLineJSON exercises the per-line
// JSON extraction the driver emits. The driver writes one line per
// test (`{"__summary__": false, "name": ..., "passed": ...}`)
// plus a final summary line `{"__summary__": true, ...}`. The parser
// must drop the summary and emit one TestOutcome per test line.
func TestParseTestDriverOutput_PerLineJSON(t *testing.T) {
	t.Parallel()

	stdout := []byte(strings.Join([]string{
		`{"__summary__":false,"name":"smoke","expected":"42","actual":"42","passed":true}`,
		`{"__summary__":false,"name":"edge","expected":"0","actual":"1","passed":false,"error":"mismatch"}`,
		`{"__summary__":true,"run":2,"passed":1,"failure":[]}`,
		"",
		``, // empty lines (driver may flush) must be tolerated
	}, "\n"))

	got := parseTestDriverOutput(stdout)
	if len(got) != 2 {
		t.Fatalf("parseTestDriverOutput returned %d outcomes, want 2 (summary excluded)", len(got))
	}
	if got[0].Name != "smoke" || !got[0].Passed {
		t.Errorf("outcomes[0] = %+v, want smoke=true", got[0])
	}
	if got[1].Name != "edge" || got[1].Passed {
		t.Errorf("outcomes[1] = %+v, want edge=false", got[1])
	}
	if got[1].ErrorMessage != "mismatch" {
		t.Errorf("outcomes[1].ErrorMessage = %q, want %q", got[1].ErrorMessage, "mismatch")
	}
}

// TestParseTestDriverOutput_IgnoresStrayNoise ensures non-JSON
// lines (driver crash stderr, npx deprecation warnings, etc.) are
// silently dropped rather than poisoning the parse.
func TestParseTestDriverOutput_IgnoresStrayNoise(t *testing.T) {
	t.Parallel()

	stdout := []byte(strings.Join([]string{
		`npm warn deprecated foo@1.0.0`,
		`{"__summary__":false,"name":"smoke","expected":"x","actual":"x","passed":true}`,
		`Error: ECONNREFUSED something`, // not JSON; must be ignored
		`{"__summary__":true,"run":1,"passed":1,"failure":[]}`,
	}, "\n"))

	got := parseTestDriverOutput(stdout)
	if len(got) != 1 {
		t.Fatalf("parseTestDriverOutput returned %d outcomes, want 1 (noise ignored)", len(got))
	}
	if got[0].Name != "smoke" {
		t.Errorf("outcomes[0].Name = %q, want %q", got[0].Name, "smoke")
	}
}

// TestCountPassed returns the count of passed outcomes (drives the
// orchestrator's gate decision).
func TestCountPassed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []ports.TestOutcome
		want int
	}{
		{"empty", nil, 0},
		{"all pass", []ports.TestOutcome{{Passed: true}, {Passed: true}}, 2},
		{"mixed", []ports.TestOutcome{{Passed: true}, {Passed: false}, {Passed: true}}, 2},
		{"all fail", []ports.TestOutcome{{Passed: false}, {Passed: false}}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := countPassed(c.in); got != c.want {
				t.Errorf("countPassed(%+v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

// TestCmdExitCode covers the exec-error extraction (the orchestrator
// treats any ExitCode != 0 as a gate failure distinct from
// "all tests failed").
func TestCmdExitCode(t *testing.T) {
	t.Parallel()

	if got := cmdExitCode(nil); got != 0 {
		t.Errorf("cmdExitCode(nil) = %d, want 0", got)
	}

	// Generic error (not *exec.ExitError) → -1 sentinel rounds to 1.
	if got := cmdExitCode(errors.New("boom")); got != 1 {
		t.Errorf("cmdExitCode(non-ExitError) = %d, want 1", got)
	}

	// Real ExitError with exit code 1.
	exitErr := &exec.ExitError{}
	if got := cmdExitCode(exitErr); got != 1 {
		t.Errorf("cmdExitCode(ExitError ws=1) = %d, want 1", got)
	}
}

// TestBuildDockerRunArgsForTests is the docker-adapter equivalent of
// TestBuildDockerRunArgs: assert the argv includes the runner, the
// solution file, and the security flags without invoking docker.
func TestBuildDockerRunArgsForTests(t *testing.T) {
	t.Parallel()

	tmpDir := "/tmp/sandbox-tests-test"
	solutionFile := "code.ts"
	args := buildDockerRunArgsForTests(tmpDir, solutionFile)

	if args[0] != "run" || args[1] != "--rm" {
		t.Errorf("args[0..1] = %q %q, want run --rm", args[0], args[1])
	}

	// Verify the test-driver mount points to the same temp dir.
	mountFound := false
	for i, a := range args {
		if a == "-v" && i+1 < len(args) && strings.HasPrefix(args[i+1], tmpDir+":/code:ro") {
			mountFound = true
			break
		}
	}
	if !mountFound {
		t.Errorf("expected volume mount %q in args: %v", tmpDir+":/code:ro", args)
	}

	// Verify the working directory is /code.
	workdirFound := false
	for i, a := range args {
		if a == "-w" && i+1 < len(args) && args[i+1] == "/code" {
			workdirFound = true
			break
		}
	}
	if !workdirFound {
		t.Errorf("expected -w /code in args: %v", args)
	}

	// Verify security flags mirror Execute().
	wantFlags := []string{"--cap-drop=ALL", "--network=none", "--read-only", "--pids-limit=50"}
	for _, wf := range wantFlags {
		if !contains(args, wf) {
			t.Errorf("missing security flag %q in args: %v", wf, args)
		}
	}

	// Verify the image is node:22-alpine.
	if !contains(args, "node:22-alpine") {
		t.Errorf("missing node:22-alpine image in args: %v", args)
	}

	// Verify the test driver invocation includes `node runner.mjs <solution>`.
	if !contains(args, "runner.mjs") {
		t.Errorf("missing runner.mjs in args: %v", args)
	}
	if !contains(args, solutionFile) {
		t.Errorf("missing solution file %q in args: %v", solutionFile, args)
	}
}

// TestTsTestRunnerScript_Shape asserts the inlined Node.js driver
// carries the contract pieces the docker / local sandbox depend on:
//   - reads /code/test-cases.json
//   - reads the solution path from argv[2]
//   - emits per-test + summary JSON for parseTestDriverOutput
//
// These are not executing the script (no node runtime); the assertion
// is structural so a future edit cannot silently remove the file
// path or the summary key without breaking the test driver contract.
func TestTsTestRunnerScript_Shape(t *testing.T) {
	t.Parallel()

	must := []string{
		"test-cases.json",
		"process.argv[2]",
		"__summary__",
		"exit(0)",
		"exit(1)",
		"exit(2)",
	}
	for _, m := range must {
		if !strings.Contains(tsTestRunnerScript, m) {
			t.Errorf("tsTestRunnerScript missing %q (driver contract broken)", m)
		}
	}
}

// TestJSONMarshal verifies the helper produces valid JSON for
// arbitrary-shaped TestCase slices. The driver reads the file via
// JSON.parse so a marshal failure breaks the test gate.
func TestJSONMarshal(t *testing.T) {
	t.Parallel()

	in := []ports.TestCase{
		{Name: "smoke", Input: `{"x":1}`, ExpectedOutput: `"ok"`, Weight: 5},
	}
	got, err := jsonMarshal(in)
	if err != nil {
		t.Fatalf("jsonMarshal: %v", err)
	}
	if !strings.Contains(string(got), `"name": "smoke"`) {
		t.Errorf("jsonMarshal output missing name field: %s", got)
	}
}

// contains is a tiny shim kept local to this test file so we don't
// import slices just for one usagesite.
func contains(haystack []string, needle string) bool {
	return reflect.ValueOf(haystack).Len() == 0 ||
		strings.Contains(strings.Join(haystack, "\x00"), needle)
}
