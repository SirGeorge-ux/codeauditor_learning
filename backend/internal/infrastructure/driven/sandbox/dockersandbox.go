package sandbox

import (
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