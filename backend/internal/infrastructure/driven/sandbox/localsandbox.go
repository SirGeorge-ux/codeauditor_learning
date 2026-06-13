package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalSandbox implements SandboxExecutor using os/exec.
type LocalSandbox struct {
	timeout time.Duration
}

// cmdReader wraps a bytes.Reader and waits for process cleanup on Close.
type cmdReader struct {
	*strings.Reader
	cmd    *exec.Cmd
	tmpDir string
}

func (r *cmdReader) Close() error {
	if r.tmpDir != "" {
		os.RemoveAll(r.tmpDir)
	}
	return nil
}

// Healthcheck verifies the sandbox runtime is responsive.
func (s *LocalSandbox) Healthcheck(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "version")
	return cmd.Run()
}

// NewLocalSandbox creates a LocalSandbox with the given default timeout.
func NewLocalSandbox(timeout time.Duration) *LocalSandbox {
	return &LocalSandbox{timeout: timeout}
}

// Execute runs the given code in the sandbox and streams output via ReadCloser.
func (s *LocalSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	var tmpDir string

	switch language {
	case "typescript", "javascript":
		cmd = exec.CommandContext(ctx, "npx", "eslint", "--format=unix", "--stdin")
		cmd.Stdin = strings.NewReader(code)
	case "go":
		var err error
		tmpDir, err = os.MkdirTemp("", "audit-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		mainFile := filepath.Join(tmpDir, "main.go")
		if err := os.WriteFile(mainFile, []byte(code), 0644); err != nil {
			os.RemoveAll(tmpDir)
			return nil, fmt.Errorf("failed to write temp file: %w", err)
		}
		cmd = exec.CommandContext(ctx, "go", "vet", "./...")
		cmd.Dir = tmpDir
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
		return nil, fmt.Errorf("start command: %w", err)
	}

	// Drain both streams concurrently.
	var wg sync.WaitGroup
	var stdoutData, stderrData strings.Builder

	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(&stdoutData, stdout)
	}()
	go func() {
		defer wg.Done()
		io.Copy(&stderrData, stderr)
	}()

	// Wait for command to finish.
	cmd.Wait()
	wg.Wait()
	stdout.Close()
	stderr.Close()

	// Build combined output.
	output := stdoutData.String()
	if stderrData.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderrData.String()
	}

	reader := &cmdReader{
		Reader: strings.NewReader(output),
		cmd:    cmd,
		tmpDir: tmpDir,
	}

	return reader, nil
}