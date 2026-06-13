package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DockerSandbox implements ports.SandboxExecutor using Docker containers.
type DockerSandbox struct {
	pullTimeout time.Duration
}

// NewDockerSandbox creates a DockerSandbox with the given pull timeout.
func NewDockerSandbox(pullTimeout time.Duration) *DockerSandbox {
	return &DockerSandbox{pullTimeout: pullTimeout}
}

// Healthcheck verifies Docker CLI is available and required images exist.
func (s *DockerSandbox) Healthcheck(ctx context.Context) error {
	// Check Docker CLI
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}

	images := []string{"node:22-alpine", "golang:1.23-alpine"}
	for _, img := range images {
		if err := exec.CommandContext(ctx, "docker", "image", "inspect", img).Run(); err != nil {
			// Image missing, try to pull
			pullCtx, cancel := context.WithTimeout(ctx, s.pullTimeout)
			defer cancel()
			if err := exec.CommandContext(pullCtx, "docker", "pull", img).Run(); err != nil {
				return fmt.Errorf("failed to pull image %s: %w", img, err)
			}
		}
	}
	return nil
}

// Execute runs code analysis inside a Docker container.
func (s *DockerSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	// Write code file
	var filename string
	switch language {
	case "typescript", "javascript":
		filename = "code.ts"
	case "go":
		filename = "main.go"
	default:
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	codeFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(codeFile, []byte(code), 0644); err != nil {
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

	switch language {
	case "typescript", "javascript":
		args = append(args, "node:22-alpine", "npx", "eslint", "--format=unix", filename)
	case "go":
		args = append(args, "golang:1.23-alpine", "go", "vet", "./...")
	}

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
		Reader:  io.MultiReader(stdout, stderr),
		cmd:     cmd,
		cancel:  cancel,
		tmpDir:  tmpDir,
	}, nil
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