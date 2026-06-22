package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// CSharpProvider audits C# code by compiling with `mcs` and executing the
// resulting assembly with `mono`. In the Docker sandbox the source is mounted
// read-only at `/code`, so the compiler writes its output to the writable
// `/tmp` tmpfs before execution.
type CSharpProvider struct{}

// NewCSharpProvider returns a ports.LanguageProvider for C#.
func NewCSharpProvider() ports.LanguageProvider { return &CSharpProvider{} }

func (p *CSharpProvider) Language() string      { return "csharp" }
func (p *CSharpProvider) FileExtension() string { return ".cs" }
func (p *CSharpProvider) DockerImage() string   { return "mono:latest" }

// DockerCommand returns the argv appended after the image in `docker run`.
// A shell wrapper compiles the source to /tmp/out.exe and then runs it with
// mono, since docker run only accepts a single entrypoint argv. The source
// path uses the read-only /code mount.
func (p *CSharpProvider) DockerCommand(filename string) []string {
	return []string{"sh", "-c", fmt.Sprintf("mcs -out:/tmp/out.exe /code/%s && mono /tmp/out.exe", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("mcs").
func (p *CSharpProvider) LocalCommand() string { return "mcs" }

// InstallHint is shown when the local executable is missing.
func (p *CSharpProvider) InstallHint() string {
	return "install Mono: apt install mono-devel (or brew install mono)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*CSharpProvider)(nil)
