package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// HtmlProvider audits HTML markup. HTML has no standard lightweight linter on
// alpine, so the Docker sandbox simply echoes the snippet back with `cat`. The
// source is mounted at `/code` and read from `/code/code.html`.
type HtmlProvider struct{}

// NewHtmlProvider returns a ports.LanguageProvider for HTML.
func NewHtmlProvider() ports.LanguageProvider { return &HtmlProvider{} }

func (p *HtmlProvider) Language() string     { return "html" }
func (p *HtmlProvider) FileExtension() string { return ".html" }
func (p *HtmlProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	cat /code/code.html
//
// The snippet is echoed back unchanged; valid and invalid markup both exit 0,
// which matches the "HTML markup echo" audit scenario.
func (p *HtmlProvider) DockerCommand(_ string) []string {
	return []string{"cat", "/code/code.html"}
}

// LocalCommand returns the executable run by LocalSandbox ("cat").
func (p *HtmlProvider) LocalCommand() string { return "cat" }

// InstallHint is shown when the local executable is missing.
func (p *HtmlProvider) InstallHint() string {
	return "cat ships with every Unix system; no install needed"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*HtmlProvider)(nil)