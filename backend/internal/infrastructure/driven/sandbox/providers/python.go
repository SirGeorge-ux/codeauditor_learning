package providers

import "github.com/anomalyco/codeauditor/backend/internal/ports"

// PythonProvider audits Python code with ruff, using the
// `python:3.12-alpine` image for Docker execution and the `ruff` binary
// for the local healthcheck.
type PythonProvider struct{}

// NewPythonProvider returns a ports.LanguageProvider for Python.
func NewPythonProvider() ports.LanguageProvider { return &PythonProvider{} }

func (p *PythonProvider) Language() string      { return "python" }
func (p *PythonProvider) FileExtension() string  { return ".py" }
func (p *PythonProvider) DockerImage() string    { return "python:3.12-alpine" }

// DockerCommand returns the argv appended after the image in `docker run`:
//   ruff check --output-format=text <filename>
func (p *PythonProvider) DockerCommand(filename string) []string {
	return []string{"ruff", "check", "--output-format=text", filename}
}

// LocalCommand returns the executable run by LocalSandbox ("ruff").
func (p *PythonProvider) LocalCommand() string { return "ruff" }

// InstallHint is shown when the local executable is missing.
func (p *PythonProvider) InstallHint() string {
	return "pip install ruff"
}

// Compile-time guard.
var _ ports.LanguageProvider = (*PythonProvider)(nil)