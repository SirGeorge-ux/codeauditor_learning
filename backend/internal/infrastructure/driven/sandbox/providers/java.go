package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// JavaProvider audits Java code with javac, using the
// `eclipse-temurin:21-jdk-alpine` image for Docker execution and the `javac`
// binary for the local healthcheck. Compilation output is directed to /tmp
// because DockerSandbox mounts /code as read-only.
type JavaProvider struct{}

// NewJavaProvider returns a ports.LanguageProvider for Java.
func NewJavaProvider() ports.LanguageProvider { return &JavaProvider{} }

func (p *JavaProvider) Language() string      { return "java" }
func (p *JavaProvider) FileExtension() string  { return ".java" }
func (p *JavaProvider) DockerImage() string    { return "eclipse-temurin:21-jdk-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   javac -d /tmp <filename>
//
// The -d /tmp flag directs .class output to the writable tmpfs, since
// DockerSandbox mounts /code as read-only.
func (p *JavaProvider) DockerCommand(filename string) []string {
	return []string{"javac", "-d", "/tmp", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("javac").
func (p *JavaProvider) LocalCommand() string { return "javac" }

// InstallHint is shown when the local executable is missing.
func (p *JavaProvider) InstallHint() string {
	return "SDKMAN: sdk install java 21-tem (or apt/brew install openjdk-21-jdk)"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*JavaProvider)(nil)
