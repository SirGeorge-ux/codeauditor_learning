package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// CssProvider audits CSS. CSS has no standard lightweight linter on alpine, so
// the Docker sandbox simply echoes the snippet back with `cat`. The source is
// mounted at `/code` and read from `/code/code.css`.
type CssProvider struct{}

// NewCssProvider returns a ports.LanguageProvider for CSS.
func NewCssProvider() ports.LanguageProvider { return &CssProvider{} }

func (p *CssProvider) Language() string     { return "css" }
func (p *CssProvider) FileExtension() string { return ".css" }
func (p *CssProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	cat /code/code.css
//
// The snippet is echoed back unchanged, mirroring the markup providers in this
// oleada.
func (p *CssProvider) DockerCommand(_ string) []string {
	return []string{"cat", "/code/code.css"}
}

// LocalCommand returns the executable run by LocalSandbox ("cat").
func (p *CssProvider) LocalCommand() string { return "cat" }

// InstallHint is shown when the local executable is missing.
func (p *CssProvider) InstallHint() string {
	return "cat ships with every Unix system; no install needed"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*CssProvider)(nil)