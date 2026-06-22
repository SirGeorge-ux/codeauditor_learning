package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// CProvider audits C code by compiling with `gcc` and executing the resulting
// binary. In the Docker sandbox the source is mounted at `/code` as read-only,
// so the compilation writes its output to `/tmp`.
type CProvider struct{}

// NewCProvider returns a ports.LanguageProvider for C.
func NewCProvider() ports.LanguageProvider { return &CProvider{} }

func (p *CProvider) Language() string     { return "c" }
func (p *CProvider) FileExtension() string { return ".c" }
func (p *CProvider) DockerImage() string   { return "gcc:15.3.0" }

// DockerCommand returns the argv appended after the image in `docker run`.
// The source lives at /tmp/code.c in the container and the binary is emitted to
// /tmp/out before execution.
func (p *CProvider) DockerCommand(_ string) []string {
	return []string{"sh", "-c", "gcc -o /tmp/out /tmp/code.c && /tmp/out"}
}

// LocalCommand returns the executable run by LocalSandbox ("gcc").
func (p *CProvider) LocalCommand() string { return "gcc" }

// InstallHint is shown when the local executable is missing.
func (p *CProvider) InstallHint() string {
	return "apt install gcc (or brew install gcc)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*CProvider)(nil)