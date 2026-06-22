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

	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// LocalSandbox implements SandboxExecutor using os/exec.
// It delegates all language-specific behavior to a ProviderRegistry so that
// adding a new language is a registry entry — never a code change here.
type LocalSandbox struct {
	timeout  time.Duration
	registry *providers.ProviderRegistry
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

// NewLocalSandbox creates a LocalSandbox with the given default timeout and a
// populated ProviderRegistry used to resolve language keys to their providers.
func NewLocalSandbox(timeout time.Duration, registry *providers.ProviderRegistry) *LocalSandbox {
	return &LocalSandbox{timeout: timeout, registry: registry}
}

// Healthcheck verifies the sandbox runtime is responsive. It iterates every
// registered provider and reports each local tool that is missing from PATH,
// including its InstallHint. It returns nil only when all tools are available.
func (s *LocalSandbox) Healthcheck(ctx context.Context) error {
	var missing []string
	for _, lang := range s.registry.Languages() {
		provider, err := s.registry.Get(lang)
		if err != nil {
			continue
		}
		binary := provider.LocalCommand()
		if _, err := exec.LookPath(binary); err != nil {
			missing = append(missing, fmt.Sprintf("%s: not found. Install with: %s", binary, provider.InstallHint()))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing local tools:\n  %s", strings.Join(missing, "\n  "))
	}
	return nil
}

// Execute runs the given code in the sandbox and streams output via ReadCloser.
// Language-specific behavior (file extension, command, args) is resolved
// entirely through the ProviderRegistry — there is no switch statement here.
func (s *LocalSandbox) Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	provider, err := s.registry.Get(language)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "audit-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	filename := "code" + provider.FileExtension()
	codeFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(codeFile, []byte(code), 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	cmd := buildLocalCommand(ctx, provider, filename, tmpDir)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("start command: %w", err)
	}

	// Drain both streams concurrently.
	var wg sync.WaitGroup
	var stdoutData, stderrData strings.Builder

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdoutData, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderrData, stderr)
	}()

	// Wait for command to finish.
	_ = cmd.Wait()
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

// buildLocalCommand constructs the exec.Cmd for a given provider. The local
// execution path mirrors the Docker argv: DockerCommand(filename) returns the
// full argv (binary + args), and the first element equals LocalCommand() for
// every provider. The temp directory is set as the working directory so the
// basename filename resolves correctly.
func buildLocalCommand(ctx context.Context, provider ports.LanguageProvider, filename, tmpDir string) *exec.Cmd {
	args := provider.DockerCommand(filename)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = tmpDir
	return cmd
}