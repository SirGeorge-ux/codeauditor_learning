package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// GroovyProvider audits Groovy code with groovyc, using the
// `groovy:4.0-jdk21-alpine` image for Docker execution and the `groovyc`
// binary for the local healthcheck. Compilation output is directed to /tmp
// because DockerSandbox mounts /code as read-only.
type GroovyProvider struct{}

// NewGroovyProvider returns a ports.LanguageProvider for Groovy.
func NewGroovyProvider() ports.LanguageProvider { return &GroovyProvider{} }

func (p *GroovyProvider) Language() string      { return "groovy" }
func (p *GroovyProvider) FileExtension() string  { return ".groovy" }
func (p *GroovyProvider) DockerImage() string    { return "groovy:4.0-jdk21-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   groovyc -d /tmp <filename>
//
// The -d /tmp flag directs .class output to the writable tmpfs, since
// DockerSandbox mounts /code as read-only.
func (p *GroovyProvider) DockerCommand(filename string) []string {
	return []string{"groovyc", "-d", "/tmp", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("groovyc").
func (p *GroovyProvider) LocalCommand() string { return "groovyc" }

// InstallHint is shown when the local executable is missing.
func (p *GroovyProvider) InstallHint() string {
	return "SDKMAN: sdk install groovy"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*GroovyProvider)(nil)
