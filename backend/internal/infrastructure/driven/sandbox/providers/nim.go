package providers

import (
	"fmt"

	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// NimProvider audits Nim code by compiling and executing it with
// `nim c -r --hints:off`, using the `nimlang/nim:alpine` image for Docker
// execution and the `nim` binary for the local healthcheck.
type NimProvider struct{}

// NewNimProvider returns a ports.LanguageProvider for Nim.
func NewNimProvider() ports.LanguageProvider { return &NimProvider{} }

func (p *NimProvider) Language() string      { return "nim" }
func (p *NimProvider) FileExtension() string  { return ".nim" }
func (p *NimProvider) DockerImage() string    { return "nimlang/nim:alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	nim c -r --hints:off /code/<filename>
//
// The source path uses the read-only /code mount.
func (p *NimProvider) DockerCommand(filename string) []string {
	return []string{"nim", "c", "-r", "--hints:off", fmt.Sprintf("/code/%s", filename)}
}

// LocalCommand returns the executable run by LocalSandbox ("nim").
func (p *NimProvider) LocalCommand() string { return "nim" }

// InstallHint is shown when the local executable is missing.
func (p *NimProvider) InstallHint() string {
	return "install Nim: https://nim-lang.org/install.html"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*NimProvider)(nil)