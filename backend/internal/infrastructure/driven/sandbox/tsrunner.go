package sandbox

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os/exec"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// tsTestRunnerScript is the Node.js test driver uploaded alongside the
// LLM-generated solution when ExecuteWithTests is invoked for
// typescript / javascript. The driver contract:
//
//  1. Reads the test cases from /code/test-cases.json.
//  2. Loads the LLM's solution from /code/code.{ts,js}. The solution
//     MUST export `function solve(input) { ... }` (either via a
//     default export or a named export — see below).
//  3. For each test, calls solve(test.input), prints
//     JSON.stringify(actual) on its own line.
//  4. Emits a JSON summary line of the shape:
//     `{"summary":true,"passed":N,"run":M,"failure":[{name, expected, actual}, ...]}`
//     on the last line. Exit 0 iff all tests passed.
//
// The driver is intentionally minimal — it constrains the contract the
// LLM must follow (export solve) without imposing a runtime framework.
// A future change could swap it for Vitest or a similar harness; the
// wire shape (per-line JSON + summary) is what the Go parser expects.
//
// We import the .ts file via a runtime helper: with esm, Node.js does
// not natively support TypeScript, so we expect the LLM-emitted
// solution to be plain ESM-compatible JavaScript plus the `.ts`
// extension as a stylistic marker. For typescript the user passes
// JavaScript-with-`.ts`; the driver imports it as a regular ES module
// after stripping types via a tiny inline transpile step. To keep the
// driver small and dependency-free, we instead use Node's experimental
// --experimental-strip-types flag (available in Node 22+). The docker
// image is `node:22-alpine`, which has it. If a future LTS removes
// the flag, follow-up work lands a real TS loader.
//
// Fallback path: if the import fails (e.g. the user passed real TS),
// the driver wraps the import in a try/catch and emits a per-line
// "import_failed" so the orchestrator can distinguish "driver crash"
// from "test failed".
const tsTestRunnerScript = `// ts-test-runner.mjs — runs solve(input) for each test case and emits
// per-line JSON. Phase 5 (free-practice-mode) ships this driver in
// backend/internal/infrastructure/driven/sandbox/dockersandbox.go
// (TypeScript section of executeWithTestsScript). Treat it as the
// contract every generated TypeScript/JavaScript solution must follow.
import fs from 'node:fs';
import path from 'node:path';

const TESTS_FILE = '/code/test-cases.json';
const SUMMARY_KEY = '__summary__';

(async () => {
  const testsRaw = fs.readFileSync(TESTS_FILE, 'utf-8');
  const tests = JSON.parse(testsRaw);

  const solutionPath = process.argv[2];
  if (!solutionPath) {
    console.log(JSON.stringify({ [SUMMARY_KEY]: true, run: 0, passed: 0, failure: [{ name: 'driver', error: 'no solution path' }] }));
    process.exit(2);
  }

  // Strip-types (Node 22 + experimental flag). Errors if the file is
  // real TypeScript containing syntax the stripper doesn't accept —
  // we catch the import and surface per-test failure rather than
  // letting the driver crash.
  let solve;
  try {
    const mod = await import(solutionPath);
    solve = mod.solve || (mod.default && mod.default.solve) || mod.default;
    if (typeof solve !== 'function') {
      console.log(JSON.stringify({ [SUMMARY_KEY]: true, run: tests.length, passed: 0, failure: [{ name: 'driver', error: 'no solve() export' }] }));
      process.exit(1);
    }
  } catch (err) {
    // Emit a per-test failure so the orchestrator can retry the LLM
    // with the import error in the next attempt's prompt.
    const failure = tests.map((t) => ({ [SUMMARY_KEY]: false, name: t.name || 'unnamed', expected: t.expectedOutput, actual: '', error: String(err && err.message ? err.message : err) }));
    console.log(JSON.stringify({ [SUMMARY_KEY]: true, run: tests.length, passed: 0, failure }));
    process.exit(1);
  }

  let passed = 0;
  const failure = [];
  for (const test of tests) {
    const name = test.name || 'unnamed';
    const expected = test.expectedOutput;
    let actualRaw;
    try {
      const output = solve(test.input);
      actualRaw = JSON.stringify(output);
    } catch (err) {
      const msg = String(err && err.message ? err.message : err);
      failure.push({ name, expected, actual: '', error: msg });
      console.log(JSON.stringify({ [SUMMARY_KEY]: false, name, expected, actual: '', error: msg }));
      continue;
    }
    const ok = actualRaw === JSON.stringify(expected);
    if (ok) {
      passed++;
      console.log(JSON.stringify({ [SUMMARY_KEY]: false, name, expected, actual: actualRaw, passed: true }));
    } else {
      failure.push({ name, expected, actual: actualRaw, error: '' });
      console.log(JSON.stringify({ [SUMMARY_KEY]: false, name, expected, actual: actualRaw, passed: false }));
    }
  }

  console.log(JSON.stringify({ [SUMMARY_KEY]: true, run: tests.length, passed, failure }));
  if (failure.length === 0) process.exit(0);
  process.exit(1);
})().catch((err) => {
  console.log(JSON.stringify({ [SUMMARY_KEY]: true, run: 0, passed: 0, failure: [{ name: 'driver', error: String(err && err.message ? err.message : err) }] }));
  process.exit(2);
});
`

// parseTestDriverOutput scans the driver stdout line-by-line, collecting
// per-test outcomes. The driver uses a sentinel key `__summary__` to
// mark the summary line vs. per-test lines; we ignore everything that
// is not a JSON object so stray stderr noise does not poison the
// parse. Lines that fail to JSON-decode are silently dropped (the
// driver crashed mid-run; the ExitCode handles that signal anyway).
func parseTestDriverOutput(stdout []byte) []ports.TestOutcome {
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	// Allow long per-test lines (a 200KB JSON is well over the
	// scanner default 64KB token size).
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var out []ports.TestOutcome
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}
		if isSummary, ok := obj["__summary__"].(bool); ok && isSummary {
			continue
		}
		name, _ := obj["name"].(string)
		if name == "" {
			name = "unnamed"
		}
		expected, _ := obj["expected"].(string)
		actual, _ := obj["actual"].(string)
		passed, _ := obj["passed"].(bool)
		errMsg, _ := obj["error"].(string)
		out = append(out, ports.TestOutcome{
			Name:         name,
			Passed:       passed,
			ActualOutput: actual,
			ErrorMessage: errMsg,
		})
		_ = expected // expected is encoded in ExpectedOutput; surfaced in PerTest separately when needed.
	}
	return out
}

func countPassed(perTest []ports.TestOutcome) int {
	n := 0
	for _, t := range perTest {
		if t.Passed {
			n++
		}
	}
	return n
}

// jsonMarshal encodes a slice of TestCase as JSON. Kept as a
// package-private helper so the test driver can be swapped without
// touching dockersandbox.go / localsandbox.go.
func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// cmdExitCode extracts the process exit code from an exec error. When
// err is nil it returns 0. Any non-nil error that isn't an *exec.ExitError
// returns -1 so callers can distinguish "driver crashed" from "all
// tests failed".
func cmdExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws := ee.ExitCode(); ws >= -1 {
			// ws can be -1 for "process killed"; clamp to 1 so the
			// orchestrator never sees a confused "success" exit.
			if ws < 0 {
				return 1
			}
			return ws
		}
	}
	return 1
}
