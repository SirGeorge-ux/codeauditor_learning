package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// PerlProvider audits Perl code with the built-in syntax check (`perl -c`),
// using the `perl:5.38-slim` image for Docker execution and the `perl` binary
// for the local healthcheck.
type PerlProvider struct{}

// NewPerlProvider returns a ports.LanguageProvider for Perl.
func NewPerlProvider() ports.LanguageProvider { return &PerlProvider{} }

func (p *PerlProvider) Language() string      { return "perl" }
func (p *PerlProvider) FileExtension() string  { return ".pl" }
func (p *PerlProvider) DockerImage() string    { return "perl:5.38-slim" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   perl -c <filename>
func (p *PerlProvider) DockerCommand(filename string) []string {
	return []string{"perl", "-c", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("perl").
func (p *PerlProvider) LocalCommand() string { return "perl" }

// InstallHint is shown when the local executable is missing.
func (p *PerlProvider) InstallHint() string {
	return "Install Perl: https://www.perl.org/get.html"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*PerlProvider)(nil)