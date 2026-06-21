package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// BashProvider audits Bash/shell scripts with shellcheck, using the
// `koalaman/shellcheck-alpine:latest` image for Docker execution and the
// `shellcheck` binary for the local healthcheck.
type BashProvider struct{}

// NewBashProvider returns a ports.LanguageProvider for Bash.
func NewBashProvider() ports.LanguageProvider { return &BashProvider{} }

func (p *BashProvider) Language() string      { return "bash" }
func (p *BashProvider) FileExtension() string  { return ".sh" }
func (p *BashProvider) DockerImage() string    { return "koalaman/shellcheck-alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   shellcheck <filename>
func (p *BashProvider) DockerCommand(filename string) []string {
	return []string{"shellcheck", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("shellcheck").
func (p *BashProvider) LocalCommand() string { return "shellcheck" }

// InstallHint is shown when the local executable is missing.
func (p *BashProvider) InstallHint() string {
	return "apt-get install shellcheck"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*BashProvider)(nil)