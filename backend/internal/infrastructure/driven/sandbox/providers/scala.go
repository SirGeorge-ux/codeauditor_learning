package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// ScalaProvider audits Scala code with scalac, using the
// `scala:3.3.1-slim` image for Docker execution and the `scalac` binary for
// the local healthcheck. Compilation output is directed to /tmp because
// DockerSandbox mounts /code as read-only.
type ScalaProvider struct{}

// NewScalaProvider returns a ports.LanguageProvider for Scala.
func NewScalaProvider() ports.LanguageProvider { return &ScalaProvider{} }

func (p *ScalaProvider) Language() string      { return "scala" }
func (p *ScalaProvider) FileExtension() string  { return ".scala" }
func (p *ScalaProvider) DockerImage() string    { return "scala:3.3.1-slim" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   scalac -d /tmp <filename>
//
// The -d /tmp flag directs .class output to the writable tmpfs, since
// DockerSandbox mounts /code as read-only.
func (p *ScalaProvider) DockerCommand(filename string) []string {
	return []string{"scalac", "-d", "/tmp", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("scalac").
func (p *ScalaProvider) LocalCommand() string { return "scalac" }

// InstallHint is shown when the local executable is missing.
func (p *ScalaProvider) InstallHint() string {
	return "SDKMAN: sdk install scala (or use Coursier: cs setup)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*ScalaProvider)(nil)
