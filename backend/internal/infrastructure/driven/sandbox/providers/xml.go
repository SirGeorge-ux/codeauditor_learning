package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// XmlProvider audits XML markup. XML has no standard lightweight linter on
// alpine, so the Docker sandbox simply echoes the snippet back with `cat`. The
// source is mounted at `/code` and read from `/code/code.xml`.
type XmlProvider struct{}

// NewXmlProvider returns a ports.LanguageProvider for XML.
func NewXmlProvider() ports.LanguageProvider { return &XmlProvider{} }

func (p *XmlProvider) Language() string     { return "xml" }
func (p *XmlProvider) FileExtension() string { return ".xml" }
func (p *XmlProvider) DockerImage() string    { return "alpine:latest" }

// DockerCommand returns the argv appended after the image in `docker run`:
//
//	cat /code/code.xml
//
// The snippet is echoed back unchanged, mirroring the other markup providers.
func (p *XmlProvider) DockerCommand(_ string) []string {
	return []string{"cat", "/code/code.xml"}
}

// LocalCommand returns the executable run by LocalSandbox ("cat").
func (p *XmlProvider) LocalCommand() string { return "cat" }

// InstallHint is shown when the local executable is missing.
func (p *XmlProvider) InstallHint() string {
	return "cat ships with every Unix system; no install needed"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*XmlProvider)(nil)