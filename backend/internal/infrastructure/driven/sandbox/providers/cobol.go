package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// CobolProvider audits Cobol code by compiling with `cobc` (GnuCOBOL) and
// executing the resulting binary. In the Docker sandbox the source is mounted
// read-only at `/code`, so the compiler writes its output to the writable
// `/tmp` tmpfs before execution. The `alpine:latest` image installs GnuCOBOL
// at run time via `apk`.
type CobolProvider struct{}

// NewCobolProvider returns a ports.LanguageProvider for Cobol.
func NewCobolProvider() ports.LanguageProvider { return &CobolProvider{} }

func (p *CobolProvider) Language() string      { return "cobol" }
func (p *CobolProvider) FileExtension() string { return ".cbl" }
func (p *CobolProvider) DockerImage() string   { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`.
// A shell wrapper installs GnuCOBOL, compiles the source to /tmp/out, and runs
// it. The source path uses the read-only /code mount.
func (p *CobolProvider) DockerCommand(filename string) []string {
	return []string{"sh", "-c", fmt.Sprintf("apk add --no-cache gnucobol && cobc -x -o /tmp/out /code/%s && /tmp/out", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("cobc").
func (p *CobolProvider) LocalCommand() string { return "cobc" }

// InstallHint is shown when the local executable is missing.
func (p *CobolProvider) InstallHint() string {
	return "install GnuCOBOL: apt install gnucobol (or brew install gnucobol)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*CobolProvider)(nil)