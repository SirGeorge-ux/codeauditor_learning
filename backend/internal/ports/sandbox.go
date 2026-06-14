package ports

import (
	"context"
	"io"
)

// SandboxExecutor defines the contract for executing code in an isolated sandbox.
// It is a driven (secondary) port — the application use cases call it.
type SandboxExecutor interface {
	// Execute runs the given code snippet in an isolated environment.
	// language must be one of: "python", "javascript", "go", "bash".
	// stdout and stderr are streamed back via the returned ReadCloser.
	Execute(ctx context.Context, language, code string, timeoutSeconds int) (io.ReadCloser, error)

	// Healthcheck verifies the sandbox runtime is responsive.
	Healthcheck(ctx context.Context) error
}
