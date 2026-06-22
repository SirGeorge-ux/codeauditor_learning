package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// KotlinProvider audits Kotlin code with kotlinc, using the
// `codeauditor/kotlin-compiler:2.0-alpine` image for Docker execution and the
// `kotlinc` binary for the local healthcheck. Compilation output is directed
// to /tmp because DockerSandbox mounts /code as read-only.
type KotlinProvider struct{}

// NewKotlinProvider returns a ports.LanguageProvider for Kotlin.
func NewKotlinProvider() ports.LanguageProvider { return &KotlinProvider{} }

func (p *KotlinProvider) Language() string      { return "kotlin" }
func (p *KotlinProvider) FileExtension() string  { return ".kt" }
func (p *KotlinProvider) DockerImage() string    { return "codeauditor/kotlin-compiler:2.0-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   kotlinc -d /tmp <filename>
//
// The -d /tmp flag directs .class output to the writable tmpfs, since
// DockerSandbox mounts /code as read-only.
func (p *KotlinProvider) DockerCommand(filename string) []string {
	return []string{"kotlinc", "-d", "/tmp", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("kotlinc").
func (p *KotlinProvider) LocalCommand() string { return "kotlinc" }

// InstallHint is shown when the local executable is missing.
func (p *KotlinProvider) InstallHint() string {
	return "SDKMAN: sdk install kotlin"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*KotlinProvider)(nil)
