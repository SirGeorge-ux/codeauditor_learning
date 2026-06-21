package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// PhpProvider audits PHP code with the built-in linter (`php -l`), using the
// `php:8.3-cli-alpine` image for Docker execution and the `php` binary
// for the local healthcheck.
type PhpProvider struct{}

// NewPhpProvider returns a ports.LanguageProvider for PHP.
func NewPhpProvider() ports.LanguageProvider { return &PhpProvider{} }

func (p *PhpProvider) Language() string      { return "php" }
func (p *PhpProvider) FileExtension() string  { return ".php" }
func (p *PhpProvider) DockerImage() string    { return "php:8.3-cli-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   php -l <filename>
func (p *PhpProvider) DockerCommand(filename string) []string {
	return []string{"php", "-l", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("php").
func (p *PhpProvider) LocalCommand() string { return "php" }

// InstallHint is shown when the local executable is missing.
func (p *PhpProvider) InstallHint() string {
	return "Install PHP: https://www.php.net/downloads"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*PhpProvider)(nil)