package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// ObjectiveCProvider audits Objective-C code by compiling with `gcc` and
// executing the resulting binary. In the Docker sandbox the source is mounted
// read-only at `/code`, so the compilation writes its output to the writable
// `/tmp` tmpfs before execution.
type ObjectiveCProvider struct{}

// NewObjectiveCProvider returns a ports.LanguageProvider for Objective-C.
func NewObjectiveCProvider() ports.LanguageProvider { return &ObjectiveCProvider{} }

func (p *ObjectiveCProvider) Language() string      { return "objective-c" }
func (p *ObjectiveCProvider) FileExtension() string { return ".m" }
func (p *ObjectiveCProvider) DockerImage() string    { return "gcc:latest" }

// DockerCommand returns the argv appended after the image in `docker run`.
// A shell wrapper compiles the source to /tmp/out and runs it, linking against
// the Objective-C runtime (-lobjc). The source path uses the read-only /code
// mount.
func (p *ObjectiveCProvider) DockerCommand(filename string) []string {
	return []string{"sh", "-c", fmt.Sprintf("gcc -x objective-c -o /tmp/out /code/%s -lobjc && /tmp/out", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("gcc").
func (p *ObjectiveCProvider) LocalCommand() string { return "gcc" }

// InstallHint is shown when the local executable is missing.
func (p *ObjectiveCProvider) InstallHint() string {
	return "apt install gcc (or brew install gcc)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ObjectiveCProvider)(nil)