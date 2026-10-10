package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// DockerSandbox implements ports.SandboxExecutor using Docker containers.
// It delegates all language-specific behavior to a ProviderRegistry so that
// adding a new language is a registry entry — never a code change here.
type DockerSandbox struct {
	pullTimeout time.Duration
	registry    *providers.ProviderRegistry
}

// NewDockerSandbox creates a DockerSandbox with the given image pull timeout
// and a populated ProviderRegistry used to resolve language keys to providers.
func NewDockerSandbox(pullTimeout time.Duration, registry *providers.ProviderRegistry) *DockerSandbox {
	return &DockerSandbox{pullTimeout: pullTimeout, registry: registry}
}

// Healthcheck verifies Docker CLI is available and required images exist. It
// iterates every registered provider, checking or pulling each image with the
// pull timeout configured at construction.
func (s *DockerSandbox) Healthcheck(ctx context.Context) error {
	// Check Docker CLI
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	for _, lang := range s.registry.Languages() {
		provider, err := s.registry.Get(lang)
		if err != nil {
			continue
		}
		image := provider.DockerImage()
		if err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Run(); err != nil {
			// Image missing, try to pull
			pullCtx, cancel := context.WithTimeout(ctx, s.pullTimeout)
			if err := exec.CommandContext(pullCtx, "docker", "pull", image).Run(); err != nil {
				cancel()
				return fmt.Errorf("failed to pull image %s: %w", image, err)
			}
			cancel()
		}
	}
	return nil
}

// Execute runs code analysis inside a Docker container.
// Language-specific behavior (docker image, command argv, file extension) is
// resolved entirely through the ProviderRegistry — there is no switch statement
// here.
func (s *DockerSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
	provider, err := s.registry.Get(language)
	if err != nil {
		return nil, err
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	filename := "code" + provider.FileExtension()
	codeFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(codeFile, []byte(code), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("write code file: %w", err)
	}

	// Set timeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)

	// Build docker run command
	args := buildDockerRunArgs(tmpDir, provider, filename)
	cmd := exec.CommandContext(ctx, "docker", args...)

	// Capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("start container: %w", err)
	}

	return &dockerCmdReader{
		Reader: io.MultiReader(stdout, stderr),
		cmd:    cmd,
		cancel: cancel,
		tmpDir: tmpDir,
	}, nil
}

// ExecuteWithTests runs solutionCode together with tests in the sandbox and
// returns the per-test outcome. Behaviour by language:
//
//   - typescript / javascript: a Node.js test driver (ts-test-runner.mjs)
//     wraps the solution, runs each test, and emits JSON-per-line. The
//     implementation lives in executeWithTestsScript to keep this method
//     readable.
//   - all other languages (degraded mode): returns
//     { TestsRun: len(tests), TestsPass: 0, ExitCode: 1 } so the
//     orchestrator treats it as a gate failure and rejects the draft.
//     Adding more language-specific drivers is a follow-up tracked
//     under Phase 5+.
//
// Docker run uses the same security flags as Execute (--cap-drop=ALL,
// --network=none, --read-only) so the gate inherits the hardened sandbox.
func (s *DockerSandbox) ExecuteWithTests(ctx context.Context, language, code string, tests []ports.TestCase, timeoutSeconds int) (ports.TestRunResult, error) {
	switch language {
	case "typescript", "javascript":
		return s.executeWithTestsScript(ctx, language, code, tests, timeoutSeconds)
	default:
		// Degraded mode: surface the unsupported-language signal so
		// the orchestrator's retry loop can record it; do NOT silently
		// mark tests as passing (that would let unverified drafts
		// leak into the dojo).
		return ports.TestRunResult{
			TestsRun:  len(tests),
			TestsPass: 0,
			ExitCode:  1,
		}, fmt.Errorf("sandbox.ExecuteWithTests: no test driver for language %q (degraded mode)", language)
	}
}

// executeWithTestsScript is the TypeScript/JavaScript concrete
// implementation: it writes the user solution + the test driver + the
// test cases JSON to a temp dir, runs the driver in the docker
// `node:22-alpine` image, parses one JSON line per test, and builds
// the TestRunResult.
//
// Layout in the temp dir:
//
//	code.ts          — the LLM-emitted solution
//	runner.mjs       — the inlined test driver
//	test-cases.json  — JSON-encoded []ports.TestCase
//
// `runner.mjs` (see tsTestRunnerScript) imports the user's solution,
// executes solve(input) for each test, prints JSON.stringify(actual),
// then exits 0 only if every test passed.
func (s *DockerSandbox) executeWithTestsScript(ctx context.Context, language, code string, tests []ports.TestCase, timeoutSeconds int) (ports.TestRunResult, error) {
	if _, err := s.registry.Get(language); err != nil {
		return ports.TestRunResult{}, err
	}

	tmpDir, err := os.MkdirTemp("", "sandbox-tests-*")
	if err != nil {
		return ports.TestRunResult{}, fmt.Errorf("create temp dir: %w", err)
	}

	solutionExt := ".ts"
	if language == "javascript" {
		solutionExt = ".js"
	}
	solutionFile := "code" + solutionExt
	if err := os.WriteFile(filepath.Join(tmpDir, solutionFile), []byte(code), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return ports.TestRunResult{}, fmt.Errorf("write solution: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "runner.mjs"), []byte(tsTestRunnerScript), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return ports.TestRunResult{}, fmt.Errorf("write runner: %w", err)
	}
	testCasesJSON, err := jsonMarshal(tests)
	if err != nil {
		os.RemoveAll(tmpDir)
		return ports.TestRunResult{}, fmt.Errorf("encode test cases: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test-cases.json"), testCasesJSON, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return ports.TestRunResult{}, fmt.Errorf("write test cases: %w", err)
	}

	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := buildDockerRunArgsForTests(tmpDir, solutionFile)
	cmd := exec.CommandContext(ctx, "docker", args...)

	output, err := cmd.CombinedOutput()
	_ = os.RemoveAll(tmpDir)
	if err != nil {
		// Driver crashed (non-zero exit). The parsed PerTest may still
		// contain partial results; the orchestrator treats any
		// ExitCode != 0 as a gate failure regardless.
		parsed := parseTestDriverOutput(output)
		return ports.TestRunResult{
			Output:    bytes.NewReader(output),
			TestsRun:  len(tests),
			TestsPass: countPassed(parsed),
			PerTest:   parsed,
			ExitCode:  cmdExitCode(err),
		}, nil
	}

	parsed := parseTestDriverOutput(output)
	return ports.TestRunResult{
		Output:    bytes.NewReader(output),
		TestsRun:  len(tests),
		TestsPass: countPassed(parsed),
		PerTest:   parsed,
		ExitCode:  0,
	}, nil
}

// buildDockerRunArgsForTests mirrors buildDockerRunArgs but injects the
// test driver alongside the solution so the docker container can run
// `node runner.mjs` against the user's code. The security flags are
// identical to Execute — no relaxation for the test path.
func buildDockerRunArgsForTests(tmpDir string, solutionFile string) []string {
	args := []string{
		"run", "--rm",
		"--memory=256m", "--cpus=0.5",
		"--read-only",
		"--network=none",
		"--pids-limit=50",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges:true",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"-v", tmpDir + ":/code:ro",
		"-w", "/code",
		"node:22-alpine",
		"node", "runner.mjs", solutionFile,
	}
	return args
}

// buildDockerRunArgs assembles the full `docker run` argv from the generic
// sandbox security flags plus the provider's image and command. It is a pure
// function so it can be unit-tested without a Docker daemon.
func buildDockerRunArgs(tmpDir string, provider ports.LanguageProvider, filename string) []string {
	args := []string{
		"run", "--rm",
		"--memory=256m", "--cpus=0.5",
		"--read-only",
		"--network=none",
		"--pids-limit=50",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges:true",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"-v", tmpDir + ":/code:ro",
		"-w", "/code",
	}
	args = append(args, provider.DockerImage())
	args = append(args, provider.DockerCommand(filename)...)
	return args
}

// dockerCmdReader wraps a reader and cleans up resources on Close.
type dockerCmdReader struct {
	io.Reader
	cmd    *exec.Cmd
	cancel context.CancelFunc
	tmpDir string
}

func (r *dockerCmdReader) Close() error {
	r.cancel()
	err := r.cmd.Wait()
	if r.tmpDir != "" {
		os.RemoveAll(r.tmpDir)
	}
	return err
}
